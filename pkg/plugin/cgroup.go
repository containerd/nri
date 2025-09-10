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

	"github.com/containerd/nri/pkg/api"
	"github.com/moby/sys/mountinfo"
)

// Supported Cgroup Path Formats:
// +--------+------------------+---------------------------------------+--------------------------------------------------+------------------------+
// | Format | Type             | Example Input                         | Example Output                                   | Used By                |
// +--------+------------------+---------------------------------------+--------------------------------------------------+------------------------+
// | 1      | Complete         | /sys/fs/cgroup/kubepods/pod123        | /sys/fs/cgroup/kubepods/pod123                   | Pre-resolved paths     |
// |        | Absolute Path    |                                       | (returned as-is)                                 |                        |
// +--------+------------------+---------------------------------------+--------------------------------------------------+------------------------+
// | 2      | Cgroupfs         | kubepods/burstable/pod123/container   | /sys/fs/cgroup/kubepods/burstable/pod123/        | cgroupfs driver        |
// |        | Relative Path    |                                       | container                                        |                        |
// +--------+------------------+---------------------------------------+--------------------------------------------------+------------------------+
// | 3      | Cgroupfs Path    | /kubelet/kubepods/besteffort/pod123/  | /sys/fs/cgroup/kubelet/kubepods/besteffort/      | kubelet with cgroupfs  |
// |        | Starting with /  | container456                          | pod123/container456                              |                        |
// +--------+------------------+---------------------------------------+--------------------------------------------------+------------------------+
// | 4      | Cgroupfs with    | kubepods.slice/kubepods-burstable.    | /sys/fs/cgroup/kubepods.slice/kubepods-          | cgroupfs with systemd  |
// |        | .slice Notation  | slice/pod123.slice/container456.scope | burstable.slice/pod123.slice/container456.scope  | naming                 |
// +--------+------------------+---------------------------------------+--------------------------------------------------+------------------------+
// | 5      | Systemd Slice    | kubepods-burstable-pod123.slice:      | /sys/fs/cgroup/kubepods.slice/kubepods-          | cri-o, containerd      |
// |        | with Colons      | cri-containerd:container456           | burstable.slice/kubepods-burstable-pod123.slice/ | systemd integration    |
// |        |                  |                                       | cri-containerd-container456.scope                |                        |
// +--------+------------------+---------------------------------------+--------------------------------------------------+------------------------+
// | 6      | Systemd Slice    | kubepods-besteffort-pod123.slice:     | /sys/fs/cgroup/kubepods.slice/kubepods-          | cri-o direct format    |
// |        | Direct Format    | crio:container456                     | besteffort.slice/kubepods-besteffort-pod123.     |                        |
// |        |                  |                                       | slice/crio-container456.scope                    |                        |
// +--------+------------------+---------------------------------------+--------------------------------------------------+------------------------+
// | 7      | Systemd Service  | system.slice/containerd.service/      | /sys/fs/cgroup/system.slice/containerd.service/  | containerd with        |
// |        | Hierarchy        | kubepods-burstable-pod123.slice/      | kubepods.slice/kubepods-burstable.slice/         | systemd                |
// |        |                  | cri-containerd:container456           | kubepods-burstable-pod123.slice/cri-containerd-  |                        |
// |        |                  |                                       | container456.scope                               |                        |
// +--------+------------------+---------------------------------------+--------------------------------------------------+------------------------+
// | 8      | Multiple Colons  | machine.slice/crio:container:runtime  | /sys/fs/cgroup/machine.slice/crio-container-     | Complex runtime        |
// |        | in Systemd       |                                       | runtime.scope                                    | configurations         |
// +--------+------------------+---------------------------------------+--------------------------------------------------+------------------------+

// GetContainerCgroupsV2AbsPath returns the absolute path to the cgroup v2 directory for a container.
// This method converts relative cgroup paths to absolute paths, for different cgroup managers,
// QoS classes, and custom cgroup hierarchies.
// It returns an empty string if the container has no Linux configuration or cgroups path.
func GetContainerCgroupsV2AbsPath(container *api.Container) string {
	if container == nil || container.Linux == nil || container.Linux.CgroupsPath == "" {
		return ""
	}

	cgroupPath := container.Linux.CgroupsPath
	return getCGroupsV2PathForContainer(cgroupPath)
}

