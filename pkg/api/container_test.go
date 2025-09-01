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
)

func TestContainer_GetCgroup2AbsPath(t *testing.T) {
	tests := []struct {
		name        string
		container   *Container
		expected    string
		description string
	}{
		{
			name:        "nil container",
			container:   nil,
			expected:    "",
			description: "should return empty string for nil container",
		},
		{
			name: "container without linux config",
			container: &Container{
				Id: "test-container",
			},
			expected:    "",
			description: "should return empty string for container without Linux config",
		},
		{
			name: "container without cgroups path",
			container: &Container{
				Id: "test-container",
				Linux: &LinuxContainer{
					CgroupsPath: "",
				},
			},
			expected:    "",
			description: "should return empty string for container without cgroups path",
		},
		{
			name: "container with absolute cgroups path",
			container: &Container{
				Id: "test-container",
				Linux: &LinuxContainer{
					CgroupsPath: "/sys/fs/cgroup/kubepods/pod123/container456",
				},
			},
			expected:    "/sys/fs/cgroup/kubepods/pod123/container456",
			description: "should return absolute path as-is",
		},
		{
			name: "container with relative cgroups path",
			container: &Container{
				Id: "test-container",
				Linux: &LinuxContainer{
					CgroupsPath: "kubepods/pod123/container456",
				},
			},
			expected:    "/sys/fs/cgroup/kubepods/pod123/container456",
			description: "should join relative path with cgroup v2 root",
		},
		{
			name: "container with systemd-style cgroups path",
			container: &Container{
				Id: "test-container",
				Linux: &LinuxContainer{
					CgroupsPath: "system.slice/containerd.service/kubepods-burstable-pod123.slice:cri-containerd:container456",
				},
			},
			expected:    "/sys/fs/cgroup/system.slice/containerd.service/kubepods-burstable-pod123.slice:cri-containerd:container456",
			description: "should handle systemd-style paths",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.container.GetCgroupsV2AbsPath()
			if result != tt.expected {
				t.Errorf("GetCgroup2AbsPath() = %v, expected %v\nDescription: %s", result, tt.expected, tt.description)
			}
		})
	}
}

func TestPodSandbox_GetCgroup2AbsPath(t *testing.T) {
	tests := []struct {
		name        string
		pod         *PodSandbox
		expected    string
		description string
	}{
		{
			name:        "nil pod",
			pod:         nil,
			expected:    "",
			description: "should return empty string for nil pod",
		},
		{
			name: "pod without linux config",
			pod: &PodSandbox{
				Id: "test-pod",
			},
			expected:    "",
			description: "should return empty string for pod without Linux config",
		},
		{
			name: "pod without cgroups path",
			pod: &PodSandbox{
				Id: "test-pod",
				Linux: &LinuxPodSandbox{
					CgroupsPath: "",
				},
			},
			expected:    "",
			description: "should return empty string for pod without cgroups path",
		},
		{
			name: "pod with absolute cgroups path",
			pod: &PodSandbox{
				Id: "test-pod",
				Linux: &LinuxPodSandbox{
					CgroupsPath: "/sys/fs/cgroup/kubepods/pod123",
				},
			},
			expected:    "/sys/fs/cgroup/kubepods/pod123",
			description: "should return absolute path as-is",
		},
		{
			name: "pod with relative cgroups path",
			pod: &PodSandbox{
				Id: "test-pod",
				Linux: &LinuxPodSandbox{
					CgroupsPath: "kubepods/pod123",
				},
			},
			expected:    "/sys/fs/cgroup/kubepods/pod123",
			description: "should join relative path with cgroup v2 root",
		},
		{
			name: "pod with QoS burstable path",
			pod: &PodSandbox{
				Id: "test-pod",
				Linux: &LinuxPodSandbox{
					CgroupsPath: "kubepods/burstable/pod123",
				},
			},
			expected:    "/sys/fs/cgroup/kubepods/burstable/pod123",
			description: "should handle QoS class paths",
		},
		{
			name: "pod with QoS besteffort path",
			pod: &PodSandbox{
				Id: "test-pod",
				Linux: &LinuxPodSandbox{
					CgroupsPath: "kubepods/besteffort/pod123",
				},
			},
			expected:    "/sys/fs/cgroup/kubepods/besteffort/pod123",
			description: "should handle besteffort QoS class",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.pod.GetCgroupsV2AbsPath()
			if result != tt.expected {
				t.Errorf("GetCgroup2AbsPath() = %v, expected %v\nDescription: %s", result, tt.expected, tt.description)
			}
		})
	}
}
