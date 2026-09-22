// Copyright (c) 2020 StackRox Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License

package client

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// clearProxyEnv resets all proxy-related environment variables so tests are not affected by
// the ambient environment. t.Setenv restores the previous values when the test finishes.
func clearProxyEnv(t *testing.T) {
	for _, v := range []string{
		"HTTP_PROXY", "http_proxy",
		"HTTPS_PROXY", "https_proxy",
		"NO_PROXY", "no_proxy",
	} {
		t.Setenv(v, "")
	}
}

// startEchoServer starts a TCP server that echoes back everything it receives.
func startEchoServer(t *testing.T) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = lis.Close() })
	go func() {
		for {
			conn, err := lis.Accept()
			if err != nil {
				return
			}
			go func() {
				defer func() { _ = conn.Close() }()
				_, _ = io.Copy(conn, conn)
			}()
		}
	}()
	return lis.Addr().String()
}

type fakeConnectProxy struct {
	addr string

	mu          sync.Mutex
	connectHits int
	lastHost    string
	lastAuth    string
}

// startFakeConnectProxy starts a minimal HTTP proxy that answers CONNECT with the given
// status code. On a 200 response it tunnels the connection to itself (echo), so callers can
// verify that bytes flow through the established tunnel.
func startFakeConnectProxy(t *testing.T, statusLine string, tunnel bool) *fakeConnectProxy {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = lis.Close() })

	p := &fakeConnectProxy{addr: lis.Addr().String()}
	go func() {
		for {
			conn, err := lis.Accept()
			if err != nil {
				return
			}
			go p.handle(conn, statusLine, tunnel)
		}
	}()
	return p
}

func (p *fakeConnectProxy) handle(conn net.Conn, statusLine string, tunnel bool) {
	defer func() { _ = conn.Close() }()
	br := bufio.NewReader(conn)
	req, err := http.ReadRequest(br)
	if err != nil {
		return
	}
	p.mu.Lock()
	p.connectHits++
	p.lastHost = req.Host
	p.lastAuth = req.Header.Get("Proxy-Authorization")
	p.mu.Unlock()

	if _, err := io.WriteString(conn, "HTTP/1.1 "+statusLine+"\r\n\r\n"); err != nil {
		return
	}
	if !tunnel {
		return
	}
	// Echo tunneled bytes, honoring anything already buffered past the request headers.
	if n := br.Buffered(); n > 0 {
		buffered := make([]byte, n)
		if _, err := io.ReadFull(br, buffered); err == nil {
			_, _ = conn.Write(buffered)
		}
	}
	_, _ = io.Copy(conn, conn)
}

func (p *fakeConnectProxy) stats() (hits int, host, auth string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.connectHits, p.lastHost, p.lastAuth
}

func roundTrip(t *testing.T, conn net.Conn, payload string) string {
	t.Helper()
	require.NoError(t, conn.SetDeadline(time.Now().Add(5*time.Second)))
	_, err := io.WriteString(conn, payload)
	require.NoError(t, err)
	buf := make([]byte, len(payload))
	_, err = io.ReadFull(conn, buf)
	require.NoError(t, err)
	return string(buf)
}

func TestDialProxyAwareContext_NoProxy(t *testing.T) {
	clearProxyEnv(t)
	echoAddr := startEchoServer(t)

	conn, err := dialProxyAwareContext(context.Background(), echoAddr)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	assert.Equal(t, "ping", roundTrip(t, conn, "ping"))
}

func TestDialProxyAwareContext_HTTPConnect(t *testing.T) {
	clearProxyEnv(t)
	proxy := startFakeConnectProxy(t, "200 Connection established", true)
	t.Setenv("HTTPS_PROXY", "http://"+proxy.addr)

	const target = "central.example.com:443"
	conn, err := dialProxyAwareContext(context.Background(), target)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	assert.Equal(t, "hello", roundTrip(t, conn, "hello"))

	hits, host, auth := proxy.stats()
	assert.Equal(t, 1, hits, "connection must go through the proxy")
	assert.Equal(t, target, host, "CONNECT must target the requested endpoint")
	assert.Empty(t, auth)
}

func TestDialProxyAwareContext_HTTPConnectBasicAuth(t *testing.T) {
	clearProxyEnv(t)
	proxy := startFakeConnectProxy(t, "200 Connection established", true)
	t.Setenv("HTTPS_PROXY", "http://user:pass@"+proxy.addr)

	conn, err := dialProxyAwareContext(context.Background(), "central.example.com:443")
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()
	_ = roundTrip(t, conn, "hi")

	_, _, auth := proxy.stats()
	assert.Equal(t, proxyBasicAuth("user", "pass"), auth)
}

func TestDialProxyAwareContext_NoProxyMatch(t *testing.T) {
	clearProxyEnv(t)
	proxy := startFakeConnectProxy(t, "200 Connection established", true)
	t.Setenv("HTTPS_PROXY", "http://"+proxy.addr)
	echoAddr := startEchoServer(t)
	// Exclude the echo server from proxying; it must be dialed directly.
	host, _, err := net.SplitHostPort(echoAddr)
	require.NoError(t, err)
	t.Setenv("NO_PROXY", host)

	conn, err := dialProxyAwareContext(context.Background(), echoAddr)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	assert.Equal(t, "direct", roundTrip(t, conn, "direct"))
	hits, _, _ := proxy.stats()
	assert.Zero(t, hits, "NO_PROXY host must bypass the proxy")
}

func TestDialProxyAwareContext_ConnectRefused(t *testing.T) {
	clearProxyEnv(t)
	proxy := startFakeConnectProxy(t, "403 Forbidden", false)
	t.Setenv("HTTPS_PROXY", "http://"+proxy.addr)

	_, err := dialProxyAwareContext(context.Background(), "central.example.com:443")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "403")
}

func TestDialProxyAwareContext_UnsupportedScheme(t *testing.T) {
	clearProxyEnv(t)
	t.Setenv("HTTPS_PROXY", "ftp://proxy.example.com:2121")

	_, err := dialProxyAwareContext(context.Background(), "central.example.com:443")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported proxy scheme")
}

func TestProxyHostPort(t *testing.T) {
	for name, tc := range map[string]struct {
		raw      string
		expected string
	}{
		"http default port":  {raw: "http://proxy.example.com", expected: "proxy.example.com:80"},
		"https default port": {raw: "https://proxy.example.com", expected: "proxy.example.com:443"},
		"explicit port":      {raw: "http://proxy.example.com:8080", expected: "proxy.example.com:8080"},
	} {
		t.Run(name, func(t *testing.T) {
			u, err := url.Parse(tc.raw)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, proxyHostPort(u))
		})
	}
}