// GetPodCgroupsV2AbsPath returns the absolute path to the cgroup v2 directory for a pod sandbox.
// This method converts relative cgroup paths to absolute paths, for different cgroup managers,
// QoS classes, and custom cgroup hierarchies.
// It returns an empty string if the pod has no Linux configuration or cgroups path.
func GetPodCgroupsV2AbsPath(pod *api.PodSandbox) string {
	if pod == nil || pod.Linux == nil || pod.Linux.CgroupsPath == "" {
		return ""
	}

	cgroupPath := pod.Linux.CgroupsPath

	// For pods, we need to check the runtime type to determine if .scope should be added
	// NOTE: The podSandbox for the cri-o runtime does not suffix ".scope" to the pod cgroup path
	runtime := detectContainerRuntime(cgroupPath)
	isPodScopeRequired := runtime != "crio"

	return getCGroupsV2PathForPodWithRuntime(cgroupPath, isPodScopeRequired)
}

// Helper functions

// getCGroupsV2PathForContainer helper for container paths
func getCGroupsV2PathForContainer(cgroupPath string) string {
	cgroupV2Root := getCgroupV2Root()
	if cgroupV2Root == "" {
		// Fallback to default cgroup v2 mount point
		cgroupV2Root = "/sys/fs/cgroup"
	}

	resolvedPath := resolveCgroupPath(cgroupV2Root, cgroupPath, true)
	return resolvedPath
}

// getCGroupsV2PathForPodWithRuntime helper for pod paths with runtime detection
func getCGroupsV2PathForPodWithRuntime(cgroupPath string, shouldAddScope bool) string {
	cgroupV2Root := getCgroupV2Root()
	if cgroupV2Root == "" {
		// Fallback to default cgroup v2 mount point
		cgroupV2Root = "/sys/fs/cgroup"
	}

	// Try to resolve the path using different cgroup drivers (cgroupfs, systemd)
	resolvedPath := resolveCgroupPath(cgroupV2Root, cgroupPath, shouldAddScope)
	return resolvedPath
}

// getCgroupV2Root finds the cgroup v2 mount point by reading /proc/mounts
// It returns an empty string if no cgroup v2 mount point is found
func getCgroupV2Root() string {
	commonPaths := []string{
		"/sys/fs/cgroup",
		"/cgroup2",
	}

	if mountPoint := findCgroupV2Mount(); mountPoint != "" {
		return mountPoint
	}

	for _, path := range commonPaths {
		if isCgroupV2Mount(path) {
			return path
		}
	}

	return "/sys/fs/cgroup"
}

// findCgroupV2Mount uses mountinfo to find the cgroup2 filesystem mount point
func findCgroupV2Mount() string {
	mounts, err := mountinfo.GetMounts(mountinfo.FSTypeFilter("cgroup2"))
	if err != nil {
		return ""
	}

	if len(mounts) > 0 {
		return mounts[0].Mountpoint
	}

	return ""
}

// isCgroupV2Mount checks if the given path is a cgroup v2 mount point
func isCgroupV2Mount(path string) bool {
	// Check if the path exists and has the cgroup.controllers file (cgroup v2 indicator)
	if _, err := os.Stat(filepath.Join(path, "cgroup.controllers")); err == nil {
		return true
	}
	return false
}

// resolveCgroupPath resolves the cgroup path by trying cgroupfs and systemd cgroup drivers
// It first detects if the path is systemd-style, then applies conversion
func resolveCgroupPath(cgroupRoot, cgroupPath string, isContainer bool) string {
	// If the path already starts with the cgroup root, return it as-is
	if strings.HasPrefix(cgroupPath, cgroupRoot) {
		return cgroupPath
	}

	if isSystemdPath(cgroupPath) {
		return convertSystemdPath(cgroupRoot, cgroupPath, isContainer)
	}

	// For non-systemd paths, use cgroupfs driver (direct filesystem path)
	cgroupfsPath := filepath.Join(cgroupRoot, cgroupPath)
	return cgroupfsPath
}

