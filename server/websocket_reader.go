package server

import (
	"context"
	"io"

	"github.com/pkg/errors"
	"golang.stackrox.io/grpc-http1/internal/grpcproto"
	"nhooyr.io/websocket"
)

// readResult stores the output from calls to (*wsReader).conn.Read
// to be sent along a channel.
type readResult struct {
	mt  websocket.MessageType
	msg []byte
	err error
}

// wsReader is an io.ReadCloser that wraps around a WebSocket's io.Reader.
type wsReader struct {
	ctx     context.Context
	conn    *websocket.Conn
	currMsg []byte

	// These are to prevent the WebSocket from closing due to
	// (*websocket.Conn).Read's context potentially expiring.
	// This can happen if Read waits indefinitely, which we prevent
	// by decoupling the (*websocket.Conn).Read and Read.
	readCtx       context.Context
	readCtxCancel context.CancelFunc
	readResultsC  chan readResult

	// Errors should be "sticky".
	err error
}

func newWebSocketReader(ctx context.Context, conn *websocket.Conn) io.ReadCloser {
	r := &wsReader{
		ctx:          ctx,
		conn:         conn,
		readResultsC: make(chan readResult),
	}
	r.readCtx, r.readCtxCancel = context.WithCancel(r.ctx)
	go r.readLoop()
	return r
}

// readLoop continuously read messages off the WebSocket connection and forwards them along the results channel.
func (r *wsReader) readLoop() {
	for {
		mt, msg, err := r.conn.Read(r.ctx)
		select {
		case r.readResultsC <- readResult{
			mt:  mt,
			msg: msg,
			err: err,
		}:
		case <-r.readCtx.Done():
			return
		}
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
	if len(r.currMsg) == 0 {
		select {
		case <-r.readCtx.Done():
			// CloseRead was called or the request's context expired.
			// This is typically done in an error-case, only.
			return 0, errors.Wrap(r.readCtx.Err(), "reading websocket message")
		case rr := <-r.readResultsC:
			if rr.err != nil {
				return 0, rr.err
			}
			if rr.mt != websocket.MessageBinary {
				return 0, errors.Errorf("incorrect message type; expected MessageBinary but got %v", rr.mt)
			}

			// Expect either an EOS message from the client or a valid data frame.
			// Headers are not expected to be handled here.

			if err := grpcproto.ValidateGRPCFrame(rr.msg); err != nil {
				return 0, err
			}
			if grpcproto.IsEndOfStream(rr.msg) {
				// This is where a connection without errors will terminate.
				return 0, io.EOF
			}
			if !grpcproto.IsDataFrame(rr.msg) {
				return 0, errors.Errorf("message is not a gRPC data frame")
			}

			r.currMsg = rr.msg
		}
	}

	n := copy(p, r.currMsg)
	r.currMsg = r.currMsg[n:]

	return n, nil
}

// Close signals the reader loops that we are no longer accepting messages.
func (r *wsReader) Close() error {
	// We cannot call (*websocket.Conn).CloseRead here. The WebSocket's closing handshake
	// may not have been called yet, so the client may not know to stop sending messages,
	// which may cause WebSocket errors.
	r.readCtxCancel()
	return nil
}
