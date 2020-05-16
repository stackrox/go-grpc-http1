package server

import (
	"context"
	"io"

	"github.com/pkg/errors"
	"golang.stackrox.io/grpc-http1/internal/grpcproto"
	"nhooyr.io/websocket"
)

// wsReader is an io.ReadCloser that wraps around a WebSocket's io.Reader.
type wsReader struct {
	ctx        context.Context
	conn       *websocket.Conn
	currOffset int
	currMsgLen int
	currMsg    []byte

	// Errors should be "sticky".
	err error
}

func newWebSocketReader(ctx context.Context, conn *websocket.Conn) io.ReadCloser {
	return &wsReader{
		ctx:  ctx,
		conn: conn,
	}
}

// Read reads from the WebSocket connection.
// Read assumes each WebSocket message is a gRPC message or metadata frame.
func (r *wsReader) Read(p []byte) (int, error) {
	var n int
	// Errors are "sticky", so if we've errored before, don't bother reading.
	if r.err == nil {
		n, r.err = r.doRead(p)
	}
	return n, r.err
}

func (r *wsReader) doRead(p []byte) (int, error) {
	if r.currOffset == r.currMsgLen {
		mt, msg, err := r.conn.Read(r.ctx)
		if err != nil {
			return 0, err
		}
		if mt != websocket.MessageBinary {
			return 0, errors.Errorf("incorrect message type; expected MessageBinary but got %v", mt)
		}

		// Expect either an EOS message from the client or a valid data frame.
		// Headers are not expected to be handled here.

		if err := grpcproto.ValidateGRPCFrame(msg); err != nil {
			return 0, err
		}
		if grpcproto.IsEndOfStream(msg) {
			return 0, io.EOF
		}
		if !grpcproto.IsDataFrame(msg) {
			return 0, errors.Errorf("message is not a gRPC data frame")
		}

		r.currOffset = 0
		r.currMsgLen = len(msg)
		r.currMsg = msg
	}

	n := copy(p, r.currMsg[r.currOffset:])
	r.currOffset += n

	return n, nil
}

// Close starts a go routine to ensure no more data is read from the WebSocket.
func (r *wsReader) Close() error {
	_ = r.conn.CloseRead(r.ctx)
	return nil
}
