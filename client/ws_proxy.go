package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"strings"

	"github.com/golang/glog"
	"github.com/pkg/errors"
	"golang.stackrox.io/grpc-http1/internal/grpcproto"
	"golang.stackrox.io/grpc-http1/internal/ioutils"
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

// Write the contents of the reqBody along the WebSocket connection.
// This is done by sending each WebSocket message as a gRPC data frame.
// Each data frame is length-prefixed message, where the prefix is 5 bytes.
// gRPC request format is specified here: https://github.com/grpc/grpc/blob/master/doc/PROTOCOL-HTTP2.md
func (h *http2WebSocketProxy) write(ctx context.Context, conn *websocket.Conn, reqBody io.ReadCloser) error {
	var msg bytes.Buffer
	for {
		// Reset the message buffer to start with a clean slate.
		msg.Reset()
		// Read request header into the msg buffer.
		if _, err := ioutils.CopyNFull(&msg, reqBody, grpcproto.MessageHeaderLength); err != nil {
			if err == io.EOF {
				// EOF here means the client has no more messages to send.
				// Send the server an EOS message.
				// TODO: Remove this log. Keeping for now for debugging purposes.
				glog.Errorln("Sending EOS")
				return conn.Write(ctx, websocket.MessageBinary, grpcproto.EndStreamHeader)
			}

			glog.Errorf("Malformed gRPC message when reading header: %v", err)
			return err
		}

		_, length, err := grpcproto.ParseMessageHeader(msg.Bytes())
		if err != nil {
			return err
		}

		// Read the rest of the message into the msg buffer.
		if n, err := io.CopyN(&msg, reqBody, int64(length)); err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
				glog.Errorf("Malformed gRPC message: fewer than the announced %d bytes in payload: %d", length, n)
			} else {
				glog.Errorf("Unable to read gRPC request message: %v", err)
			}
			return err
		}

		// TODO: Remove this log. Keeping it for debugging purposes, for now.
		glog.Errorln(msg.String())

		if err := conn.Write(ctx, websocket.MessageBinary, msg.Bytes()); err != nil {
			return err
		}
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

	// Channel to capture the output of (*http2WebSocketProxy).write.
	errC := make(chan error)

	go func() {
		errC <- h.write(ctx, conn, req.Body)
	}()

	select {
	case err := <-errC:
		if err != nil {
			writeError(w, err)
			_ = conn.Close(websocket.StatusInternalError, err.Error())
			return
		}
	}

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
