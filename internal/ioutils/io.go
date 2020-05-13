package ioutils

import "io"

// CopyNFull does the same as io.CopyN, but it returns io.ErrUnexpectedEOF
// if CopyN returns io.EOF and the number of bytes written greater than zero.
func CopyNFull(dst io.Writer, src io.Reader, n int64) (int64, error) {
	written, err := io.CopyN(dst, src, n)
	if err == io.EOF && written != 0 {
		err = io.ErrUnexpectedEOF
	}

	return written, err
}
