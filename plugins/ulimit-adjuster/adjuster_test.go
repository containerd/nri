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

package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	api "github.com/containerd/nri/pkg/api/v1beta1"
)

func TestParseAnnotations(t *testing.T) {
	tests := map[string]struct {
		container   string
		annotations map[string]string
		expected    []ulimit
		errStr      string
	}{
		"no-annotations": {
			container: "foo",
		},
		"unrelated-annotation": {
			container:   "foo",
			annotations: map[string]string{"bar": "baz"},
		},
		"one-valid": {
			container: "foo",
			annotations: map[string]string{
				"ulimits.noderesource.dev/container.foo": `
- type: RLIMIT_NOFILE
  soft: 123
  hard: 456
`},
			expected: []ulimit{{
				Type: "RLIMIT_NOFILE",
				Hard: 456,
				Soft: 123,
			}},
		},
		"multiple-valid": {
			container: "foo",
			annotations: map[string]string{
				"ulimits.noderesource.dev/container.foo": `
- type: RLIMIT_NOFILE
  soft: 123
  hard: 456
- type: RLIMIT_NPROC
  soft: 456
  hard: 789
`},
			expected: []ulimit{{
				Type: "RLIMIT_NOFILE",
				Hard: 456,
				Soft: 123,
			}, {
				Type: "RLIMIT_NPROC",
				Hard: 789,
				Soft: 456,
			}},
		},
		"missing-prefix": {
			container: "foo",
			annotations: map[string]string{
				"ulimits.noderesource.dev/container.foo": `
- type: AS
  soft: 123
  hard: 456
`},
			expected: []ulimit{{
				Type: "RLIMIT_AS",
				Hard: 456,
				Soft: 123,
			}},
		},
		"lower-case": {
			container: "foo",
			annotations: map[string]string{
				"ulimits.noderesource.dev/container.foo": `
- type: rlimit_core
  soft: 123
  hard: 456
`},
			expected: []ulimit{{
				Type: "RLIMIT_CORE",
				Hard: 456,
				Soft: 123,
			}},
		},
		"lower-case-missing-prefix": {
			container: "foo",
			annotations: map[string]string{
				"ulimits.noderesource.dev/container.foo": `
- type: cpu
  soft: 123
  hard: 456
`},
			expected: []ulimit{{
				Type: "RLIMIT_CPU",
				Hard: 456,
				Soft: 123,
			}},
		},
		"invalid-prefix": {
			container: "foo",
			annotations: map[string]string{
				"ulimits.noderesource.dev/container.foo": `
- type: ULIMIT_NOFILE
  soft: 123
  hard: 456
`},
			errStr: `failed to parse type: "ULIMIT_NOFILE"`,
		},
		"invalid-rlimit": {
			container: "foo",
			annotations: map[string]string{
				"ulimits.noderesource.dev/container.foo": `
- type: RLIMIT_FOO
  soft: 123
  hard: 456
`},
			errStr: `failed to parse type: "RLIMIT_FOO"`,
		},
		"one-invalid": {
			container: "foo",
			annotations: map[string]string{
				"ulimits.noderesource.dev/container.foo": `
- type: RLIMIT_NICE
  soft: 456
  hard: 789
- type: RLIMIT_BAR
  soft: 123
  hard: 456
`},
			errStr: `failed to parse type: "RLIMIT_BAR"`,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ulimits, err := parseUlimits(context.Background(), tc.container, tc.annotations)
			if tc.errStr != "" {
				assert.EqualError(t, err, tc.errStr)
				assert.Nil(t, ulimits)
			} else {
				assert.NoError(t, err)
				assert.EqualValues(t, tc.expected, ulimits)
			}
		})
	}
}

func TestAdjustUlimits(t *testing.T) {
	tests := map[string]struct {
		ulimits  []ulimit
		expected *api.ContainerAdjustment
		errStr   string
	}{
		"empty": {
			ulimits:  nil,
			expected: &api.ContainerAdjustment{},
		},
		"invalid-hard": {
			ulimits: []ulimit{{
				Type: "RLIMIT_NOFILE",
				Hard: 0,
				Soft: 100,
			}},
			errStr: `ulimit "RLIMIT_NOFILE" must have hard limit >= soft limit`,
		},
		"one": {
			ulimits: []ulimit{{
				Type: "RLIMIT_MEMLOCK",
				Hard: 100,
				Soft: 99,
			}},
			expected: &api.ContainerAdjustment{Rlimits: []*api.POSIXRlimit{{
				Type: "RLIMIT_MEMLOCK",
				Hard: 100,
				Soft: 99,
			}}},
		},
		"one-invalid": {
			ulimits: []ulimit{{
				Type: "RLIMIT_MEMLOCK",
				Hard: 100,
				Soft: 99,
			}, {
				Type: "RLIMIT_NOFILE",
				Hard: 0,
				Soft: 100,
			}},
			errStr: `ulimit "RLIMIT_NOFILE" must have hard limit >= soft limit`,
		},
		"multiple-valid": {
			ulimits: []ulimit{{
				Type: "RLIMIT_MEMLOCK",
				Hard: 100,
				Soft: 99,
			}, {
				Type: "RLIMIT_AS",
				Hard: 10,
				Soft: 0,
			}},
			expected: &api.ContainerAdjustment{Rlimits: []*api.POSIXRlimit{{
				Type: "RLIMIT_MEMLOCK",
				Hard: 100,
				Soft: 99,
			}, {
				Type: "RLIMIT_AS",
				Hard: 10,
				Soft: 0,
			}}},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			adjust, err := adjustUlimits(context.Background(), tc.ulimits)
			if tc.errStr != "" {
				assert.EqualError(t, err, tc.errStr)
				assert.Nil(t, adjust)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, adjust)
			}
		})
	}
}
