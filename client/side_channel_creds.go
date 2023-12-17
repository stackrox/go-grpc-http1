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
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"

	"golang.org/x/net/proxy"
	"google.golang.org/grpc/credentials"
)

// sideChannelCreds implements gRPC transport credentials that do not modify the connection passed to `ClientHandshake`,
// but instead takes the `AuthInfo` from a connection established via a side channel.
type sideChannelCreds struct {
	credentials.TransportCredentials
	endpoint string

	authInfo      credentials.AuthInfo
	authInfoMutex sync.Mutex
}

func newCredsFromSideChannel(endpoint string, creds credentials.TransportCredentials) credentials.TransportCredentials {
	return &sideChannelCreds{
		TransportCredentials: creds,
		endpoint:             endpoint,
	}
}

func (c *sideChannelCreds) ClientHandshake(ctx context.Context, authority string, rawConn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	c.authInfoMutex.Lock()
	defer c.authInfoMutex.Unlock()

	if c.authInfo != nil {
		return rawConn, c.authInfo, nil
	}

	sideChannelConn, err := proxy.Dial(ctx, "tcp", c.endpoint)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = sideChannelConn.Close() }()

	_, authInfo, err := c.TransportCredentials.ClientHandshake(ctx, authority, sideChannelConn)
	if err != nil {
		return nil, nil, err
	}

	c.authInfo = authInfo
	return rawConn, authInfo, nil
}

// httpProxy is a HTTP/HTTPS connect proxy.
type httpProxy struct {
	host     string
	haveAuth bool
	username string
	password string
	forward  proxy.Dialer
}

func newHTTPProxy(uri *url.URL, forward proxy.Dialer) (proxy.Dialer, error) {
	s := new(httpProxy)
	s.host = uri.Host
	s.forward = forward
	if uri.User != nil {
		s.haveAuth = true
		s.username = uri.User.Username()
		s.password, _ = uri.User.Password()
	}

	return s, nil
}

func (s *httpProxy) Dial(network, addr string) (net.Conn, error) {
	// Dial and create the https client connection.
	c, err := s.forward.Dial("tcp", s.host)
	if err != nil {
		return nil, err
	}

	// HACK. http.ReadRequest also does this.
	reqURL, err := url.Parse("http://" + addr)
	if err != nil {
		c.Close()
		return nil, err
	}
	reqURL.Scheme = ""

	req, err := http.NewRequest("CONNECT", reqURL.String(), nil)
	if err != nil {
		c.Close()
		return nil, err
	}
	req.Close = false
	if s.haveAuth {
		req.SetBasicAuth(s.username, s.password)
	}
	// req.Header.Set("User-Agent", "Powerby Gota")

	err = req.Write(c)
	if err != nil {
		c.Close()
		return nil, err
	}

	resp, err := http.ReadResponse(bufio.NewReader(c), req)
	if err != nil {
		// TODO close resp body ?
		resp.Body.Close()
		c.Close()
		return nil, err
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		c.Close()
		err = fmt.Errorf("Connect server using proxy error, StatusCode [%d]", resp.StatusCode)
		return nil, err
	}

	return c, nil
}

func FromURL(u *url.URL, forward proxy.Dialer) (proxy.Dialer, error) {
	return proxy.FromURL(u, forward)
}

func FromEnvironment() proxy.Dialer {
	return proxy.FromEnvironment()
}

func init() {
	proxy.RegisterDialerType("http", newHTTPProxy)
	proxy.RegisterDialerType("https", newHTTPProxy)
}
