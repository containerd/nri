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

	"github.com/stretchr/testify/require"
)

func TestParseEventMaskWhitespace(t *testing.T) {
	for _, event := range []string{"all", "pod", "podsandbox", "container", "RunPodSandbox"} {
		t.Run(event, func(t *testing.T) {
			want, err := ParseEventMask(event)
			require.NoError(t, err)
			for _, input := range []string{" " + event, event + " ", "\t" + event + "\n"} {
				got, err := ParseEventMask(input)
				require.NoError(t, err)
				require.Equal(t, want, got)
			}
		})
	}
	t.Run("comma separated groups", func(t *testing.T) {
		want, err := ParseEventMask("pod,container")
		require.NoError(t, err)
		got, err := ParseEventMask("pod, container")
		require.NoError(t, err)
		require.Equal(t, want, got)
	})
	t.Run("unknown events remain invalid", func(t *testing.T) {
		_, err := ParseEventMask(" pod, unknown ")
		require.Error(t, err)
	})
}
