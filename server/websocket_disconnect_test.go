package server

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/require"
	"golang.stackrox.io/grpc-http1/internal/grpcproto"
	"golang.stackrox.io/grpc-http1/internal/grpcwebsocket"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
)

type infiniteStreamServer struct {
	started chan struct{}
	done    chan error
}

func (s *infiniteStreamServer) handle(_ any, stream grpc.ServerStream) error {
	close(s.started)
	payload := &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_SERVING}
	for {
		if err := stream.SendMsg(payload); err != nil {
			s.done <- err
			return err
		}
	}
}

type wsHandlerTracker struct {
	grpcSrv *grpc.Server
	done    chan struct{}
}

func (h *wsHandlerTracker) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	defer close(h.done)
	handleGRPCWS(w, req, h.grpcSrv)
}

type connRecorder struct {
	mu    sync.Mutex
	conns []net.Conn
}

func (r *connRecorder) dial(ctx context.Context, network, addr string) (net.Conn, error) {
	conn, err := (&net.Dialer{}).DialContext(ctx, network, addr)
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	r.conns = append(r.conns, conn)
	r.mu.Unlock()
	return conn, nil
}

func (r *connRecorder) closeAll() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, conn := range r.conns {
		_ = conn.Close()
	}
}

func TestHandleGRPCWS_ReleasesStreamWhenClientConnectionDrops(t *testing.T) {
	stream := &infiniteStreamServer{started: make(chan struct{}), done: make(chan error, 1)}
	grpcSrv := grpc.NewServer(grpc.UnknownServiceHandler(stream.handle))
	defer grpcSrv.Stop()

	tracker := &wsHandlerTracker{grpcSrv: grpcSrv, done: make(chan struct{})}
	httpSrv := httptest.NewServer(tracker)
	defer httpSrv.Close()

	recorder := &connRecorder{}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(httpSrv.URL, "http")+"/test.Service/Stream", &websocket.DialOptions{
		HTTPClient:   &http.Client{Transport: &http.Transport{DialContext: recorder.dial}},
		HTTPHeader:   http.Header{"Content-Type": []string{"application/grpc"}},
		Subprotocols: []string{grpcwebsocket.SubprotocolName},
	})
	require.NoError(t, err)
	conn.SetReadLimit(-1)

	require.NoError(t, conn.Write(ctx, websocket.MessageBinary, grpcproto.MakeMessageHeader(0, 0)))

	select {
	case <-stream.started:
	case <-ctx.Done():
		t.Fatal("stream handler did not start")
	}

	_, _, err = conn.Read(ctx)
	require.NoError(t, err)

	recorder.closeAll()

	select {
	case <-stream.done:
	case <-time.After(5 * time.Second):
		t.Fatal("stream handler still blocked after the client connection dropped")
	}

	select {
	case <-tracker.done:
	case <-time.After(5 * time.Second):
		t.Fatal("handleGRPCWS still blocked after the client connection dropped")
	}
}
