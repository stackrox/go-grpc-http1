// Copyright (c) 2020 StackRox Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License

package grpcweb

import (
	"bytes"
	"encoding/binary"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func frame(trailers bool, dataStr string) []byte {
	data := []byte(dataStr)
	buf := make([]byte, completeHeaderLen, completeHeaderLen+len(data))
	if trailers {
		buf[0] |= trailerMessageFlag
	}
	binary.BigEndian.PutUint32(buf[1:5], uint32(len(data)))
	buf = append(buf, data...)
	return buf
}

func concat(data ...[]byte) []byte {
	var allData []byte
	for _, d := range data {
		allData = append(allData, d...)
	}
	return allData
}

func stream(data ...[]byte) io.ReadCloser {
	return io.NopCloser(bytes.NewReader(concat(data...)))
}

func TestReadOK(t *testing.T) {
	messagePayload := concat(
		frame(false, "foo bar baz"),
		frame(false, "qux"),
	)

	input := stream(
		messagePayload,
		frame(true, "Trailer-Value: foo\r\nTrailer2-Value: bar\r\n"),
	)

	expectedTrailers := make(http.Header)
	expectedTrailers.Set("Trailer-Value", "foo")
	expectedTrailers.Set("Trailer2-Value", "bar")

	trailers := make(http.Header)

	webResponseReader := NewResponseReader(input, &trailers, nil)

	readData, err := io.ReadAll(webResponseReader)
	assert.NoError(t, err)
	assert.Equal(t, messagePayload, readData)

	assert.Equal(t, expectedTrailers, trailers)
}

func TestNoDataOK(t *testing.T) {
	input := stream()

	trailers := make(http.Header)

	webResponseReader := NewResponseReader(input, &trailers, nil)

	readData, err := io.ReadAll(webResponseReader)
	assert.NoError(t, err)
	assert.Empty(t, readData)
	assert.Empty(t, trailers)
}

func TestExtraDataError(t *testing.T) {
	messagePayload := concat(
		frame(false, "foo bar baz"),
		frame(false, "qux"),
	)

	input := stream(
		messagePayload,
		frame(true, "Trailer-Value: foo\r\nTrailer2-Value: bar\r\n"),
		[]byte("some data"),
	)

	expectedTrailers := make(http.Header)
	expectedTrailers.Set("Trailer-Value", "foo")
	expectedTrailers.Set("Trailer2-Value", "bar")

	trailers := make(http.Header)

	webResponseReader := NewResponseReader(input, &trailers, nil)

	readData, err := io.ReadAll(webResponseReader)
	assert.Error(t, err)
	assert.Equal(t, messagePayload, readData)
	assert.Equal(t, expectedTrailers, trailers)
}

func TestNoTrailersError(t *testing.T) {
	messagePayload := concat(
		frame(false, "foo bar baz"),
		frame(false, "qux"),
	)

	input := stream(messagePayload)

	trailers := make(http.Header)

	webResponseReader := NewResponseReader(input, &trailers, nil)

	readData, err := io.ReadAll(webResponseReader)
	assert.Error(t, err)
	assert.Equal(t, messagePayload, readData)
	assert.Empty(t, trailers)
}
