package grpcproto

import (
	"encoding/binary"
)

const (
	// MessageHeaderLength is the length of a gRPC data frame message header.
	MessageHeaderLength = 5
)

// MessageFlags type represents the flags set in the header of a gRPC data frame.
type MessageFlags uint8

// ParseMessageHeader parses a byte slice into a gRPC data frame header.
func ParseMessageHeader(data [MessageHeaderLength]byte) (MessageFlags, uint32) {
	return MessageFlags(data[0]), binary.BigEndian.Uint32(data[1:])
}
