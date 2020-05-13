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
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"

	"github.com/golang/glog"
	"github.com/pkg/errors"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"golang.stackrox.io/grpc-http1/internal/grpcweb"
	"golang.stackrox.io/grpc-http1/internal/pipeconn"
	"golang.stackrox.io/grpc-http1/internal/stringutils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
)

func modifyResponse(resp *http.Response) error {
	if resp.ContentLength == 0 {
		// Make sure headers do not get flushed, as otherwise the gRPC client will complain about missing trailers.
		resp.Header.Set(dontFlushHeadersHeaderKey, "true")
	}
	contentType, contentSubType := stringutils.Split2(resp.Header.Get("Content-Type"), "+")
	if contentType != "application/grpc-web" {
		// No modification necessary if we aren't handling a gRPC web response.
		return nil
	}

	respCT := "application/grpc"
	if contentSubType != "" {
		respCT += "+" + contentSubType
	}
	resp.Header.Set("Content-Type", respCT)

	if resp.Body != nil {
		resp.Body = grpcweb.NewResponseReader(resp.Body, &resp.Trailer, nil)
	}
	return nil
}

// Fake a gRPC status with the given transport error
func writeError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/grpc")
	w.Header().Add("Trailer", "Grpc-Status")
	w.Header().Add("Trailer", "Grpc-Message")
	w.WriteHeader(http.StatusOK)

	w.Header().Set("Grpc-Status", fmt.Sprintf("%d", codes.Unavailable))
	w.Header().Set("Grpc-Message", errors.Wrap(err, "transport").Error())
}

func createReverseProxy(endpoint string, transport http.RoundTripper, insecure bool) *httputil.ReverseProxy {
	scheme := "https"
	if insecure {
		scheme = "http"
	}
	return &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.Header.Add("Accept", "application/grpc")
			req.Header.Add("Accept", "application/grpc-web")
			req.URL.Scheme = scheme
			req.URL.Host = endpoint
		},
		Transport:      transport,
		ModifyResponse: modifyResponse,
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, err error) {
			writeError(w, err)
		},
		// No need to set FlushInterval, as we force the writer to operate in unbuffered mode/flushing after every
		// write.
	}
}

func createTransport(tlsClientConf *tls.Config, forceHTTP2 bool, extraH2ALPNs []string) (http.RoundTripper, error) {
	if forceHTTP2 {
		transport := &http2.Transport{
			AllowHTTP:       true,
			TLSClientConfig: tlsClientConf,
		}
		if tlsClientConf == nil {
			transport.DialTLS = func(network, addr string, _ *tls.Config) (net.Conn, error) {
				return net.Dial(network, addr)
			}
		}
		return transport, nil
	}

	transport := &http.Transport{
		ForceAttemptHTTP2: true,
	}

	if tlsClientConf != nil {
		transport.TLSClientConfig = tlsClientConf.Clone()
	}
	if err := http2.ConfigureTransport(transport); err != nil {
		return nil, errors.Wrap(err, "configuring transport for HTTP/2 use")
	}

	// Make sure the transport for any extra HTTP/2-like ALPN string behaves like for HTTP/2.
	for _, extraALPN := range extraH2ALPNs {
		transport.TLSNextProto[extraALPN] = transport.TLSNextProto["h2"]
	}

	return transport, nil
}

func createClientProxy(endpoint string, tlsClientConf *tls.Config, forceHTTP2 bool, extraH2ALPNs []string) (*http.Server, pipeconn.DialContextFunc, error) {
	transport, err := createTransport(tlsClientConf, forceHTTP2, extraH2ALPNs)
	if err != nil {
		return nil, nil, errors.Wrap(err, "creating transport")
	}
	proxy := createReverseProxy(endpoint, transport, tlsClientConf == nil)
	return makeProxyServer(proxy)
}

// ConnectViaProxy establishes a gRPC client connection via a HTTP/2 proxy that handles endpoints behind HTTP/1 proxies.
func ConnectViaProxy(ctx context.Context, endpoint string, tlsClientConf *tls.Config, opts ...ConnectOption) (*grpc.ClientConn, error) {
	var connectOpts connectOptions
	for _, opt := range opts {
		opt.apply(&connectOpts)
	}

	proxy, dialCtx, err := createClientProxy(endpoint, tlsClientConf, connectOpts.forceHTTP2, connectOpts.extraH2ALPNs)
	if err != nil {
		return nil, errors.Wrap(err, "creating client proxy")
	}

	return dialGRPCServer(ctx, proxy, makeDialOpts(endpoint, dialCtx, tlsClientConf, opts...))
}

func makeProxyServer(handler http.Handler) (*http.Server, pipeconn.DialContextFunc, error) {
	lis, dialCtx := pipeconn.NewPipeListener()

	var http2Srv http2.Server
	srv := &http.Server{
		Addr:    lis.Addr().String(),
		Handler: h2c.NewHandler(nonBufferingHandler(handler), &http2Srv),
	}
	if err := http2.ConfigureServer(srv, &http2Srv); err != nil {
		return nil, nil, errors.Wrap(err, "configuring HTTP/2 server")
	}

	go func() {
		if err := srv.Serve(lis); err != nil && err != http.ErrServerClosed {
			glog.Warningf("Unexpected error returned from serving gRPC proxy server: %v", err)
		}
	}()

	return srv, dialCtx, nil
}

func makeDialOpts(endpoint string, dialCtx pipeconn.DialContextFunc, tlsClientConf *tls.Config, opts ...ConnectOption) []grpc.DialOption {
	var connectOpts connectOptions
	for _, opt := range opts {
		opt.apply(&connectOpts)
	}

	dialOpts := make([]grpc.DialOption, 0, len(connectOpts.dialOpts)+2)
	dialOpts = append(dialOpts, grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
		return dialCtx(ctx)
	}))
	if tlsClientConf != nil {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(newCredsFromSideChannel(endpoint, credentials.NewTLS(tlsClientConf))))
	}
	dialOpts = append(dialOpts, connectOpts.dialOpts...)

	return dialOpts
}

func dialGRPCServer(ctx context.Context, proxy *http.Server, dialOpts []grpc.DialOption) (*grpc.ClientConn, error) {
	cc, err := grpc.DialContext(ctx, proxy.Addr, dialOpts...)
	if err != nil {
		_ = proxy.Close()
		return nil, err
	}
	go closeServerOnConnShutdown(proxy, cc)
	return cc, nil
}

func closeServerOnConnShutdown(srv *http.Server, cc *grpc.ClientConn) {
	for state := cc.GetState(); state != connectivity.Shutdown; state = cc.GetState() {
		cc.WaitForStateChange(context.Background(), state)
	}
	if err := srv.Close(); err != nil {
		glog.Warningf("Error closing gRPC proxy server: %v", err)
	}
}