package integrationtests

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"golang.stackrox.io/grpc-http1/client"
	"golang.stackrox.io/grpc-http1/server"
	"google.golang.org/grpc"
	"google.golang.org/grpc/examples/features/proto/echo"
	"google.golang.org/grpc/metadata"
)

func listenLocal(t *testing.T) net.Listener {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	return lis
}

func newCtx(t *testing.T, checkHeaders bool, checkTrailers bool) (context.Context, []grpc.CallOption, func()) {
	headerStr := fmt.Sprintf("%s-Hdr", t.Name())
	trailerStr := fmt.Sprintf("%s-Trl", t.Name())

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	ctx = metadata.AppendToOutgoingContext(
		ctx,
		"header-echo", headerStr,
		"trailer-echo", trailerStr,
	)

	var respHeaders, respTrailers metadata.MD
	callOpts := []grpc.CallOption{grpc.Header(&respHeaders), grpc.Trailer(&respTrailers)}

	finalize := func() {
		cancel()

		if checkHeaders {
			assert.ElementsMatch(t, respHeaders.Get("header-echo-response"), []string{headerStr})
		}
		if checkTrailers {
			assert.ElementsMatch(t, respTrailers.Get("trailer-echo-response"), []string{trailerStr})
		}
	}
	return ctx, callOpts, finalize
}

func TestWithEchoService(t *testing.T) {
	testCfg := newTestConfig(t)
	defer testCfg.TearDown()

	cases := []testCase{
		{
			targetID:             "raw-grpc",
			useProxy:             false,
			expectUnaryOK:        true,
			expectServerStreamOK: true,
			expectClientStreamOK: true,
			expectBidiStreamOK:   true,
		},
		{
			targetID:                "raw-grpc",
			behindHTTP1ReverseProxy: true,
			useProxy:                false,
			expectUnaryOK:           false,
			expectServerStreamOK:    false,
			expectClientStreamOK:    false,
			expectBidiStreamOK:      false,
		},
		{
			targetID:             "raw-grpc",
			useProxy:             true,
			expectUnaryOK:        true,
			expectServerStreamOK: true,
			expectClientStreamOK: true,
			expectBidiStreamOK:   true,
		},
		{
			targetID:                "raw-grpc",
			behindHTTP1ReverseProxy: true,
			useProxy:                true,
			expectUnaryOK:           false,
			expectServerStreamOK:    false,
			expectClientStreamOK:    false,
			expectBidiStreamOK:      false,
		},
		{
			targetID:             "downgrading-grpc",
			useProxy:             false,
			expectUnaryOK:        true,
			expectServerStreamOK: true,
			expectClientStreamOK: true,
			expectBidiStreamOK:   true,
		},
		{
			targetID:                "downgrading-grpc",
			behindHTTP1ReverseProxy: true,
			useProxy:                false,
			expectUnaryOK:           false,
			expectServerStreamOK:    false,
			expectClientStreamOK:    false,
			expectBidiStreamOK:      false,
		},
		{
			targetID:             "downgrading-grpc",
			useProxy:             true,
			expectUnaryOK:        true,
			expectServerStreamOK: true,
			expectClientStreamOK: true,
			expectBidiStreamOK:   true,
		},
		{
			targetID:                "downgrading-grpc",
			behindHTTP1ReverseProxy: true,
			useProxy:                true,
			expectUnaryOK:           true,
			expectServerStreamOK:    true,
			expectClientStreamOK:    false,
			expectBidiStreamOK:      false,
		},
	}

	for _, c := range cases {
		t.Run(c.Name(), func(t *testing.T) {
			c.Run(t, testCfg)
		})
	}
}

func TestWSWithEchoService(t *testing.T) {
	testCfg := newTestConfig(t)
	defer testCfg.TearDown()

	cases := []testCase{
		{
			targetID:             "raw-grpc",
			useProxy:             false,
			useWebSocket:         true,
			expectUnaryOK:        true,
			expectServerStreamOK: true,
			expectClientStreamOK: true,
			expectBidiStreamOK:   true,
		},
		{
			targetID:                "raw-grpc",
			behindHTTP1ReverseProxy: true,
			useProxy:                false,
			useWebSocket:            true,
			expectUnaryOK:           false,
			expectServerStreamOK:    false,
			expectClientStreamOK:    false,
			expectBidiStreamOK:      false,
		},
		{
			targetID:             "raw-grpc",
			useProxy:             true,
			useWebSocket:         true,
			expectUnaryOK:        false,
			expectServerStreamOK: false,
			expectClientStreamOK: false,
			expectBidiStreamOK:   false,
		},
		{
			targetID:                "raw-grpc",
			behindHTTP1ReverseProxy: true,
			useProxy:                true,
			useWebSocket:            true,
			expectUnaryOK:           false,
			expectServerStreamOK:    false,
			expectClientStreamOK:    false,
			expectBidiStreamOK:      false,
		},
		{
			targetID:             "downgrading-grpc",
			useProxy:             false,
			useWebSocket:         true,
			expectUnaryOK:        true,
			expectServerStreamOK: true,
			expectClientStreamOK: true,
			expectBidiStreamOK:   true,
		},
		{
			targetID:                "downgrading-grpc",
			behindHTTP1ReverseProxy: true,
			useProxy:                false,
			useWebSocket:            true,
			expectUnaryOK:           false,
			expectServerStreamOK:    false,
			expectClientStreamOK:    false,
			expectBidiStreamOK:      false,
		},
		{
			targetID:             "downgrading-grpc",
			useProxy:             true,
			useWebSocket:         true,
			expectUnaryOK:        true,
			expectServerStreamOK: true,
			expectClientStreamOK: true,
			expectBidiStreamOK:   true,
		},
		{
			targetID:                "downgrading-grpc",
			behindHTTP1ReverseProxy: true,
			useProxy:                true,
			useWebSocket:            true,
			expectUnaryOK:           true,
			expectServerStreamOK:    true,
			expectClientStreamOK:    true,
			expectBidiStreamOK:      true,
		},
	}

	for _, c := range cases {
		t.Run(c.Name(), func(t *testing.T) {
			c.Run(t, testCfg)
		})
	}
}

