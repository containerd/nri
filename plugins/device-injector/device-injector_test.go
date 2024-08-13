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
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseDevices(t *testing.T) {
	type testCase struct {
		name        string
		annotations map[string]string
		result      []device
	}

	for _, tc := range []*testCase{
		{
			name: "no annotated devices",
			annotations: map[string]string{
				"foo": "bar",
			},
		},
		{
			name: "a single annotated device",
			annotations: map[string]string{
				"devices.nri.io/container.ctr0": `
- path: /dev/test-null
  type: c
  major: 1
  minor: 3
`,
			},
			result: []device{
				{
					Path:  "/dev/test-null",
					Type:  "c",
					Major: 1,
					Minor: 3,
				},
			},
		},
		{
			name: "multiple annotated devices",
			annotations: map[string]string{
				"devices.nri.io/container.ctr0": `
- path: /dev/test-null
  type: c
  major: 1
  minor: 3
- path: /dev/test-zero
  type: c
  major: 1
  minor: 5
`,
			},
			result: []device{
				{
					Path:  "/dev/test-null",
					Type:  "c",
					Major: 1,
					Minor: 3,
				},
				{
					Path:  "/dev/test-zero",
					Type:  "c",
					Major: 1,
					Minor: 5,
				},
			},
		},
		{
			name: "annotated devices for non-matching container name",
			annotations: map[string]string{
				"devices.nri.io/container.ctr1": `
- path: /dev/test-null
  type: c
  major: 1
  minor: 3
- path: /dev/test-zero
  type: c
  major: 1
  minor: 5
`,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			devices, err := parseDevices("ctr0", tc.annotations)
			require.Nil(t, err, "device parsing error")
			require.Equal(t, tc.result, devices, "parsed devices")
		})
	}
}

func TestParseCDIDevices(t *testing.T) {
	type testCase struct {
		name        string
		annotations map[string]string
		result      []string
	}

	for _, tc := range []*testCase{
		{
			name: "no annotated CDI devices",
			annotations: map[string]string{
				"foo": "bar",
			},
		},
		{
			name: "a single annotated CDI device",
			annotations: map[string]string{
				"cdi-devices.nri.io/container.ctr0": `
- vendor0.com/device=null
`,
			},
			result: []string{
				"vendor0.com/device=null",
			},
		},
		{
			name: "multiple annotated CDI devices",
			annotations: map[string]string{
				"cdi-devices.nri.io/container.ctr0": `
- vendor0.com/device=null
- vendor0.com/device=zero
`,
			},
			result: []string{
				"vendor0.com/device=null",
				"vendor0.com/device=zero",
			},
		},
		{
			name: "annotated CDI devices for non-matching container name",
			annotations: map[string]string{
				"cdi-devices.nri.io/container.ctr1": `
- vendor0.com/device=null
- vendor0.com/device=zero
`,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			devices, err := parseCDIDevices("ctr0", tc.annotations)
			require.Nil(t, err, "CDI device parsing error")
			require.Equal(t, tc.result, devices, "parsed CDI devices")
		})
	}
}

func TestParseMounts(t *testing.T) {
	type testCase struct {
		name        string
		annotations map[string]string
		result      []mount
	}

	for _, tc := range []*testCase{
		{
			name: "no annotated mounts",
			annotations: map[string]string{
				"foo": "bar",
			},
		},
		{
			name: "a single annotated mount",
			annotations: map[string]string{
				"mounts.nri.io/container.ctr0": `
- source: /foo
  destination: /host/foo
  type: bind
  options:
    - bind
    - ro
`,
			},
			result: []mount{
				{
					Source:      "/foo",
					Destination: "/host/foo",
					Type:        "bind",
					Options: []string{
						"bind",
						"ro",
					},
				},
			},
		},
		{
			name: "multiple annotated mounts",
			annotations: map[string]string{
				"mounts.nri.io/container.ctr0": `
- source: /foo
  destination: /host/foo
  type: bind
  options:
    - bind
- source: /bar
  destination: /host/bar
  type: bind
  options:
    - bind
    - ro
`,
			},
			result: []mount{
				{
					Source:      "/foo",
					Destination: "/host/foo",
					Type:        "bind",
					Options: []string{
						"bind",
					},
				},
				{
					Source:      "/bar",
					Destination: "/host/bar",
					Type:        "bind",
					Options: []string{
						"bind",
						"ro",
					},
				},
			},
		},
		{
			name: "annotated mounts for non-matching container name",
			annotations: map[string]string{
				"mounts.nri.io/container.ctr1": `
- source: /foo
  destination: /host/foo
  type: bind
  options:
    - bind
- source: /bar
  destination: /host/bar
  type: bind
  options:
    - bind
    - ro
`,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			devices, err := parseMounts("ctr0", tc.annotations)
			require.Nil(t, err, "mount parsing error")
			require.Equal(t, tc.result, devices, "parsed mounts")
		})
	}
}
