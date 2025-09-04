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

package plugin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/containerd/nri/pkg/api"
)

func TestGetCgroupsV2AbsPath(t *testing.T) {
	tests := []struct {
		name        string
		container   *api.Container
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
			container: &api.Container{
				Id: "test-container",
			},
			expected:    "",
			description: "should return empty string for container without Linux config",
		},
		{
			name: "container without cgroups path",
			container: &api.Container{
				Id: "test-container",
				Linux: &api.LinuxContainer{
					CgroupsPath: "",
				},
			},
			expected:    "",
			description: "should return empty string for container without cgroups path",
		},
		{
			name: "container with absolute cgroups path",
			container: &api.Container{
				Id: "test-container",
				Linux: &api.LinuxContainer{
					CgroupsPath: "/sys/fs/cgroup/kubepods/pod123/container456",
				},
			},
			expected:    "/sys/fs/cgroup/kubepods/pod123/container456",
			description: "should return absolute path as-is",
		},
		{
			name: "container with relative cgroups path",
			container: &api.Container{
				Id: "test-container",
				Linux: &api.LinuxContainer{
					CgroupsPath: "kubepods/pod123/container456",
				},
			},
			expected:    "/sys/fs/cgroup/kubepods/pod123/container456",
			description: "should join relative path with cgroup v2 root",
		},
		{
			name: "container with systemd-style cgroups path",
			container: &api.Container{
				Id: "test-container",
				Linux: &api.LinuxContainer{
					CgroupsPath: "system.slice/containerd.service/kubepods-burstable-pod123.slice/cri-containerd:container456",
				},
			},
			expected:    "/sys/fs/cgroup/system.slice/containerd.service/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-pod123.slice/cri-containerd-container456.scope",
			description: "should handle systemd-style paths with proper colon conversion",
		},
		{
			name: "container with cgroupfs driver path",
			container: &api.Container{
				Id: "test-container",
				Linux: &api.LinuxContainer{
					CgroupsPath: "kubepods/burstable/pod123/container456",
				},
			},
			expected:    "/sys/fs/cgroup/kubepods/burstable/pod123/container456",
			description: "should handle cgroupfs driver paths",
		},
		{
			name: "container with cgroupfs path containing .slice",
			container: &api.Container{
				Id: "test-container",
				Linux: &api.LinuxContainer{
					CgroupsPath: "kubepods.slice/kubepods-burstable.slice/kubepods-burstable-pod123.slice/cri-containerd-container456.scope",
				},
			},
			expected:    "/sys/fs/cgroup/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-pod123.slice/cri-containerd-container456.scope",
			description: "should handle cgroupfs driver paths with .slice notation",
		},
		{
			name: "container with complex systemd path",
			container: &api.Container{
				Id: "test-container",
				Linux: &api.LinuxContainer{
					CgroupsPath: "machine.slice/crio:container:runtime",
				},
			},
			expected:    "/sys/fs/cgroup/machine.slice/crio-container-runtime.scope",
			description: "should handle complex systemd paths with multiple colons",
		},
		{
			name: "container with real-world slice:container format",
			container: &api.Container{
				Id: "test-container",
				Linux: &api.LinuxContainer{
					CgroupsPath: "kubepods-besteffort-podf8952339_1101_46ca_948d_1906de5016b8.slice:crio:656a5b06e0c7490f743b43c20cb984b9a5fd79ea0e49211d84ee0ec3d7ed0307",
				},
			},
			expected:    "/sys/fs/cgroup/kubepods.slice/kubepods-besteffort.slice/kubepods-besteffort-podf8952339_1101_46ca_948d_1906de5016b8.slice/crio-656a5b06e0c7490f743b43c20cb984b9a5fd79ea0e49211d84ee0ec3d7ed0307.scope",
			description: "should handle real-world slice:container format without directory separator",
		},
		{
			name: "container with cgroupfs path starting with /kubelet",
			container: &api.Container{
				Id: "test-container",
				Linux: &api.LinuxContainer{
					CgroupsPath: "/kubelet/kubepods/besteffort/pod346db9bc-06d5-450e-a97c-ce1d8209c72b/2d15832bd72e848c0583ec220826cd4bcde4d00ce49a82cd5d3a19ba2b39063a",
				},
			},
			expected:    "/sys/fs/cgroup/kubelet/kubepods/besteffort/pod346db9bc-06d5-450e-a97c-ce1d8209c72b/2d15832bd72e848c0583ec220826cd4bcde4d00ce49a82cd5d3a19ba2b39063a",
			description: "should properly join cgroupfs paths starting with /kubelet to cgroup root",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetContainerCgroupsV2AbsPath(tt.container)
			if result != tt.expected {
				t.Errorf("GetContainerCgroupsV2AbsPath() = %v, expected %v\nDescription: %s", result, tt.expected, tt.description)
			}
		})
	}
}

