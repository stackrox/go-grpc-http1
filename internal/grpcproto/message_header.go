package grpcproto

import (
	"encoding/binary"

	"github.com/pkg/errors"
)

const (
	// MessageHeaderLength is the length of a gRPC data frame message header.
	MessageHeaderLength = 5

	// We differentiate between a gRPC message and metadata by the MSB.
	// 1 means it is metadata.
	metadataMask = 1 << 7
)

var (
	// EndStreamHeader is a gRPC frame header that indicates EOS.
	// This is ok because the MSB of the data frame header will never be used by
	// the gRPC protocol. gRPC-Web utilizes it to distinguish between normal data and trailers,
	// which implies we may also use it for our own purposes.
	// We use it to indicate that the stream is complete.
	EndStreamHeader = []byte{metadataMask, 0, 0, 0, 0}
)

// MessageFlags type represents the flags set in the header of a gRPC data frame.
type MessageFlags uint8

// ParseMessageHeader parses a byte slice into a gRPC data frame header.
func ParseMessageHeader(header []byte) (MessageFlags, uint32, error) {
	if len(header) != MessageHeaderLength {
		return 0, 0, errors.Errorf("gRPC message header must be %d bytes, but got %d", MessageHeaderLength, len(header))
	}
	return MessageFlags(header[0]), binary.BigEndian.Uint32(header[1:MessageHeaderLength]), nil
}