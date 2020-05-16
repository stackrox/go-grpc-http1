package stringutils

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSplit2(t *testing.T) {
	for _, testCase := range []struct {
		s        string
		sep      string
		expected []string
	}{
		{"Helo", "l", []string{"He", "o"}},
		{"Hello", "l", []string{"He", "lo"}},
		{"Hello", "ll", []string{"He", "o"}},
		{"", "a", []string{"", ""}},
	} {
		c := testCase
		t.Run(fmt.Sprintf("%+v", c), func(t *testing.T) {
			first, second := Split2(c.s, c.sep)
			assert.Equal(t, c.expected, []string{first, second})
		})
	}
}
