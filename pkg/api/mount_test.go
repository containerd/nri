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
	"slices"
	"testing"
)

func TestMountCmp(t *testing.T) {
	base := &Mount{
		Destination: "/data",
		Type:        "bind",
		Source:      "/host/data",
		Options:     []string{"bind", "ro"},
	}

	tests := []struct {
		name  string
		other *Mount
		want  bool
	}{
		{
			name:  "same options",
			other: &Mount{Destination: "/data", Type: "bind", Source: "/host/data", Options: []string{"bind", "ro"}},
			want:  true,
		},
		{
			name:  "options order does not matter",
			other: &Mount{Destination: "/data", Type: "bind", Source: "/host/data", Options: []string{"ro", "bind"}},
			want:  true,
		},
		{
			name:  "different options are not equal",
			other: &Mount{Destination: "/data", Type: "bind", Source: "/host/data", Options: []string{"bind", "rw"}},
			want:  false,
		},
		{
			name:  "different option counts are not equal",
			other: &Mount{Destination: "/data", Type: "bind", Source: "/host/data", Options: []string{"bind"}},
			want:  false,
		},
		{
			name: "nil mount is not equal",
			want: false,
		},
		{
			name:  "different source is not equal",
			other: &Mount{Destination: "/data", Type: "bind", Source: "/other/data", Options: []string{"bind", "ro"}},
			want:  false,
		},
		{
			name:  "different destination is not equal",
			other: &Mount{Destination: "/other", Type: "bind", Source: "/host/data", Options: []string{"bind", "ro"}},
			want:  false,
		},
		{
			name:  "different type is not equal",
			other: &Mount{Destination: "/data", Type: "tmpfs", Source: "/host/data", Options: []string{"bind", "ro"}},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalOptions := slices.Clone(base.Options)
			var otherOptions []string
			if tt.other != nil {
				otherOptions = slices.Clone(tt.other.Options)
			}
			if got := base.Cmp(tt.other); got != tt.want {
				t.Errorf("Cmp() = %v, want %v", got, tt.want)
			}
			if !slices.Equal(base.Options, originalOptions) ||
				(tt.other != nil && !slices.Equal(tt.other.Options, otherOptions)) {
				t.Error("Cmp modified mount options")
			}
		})
	}
}
