package server

import (
	"context"
	"errors"
	"io"
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

type wsTestEnv struct {
	conn     *websocket.Conn
	recorder *connRecorder
	tracker  *wsHandlerTracker
}

func startWSStream(t *testing.T, ctx context.Context, handler grpc.StreamHandler) *wsTestEnv {
	grpcSrv := grpc.NewServer(grpc.UnknownServiceHandler(handler))
	t.Cleanup(func() { stopWithTimeout(t, grpcSrv) })

	tracker := &wsHandlerTracker{grpcSrv: grpcSrv, done: make(chan struct{})}
	httpSrv := httptest.NewServer(tracker)
	t.Cleanup(httpSrv.Close)

	recorder := &connRecorder{}
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(httpSrv.URL, "http")+"/test.Service/Stream", &websocket.DialOptions{
		HTTPClient:   &http.Client{Transport: &http.Transport{DialContext: recorder.dial}},
		HTTPHeader:   http.Header{"Content-Type": []string{"application/grpc"}},
		Subprotocols: []string{grpcwebsocket.SubprotocolName},
	})
	require.NoError(t, err)
	conn.SetReadLimit(-1)
	t.Cleanup(recorder.closeAll)

	return &wsTestEnv{conn: conn, recorder: recorder, tracker: tracker}
}

func stopWithTimeout(t *testing.T, grpcSrv *grpc.Server) {
	stopped := make(chan struct{})
	go func() {
		grpcSrv.Stop()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Error("gRPC server cleanup did not complete")
	}
}

func waitFor(t *testing.T, done <-chan struct{}, msg string) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal(msg)
	}
}

func TestHandleGRPCWS_ReleasesStreamWhenClientConnectionDrops(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stream := &infiniteStreamServer{started: make(chan struct{}), done: make(chan error, 1)}
	env := startWSStream(t, ctx, stream.handle)

	require.NoError(t, env.conn.Write(ctx, websocket.MessageBinary, grpcproto.MakeMessageHeader(0, 0)))
	waitFor(t, stream.started, "stream handler did not start")

	_, _, err := env.conn.Read(ctx)
	require.NoError(t, err)

	env.recorder.closeAll()

	select {
	case <-stream.done:
	case <-time.After(5 * time.Second):
		t.Fatal("stream handler still blocked after the client connection dropped")
	}
	waitFor(t, env.tracker.done, "handleGRPCWS still blocked after the client connection dropped")
}

func TestHandleGRPCWS_ReleasesIdleStreamWhenClientConnectionDrops(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	started := make(chan struct{})
	done := make(chan struct{})
	env := startWSStream(t, ctx, func(_ any, stream grpc.ServerStream) error {
		defer close(done)
		if err := stream.SendMsg(&grpc_health_v1.HealthCheckResponse{}); err != nil {
			return err
		}
		close(started)
		<-stream.Context().Done()
		return stream.Context().Err()
	})

	require.NoError(t, env.conn.Write(ctx, websocket.MessageBinary, grpcproto.MakeMessageHeader(0, 0)))
	waitFor(t, started, "stream handler did not start")

	env.recorder.closeAll()

	waitFor(t, done, "idle stream handler still blocked after the client connection dropped")
	waitFor(t, env.tracker.done, "handleGRPCWS still blocked after the client connection dropped")
}

func TestHandleGRPCWS_EndOfStreamIsHalfClose(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	received := make(chan struct{})
	env := startWSStream(t, ctx, func(_ any, stream grpc.ServerStream) error {
		for {
			err := stream.RecvMsg(&grpc_health_v1.HealthCheckRequest{})
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return err
			}
		}
		close(received)
		time.Sleep(100 * time.Millisecond)
		if err := stream.Context().Err(); err != nil {
			return err
		}
		return stream.SendMsg(&grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_SERVING})
	})

	require.NoError(t, env.conn.Write(ctx, websocket.MessageBinary, grpcproto.MakeMessageHeader(0, 0)))
	require.NoError(t, env.conn.Write(ctx, websocket.MessageBinary, grpcproto.EndStreamHeader))
	waitFor(t, received, "stream handler did not observe end of stream")

	for {
		_, msg, err := env.conn.Read(ctx)
		require.NoError(t, err, "expected a response data frame after half-close")
		if grpcproto.IsDataFrame(msg) {
			break
		}
	}

	for {
		if _, _, err := env.conn.Read(ctx); err != nil {
			require.Equal(t, websocket.StatusNormalClosure, websocket.CloseStatus(err))
			break
		}
	}
	waitFor(t, env.tracker.done, "handleGRPCWS did not finish after the stream completed")
}
