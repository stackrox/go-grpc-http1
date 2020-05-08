package client

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/golang/glog"
	"github.com/pkg/errors"
	"golang.stackrox.io/grpc-http1/internal/grpcproto"
	"golang.stackrox.io/grpc-http1/internal/pipeconn"
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

// write writes the contents of the reqBody along the WebSocket connection.
func (h *http2WebSocketProxy) write(ctx context.Context, conn *websocket.Conn, reqBody io.ReadCloser) {
	// Write each WebSocket message as a gRPC data frame.
	// Each data frame is length-prefixed message, where the prefix is 5 bytes.
	// gRPC request format is specified here: https://github.com/grpc/grpc/blob/master/doc/PROTOCOL-HTTP2.md
	var gRPCMsgHdr [grpcproto.MessageHeaderLength]byte
	for {
		_, err := io.ReadFull(reqBody, gRPCMsgHdr[:])
		if err != nil {
			if err == io.EOF {
				// EOF is ok here, as it means it's the end of the gRPC message.
				err = nil
			} else {
				glog.Errorf("Malformed gRPC message: %v", err)
			}
			break
		}

		_, length := grpcproto.ParseMessageHeader(gRPCMsgHdr)
		msg := make([]byte, grpcproto.MessageHeaderLength+length)
		n, err := io.ReadFull(reqBody, msg[grpcproto.MessageHeaderLength:])
		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				glog.Errorf("Malformed gRPC message: fewer than the announced %d bytes in payload: %d", length, n)
			} else {
				glog.Errorf("Unable to read gRPC request message: %v", err)
			}
			break
		}
		// Message to send out should be the entire gRPC data frame, including headers.
		copy(msg, gRPCMsgHdr[:])

		// TODO: Remove this log. Keeping it for debugging purposes, for now.
		glog.Errorln(string(msg[grpcproto.MessageHeaderLength+2:]))

		_ = conn.Write(ctx, websocket.MessageBinary, msg)
	}
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

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		h.write(ctx, conn, req.Body)
		wg.Done()
	}()

	wg.Wait()

	// TODO: Write back gRPC headers upon failure.
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
