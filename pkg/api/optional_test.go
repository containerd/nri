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

func TestOptionalRepeatedString(t *testing.T) {
	nilSlice := []string(nil)
	tcs := []struct {
		name          string
		input         interface{}
		expectNil     bool
		expectedValue *[]string
	}{
		{
			name:          "nil input",
			input:         nil,
			expectNil:     true,
			expectedValue: nil,
		},
		{
			name:          "nil slice",
			input:         []string(nil),
			expectNil:     false,
			expectedValue: &nilSlice,
		},
		{
			name:          "empty slice",
			input:         []string{},
			expectNil:     false,
			expectedValue: &[]string{},
		},
		{
			name:          "non-empty slice",
			input:         []string{"value1", "value2"},
			expectNil:     false,
			expectedValue: &[]string{"value1", "value2"},
		},
		{
			name:          "pointer to slice",
			input:         &[]string{"value1", "value2"},
			expectNil:     false,
			expectedValue: &[]string{"value1", "value2"},
		},
		{
			name:          "non-slice type",
			input:         "not a slice",
			expectNil:     true,
			expectedValue: nil,
		},
		{
			name:          "pointer to nil",
			input:         (*[]string)(nil),
			expectNil:     true,
			expectedValue: nil,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			result := RepeatedString(tc.input)
			if tc.expectNil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
			}

			assert.Equal(t, tc.expectedValue, result.Get())
		})
	}
}
