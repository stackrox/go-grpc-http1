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

// ShallowClone clones a slice, creating a new slice
// and copying the contents of the underlying array.
// If `in` is a nil slice, a nil slice is returned.
// If `in` is an empty slice, an empty slice is returned.
func ShallowClone[T any](in []T) []T {
	if in == nil {
		return nil
	}
	if len(in) == 0 {
		return []T{}
	}
	out := make([]T, len(in))
	copy(out, in)
	return out
}

// Find returns, given a slice and an element, the first index of elem in the slice, or -1 if the slice does
// not contain elem.
func Find[T comparable](slice []T, elem T) int {
	for i, sliceElem := range slice {
		if sliceElem == elem {
			return i
		}
	}
	return -1
}
