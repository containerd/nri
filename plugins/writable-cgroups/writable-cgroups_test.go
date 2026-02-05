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
	"fmt"
	"os"
	"sort"
	"testing"

	"github.com/containerd/nri/pkg/api"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckHostNsDelegate(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name:     "valid nsdelegate in VFSOptions",
			content:  "36 35 98:0 / /sys/fs/cgroup rw,nosuid,nodev,noexec,relatime - cgroup2 cgroup2 rw,nsdelegate\n",
			expected: true,
		},
		{
			name:     "missing nsdelegate",
			content:  "36 35 98:0 / /sys/fs/cgroup rw,nosuid,nodev,noexec,relatime - cgroup2 cgroup2 rw\n",
			expected: false,
		},
		{
			name:     "wrong mountpoint",
			content:  "36 35 98:0 / /other/path rw,nosuid,nodev,noexec,relatime - cgroup2 cgroup2 rw,nsdelegate\n",
			expected: false,
		},
		{
			name:     "mixed content",
			content:  "35 24 0:31 / /run rw,nosuid,nodev,relatime - tmpfs tmpfs rw,mode=755\n36 35 98:0 / /sys/fs/cgroup rw,nosuid,nodev,noexec,relatime - cgroup2 cgroup2 rw,nsdelegate\n",
			expected: true,
		},
		{
			name:     "no cgroup2",
			content:  "35 24 0:31 / /run rw,nosuid,nodev,relatime - tmpfs tmpfs rw,mode=755\n",
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpfile, err := os.CreateTemp("", "mountinfo")
			require.NoError(t, err)
			defer os.Remove(tmpfile.Name())

			_, err = tmpfile.WriteString(tc.content)
			require.NoError(t, err)
			err = tmpfile.Close()
			require.NoError(t, err)

			actual, err := checkHostNsDelegate(tmpfile.Name())
			require.NoError(t, err)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestWritableCgroupsPlugin(t *testing.T) {
	log = logrus.StandardLogger()
	p := &plugin{}

	roMountNoNsDelegate := &api.Mount{
		Destination: CgroupsMount,
		Type:        "cgroup",
		Source:      "cgroup",
		Options:     []string{"ro", "nosuid", "noexec", "nodev"},
	}
	roMount := &api.Mount{
		Destination: CgroupsMount,
		Type:        "cgroup",
		Source:      "cgroup",
		Options:     []string{"ro", "nosuid", "noexec", "nodev", "nsdelegate"},
	}
	rwMount := &api.Mount{
		Destination: CgroupsMount,
		Type:        "cgroup",
		Source:      "cgroup",
		Options:     []string{"rw", "nosuid", "noexec", "nodev", "nsdelegate"},
	}
	noRoRwMount := &api.Mount{
		Destination: CgroupsMount,
		Type:        "cgroup",
		Source:      "cgroup",
		Options:     []string{"nosuid", "noexec", "nodev", "nsdelegate"},
	}

	expectedAdjustWithRw := &api.ContainerAdjustment{}
	expectedAdjustWithRw.AddMount(&api.Mount{
		Destination: CgroupsMount,
		Type:        "cgroup",
		Source:      "cgroup",
		Options:     []string{"nosuid", "noexec", "nodev", "nsdelegate", "rw"},
	})
	expectedAdjustWithRw.RemoveMount(CgroupsMount)

	testCases := []struct {
		name           string
		pod            *api.PodSandbox
		container      *api.Container
		hostNsDelegate bool
		expectedAdjust *api.ContainerAdjustment
		expectedUpdate []*api.ContainerUpdate
		expectedErr    error
	}{
		{
			name:           "nil pod",
			pod:            nil,
			container:      &api.Container{Name: "test-container"},
			hostNsDelegate: true,
			expectedAdjust: nil,
			expectedErr:    nil,
		},
		{
			name:           "nil container",
			pod:            &api.PodSandbox{Name: "test-pod"},
			container:      nil,
			hostNsDelegate: true,
			expectedAdjust: nil,
			expectedErr:    nil,
		},
		{
			name: "no annotation",
			pod: &api.PodSandbox{
				Name:        "test-pod",
				Namespace:   "test-ns",
				Annotations: map[string]string{},
			},
			container: &api.Container{
				Name:   "test-container",
				Mounts: []*api.Mount{roMount},
			},
			hostNsDelegate: true,
			expectedAdjust: nil,
			expectedErr:    nil,
		},
		{
			name: "annotation disabled",
			pod: &api.PodSandbox{
				Name:      "test-pod",
				Namespace: "test-ns",
				Annotations: map[string]string{
					WritableCgroupsAnnotation: "false",
				},
			},
			container: &api.Container{
				Name:   "test-container",
				Mounts: []*api.Mount{roMount},
			},
			hostNsDelegate: true,
			expectedAdjust: nil,
			expectedErr:    nil,
		},
		{
			name: "annotation enabled, no cgroup mount",
			pod: &api.PodSandbox{
				Name:      "test-pod",
				Namespace: "test-ns",
				Annotations: map[string]string{
					WritableCgroupsAnnotation: "true",
				},
			},
			container: &api.Container{
				Name:   "test-container",
				Mounts: []*api.Mount{},
			},
			hostNsDelegate: true,
			expectedAdjust: nil,
			expectedErr:    fmt.Errorf("no /sys/fs/cgroup mount found to modify, this is unexpected"),
		},
		{
			name: "annotation enabled, cgroup mount is ro",
			pod: &api.PodSandbox{
				Name:      "test-pod",
				Namespace: "test-ns",
				Annotations: map[string]string{
					WritableCgroupsAnnotation: "true",
				},
			},
			container: &api.Container{
				Name:   "test-container",
				Mounts: []*api.Mount{roMount},
			},
			hostNsDelegate: true,
			expectedAdjust: expectedAdjustWithRw,
			expectedErr:    nil,
		},
		{
			name: "annotation enabled, cgroup mount is already rw",
			pod: &api.PodSandbox{
				Name:      "test-pod",
				Namespace: "test-ns",
				Annotations: map[string]string{
					WritableCgroupsAnnotation: "true",
				},
			},
			container: &api.Container{
				Name:   "test-container",
				Mounts: []*api.Mount{rwMount},
			},
			hostNsDelegate: true,
			expectedAdjust: nil,
			expectedErr:    nil,
		},
		{
			name: "annotation enabled, cgroup mount has no ro/rw",
			pod: &api.PodSandbox{
				Name:      "test-pod",
				Namespace: "test-ns",
				Annotations: map[string]string{
					WritableCgroupsAnnotation: "true",
				},
			},
			container: &api.Container{
				Name:   "test-container",
				Mounts: []*api.Mount{noRoRwMount},
			},
			hostNsDelegate: true,
			expectedAdjust: expectedAdjustWithRw,
			expectedErr:    nil,
		},
		{
			name: "per-container annotation enabled",
			pod: &api.PodSandbox{
				Name:      "test-pod",
				Namespace: "test-ns",
				Annotations: map[string]string{
					fmt.Sprintf("%s.container.test-container", WritableCgroupsAnnotation): "true",
				},
			},
			container: &api.Container{
				Name:   "test-container",
				Mounts: []*api.Mount{roMount},
			},
			hostNsDelegate: true,
			expectedAdjust: expectedAdjustWithRw,
			expectedErr:    nil,
		},
		{
			name: "per-container annotation disabled, pod annotation enabled",
			pod: &api.PodSandbox{
				Name:      "test-pod",
				Namespace: "test-ns",
				Annotations: map[string]string{
					WritableCgroupsAnnotation: "true",
					fmt.Sprintf("%s.container.test-container", WritableCgroupsAnnotation): "false",
				},
			},
			container: &api.Container{
				Name:   "test-container",
				Mounts: []*api.Mount{roMount},
			},
			hostNsDelegate: true,
			expectedAdjust: nil,
			expectedErr:    nil,
		},
		{
			name: "another container has annotation enabled",
			pod: &api.PodSandbox{
				Name:      "test-pod",
				Namespace: "test-ns",
				Annotations: map[string]string{
					fmt.Sprintf("%s.container.another-container", WritableCgroupsAnnotation): "true",
				},
			},
			container: &api.Container{
				Name:   "test-container",
				Mounts: []*api.Mount{roMount},
			},
			hostNsDelegate: true,
			expectedAdjust: nil,
			expectedErr:    nil,
		},
		{
			name: "annotation enabled but host missing nsdelegate",
			pod: &api.PodSandbox{
				Name:      "test-pod",
				Namespace: "test-ns",
				Annotations: map[string]string{
					WritableCgroupsAnnotation: "true",
				},
			},
			container: &api.Container{
				Name: "test-container",
				// In this case, host doesn't have nsdelegate, so container mount likely won't either.
				Mounts: []*api.Mount{roMountNoNsDelegate},
			},
			hostNsDelegate: false,
			expectedAdjust: nil, // Should ignore request.
			expectedErr:    nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			p.hostNsDelegate = tc.hostNsDelegate
			adjust, update, err := p.CreateContainer(context.Background(), tc.pod, tc.container)

			if tc.expectedErr != nil {
				assert.Error(t, err)
				assert.Equal(t, tc.expectedErr.Error(), err.Error())
			} else {
				assert.NoError(t, err)
			}

			if tc.expectedAdjust != nil && len(tc.expectedAdjust.GetMounts()) > 0 {
				// Sort options to avoid flaky tests.
				if len(adjust.GetMounts()) > 0 {
					sort.Strings(adjust.GetMounts()[0].Options)
				}
				if len(tc.expectedAdjust.GetMounts()) > 0 {
					sort.Strings(tc.expectedAdjust.GetMounts()[0].Options)
				}
			}
			assert.Equal(t, tc.expectedAdjust, adjust)
			assert.Equal(t, tc.expectedUpdate, update)
		})
	}
}
