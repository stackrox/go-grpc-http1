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

package sliceutils

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDifference(t *testing.T) {
	cases := []struct {
		slice1, slice2 []string
		expectedSlice  []string
	}{
		{
			slice1:        []string{},
			slice2:        []string{},
			expectedSlice: []string{},
		},
		{
			slice1:        []string{"A"},
			slice2:        []string{},
			expectedSlice: []string{"A"},
		},
		{
			slice1:        []string{},
			slice2:        []string{"A"},
			expectedSlice: []string{},
		},
		{
			slice1:        []string{"A"},
			slice2:        []string{"B"},
			expectedSlice: []string{"A"},
		},
		{
			slice1:        []string{"A"},
			slice2:        []string{"A"},
			expectedSlice: nil,
		},
		{
			slice1:        []string{"A", "B", "C"},
			slice2:        []string{"B"},
			expectedSlice: []string{"A", "C"},
		},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%s - %s", strings.Join(c.slice1, " "), strings.Join(c.slice2, " ")), func(t *testing.T) {
			assert.Equal(t, c.expectedSlice, StringDifference(c.slice1, c.slice2))
		})
	}
}

func TestUnion(t *testing.T) {
	cases := []struct {
		slice1, slice2 []string
		expectedSlice  []string
	}{
		{
			slice1:        []string{},
			slice2:        []string{},
			expectedSlice: []string{},
		},
		{
			slice1:        []string{"A"},
			slice2:        []string{},
			expectedSlice: []string{"A"},
		},
		{
			slice1:        []string{},
			slice2:        []string{"A"},
			expectedSlice: []string{"A"},
		},
		{
			slice1:        []string{"A"},
			slice2:        []string{"B"},
			expectedSlice: []string{"A", "B"},
		},
		{
			slice1:        []string{"A"},
			slice2:        []string{"A"},
			expectedSlice: []string{"A"},
		},
		{
			slice1:        []string{"A", "B", "C"},
			slice2:        []string{"B"},
			expectedSlice: []string{"A", "B", "C"},
		},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%s - %s", strings.Join(c.slice1, " "), strings.Join(c.slice2, " ")), func(t *testing.T) {
			assert.Equal(t, c.expectedSlice, StringUnion(c.slice1, c.slice2))
		})
	}
}
