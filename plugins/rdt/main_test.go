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

	"github.com/containerd/nri/pkg/api"
)

func TestAdjustRdt(t *testing.T) {
	const containerName = "test-container"
	tcs := []struct {
		name        string
		annotations map[string]string
		expected    *api.LinuxRdt
		expectErr   bool
	}{
		{
			name:        "empty annotations",
			annotations: map[string]string{},
			expected:    nil,
		},
		{
			name: "adjust all fields",
			annotations: map[string]string{
				"closid.rdt.noderesource.dev/container." + containerName:           "foo",
				"schemata.rdt.noderesource.dev/container." + containerName:         "L3:0=f,MB:0=50",
				"enablemonitoring.rdt.noderesource.dev/container." + containerName: "true",
			},
			expected: &api.LinuxRdt{
				ClosId:           api.String("foo"),
				Schemata:         api.RepeatedString([]string{"L3:0=f", "MB:0=50"}),
				EnableMonitoring: api.Bool(true),
			},
		},
		{
			name: "unknown annotation",
			annotations: map[string]string{
				"unknown.rdt.noderesource.dev/container." + containerName: "foo",
			},
			expected: nil,
		},
		{
			name: "known and unknown annotations",
			annotations: map[string]string{
				"closid.rdt.noderesource.dev/container." + containerName:  "clos-1",
				"unknown.rdt.noderesource.dev/container." + containerName: "foo",
			},
			expected: &api.LinuxRdt{
				ClosId: api.String("clos-1"),
			},
		},
		{
			name: "wrong container name",
			annotations: map[string]string{
				"closid.rdt.noderesource.dev/container.wrong-name":         "clos-1",
				"schemata.rdt.noderesource.dev/container." + containerName: "L3:0=ff,MB:0=100",
			},
			expected: &api.LinuxRdt{
				Schemata: api.RepeatedString([]string{"L3:0=ff", "MB:0=100"}),
			},
		},
		{
			name: "invalid boolean",
			annotations: map[string]string{
				"enablemonitoring.rdt.noderesource.dev/container." + containerName: "not-a-boolean",
			},
			expected:  nil,
			expectErr: true,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			adjustment := &api.ContainerAdjustment{}
			err := adjustRdt(context.TODO(), adjustment, containerName, tc.annotations)
			if tc.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			expected := api.ContainerAdjustment{}
			if tc.expected != nil {
				expected.Linux = &api.LinuxContainerAdjustment{
					Rdt: tc.expected,
				}
			}
			assert.Equal(t, expected, *adjustment)
		})
	}
}
