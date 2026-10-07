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
		{
			name:     "prerelease version",
			version:  "v1.2.3-45",
			expected: "v1.2.3-45",
		},
		{
			name:     "prerelease version, looking more like a git suffix",
			version:  "v1.2.3-45-g",
			expected: "v1.2.3-45-g",
		},
		{
			name:     "another prerelease version",
			version:  "v1.2.3-45-gz",
			expected: "v1.2.3-45-gz",
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
		{
			name:       "another closest match for a working tree version",
			version:    "v0.10.9-37-g41b9c58",
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

func TestCompareVersion(t *testing.T) {
	for _, tc := range []struct {
		name string
		a    string
		b    string
		want int
	}{
		{
			name: "equal semvers",
			a:    "v1.2.3",
			b:    "v1.2.3",
			want: 0,
		},
		{
			name: "semver vs semver with pre-release",
			a:    "v1.2.3",
			b:    "v1.2.3-alpha.1",
			want: 1,
		},
		{
			name: "semver with pre-release vs semver",
			a:    "v1.2.3-alpha.1",
			b:    "v1.2.3",
			want: -1,
		},
		{
			name: "semver with build metadata vs semver",
			a:    "v1.2.3+build",
			b:    "v1.2.3",
			want: 0,
		},
		{
			name: "semver with pre-release and build metadata vs semver",
			a:    "v1.2.3-alpha.1+build",
			b:    "v1.2.3-alpha.1",
			want: 0,
		},
		{
			name: "compare two semvers with numeric identifiers in their pre-release",
			a:    "v1.2.3-alpha.10",
			b:    "v1.2.3-alpha.2",
			want: 1,
		},
		{
			name: "shorter pre-release is lower than a longer one with the same prefix",
			a:    "v1.2.3-alpha",
			b:    "v1.2.3-alpha.1",
			want: -1,
		},
		{
			name: "numeric pre-release identifier is lower than an alphanumeric one",
			a:    "v1.2.3-alpha.1",
			b:    "v1.2.3-alpha.beta",
			want: -1,
		},
		{
			name: "alphanumeric pre-release identifier is higher than a numeric one",
			a:    "v1.2.3-alpha.beta",
			b:    "v1.2.3-alpha.1",
			want: 1,
		},
		{
			name: "trailing pre-release separator is invalid",
			a:    "v1.2.3-",
			b:    "v1.2.3",
			want: -1,
		},
		{
			name: "missing v prefix is invalid",
			a:    "1.2.3",
			b:    "v0.0.1",
			want: -1,
		},
		{
			name: "leading zero is invalid",
			a:    "v1.02.3",
			b:    "v1.2.3",
			want: -1,
		},
		{
			name: "leading zero in numeric pre-release identifier is invalid",
			a:    "v1.2.3-alpha.01",
			b:    "v1.2.3-alpha.1",
			want: -1,
		},
		{
			name: "empty pre-release identifier is invalid",
			a:    "v1.2.3-alpha..1",
			b:    "v1.2.3-alpha.1",
			want: -1,
		},
		{
			name: "extra build separator is invalid",
			a:    "v1.2.3+a+b",
			b:    "v1.2.3",
			want: -1,
		},
		{
			name: "empty build metadata is invalid",
			a:    "v1.2.3+",
			b:    "v1.2.3",
			want: -1,
		},
		{
			name: "too many components is invalid",
			a:    "v1.2.3.4",
			b:    "v1.2.3",
			want: -1,
		},
		{
			name: "shorthand with pre-release is invalid",
			a:    "v1-alpha",
			b:    "v1",
			want: -1,
		},
		{
			name: "valid is greater than invalid",
			a:    "v0.0.1",
			b:    "bogus",
			want: 1,
		},
		{
			name: "two invalid versions are equal",
			a:    "bogus",
			b:    "v1.2.3-",
			want: 0,
		},
		{
			name: "shorthand versions are valid",
			a:    "v1.2",
			b:    "v1.2.0",
			want: 0,
		},
		{
			name: "major-only shorthand version equals full version",
			a:    "v1",
			b:    "v1.0.0",
			want: 0,
		},
		{
			name: "can support versions with very large numbers",
			a:    "v999999999999999999999.0.0",
			b:    "v1.2.3",
			want: 1,
		},
		// the following test cases are taken from golang.org/x/mod/semver
		// this is done to make sure that local semver implementation
		// is still aligned with the x/mod/semver one.
		// https://cs.opensource.google/go/x/mod/+/master:semver/semver_test.go;l=19
		{
			name: "bad and v1-alpha.beta.gamma both invalid",
			a:    "bad",
			b:    "v1-alpha.beta.gamma",
			want: 0,
		},
		{
			name: "v1-alpha.beta.gamma and v1-pre both invalid",
			a:    "v1-alpha.beta.gamma",
			b:    "v1-pre",
			want: 0,
		},
		{
			name: "v1-pre and v1+meta both invalid",
			a:    "v1-pre",
			b:    "v1+meta",
			want: 0,
		},
		{
			name: "v1+meta and v1-pre+meta both invalid",
			a:    "v1+meta",
			b:    "v1-pre+meta",
			want: 0,
		},
		{
			name: "v1-pre+meta and v1.2-pre both invalid",
			a:    "v1-pre+meta",
			b:    "v1.2-pre",
			want: 0,
		},
		{
			name: "v1.2-pre and v1.2+meta both invalid",
			a:    "v1.2-pre",
			b:    "v1.2+meta",
			want: 0,
		},
		{
			name: "v1.2+meta and v1.2-pre+meta both invalid",
			a:    "v1.2+meta",
			b:    "v1.2-pre+meta",
			want: 0,
		},
		{
			name: "invalid v1.2-pre+meta is less than v1.0.0-alpha",
			a:    "v1.2-pre+meta",
			b:    "v1.0.0-alpha",
			want: -1,
		},
		{
			name: "v1.0.0-alpha is less than v1.0.0-alpha.1",
			a:    "v1.0.0-alpha",
			b:    "v1.0.0-alpha.1",
			want: -1,
		},
		{
			name: "v1.0.0-alpha.1 is less than v1.0.0-alpha.beta",
			a:    "v1.0.0-alpha.1",
			b:    "v1.0.0-alpha.beta",
			want: -1,
		},
		{
			name: "v1.0.0-alpha.beta is less than v1.0.0-beta",
			a:    "v1.0.0-alpha.beta",
			b:    "v1.0.0-beta",
			want: -1,
		},
		{
			name: "v1.0.0-beta is less than v1.0.0-beta.2",
			a:    "v1.0.0-beta",
			b:    "v1.0.0-beta.2",
			want: -1,
		},
		{
			name: "v1.0.0-beta.2 is less than v1.0.0-beta.11",
			a:    "v1.0.0-beta.2",
			b:    "v1.0.0-beta.11",
			want: -1,
		},
		{
			name: "v1.0.0-beta.11 is less than v1.0.0-rc.1",
			a:    "v1.0.0-beta.11",
			b:    "v1.0.0-rc.1",
			want: -1,
		},
		{
			name: "v1.0.0-rc.1 is less than v1",
			a:    "v1.0.0-rc.1",
			b:    "v1",
			want: -1,
		},
		{
			name: "v1 equals v1.0",
			a:    "v1",
			b:    "v1.0",
			want: 0,
		},
		{
			name: "v1.0 equals v1.0.0",
			a:    "v1.0",
			b:    "v1.0.0",
			want: 0,
		},
		{
			name: "v1.0.0 is less than v1.2",
			a:    "v1.0.0",
			b:    "v1.2",
			want: -1,
		},
		{
			name: "v1.2 equals v1.2.0",
			a:    "v1.2",
			b:    "v1.2.0",
			want: 0,
		},
		{
			name: "v1.2.0 is less than v1.2.3-456",
			a:    "v1.2.0",
			b:    "v1.2.3-456",
			want: -1,
		},
		{
			name: "v1.2.3-456 is less than v1.2.3-456.789",
			a:    "v1.2.3-456",
			b:    "v1.2.3-456.789",
			want: -1,
		},
		{
			name: "v1.2.3-456.789 is less than v1.2.3-456-789",
			a:    "v1.2.3-456.789",
			b:    "v1.2.3-456-789",
			want: -1,
		},
		{
			name: "v1.2.3-456-789 is less than v1.2.3-456a",
			a:    "v1.2.3-456-789",
			b:    "v1.2.3-456a",
			want: -1,
		},
		{
			name: "v1.2.3-456a is less than v1.2.3-pre",
			a:    "v1.2.3-456a",
			b:    "v1.2.3-pre",
			want: -1,
		},
		{
			name: "v1.2.3-pre equals v1.2.3-pre+meta",
			a:    "v1.2.3-pre",
			b:    "v1.2.3-pre+meta",
			want: 0,
		},
		{
			name: "v1.2.3-pre+meta is less than v1.2.3-pre.1",
			a:    "v1.2.3-pre+meta",
			b:    "v1.2.3-pre.1",
			want: -1,
		},
		{
			name: "v1.2.3-pre.1 is less than v1.2.3-zzz",
			a:    "v1.2.3-pre.1",
			b:    "v1.2.3-zzz",
			want: -1,
		},
		{
			name: "v1.2.3-zzz is less than v1.2.3",
			a:    "v1.2.3-zzz",
			b:    "v1.2.3",
			want: -1,
		},
		{
			name: "v1.2.3 equals v1.2.3+meta",
			a:    "v1.2.3",
			b:    "v1.2.3+meta",
			want: 0,
		},
		{
			name: "v1.2.3+meta equals v1.2.3+meta-pre",
			a:    "v1.2.3+meta",
			b:    "v1.2.3+meta-pre",
			want: 0,
		},
		{
			name: "v1.2.3+meta-pre equals v1.2.3+meta-pre.sha.256a",
			a:    "v1.2.3+meta-pre",
			b:    "v1.2.3+meta-pre.sha.256a",
			want: 0,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, compareVersion(tc.a, tc.b))
		})
	}
}
