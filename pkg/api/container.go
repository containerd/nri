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
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (x *Container) GetCreatedAtTime() time.Time {
	t := time.Time{}
	if x != nil {
		return t.Add(time.Duration(x.CreatedAt) * time.Nanosecond)
	}
	return t
}

func (x *Container) GetStartedAtTime() time.Time {
	t := time.Time{}
	if x != nil {
		return t.Add(time.Duration(x.StartedAt) * time.Nanosecond)
	}
	return t
}

func (x *Container) GetFinishedAtTime() time.Time {
	t := time.Time{}
	if x != nil {
		return t.Add(time.Duration(x.FinishedAt) * time.Nanosecond)
	}
	return t
}

// GetCgroupsV2AbsPath returns the absolute path to the cgroup v2 directory for this container.
// This method converts relative cgroup paths to absolute paths, for different cgroup managers,
// QoS classes, and custom cgroup hierarchies.
// It returns an empty string if the container has no Linux configuration or cgroups path.
func (x *Container) GetCgroupsV2AbsPath() string {
	if x == nil || x.Linux == nil || x.Linux.CgroupsPath == "" {
		return ""
	}

	cgroupPath := x.Linux.CgroupsPath
	return getCGroupsV2Path(cgroupPath)
}

// GetCgroupsV2AbsPath returns the absolute path to the cgroup v2 directory for this pod sandbox.
// This method converts relative cgroup paths to absolute paths, for different cgroup managers,
// QoS classes, and custom cgroup hierarchies.
// It returns an empty string if the container has no Linux configuration or cgroups path.
func (x *PodSandbox) GetCgroupsV2AbsPath() string {
	if x == nil || x.Linux == nil || x.Linux.CgroupsPath == "" {
		return ""
	}

	cgroupPath := x.Linux.CgroupsPath
	return getCGroupsV2Path(cgroupPath)
}

// Helper functions

// getCGroupsV2Path helper
// Same implementation for both sandbox and container
func getCGroupsV2Path(cgroupPath string) string {
	if filepath.IsAbs(cgroupPath) {
		return cgroupPath
	}

	cgroupV2Root := getCgroupV2Root()
	if cgroupV2Root == "" {
		// Fallback to default cgroup v2 mount point
		cgroupV2Root = "/sys/fs/cgroup"
	}

	return filepath.Join(cgroupV2Root, cgroupPath)
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

// findCgroupV2Mount reads /proc/mounts to find the cgroup2 filesystem mount point
func findCgroupV2Mount() string {
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return ""
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[2] == "cgroup2" {
			return fields[1]
		}
	}
	return ""
}

func isCgroupV2Mount(path string) bool {
	if _, err := os.Stat(filepath.Join(path, "cgroup.controllers")); err == nil {
		return true
	}
	return false
}
