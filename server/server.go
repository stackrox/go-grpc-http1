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

package server

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/golang/glog"
	"golang.stackrox.io/grpc-http1/internal/grpcweb"
	"golang.stackrox.io/grpc-http1/internal/grpcwebsocket"
	"golang.stackrox.io/grpc-http1/internal/size"
	"golang.stackrox.io/grpc-http1/internal/sliceutils"
	"golang.stackrox.io/grpc-http1/internal/stringutils"
	"google.golang.org/grpc"
	"nhooyr.io/websocket"
)

const (
	name = "server"
)

// handleGRPCWS handles gRPC requests via WebSockets.
func handleGRPCWS(w http.ResponseWriter, req *http.Request, grpcSrv *grpc.Server) {
	// TODO: Accept the websocket on-demand. For now, this is fine.
	// Accept a WebSocket connection.
	conn, err := websocket.Accept(w, req, nil)
	if err != nil {
		http.Error(w, fmt.Sprintf("accepting websocket connection: %v", err), http.StatusInternalServerError)
		return
	}
	conn.SetReadLimit(64 * size.MB)

	ctx := req.Context()

	grpcReq := req.Clone(ctx)
	grpcReq.ProtoMajor, grpcReq.ProtoMinor, grpcReq.Proto = 2, 0, "HTTP/2.0"
	grpcReq.Method = http.MethodPost // gRPC requests are always POST requests.

	// Filter out all WebSocket-specific headers.
	hdr := grpcReq.Header
	hdr.Del("Connection")
	hdr.Del("Upgrade")
	for k := range hdr {
		if strings.HasPrefix(k, "Sec-Websocket-") {
			delete(hdr, k)
		}
	}
	// Remove content-length header info.
	hdr.Del("Content-Length")
	grpcReq.ContentLength = -1

	// Set the body to a custom WebSocket reader.
	grpcReq.Body = newWebSocketReader(ctx, conn)

	// Use a custom WebSocket http.ResponseWriter to write messages back to the client.
	grpcResponseWriter, respReader := newWebSocketResponseWriter()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := grpcwebsocket.Write(ctx, conn, respReader, name); err != nil {
			_ = conn.Close(websocket.StatusInternalError, err.Error())
		}
	}()

	grpcSrv.ServeHTTP(grpcResponseWriter, grpcReq)
	if err := grpcResponseWriter.Close(); err != nil {
		_ = conn.Close(websocket.StatusInternalError, err.Error())
	}

	wg.Wait()
	// It's ok to potentially close the connection multiple times.
	// Only the first time matters.
	_ = conn.Close(websocket.StatusNormalClosure, "")
}

func handleGRPCWeb(w http.ResponseWriter, req *http.Request, validPaths map[string]struct{}, grpcSrv *grpc.Server) {
	// Check for HTTP/2.
	if req.ProtoMajor != 2 {
		if _, ok := validPaths[req.URL.Path]; !ok {
			// Client-streaming only works with HTTP/2.
			http.Error(w, "Method cannot be downgraded", http.StatusInternalServerError)
			return
		}
		req.ProtoMajor, req.ProtoMinor, req.Proto = 2, 0, "HTTP/2.0"
	}

	if req.Header.Get("TE") == "trailers" {
		// Yay, client accepts trailers! Let the normal gRPC handler handle the request.
		grpcSrv.ServeHTTP(w, req)
		return
	}

	acceptGRPCWeb := sliceutils.StringFind(req.Header["Accept"], "application/grpc-web") != -1
	if !acceptGRPCWeb {
		// Client doesn't support trailers and doesn't accept a response downgraded to gRPC web.
		http.Error(w, "Client neither supports trailers nor gRPC web responses", http.StatusInternalServerError)
		return
	}

	// Tell the server we would accept trailers (the gRPC server currently (v1.29.1) doesn't check for this but it
	// really should, as the purpose of the TE header according to the gRPC spec is to detect incompatible proxies).
	req.Header.Set("TE", "trailers")

	// Downgrade response to gRPC web.
	transcodingWriter, finalize := grpcweb.NewResponseWriter(w)
	grpcSrv.ServeHTTP(transcodingWriter, req)
	if err := finalize(); err != nil {
		glog.Errorf("Error sending trailers in downgraded gRPC web response: %v", err)
	}
}

// CreateDowngradingHandler takes a gRPC server and a plain HTTP handler, and returns an HTTP handler that has the
// capability of handling HTTP requests and gRPC requests that may require downgrading the response to gRPC-Web or gRPC-WebSocket.
func CreateDowngradingHandler(grpcSrv *grpc.Server, httpHandler http.Handler) http.Handler {
	// Only allow paths corresponding to gRPC methods that do not use client streaming for gRPC-Web.
	validGRPCWebPaths := make(map[string]struct{})

	for svcName, svcInfo := range grpcSrv.GetServiceInfo() {
		for _, methodInfo := range svcInfo.Methods {
			if methodInfo.IsClientStream {
				// Filter out client-streaming methods.
				continue
			}

			fullMethodName := fmt.Sprintf("/%s/%s", svcName, methodInfo.Name)
			validGRPCWebPaths[fullMethodName] = struct{}{}
		}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if isWebSocketUpgrade(req.Header) {
			handleGRPCWS(w, req, grpcSrv)
			return
		}

		if contentType, _ := stringutils.Split2(req.Header.Get("Content-Type"), "+"); contentType != "application/grpc" {
			// Non-gRPC request to the same port.
			httpHandler.ServeHTTP(w, req)
			return
		}

		handleGRPCWeb(w, req, validGRPCWebPaths, grpcSrv)
	})
}

func isWebSocketUpgrade(header http.Header) bool {
	return header.Get("Connection") == "Upgrade" &&
		header.Get("Upgrade") == "websocket" &&
		header.Get("Sec-Websocket-Protocol") == "grpc-ws"
}
