// Copyright (c) 2022 StackRox Inc.
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

package sliceutils

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShallowClone(t *testing.T) {
	cases := []struct {
		slice         []string
		expectedSlice []string
	}{
		{
			slice:         nil,
			expectedSlice: nil,
		},
		{
			slice:         []string{},
			expectedSlice: []string{},
		},
		{
			slice:         []string{"A", "B", "C"},
			expectedSlice: []string{"A", "B", "C"},
		},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%s - %s", strings.Join(c.slice, " "), strings.Join(c.expectedSlice, " ")), func(t *testing.T) {
			assert.Equal(t, c.expectedSlice, ShallowClone(c.slice))
		})
	}
}

func TestFind(t *testing.T) {
	cases := []struct {
		slice         []string
		elem          string
		expectedIndex int
	}{
		{
			slice:         nil,
			elem:          "A",
			expectedIndex: -1,
		},
		{
			slice:         []string{},
			elem:          "A",
			expectedIndex: -1,
		},
		{
			slice:         []string{"A", "B", "C"},
			elem:          "",
			expectedIndex: -1,
		},
		{
			slice:         []string{"A", "B", "C"},
			elem:          "D",
			expectedIndex: -1,
		},
		{
			slice:         []string{"A", "B", "C"},
			elem:          "B",
			expectedIndex: 1,
		},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%s in %s - %d", c.elem, strings.Join(c.slice, " "), c.expectedIndex), func(t *testing.T) {
			assert.Equal(t, c.expectedIndex, Find(c.slice, c.elem))
		})
	}
}
