package grpcweb

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGRPCWebOnlyHeaderNameIsCanonical(t *testing.T) {
	assert.Equal(t, GRPCWebOnlyHeader, http.CanonicalHeaderKey(GRPCWebOnlyHeader))
}