func newHTTP1Proxy(target string) *http.Server {
	transport := &http.Transport{
		ForceAttemptHTTP2: false,
	}

	handler := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Host = target
			if req.URL.Scheme == "" {
				req.URL.Scheme = "http"
			}
			req.ProtoMajor, req.ProtoMinor, req.Proto = 1, 1, "HTTP/1.1"
		},
		Transport: transport,
	}
	return &http.Server{
		Handler: handler,
	}
}

type testCase struct {
	targetID                string
	behindHTTP1ReverseProxy bool
	useProxy                bool
	useWebSocket            bool

	expectUnaryOK        bool
	expectClientStreamOK bool
	expectServerStreamOK bool
	expectBidiStreamOK   bool
}

func (c *testCase) Name() string {
	var sb strings.Builder
	sb.WriteString(c.targetID)

	if c.useWebSocket {
		sb.WriteString("-ws")
	}

	if c.behindHTTP1ReverseProxy {
		sb.WriteString("-behind-http1-revproxy")
	}

	if c.useProxy {
		sb.WriteString("-via-client-proxy")
	} else {
		sb.WriteString("-direct")
	}
	return sb.String()
}

func (c *testCase) Run(t *testing.T, cfg *testConfig) {
	targetAddr := cfg.TargetAddr(t, c.targetID)

	if c.behindHTTP1ReverseProxy {
		lis := listenLocal(t)
		revProxySrv := newHTTP1Proxy(targetAddr)
		go revProxySrv.Serve(lis)

		defer revProxySrv.Shutdown(context.Background())

		targetAddr = lis.Addr().String()
	}

	var cc *grpc.ClientConn
	var err error

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	headerStr := fmt.Sprintf("%s-Hdr", t.Name())
	trailerStr := fmt.Sprintf("%s-Trl", t.Name())

	ctx = metadata.AppendToOutgoingContext(
		ctx,
		"header-echo", headerStr,
		"trailer-echo", trailerStr,
	)

	if c.useProxy {
		opts := []client.ConnectOption{client.DialOpts(grpc.WithInsecure())}
		if !c.behindHTTP1ReverseProxy {
			opts = append(opts, client.ForceHTTP2())
		}
		if c.useWebSocket {
			cc, err = client.ConnectViaWSProxy(ctx, targetAddr, nil, opts...)
		} else {
			cc, err = client.ConnectViaProxy(ctx, targetAddr, nil, opts...)
		}
	} else {
		cc, err = grpc.DialContext(ctx, targetAddr, grpc.WithInsecure())
	}
	require.NoError(t, err, "failed to establish connection")

	echoClient := echo.NewEchoClient(cc)

	t.Run("unary", func(t *testing.T) {
		c.testUnary(t, echoClient)
	})

	t.Run("clientStreaming", func(t *testing.T) {
		c.testClientStreaming(t, echoClient)
	})

	t.Run("serverStreaming", func(t *testing.T) {
		c.testServerStreaming(t, echoClient)
	})

	t.Run("bidiStreaming", func(t *testing.T) {
		c.testBidiStreaming(t, echoClient)
	})
}

func (c *testCase) testUnary(t *testing.T, client echo.EchoClient) {
	ctx, callOpts, finalize := newCtx(t, c.expectUnaryOK, c.expectUnaryOK)
	defer finalize()

	msg := fmt.Sprintf("Message for %s", t.Name())
	resp, err := client.UnaryEcho(ctx, &echo.EchoRequest{Message: msg}, callOpts...)
	if c.expectUnaryOK {
		require.NoError(t, err)
		assert.Equal(t, msg, resp.GetMessage())
	} else {
		assert.Error(t, err)
	}
}

