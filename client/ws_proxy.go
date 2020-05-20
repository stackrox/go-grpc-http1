package client

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"net/textproto"
	"strings"
	"sync"

	"github.com/golang/glog"
	"github.com/pkg/errors"
	"golang.stackrox.io/grpc-http1/internal/grpcproto"
	"golang.stackrox.io/grpc-http1/internal/grpcwebsocket"
	"golang.stackrox.io/grpc-http1/internal/pipeconn"
	"golang.stackrox.io/grpc-http1/internal/size"
	"google.golang.org/grpc"
	"nhooyr.io/websocket"
)

var (
	subprotocols = []string{"grpc-ws"}
)

type http2WebSocketProxy struct {
	insecure   bool
	endpoint   string
	httpClient *http.Client
}

// readHeader reads gRPC response headers. Trailers-Only messages are treated as response headers.
func (h *http2WebSocketProxy) readHeader(ctx context.Context, conn *websocket.Conn, w http.ResponseWriter) error {
	mt, msg, err := conn.Read(ctx)
	if err != nil {
		return err
	}
	if mt != websocket.MessageBinary {
		return errors.Errorf("incorrect message type; expected MessageBinary but got %v", mt)
	}

	if err := grpcproto.ValidateGRPCFrame(msg); err != nil {
		return err
	}
	if !grpcproto.IsMetadataFrame(msg) {
		return errors.New("did not receive metadata message")
	}

	return setHeader(w, msg[grpcproto.MessageHeaderLength:], false)
}

// Read gRPC response messages from the server and write them back to the gRPC client.
func (h *http2WebSocketProxy) readFromServer(ctx context.Context, conn *websocket.Conn, w http.ResponseWriter) error {
	// Handle normal and trailers-only messages.
	// Treat trailers-only the same as a headers-only response.
	if err := h.readHeader(ctx, conn, w); err != nil {
		return errors.Wrap(err, "reading response header")
	}
	w.WriteHeader(http.StatusOK)

	// "State" variables.
	// EOF is ok if we receive no data (so headers-only or trailers-only response)
	// or after receiving trailers.
	// Data is ok after receiving the headers (above), but not after receiving trailers.
	eofOk, dataOk := true, true
	for {
		mt, msg, err := conn.Read(ctx)
		if err != nil {
			if err == io.EOF && eofOk {
				return nil
			}

			return errors.Wrap(err, "reading response body")
		}
		if !dataOk {
			// Did not read io.EOF after already receiving trailers.
			return errors.New("received message after receiving trailers")
		}
		if mt != websocket.MessageBinary {
			return errors.Errorf("incorrect message type; expected MessageBinary but got %v", mt)
		}

		if err := grpcproto.ValidateGRPCFrame(msg); err != nil {
			return err
		}
		if grpcproto.IsDataFrame(msg) {
			eofOk = false
			if _, err := w.Write(msg); err != nil {
				return err
			}
		} else if grpcproto.IsMetadataFrame(msg) {
			if grpcproto.IsCompressed(msg) {
				return errors.New("compression flag is set; compressed metadata is not supported")
			}
			eofOk = true
			dataOk = false
			if err := setHeader(w, msg[grpcproto.MessageHeaderLength:], true); err != nil {
				return err
			}
		} else {
			return errors.New("received an invalid message: expected either data or trailers")
		}
	}
}

// Set the http.Header. If isTrailers is true, http.TrailerPrefix is prepended to each key.
func setHeader(w http.ResponseWriter, msg []byte, isTrailers bool) error {
	hdr, err := textproto.NewReader(
		bufio.NewReader(
			io.MultiReader(
				bytes.NewReader(msg),
				strings.NewReader("\r\n"),
			),
		),
	).ReadMIMEHeader()
	if err != nil {
		return err
	}

	wHdr := w.Header()
	for k, vs := range hdr {
		if isTrailers {
			// Any trailers have had the prefix stripped off, so we replace it here.
			k = http.TrailerPrefix + k
		}
		for _, v := range vs {
			wHdr.Add(k, v)
		}
	}

	return nil
}

// ServeHTTP handles gRPC-WebSocket traffic.
func (h *http2WebSocketProxy) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.ProtoMajor != 2 || !strings.HasPrefix(req.Header.Get("Content-Type"), "application/grpc") {
		glog.Error("Request is not a valid gRPC request")
		w.WriteHeader(http.StatusUnsupportedMediaType)
		return
	}

	ctx := req.Context()

	scheme := "https"
	if h.insecure {
		scheme = "http"
	}

	url := *req.URL // Copy the value, so we do not overwrite the URL.
	url.Scheme = scheme
	url.Host = h.endpoint
	conn, _, err := websocket.Dial(ctx, url.String(), &websocket.DialOptions{
		// Add the gRPC headers to the WebSocket handshake request.
		HTTPHeader:   req.Header,
		HTTPClient:   h.httpClient,
		Subprotocols: subprotocols,
	})
	if err != nil {
		writeError(w, errors.Wrapf(err, "connecting to gRPC server %q", url.String()))
		return
	}
	conn.SetReadLimit(64 * size.MB)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := grpcwebsocket.Write(ctx, conn, req.Body); err != nil {
			_ = conn.Close(websocket.StatusInternalError, err.Error())
			return
		}
		// Signal the server there are no more messages in the stream.
		if err := conn.Write(ctx, websocket.MessageBinary, grpcproto.EndStreamHeader); err != nil {
			_ = conn.Close(websocket.StatusInternalError, err.Error())
		}
	}()

	if err := h.readFromServer(ctx, conn, w); err != nil {
		_ = conn.Close(websocket.StatusInternalError, err.Error())
	}

	wg.Wait()
	// It's ok to potentially close the connection multiple times.
	// Only the first time matters.
	_ = conn.Close(websocket.StatusNormalClosure, "")
}

func createClientWSProxy(endpoint string, tlsClientConf *tls.Config) (*http.Server, pipeconn.DialContextFunc, error) {
	handler := &http2WebSocketProxy{
		insecure: tlsClientConf == nil,
		endpoint: endpoint,
		httpClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: tlsClientConf,
			},
		},
	}
	return makeProxyServer(handler)
}

// ConnectViaWSProxy establishes a gRPC client connection via a HTTP/2 proxy that handles
// endpoints behind HTTP/1 proxies via WebSocket.
// This proxy supports unary, server-side, client-side, and bidirectional streaming.
func ConnectViaWSProxy(ctx context.Context, endpoint string, tlsClientConf *tls.Config, opts ...ConnectOption) (*grpc.ClientConn, error) {
	proxy, dialCtx, err := createClientWSProxy(endpoint, tlsClientConf)
	if err != nil {
		return nil, errors.Wrap(err, "creating client proxy")
	}

	return dialGRPCServer(ctx, proxy, makeDialOpts(endpoint, dialCtx, tlsClientConf, opts...))
}
