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

package integrationtests

import (
	"context"
	"io"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/examples/features/proto/echo"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

var (
	_ = echo.EchoServer(echoService{})
)

// echoService implements an echo server, which also sets headers and trailers.
// Given the 'ERROR:' keyword in the message or 'error' in the header, the call will trigger an error.
// This allows for testing for errors during various stages of the response.
type echoService struct{}

func (echoService) echoHeadersAndTrailers(ctx context.Context) error {
	md, _ := metadata.FromIncomingContext(ctx)

	if hdrEcho := md.Get("header-echo"); len(hdrEcho) > 0 {
		headers := metadata.MD{"header-echo-response": hdrEcho}
		if err := grpc.SetHeader(ctx, headers); err != nil {
			return err
		}
	}

	if trailerEcho := md.Get("trailer-echo"); len(trailerEcho) > 0 {
		headers := metadata.MD{"trailer-echo-response": trailerEcho}
		if err := grpc.SetTrailer(ctx, headers); err != nil {
			return err
		}
	}

	if errMsg := md.Get("error"); len(errMsg) > 0 {
		return status.Error(codes.InvalidArgument, errMsg[0])
	}

	return nil
}

func (s echoService) UnaryEcho(ctx context.Context, req *echo.EchoRequest) (*echo.EchoResponse, error) {
	if err := s.echoHeadersAndTrailers(ctx); err != nil {
		return nil, err
	}

	if strings.HasPrefix(req.GetMessage(), "ERROR:") {
		return nil, status.Error(codes.InvalidArgument, req.GetMessage()[6:])
	}

	return &echo.EchoResponse{
		Message: req.GetMessage(),
	}, nil
}

func (s echoService) ServerStreamingEcho(req *echo.EchoRequest, server echo.Echo_ServerStreamingEchoServer) error {
	if err := s.echoHeadersAndTrailers(server.Context()); err != nil {
		return err
	}

	lines := strings.Split(req.GetMessage(), "\n")

	for _, line := range lines {
		if strings.HasPrefix(line, "ERROR:") {
			return status.Error(codes.InvalidArgument, line[6:])
		}
		if line == "HEADERS" {
			if err := server.SendHeader(metadata.MD{}); err != nil {
				return err
			}
			continue
		}
		resp := &echo.EchoResponse{Message: line}
		if err := server.Send(resp); err != nil {
			return err
		}
	}

	return nil
}

func (s echoService) ClientStreamingEcho(server echo.Echo_ClientStreamingEchoServer) error {
	if err := s.echoHeadersAndTrailers(server.Context()); err != nil {
		return err
	}

	var msgs []string

	for {
		msg, err := server.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if strings.HasPrefix(msg.GetMessage(), "ERROR:") {
			return status.Error(codes.InvalidArgument, msg.GetMessage()[6:])
		}
		if msg.GetMessage() == "HEADERS" {
			if err := server.SendHeader(metadata.MD{}); err != nil {
				return err
			}
			continue
		}

		msgs = append(msgs, msg.GetMessage())
	}

	resp := &echo.EchoResponse{
		Message: strings.Join(msgs, "\n"),
	}

	return server.SendAndClose(resp)
}

func (s echoService) BidirectionalStreamingEcho(server echo.Echo_BidirectionalStreamingEchoServer) error {
	if err := s.echoHeadersAndTrailers(server.Context()); err != nil {
		return err
	}

	for {
		msg, err := server.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if strings.HasPrefix(msg.GetMessage(), "ERROR:") {
			return status.Error(codes.InvalidArgument, msg.GetMessage()[6:])
		}
		if msg.GetMessage() == "HEADERS" {
			if err := server.SendHeader(metadata.MD{}); err != nil {
				return err
			}
			continue
		}

		resp := &echo.EchoResponse{
			Message: msg.GetMessage(),
		}

		if err := server.Send(resp); err != nil {
			return err
		}
	}

	return nil
}