func (c *testCase) testClientStreaming(t *testing.T, client echo.EchoClient) {
	ctx, callOpts, finalize := newCtx(t, c.expectClientStreamOK, c.expectClientStreamOK)
	defer finalize()

	stream, err := client.ClientStreamingEcho(ctx, callOpts...)
	if c.expectClientStreamOK {
		require.NoError(t, err)
	} else if err != nil {
		return
	}

	var sentMsgs []string
	for _, i := range []int{1, 2, 3} {
		msg := fmt.Sprintf("Message %d for %s", i, t.Name())
		err := stream.Send(&echo.EchoRequest{Message: msg})
		if c.expectClientStreamOK {
			require.NoError(t, err)
		} else if err != nil {
			return
		}
		sentMsgs = append(sentMsgs, msg)
	}

	resp, err := stream.CloseAndRecv()
	if c.expectClientStreamOK {
		require.NoError(t, err)
	} else {
		require.Error(t, err)
		return
	}

	expectedRespMsg := strings.Join(sentMsgs, "\n")
	assert.Equal(t, expectedRespMsg, resp.GetMessage())
}

func (c *testCase) testServerStreaming(t *testing.T, client echo.EchoClient) {
	ctx, callOpts, finalize := newCtx(t, c.expectServerStreamOK, c.expectServerStreamOK)
	defer finalize()

	var lines []string
	for _, i := range []int{1, 2, 3} {
		line := fmt.Sprintf("Message %d for %s", i, t.Name())
		lines = append(lines, line)
	}

	msg := strings.Join(lines, "\n")

	stream, err := client.ServerStreamingEcho(ctx, &echo.EchoRequest{Message: msg}, callOpts...)
	if c.expectServerStreamOK {
		require.NoError(t, err)
	} else if err != nil {
		return
	}

	i := 0
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}

		if c.expectServerStreamOK {
			require.NoError(t, err)
		} else if err != nil {
			return
		}

		assert.Equal(t, lines[i], resp.GetMessage())
		i++
	}

	assert.True(t, c.expectServerStreamOK)
	assert.Equal(t, 3, i)
}

func (c *testCase) testBidiStreaming(t *testing.T, client echo.EchoClient) {
	ctx, callOpts, finalize := newCtx(t, c.expectBidiStreamOK, c.expectBidiStreamOK)
	defer finalize()

	stream, err := client.BidirectionalStreamingEcho(ctx, callOpts...)
	if c.expectBidiStreamOK {
		require.NoError(t, err)
	} else if err != nil {
		return
	}

	for i := 0; i < 10; i++ {
		msg := fmt.Sprintf("Message %d for %s", i, t.Name())
		err := stream.Send(&echo.EchoRequest{Message: msg})
		if c.expectBidiStreamOK {
			require.NoError(t, err)
		} else if err != nil {
			return
		}

		resp, err := stream.Recv()
		if c.expectBidiStreamOK {
			require.NoError(t, err)
		} else if err != nil {
			return
		}

		assert.Equal(t, msg, resp.GetMessage())
	}

	err = stream.CloseSend()
	if c.expectBidiStreamOK {
		require.NoError(t, err)
	} else if err != nil {
		return
	}

	_, err = stream.Recv()
	if c.expectBidiStreamOK {
		require.Equal(t, io.EOF, err)
	} else {
		require.NotEqual(t, io.EOF, err)
		require.Error(t, err)
	}
}

type testConfig struct {
	grpcSrv *grpc.Server
	httpSrv *http.Server

	targetAddrs map[string]string
}

func newTestConfig(t *testing.T) *testConfig {
	targetAddrs := make(map[string]string)
	grpcSrv := grpc.NewServer()
	echo.RegisterEchoServer(grpcSrv, echoService{})

	lis := listenLocal(t)
	go grpcSrv.Serve(lis)
	targetAddrs["raw-grpc"] = lis.Addr().String()

	downgradingSrv := &http.Server{}
	var h2Srv http2.Server
	require.NoError(t, http2.ConfigureServer(downgradingSrv, &h2Srv))
	downgradingSrv.Handler = h2c.NewHandler(
		server.CreateDowngradingHandler(grpcSrv, http.NotFoundHandler()),
		&h2Srv)

	lis = listenLocal(t)
	go downgradingSrv.Serve(lis)
	targetAddrs["downgrading-grpc"] = lis.Addr().String()

	return &testConfig{
		grpcSrv:     grpcSrv,
		httpSrv:     downgradingSrv,
		targetAddrs: targetAddrs,
	}
}

func (s *testConfig) TargetAddr(t *testing.T, targetID string) string {
	addr := s.targetAddrs[targetID]
	require.NotEmptyf(t, addr, "invalid target %q", targetID)
	return addr
}

func (s *testConfig) TearDown() {
	s.grpcSrv.GracefulStop()
	s.httpSrv.Shutdown(context.Background())
}