func TestGetPodCgroupsV2AbsPath(t *testing.T) {
	tests := []struct {
		name        string
		pod         *api.PodSandbox
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
			pod: &api.PodSandbox{
				Id: "test-pod",
			},
			expected:    "",
			description: "should return empty string for pod without Linux config",
		},
		{
			name: "pod without cgroups path",
			pod: &api.PodSandbox{
				Id: "test-pod",
				Linux: &api.LinuxPodSandbox{
					CgroupsPath: "",
				},
			},
			expected:    "",
			description: "should return empty string for pod without cgroups path",
		},
		{
			name: "pod with absolute cgroups path",
			pod: &api.PodSandbox{
				Id: "test-pod",
				Linux: &api.LinuxPodSandbox{
					CgroupsPath: "/sys/fs/cgroup/kubepods/pod123",
				},
			},
			expected:    "/sys/fs/cgroup/kubepods/pod123",
			description: "should return absolute path as-is",
		},
		{
			name: "pod with relative cgroups path",
			pod: &api.PodSandbox{
				Id: "test-pod",
				Linux: &api.LinuxPodSandbox{
					CgroupsPath: "kubepods/pod123",
				},
			},
			expected:    "/sys/fs/cgroup/kubepods/pod123",
			description: "should join relative path with cgroup v2 root",
		},
		{
			name: "pod with QoS burstable path",
			pod: &api.PodSandbox{
				Id: "test-pod",
				Linux: &api.LinuxPodSandbox{
					CgroupsPath: "kubepods/burstable/pod123",
				},
			},
			expected:    "/sys/fs/cgroup/kubepods/burstable/pod123",
			description: "should handle QoS class paths",
		},
		{
			name: "pod with QoS besteffort path",
			pod: &api.PodSandbox{
				Id: "test-pod",
				Linux: &api.LinuxPodSandbox{
					CgroupsPath: "kubepods/besteffort/pod123",
				},
			},
			expected:    "/sys/fs/cgroup/kubepods/besteffort/pod123",
			description: "should handle besteffort QoS class",
		},
		{
			name: "crio pod with systemd path - no .scope suffix",
			pod: &api.PodSandbox{
				Id: "test-pod",
				Linux: &api.LinuxPodSandbox{
					CgroupsPath: "kubepods-besteffort-pod123.slice:crio:container456",
				},
			},
			expected:    "/sys/fs/cgroup/kubepods.slice/kubepods-besteffort.slice/kubepods-besteffort-pod123.slice/crio-container456",
			description: "should handle crio pod without .scope suffix",
		},
		{
			name: "containerd pod with systemd path - adds .scope suffix",
			pod: &api.PodSandbox{
				Id: "test-pod",
				Linux: &api.LinuxPodSandbox{
					CgroupsPath: "system.slice/containerd.service/kubepods-burstable-pod123.slice/cri-containerd:container456",
				},
			},
			expected:    "/sys/fs/cgroup/system.slice/containerd.service/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-pod123.slice/cri-containerd-container456.scope",
			description: "should handle containerd pod with .scope suffix",
		},
		{
			name: "docker pod with systemd path - adds .scope suffix",
			pod: &api.PodSandbox{
				Id: "test-pod",
				Linux: &api.LinuxPodSandbox{
					CgroupsPath: "docker.slice:docker:container123",
				},
			},
			expected:    "/sys/fs/cgroup/docker.slice/docker-container123.scope",
			description: "should handle docker pod with .scope suffix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetPodCgroupsV2AbsPath(tt.pod)
			if result != tt.expected {
				t.Errorf("GetPodCgroupsV2AbsPath() = %v, expected %v\nDescription: %s", result, tt.expected, tt.description)
			}
		})
	}
}

func TestIsSystemdPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "systemd slice path",
			path:     "system.slice/containerd.service",
			expected: true,
		},
		{
			name:     "systemd path with colons",
			path:     "kubepods-burstable-pod123.slice:cri-containerd:container456",
			expected: true,
		},
		{
			name:     "systemd path with colon only",
			path:     "machine.slice/crio:container123",
			expected: true,
		},
		{
			name:     "cgroupfs path",
			path:     "kubepods/burstable/pod123/container456",
			expected: false,
		},
		{
			name:     "cgroupfs path with multiple .slice components",
			path:     "kubepods.slice/kubepods-burstable.slice/kubepods-burstable-pod123.slice/container456",
			expected: false,
		},
		{
			name:     "simple path",
			path:     "kubepods/pod123",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isSystemdPath(tt.path)
			if result != tt.expected {
				t.Errorf("isSystemdPath(%s) = %v, expected %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestConvertSystemdPath(t *testing.T) {
	tests := []struct {
		name        string
		cgroupRoot  string
		systemdPath string
		expected    string
	}{
		{
			name:        "real-world crio systemd path",
			cgroupRoot:  "/sys/fs/cgroup",
			systemdPath: "kubepods-besteffort-pod7b65ebd5_94f2_4f8e_9b82_10d79e214db2.slice/crio:d7b85095530d61a998543f5cb12c079ac23547fff116cdf20c39f8be5536fd05",
			expected:    "/sys/fs/cgroup/kubepods.slice/kubepods-besteffort.slice/kubepods-besteffort-pod7b65ebd5_94f2_4f8e_9b82_10d79e214db2.slice/crio-d7b85095530d61a998543f5cb12c079ac23547fff116cdf20c39f8be5536fd05.scope",
		},
		{
			name:        "systemd slice hierarchy expansion",
			cgroupRoot:  "/sys/fs/cgroup",
			systemdPath: "kubepods-burstable-pod123.slice",
			expected:    "/sys/fs/cgroup/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-pod123.slice",
		},
		{
			name:        "containerd systemd path with colons",
			cgroupRoot:  "/sys/fs/cgroup",
			systemdPath: "system.slice/containerd.service/kubepods-burstable-pod123.slice/cri-containerd:container456",
			expected:    "/sys/fs/cgroup/system.slice/containerd.service/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-pod123.slice/cri-containerd-container456.scope",
		},
		{
			name:        "systemd path without colons",
			cgroupRoot:  "/sys/fs/cgroup",
			systemdPath: "system.slice/containerd.service",
			expected:    "/sys/fs/cgroup/system.slice/containerd.service",
		},
		{
			name:        "multiple colon conversion",
			cgroupRoot:  "/sys/fs/cgroup",
			systemdPath: "machine.slice/crio:runtime:container123",
			expected:    "/sys/fs/cgroup/machine.slice/crio-runtime-container123.scope",
		},
		{
			name:        "slice followed directly by colons",
			cgroupRoot:  "/sys/fs/cgroup",
			systemdPath: "kubepods-besteffort-podf8952339_1101_46ca_948d_1906de5016b8.slice:crio:656a5b06e0c7490f743b43c20cb984b9a5fd79ea0e49211d84ee0ec3d7ed0307",
			expected:    "/sys/fs/cgroup/kubepods.slice/kubepods-besteffort.slice/kubepods-besteffort-podf8952339_1101_46ca_948d_1906de5016b8.slice/crio-656a5b06e0c7490f743b43c20cb984b9a5fd79ea0e49211d84ee0ec3d7ed0307.scope",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test as container path (most test cases expect .scope suffix)
			isContainer := strings.Contains(tt.systemdPath, ":")
			result := convertSystemdPath(tt.cgroupRoot, tt.systemdPath, isContainer)
			if result != tt.expected {
				t.Errorf("convertSystemdPath(%s, %s, %v) = %s, expected %s", tt.cgroupRoot, tt.systemdPath, isContainer, result, tt.expected)
			}
		})
	}
}

func TestResolveCgroupPath(t *testing.T) {
	tmpDir := t.TempDir()

	cgroupfsPath := filepath.Join(tmpDir, "kubepods", "burstable", "pod123")
	systemdPath := filepath.Join(tmpDir, "system.slice", "containerd.service", "kubepods-burstable-pod123.slice")

	err := os.MkdirAll(cgroupfsPath, 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	err = os.MkdirAll(systemdPath, 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	tests := []struct {
		name       string
		cgroupRoot string
		cgroupPath string
		expected   string
	}{
		{
			name:       "existing cgroupfs path",
			cgroupRoot: tmpDir,
			cgroupPath: "kubepods/burstable/pod123",
			expected:   filepath.Join(tmpDir, "kubepods/burstable/pod123"),
		},
		{
			name:       "systemd path conversion",
			cgroupRoot: tmpDir,
			cgroupPath: "system.slice/containerd.service/kubepods-burstable-pod123.slice/cri-containerd:container456",
			expected:   filepath.Join(tmpDir, "system.slice/containerd.service/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-pod123.slice/cri-containerd-container456.scope"),
		},
		{
			name:       "non-existing path falls back to cgroupfs",
			cgroupRoot: tmpDir,
			cgroupPath: "nonexistent/path",
			expected:   filepath.Join(tmpDir, "nonexistent/path"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test as container path for systemd paths (those with colons)
			isContainer := strings.Contains(tt.cgroupPath, ":")
			result := resolveCgroupPath(tt.cgroupRoot, tt.cgroupPath, isContainer)
			if result != tt.expected {
				t.Errorf("resolveCgroupPath(%s, %s, %v) = %s, expected %s", tt.cgroupRoot, tt.cgroupPath, isContainer, result, tt.expected)
			}
		})
	}
}

func TestExpandSliceHierarchy(t *testing.T) {
	tests := []struct {
		name      string
		sliceName string
		expected  []string
	}{
		{
			name:      "simple slice",
			sliceName: "kubepods.slice",
			expected:  []string{"kubepods.slice"},
		},
		{
			name:      "two-level slice",
			sliceName: "kubepods-burstable.slice",
			expected:  []string{"kubepods.slice", "kubepods-burstable.slice"},
		},
		{
			name:      "three-level slice",
			sliceName: "kubepods-burstable-pod123.slice",
			expected:  []string{"kubepods.slice", "kubepods-burstable.slice", "kubepods-burstable-pod123.slice"},
		},
		{
			name:      "real-world complex slice",
			sliceName: "kubepods-besteffort-pod7b65ebd5_94f2_4f8e_9b82_10d79e214db2.slice",
			expected:  []string{"kubepods.slice", "kubepods-besteffort.slice", "kubepods-besteffort-pod7b65ebd5_94f2_4f8e_9b82_10d79e214db2.slice"},
		},
		{
			name:      "non-slice name",
			sliceName: "containerd.service",
			expected:  []string{"containerd.service"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandSliceHierarchy(tt.sliceName)
			if len(result) != len(tt.expected) {
				t.Errorf("expandSliceHierarchy(%s) returned %d items, expected %d", tt.sliceName, len(result), len(tt.expected))
				return
			}
			for i, expected := range tt.expected {
				if result[i] != expected {
					t.Errorf("expandSliceHierarchy(%s)[%d] = %s, expected %s", tt.sliceName, i, result[i], expected)
				}
			}
		})
	}
}
