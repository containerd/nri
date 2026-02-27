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

package version

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStripGitSuffix(t *testing.T) {
	for _, tc := range []*struct {
		name     string
		version  string
		expected string
	}{
		{
			name:     "no/empty version",
			version:  "",
			expected: "",
		},
		{
			name:     "major.minor.patch version without suffix",
			version:  "v1.2.3",
			expected: "v1.2.3",
		},
		{
			name:     "major.minor version without suffix",
			version:  "v1.2",
			expected: "v1.2",
		},
		{
			name:     "major version without suffix",
			version:  "v1",
			expected: "v1",
		},
		{
			name:     "prerelease version without git suffix",
			version:  "v1.2.3-alpha.1",
			expected: "v1.2.3-alpha.1",
		},
		{
			name:     "working tree version with git suffix",
			version:  "v0.11.0-37-g41b9c58",
			expected: "v0.11.0",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stripped := stripGitSuffix(tc.version)
			require.Equal(t, tc.expected, stripped, "stripGitSuffix(%q) = %q, want %q",
				tc.version, stripped, tc.expected)
		})
	}
}

func TestFindClosestMatch(t *testing.T) {
	candidates := []string{
		"v0.0.10",
		"v0.1.0",
		"v0.1.1",
		"v0.2.0",
		"v0.10.9",
		"v1.0",
		"v1.2.3",
		"v2",
		"v2.0.5",
		"v2.1",
		"v2.1.2",
		"v3.0.5",
		"v4",
	}
	for _, tc := range []*struct {
		name       string
		version    string
		candidates []string
		expected   string
	}{
		{
			name:       "no candidates",
			version:    "v1.2.3",
			candidates: []string{},
			expected:   "",
		},
		{
			name:       "no closest match",
			version:    "v0.0.1",
			candidates: candidates,
			expected:   "",
		},
		{
			name:       "exact match",
			version:    "v1.2.3",
			candidates: candidates,
			expected:   "v1.2.3",
		},
		{
			name:       "closest match has no patch",
			version:    "v1.1.1",
			candidates: candidates,
			expected:   "v1.0",
		},
		{
			name:       "closest match has patch",
			version:    "v2.0.6",
			candidates: candidates,
			expected:   "v2.0.5",
		},
		{
			name:       "closest match has no minor",
			version:    "v2.0.1",
			candidates: candidates,
			expected:   "v2",
		},
		{
			name:       "version has no patch",
			version:    "v3.1",
			candidates: candidates,
			expected:   "v3.0.5",
		},
		{
			name:       "version has no minor",
			version:    "v5",
			candidates: candidates,
			expected:   "v4",
		},
		{
			name:       "closest match for a pre-release version",
			version:    "v1.2.4-alpha.1",
			candidates: candidates,
			expected:   "v1.2.3",
		},
		{
			name:       "closest match for a working tree version",
			version:    "v0.11.0-37-g41b9c58",
			candidates: candidates,
			expected:   "v0.10.9",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			match := FindClosestMatch(tc.version, tc.candidates)
			require.Equal(t, tc.expected, match, "FindClosestMatch(%q, %v) = %q, want %q",
				tc.version, tc.candidates, match, tc.expected)
		})
	}
}
