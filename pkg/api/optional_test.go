/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRepeatedString(t *testing.T) {
	nilSlice := []string(nil)

	t.Run("nil slice", func(t *testing.T) {
		result := NewRepeatedString(nil)
		assert.NotNil(t, result)
		assert.Equal(t, &nilSlice, result.Get())
	})

	t.Run("empty slice", func(t *testing.T) {
		result := NewRepeatedString([]string{})
		assert.NotNil(t, result)
		assert.Equal(t, &[]string{}, result.Get())
	})

	t.Run("non-empty slice", func(t *testing.T) {
		result := NewRepeatedString([]string{"value1", "value2"})
		assert.NotNil(t, result)
		assert.Equal(t, &[]string{"value1", "value2"}, result.Get())
	})
}

func TestRepeatedStringPtr(t *testing.T) {
	t.Run("nil pointer", func(t *testing.T) {
		result := RepeatedStringPtr(nil)
		assert.Nil(t, result)
	})

	t.Run("pointer to slice", func(t *testing.T) {
		s := []string{"value1", "value2"}
		result := RepeatedStringPtr(&s)
		assert.NotNil(t, result)
		assert.Equal(t, &[]string{"value1", "value2"}, result.Get())
	})
}

func TestOptionalRepeatedStringCopy(t *testing.T) {
	t.Run("nil receiver", func(t *testing.T) {
		var o *OptionalRepeatedString
		assert.Nil(t, o.Copy())
	})

	t.Run("non-nil", func(t *testing.T) {
		o := NewRepeatedString([]string{"a", "b"})
		c := o.Copy()
		assert.Equal(t, o.Value, c.Value)
		// mutation of copy does not affect original
		c.Value[0] = "z"
		assert.Equal(t, "a", o.Value[0])
	})
}
