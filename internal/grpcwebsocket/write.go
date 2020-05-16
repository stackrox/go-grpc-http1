package grpcwebsocket

import (
	"bytes"
	"context"
	"io"

	"github.com/golang/glog"
	"golang.stackrox.io/grpc-http1/internal/grpcproto"
	"golang.stackrox.io/grpc-http1/internal/ioutils"
	"nhooyr.io/websocket"
)

// Write the contents of the reader along the WebSocket connection.
// This is done by sending each WebSocket message as a gRPC message frame.
// Each message frame is length-prefixed message, where the prefix is 5 bytes.
// gRPC request format is specified here: https://github.com/grpc/grpc/blob/master/doc/PROTOCOL-HTTP2.md.
func Write(ctx context.Context, conn *websocket.Conn, r io.Reader) error {
	var msg bytes.Buffer
	for {
		// Reset the message buffer to start with a clean slate.
		msg.Reset()
		// Read message header into the msg buffer.
		if _, err := ioutils.CopyNFull(&msg, r, grpcproto.MessageHeaderLength); err != nil {
			if err == io.EOF {
				// EOF here means the sender has no more messages to send.
				return nil
			}

			glog.Errorf("Malformed gRPC message when reading header: %v", err)
			return err
		}

		_, length, err := grpcproto.ParseMessageHeader(msg.Bytes())
		if err != nil {
			return err
		}

		// Read the rest of the message into the msg buffer.
		if n, err := io.CopyN(&msg, r, int64(length)); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				err = io.ErrUnexpectedEOF
				glog.Errorf("Malformed gRPC message: fewer than the announced %d bytes in payload: %d", length, n)
			} else {
				glog.Errorf("Unable to read gRPC message: %v", err)
			}
			return err
		}

		// Write the entire message frame along the WebSocket connection.
		if err := conn.Write(ctx, websocket.MessageBinary, msg.Bytes()); err != nil {
			glog.Errorf("Unable to write gRPC message: %v", err)
			return err
		}
	}
}
