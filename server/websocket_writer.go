package server

import (
	"io"
	"net/http"
)

// wsResponseWriter is a http.ResponseWriter to be used for WebSocket connections.
// (*wsResponseWriter).Close *must* be called when the struct is no longer needed.
type wsResponseWriter struct {
	io.Writer
	header            http.Header
	announcedTrailers []string
}

// newResponseWriter returns a new WebSocket response writer and its relative io.ReadCloser.
func newResponseWriter() (*wsResponseWriter, io.ReadCloser) {
	r, w := io.Pipe()
	rw := &wsResponseWriter{
		Writer: w,
		header: make(http.Header),
	}
	return rw, r
}

func (w *wsResponseWriter) Write(p []byte) (int, error) {
	// TODO
	return 0, nil
}

func (w *wsResponseWriter) Header() http.Header {
	return w.header
}

func (w *wsResponseWriter) WriteHeader(statusCode int) {
	// TODO
}

// Flush is a No-Op since the underlying writer is a PipeWriter, which does no internal buffering.
func (w *wsResponseWriter) Flush() {}

// Close sends over trailers for normal and Trailer-Only gRPC responses.
func (w *wsResponseWriter) Close() error {
	// TODO
	return nil
}
