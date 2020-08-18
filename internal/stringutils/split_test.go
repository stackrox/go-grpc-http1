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
