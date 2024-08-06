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
	} {
		t.Run(tc.name, func(t *testing.T) {
			devices, err := parseCDIDevices("ctr0", tc.annotations)
			require.Nil(t, err, "CDI device parsing error")
			require.Equal(t, tc.result, devices, "parsed CDI devices")
		})
	}
}
