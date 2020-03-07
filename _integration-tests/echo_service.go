package integrationtests

import (
	"context"
	"io"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/examples/features/proto/echo"
	"google.golang.org/grpc/metadata"
)

var (
	_ = echo.EchoServer(echoService{})
)

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

	return nil
}

func (s echoService) UnaryEcho(ctx context.Context, req *echo.EchoRequest) (*echo.EchoResponse, error) {
	if err := s.echoHeadersAndTrailers(ctx); err != nil {
		return nil, err
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

		resp := &echo.EchoResponse{
			Message: msg.GetMessage(),
		}

		if err := server.Send(resp); err != nil {
			return err
		}
	}

	return nil
}
