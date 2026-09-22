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
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"io"
	"net"
	"net/http"
	"net/url"

	"github.com/pkg/errors"
	"golang.org/x/net/http/httpproxy"
	"golang.org/x/net/proxy"
)

// dialProxyAwareContext establishes a raw TCP connection to targetAddr, honoring the
// HTTP(S)_PROXY/NO_PROXY environment variables the same way net/http does. This is used
// for the side-channel TLS handshake, which would otherwise dial the target directly and
// hang when direct egress is blocked but a proxy is available (ROX-37149).
//
// The scheme is fixed to "https" because the side channel always performs a TLS handshake
// with the target; this mirrors the reverse-proxy data path, which resolves its proxy for
// the same "https" scheme. httpproxy.FromEnvironment is used rather than
// http.ProxyFromEnvironment because the latter caches the environment process-wide.
func dialProxyAwareContext(ctx context.Context, targetAddr string) (net.Conn, error) {
	reqURL := &url.URL{Scheme: "https", Host: targetAddr}
	proxyURL, err := httpproxy.FromEnvironment().ProxyFunc()(reqURL)
	if err != nil {
		return nil, errors.Wrap(err, "determining proxy from environment")
	}
	if proxyURL == nil {
		return (&net.Dialer{}).DialContext(ctx, "tcp", targetAddr)
	}

	switch proxyURL.Scheme {
	case "socks5", "socks5h":
		return dialViaSOCKS5(ctx, proxyURL, targetAddr)
	case "http", "https", "":
		return dialViaHTTPConnect(ctx, proxyURL, targetAddr)
	default:
		return nil, errors.Errorf("unsupported proxy scheme %q for proxy %q", proxyURL.Scheme, proxyURL.Redacted())
	}
}

// dialViaSOCKS5 dials targetAddr through a SOCKS5 proxy.
func dialViaSOCKS5(ctx context.Context, proxyURL *url.URL, targetAddr string) (net.Conn, error) {
	var auth *proxy.Auth
	if u := proxyURL.User; u != nil {
		password, _ := u.Password()
		auth = &proxy.Auth{User: u.Username(), Password: password}
	}
	dialer, err := proxy.SOCKS5("tcp", proxyURL.Host, auth, proxy.Direct)
	if err != nil {
		return nil, errors.Wrap(err, "creating SOCKS5 dialer")
	}
	if ctxDialer, ok := dialer.(proxy.ContextDialer); ok {
		return ctxDialer.DialContext(ctx, "tcp", targetAddr)
	}
	return dialer.Dial("tcp", targetAddr)
}

// dialViaHTTPConnect establishes a tunnel to targetAddr through an HTTP(S) proxy using the
// HTTP CONNECT method, mirroring what net/http.Transport does internally.
func dialViaHTTPConnect(ctx context.Context, proxyURL *url.URL, targetAddr string) (net.Conn, error) {
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", proxyHostPort(proxyURL))
	if err != nil {
		return nil, errors.Wrapf(err, "dialing proxy %q", proxyURL.Redacted())
	}
	// Close the connection unless the CONNECT succeeds, so a failed attempt does not leak it.
	success := false
	defer func() {
		if !success {
			_ = conn.Close()
		}
	}()

	if proxyURL.Scheme == "https" {
		tlsConn := tls.Client(conn, &tls.Config{ServerName: proxyURL.Hostname()})
		if hsErr := tlsConn.HandshakeContext(ctx); hsErr != nil {
			return nil, errors.Wrapf(hsErr, "TLS handshake with proxy %q", proxyURL.Redacted())
		}
		conn = tlsConn
	}

	// Abort the CONNECT exchange when the context is done. net.Conn has no context-aware
	// read/write, so closing the connection is how we unblock it.
	if done := ctx.Done(); done != nil {
		stop := make(chan struct{})
		defer close(stop)
		go func() {
			select {
			case <-done:
				_ = conn.Close()
			case <-stop:
			}
		}()
	}

	connectReq := &http.Request{
		Method: http.MethodConnect,
		URL:    &url.URL{Opaque: targetAddr},
		Host:   targetAddr,
		Header: make(http.Header),
	}
	if u := proxyURL.User; u != nil {
		password, _ := u.Password()
		connectReq.Header.Set("Proxy-Authorization", proxyBasicAuth(u.Username(), password))
	}
	if err = connectReq.Write(conn); err != nil {
		return nil, errors.Wrapf(err, "sending CONNECT to proxy %q", proxyURL.Redacted())
	}

	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, connectReq)
	if err != nil {
		return nil, errors.Wrapf(err, "reading CONNECT response from proxy %q", proxyURL.Redacted())
	}
	// The tunnel is now open; the body carries no meaningful content.
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("proxy %q refused CONNECT to %q: %s", proxyURL.Redacted(), targetAddr, resp.Status)
	}

	// A well-behaved proxy sends nothing before the tunneled peer does, but preserve any
	// bytes the reader buffered past the response headers rather than dropping them.
	tunnel := conn
	if n := br.Buffered(); n > 0 {
		buffered := make([]byte, n)
		if _, err := io.ReadFull(br, buffered); err != nil {
			return nil, errors.Wrap(err, "reading buffered bytes after CONNECT")
		}
		tunnel = &prefixConn{Conn: conn, prefix: io.MultiReader(bytes.NewReader(buffered), conn)}
	}

	success = true
	return tunnel, nil
}

// prefixConn is a net.Conn whose reads are served from prefix first (used to replay bytes
// that were buffered past the CONNECT response headers).
type prefixConn struct {
	net.Conn
	prefix io.Reader
}

func (c *prefixConn) Read(p []byte) (int, error) {
	return c.prefix.Read(p)
}

// proxyHostPort returns the proxy address with a scheme-appropriate default port applied.
func proxyHostPort(proxyURL *url.URL) string {
	if proxyURL.Port() != "" {
		return proxyURL.Host
	}
	port := "80"
	if proxyURL.Scheme == "https" {
		port = "443"
	}
	return net.JoinHostPort(proxyURL.Hostname(), port)
}

func proxyBasicAuth(username, password string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+password))
}