// isSystemdPath checks if the path looks like a systemd slice path
func isSystemdPath(path string) bool {
	// Systemd paths have : notation OR contain .slice but not as nested directory paths
	// e.g., "kubepods-burstable-pod123.slice/crio:container" is systemd
	// but "kubepods.slice/kubepods-burstable.slice/..." is cgroupfs
	if strings.Contains(path, ":") {
		return true
	}

	if strings.Contains(path, ".slice") {
		parts := strings.Split(path, "/")
		// If we have multiple parts and multiple contain .slice, it's cgroupfs
		sliceCount := 0
		for _, part := range parts {
			if strings.Contains(part, ".slice") {
				sliceCount++
			}
		}
		// If there's only one .slice component, it's systemd
		return sliceCount == 1
	}

	return false
}

// convertSystemdPath converts systemd slice notation to filesystem path
func convertSystemdPath(cgroupRoot, systemdPath string, isContainer bool) string {
	// Convert systemd slice notation to filesystem path
	var pathComponents []string

	// First, check if we have a slice name followed directly by colons (format 2)
	if strings.Contains(systemdPath, ":") {
		// Find the first colon
		colonIndex := strings.Index(systemdPath, ":")
		beforeColon := systemdPath[:colonIndex]
		afterColon := systemdPath[colonIndex+1:]

		// If the part before colon ends with .slice, treat it as a slice
		if strings.HasSuffix(beforeColon, ".slice") {
			// Expand the slice hierarchy
			expandedSlices := expandSliceHierarchy(beforeColon)
			pathComponents = append(pathComponents, expandedSlices...)

			// Process the part after the colon
			containerParts := strings.Split(afterColon, ":")
			containerName := strings.Join(containerParts, "-")

			// Add .scope suffix for containers
			if isContainer {
				containerName += ".scope"
			}

			pathComponents = append(pathComponents, containerName)

			return filepath.Join(cgroupRoot, filepath.Join(pathComponents...))
		}
	}

	parts := strings.Split(systemdPath, "/")
	for _, part := range parts {
		if strings.Contains(part, ":") {
			// Handle colon-separated components (like "crio:container456")
			colonParts := strings.Split(part, ":")
			sliceName := colonParts[0]
			containerName := strings.Join(colonParts[1:], ":")
			// Convert colons to dashes in container name
			containerName = strings.ReplaceAll(containerName, ":", "-")

			// Add .scope suffix for containers
			if isContainer {
				containerName += ".scope"
			}

			pathComponents = append(pathComponents, sliceName+"-"+containerName)
		} else if strings.HasSuffix(part, ".slice") {
			// Handle slice hierarchy expansion
			expandedSlices := expandSliceHierarchy(part)
			pathComponents = append(pathComponents, expandedSlices...)
		} else {
			// Regular path component
			pathComponents = append(pathComponents, part)
		}
	}

	return filepath.Join(cgroupRoot, filepath.Join(pathComponents...))
}

// expandSliceHierarchy expands a systemd slice name into its hierarchical components
func expandSliceHierarchy(sliceName string) []string {
	if !strings.HasSuffix(sliceName, ".slice") {
		return []string{sliceName}
	}

	baseName := strings.TrimSuffix(sliceName, ".slice")

	parts := strings.Split(baseName, "-")

	var slices []string
	var currentSlice strings.Builder

	for i, part := range parts {
		if i > 0 {
			currentSlice.WriteString("-")
		}
		currentSlice.WriteString(part)
		slices = append(slices, currentSlice.String()+".slice")
	}

	return slices
}

// detectContainerRuntime detects the container runtime from the cgroup path
func detectContainerRuntime(cgroupPath string) string {
	if strings.Contains(cgroupPath, "crio") || strings.Contains(cgroupPath, "cri-o") {
		return "crio"
	}

	return "containerd"
}
