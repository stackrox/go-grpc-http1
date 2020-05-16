package grpcproto

import (
	"bytes"

	"github.com/pkg/errors"
)

// IsDataFrame returns true if the message is a gRPC data frame.
// A data frame has its MSB unset.
func IsDataFrame(msg []byte) bool {
	return msg[0]&metadataMask == 0
}

// IsMetadataFrame returns true if the message is a gRPC metadata frame.
// A metadata frame has its MSB set.
func IsMetadataFrame(msg []byte) bool {
	return msg[0]&metadataMask != 0
}

// IsCompressed returns true if the message header sets the compression flag.
func IsCompressed(msg []byte) bool {
	return msg[0]&compressionMask != 0
}

// ValidateGRPCFrame ensures the message is a well-formed gRPC message.
// A well-formed message has at least a well-formed header and a length equal to the declared length.
func ValidateGRPCFrame(msg []byte) error {
	msgLen := len(msg)
	if msgLen < MessageHeaderLength {
		return errors.Errorf("Message length is less than the length of the header: %d < %d", msgLen, MessageHeaderLength)
	}
	_, length, err := ParseMessageHeader(msg[:MessageHeaderLength])
	if err != nil {
		// Cannot be a valid frame if the header errors out.
		return err
	}

	if msgLen != MessageHeaderLength+int(length) {
		return errors.Errorf("Declared message length (%d) does not equal actual message length (%d)", length, msgLen-MessageHeaderLength)
	}

	return nil
}

// IsEndOfStream returns true if the header sets the EOS flag and the message is empty.
func IsEndOfStream(msg []byte) bool {
	return bytes.Equal(msg, EndStreamHeader)
}
