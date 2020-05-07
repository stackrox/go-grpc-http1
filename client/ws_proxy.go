package client

import (
	"context"
	"crypto/tls"
	"net/http"
	"strings"

	"github.com/golang/glog"
	"github.com/pkg/errors"
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

func (h *http2WebSocketProxy) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.ProtoMajor != 2 || !strings.HasPrefix(req.Header.Get("Content-Type"), "application/grpc") {
		glog.Error("Request is not a valid gRPC request")
		w.WriteHeader(http.StatusUnsupportedMediaType)
		return
	}

	scheme := "https"
	if h.insecure {
		scheme = "http"
	}

	url := *req.URL // Copy the value, so we do not overwrite the URL.
	url.Scheme = scheme
	url.Host = h.endpoint
	conn, _, err := websocket.Dial(req.Context(), url.String(), &websocket.DialOptions{
		// Add the gRPC headers to the WebSocket handshake request.
		HTTPHeader:   req.Header,
		HTTPClient:   h.httpClient,
		Subprotocols: subprotocols,
	})
	if err != nil {
		writeError(w, errors.Wrapf(err, "connecting to gRPC server %q", url.String()))
		return
	}

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
