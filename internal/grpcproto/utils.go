package grpcproto

import (
	"bytes"
)

// IsValidMessageFrame returns true if the message is a valid gRPC message frame.
// A valid frame is a length-prefixed message whose announced length and actual length are equal,
// and whose MSB is 0.
func IsValidMessageFrame(msg []byte) bool {
	msgLen := len(msg)
	if msgLen < MessageHeaderLength {
		return false
	}
	flags, length, err := ParseMessageHeader(msg[:MessageHeaderLength])
	if err != nil {
		// Cannot be a valid frame if the header errors out.
		return false
	}
	// A message frame has a 0 as the MSB and has a declared length equal to the length of the message.
	return flags&metadataMask == 0 && msgLen == MessageHeaderLength+int(length)
}

// IsEndOfStream returns true if the header sets the EOS flag and the message is empty.
func IsEndOfStream(msg []byte) bool {
	return bytes.Equal(msg, EndStreamHeader)
}
