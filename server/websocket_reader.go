package server

import (
	"context"
	"io"

	"github.com/golang/glog"
	"github.com/pkg/errors"
	"golang.stackrox.io/grpc-http1/internal/grpcproto"
	"nhooyr.io/websocket"
)

// wsReader is an io.ReadCloser that wraps a WebSocket's io.Reader.
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
	// TODO: Remove log. Only here for debugging purposes.
	glog.Errorf("Read Called with buffer of length %d", len(p))
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
			return 0, errors.Errorf("incorrect message type; expected MessageBinary but got %s", mt)
		}

		// TODO: remove log. Here for debugging for now.
		glog.Errorln(string(msg))

		if grpcproto.IsEndOfStream(msg) {
			return 0, io.EOF
		}
		if !grpcproto.IsValidMessageFrame(msg) {
			return 0, errors.Errorf("message is an invalid gRPC message frame")
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
