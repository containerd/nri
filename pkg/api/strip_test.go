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

package api_test

import (
	fmt "fmt"
	"testing"

	"github.com/containerd/nri/pkg/api"

	faker "github.com/brianvoe/gofakeit/v7"
	"github.com/stretchr/testify/require"
)

func TestNonEmptyContainerAdjustmentStrip(t *testing.T) {
	t.Run("annotations", func(t *testing.T) {
		type Data struct {
			Annotations map[string]string
		}

		var (
			random = &Data{}
			adjust = &api.ContainerAdjustment{}
		)

		faker.Struct(random)

		adjust.Annotations = random.Annotations
		require.Equal(t, adjust, adjust.Strip(),
			"non-empty annotations should not be stripped")
	})

	t.Run("mounts", func(t *testing.T) {
		type Data struct {
			Data []*api.Mount
		}

		var (
			random = &Data{}
			adjust = &api.ContainerAdjustment{}
		)

		faker.Struct(random)

		adjust.Mounts = random.Data
		require.Equal(t, adjust, adjust.Strip(),
			"non-empty mounts should not be stripped")
	})

	t.Run("hooks", func(t *testing.T) {
		type Data struct {
			Data *api.Hooks
		}

		var (
			random = &Data{}
			adjust = &api.ContainerAdjustment{}
		)

		faker.Struct(random)

		adjust.Hooks = random.Data
		require.Equal(t, adjust, adjust.Strip(),
			"non-empty hooks should not be stripped")
	})

	t.Run("RLimits", func(t *testing.T) {
		type Data struct {
			Data []*api.POSIXRlimit
		}

		var (
			random = &Data{}
			adjust = &api.ContainerAdjustment{}
		)

		faker.Struct(random)

		adjust.Rlimits = random.Data
		require.Equal(t, adjust, adjust.Strip(),
			"non-empty rlimits should not be stripped")
	})

	t.Run("CDI devices", func(t *testing.T) {
		type Data struct {
			Data []*api.CDIDevice
		}

		var (
			random = &Data{}
			adjust = &api.ContainerAdjustment{}
		)

		faker.Struct(random)

		adjust.CDIDevices = random.Data
		require.Equal(t, adjust, adjust.Strip(),
			"non-empty CDI Devices should not be stripped")
	})

	t.Run("args", func(t *testing.T) {
		type Data struct {
			Data []string
		}

		var (
			random = &Data{}
			adjust = &api.ContainerAdjustment{}
		)

		faker.Struct(random)

		adjust.Args = random.Data
		require.Equal(t, adjust, adjust.Strip(),
			"non-empty args should not be stripped")
	})

	t.Run("Linux devices", func(t *testing.T) {
		type Data struct {
			Data []*api.LinuxDevice
		}

		var (
			random = &Data{}
			adjust = &api.ContainerAdjustment{
				Linux: &api.LinuxContainerAdjustment{},
			}
		)

		faker.Struct(random)

		adjust.Linux.Devices = random.Data
		require.Equal(t, adjust, adjust.Strip(),
			"non-empty Linux devices should not be stripped")
	})

	t.Run("Linux resources", func(t *testing.T) {
		type Data struct {
			Data *api.LinuxResources
		}

		var (
			random = &Data{}
			adjust = &api.ContainerAdjustment{
				Linux: &api.LinuxContainerAdjustment{},
			}
		)

		faker.Struct(random)

		adjust.Linux.Resources = random.Data
		require.Equal(t, adjust, adjust.Strip(),
			"non-empty Linux resources should not be stripped")
	})

	t.Run("Linux cgroups path", func(t *testing.T) {
		type Data struct {
			Data string
		}

		var (
			random = &Data{}
			adjust = &api.ContainerAdjustment{
				Linux: &api.LinuxContainerAdjustment{},
			}
		)

		faker.Struct(random)

		adjust.Linux.CgroupsPath = random.Data
		require.Equal(t, adjust, adjust.Strip(),
			"non-empty Linux cgroups path should not be stripped")
	})

	t.Run("Linux OOM score adjustment", func(t *testing.T) {
		type Data struct {
			Data *api.OptionalInt
		}

		var (
			random = &Data{}
			adjust = &api.ContainerAdjustment{
				Linux: &api.LinuxContainerAdjustment{},
			}
		)

		faker.Struct(random)

		adjust.Linux.OomScoreAdj = random.Data
		require.Equal(t, adjust, adjust.Strip(),
			"non-empty Linux OOM score adjustment should not be stripped")
	})

	t.Run("Linux I/O priority adjustment", func(t *testing.T) {
		type Data struct {
			Data *api.LinuxIOPriority
		}

		var (
			random = &Data{}
			adjust = &api.ContainerAdjustment{
				Linux: &api.LinuxContainerAdjustment{},
			}
		)

		faker.Struct(random)

		adjust.Linux.IoPriority = random.Data
		require.Equal(t, adjust, adjust.Strip(),
			"non-empty Linux I/O priority adjustment should not be stripped")
	})

	t.Run("Linux seccomp policy adjustment", func(t *testing.T) {
		type Data struct {
			Data *api.LinuxSeccomp
		}

		var (
			random = &Data{}
			adjust = &api.ContainerAdjustment{
				Linux: &api.LinuxContainerAdjustment{},
			}
		)

		faker.Struct(random)

		adjust.Linux.SeccompPolicy = random.Data
		require.Equal(t, adjust, adjust.Strip(),
			"non-empty Linux seccomp policy adjustment should not be stripped")
	})

	t.Run("Linux namespaces adjustment", func(t *testing.T) {
		type Data struct {
			Data []*api.LinuxNamespace
		}

		var (
			random = &Data{}
			adjust = &api.ContainerAdjustment{
				Linux: &api.LinuxContainerAdjustment{},
			}
		)

		faker.Struct(random)

		adjust.Linux.Namespaces = random.Data
		require.Equal(t, adjust, adjust.Strip(),
			"non-empty Linux namespaces adjustment should not be stripped")
	})

	t.Run("Linux sysctl adjustment", func(t *testing.T) {
		type Data struct {
			Data map[string]string
		}

		var (
			random = &Data{}
			adjust = &api.ContainerAdjustment{
				Linux: &api.LinuxContainerAdjustment{},
			}
		)

		faker.Struct(random)

		adjust.Linux.Sysctl = random.Data
		require.Equal(t, adjust, adjust.Strip(),
			"non-empty Linux sysctl adjustment should not be stripped")
	})

	t.Run("Linux net devices adjustment", func(t *testing.T) {
		type Data struct {
			Data map[string]*api.LinuxNetDevice
		}

		var (
			random = &Data{}
			adjust = &api.ContainerAdjustment{
				Linux: &api.LinuxContainerAdjustment{},
			}
		)

		faker.Struct(random)

		adjust.Linux.NetDevices = random.Data
		require.Equal(t, adjust, adjust.Strip(),
			"non-empty Linux net devices adjustment should not be stripped")
	})

	t.Run("Linux scheduler policy adjustment", func(t *testing.T) {
		type Data struct {
			Data *api.LinuxScheduler
		}

		var (
			random = &Data{}
			adjust = &api.ContainerAdjustment{
				Linux: &api.LinuxContainerAdjustment{},
			}
		)

		faker.Struct(random)
		fmt.Printf("random data: %v\n", random)

		adjust.Linux.Scheduler = random.Data
		require.Equal(t, adjust, adjust.Strip(),
			"non-empty Linux scheduler policy adjustment should not be stripped")
	})
}

func TestEmptyContainerAdjustmentStrip(t *testing.T) {
	t.Run("annotations", func(t *testing.T) {
		adjust := &api.ContainerAdjustment{
			Annotations: map[string]string{},
		}

		require.Nil(t, adjust.Strip(), "empty annotations should be reduced to nil")
	})

	t.Run("mounts", func(t *testing.T) {
		adjust := &api.ContainerAdjustment{
			Mounts: []*api.Mount{},
		}

		require.Nil(t, adjust.Strip(), "empty mounts should be reduced to nil")
	})

	t.Run("hooks", func(t *testing.T) {
		adjust := &api.ContainerAdjustment{
			Hooks: &api.Hooks{},
		}

		require.Nil(t, adjust.Strip(), "empty hooks should reduced to nil")
	})

	t.Run("RLimits", func(t *testing.T) {
		adjust := &api.ContainerAdjustment{
			Rlimits: []*api.POSIXRlimit{},
		}

		require.Nil(t, adjust.Strip(), "empty rlimits should be reduced to nil")
	})

	t.Run("CDI devices", func(t *testing.T) {
		adjust := &api.ContainerAdjustment{
			CDIDevices: []*api.CDIDevice{},
		}

		require.Nil(t, adjust.Strip(), "empty CDI Devices should not reduced to nil")
	})

	t.Run("args", func(t *testing.T) {
		adjust := &api.ContainerAdjustment{
			Args: []string{},
		}

		require.Nil(t, adjust.Strip(), "empty args should be reduced to nil")
	})

	t.Run("Linux devices", func(t *testing.T) {
		adjust := &api.ContainerAdjustment{
			Linux: &api.LinuxContainerAdjustment{
				Devices: []*api.LinuxDevice{},
			},
		}

		require.Nil(t, adjust.Strip(), "empty Linux devices should be reduced to nil")
	})

	t.Run("Linux resources", func(t *testing.T) {
		adjust := &api.ContainerAdjustment{
			Linux: &api.LinuxContainerAdjustment{
				Resources: &api.LinuxResources{},
			},
		}

		require.Nil(t, adjust.Strip(), "empty Linux resources should be reduced to nil")
	})

	t.Run("Linux cgroups path", func(t *testing.T) {
		adjust := &api.ContainerAdjustment{
			Linux: &api.LinuxContainerAdjustment{
				CgroupsPath: "",
			},
		}

		require.Nil(t, adjust.Strip(), "empty Linux cgroups path should be reduced to nil")
	})

	t.Run("Linux I/O priority adjustment", func(t *testing.T) {
		adjust := &api.ContainerAdjustment{
			Linux: &api.LinuxContainerAdjustment{
				IoPriority: &api.LinuxIOPriority{},
			},
		}

		require.Nil(t, adjust.Strip(), "empty Linux I/O priority should be reduced to nil")
	})

	t.Run("Linux seccomp policy adjustment", func(t *testing.T) {
		adjust := &api.ContainerAdjustment{
			Linux: &api.LinuxContainerAdjustment{
				SeccompPolicy: &api.LinuxSeccomp{},
			},
		}

		require.Nil(t, adjust.Strip(), "empty Linux seccomp policy should be reduced to nil")
	})

	t.Run("Linux namespaces adjustment", func(t *testing.T) {
		adjust := &api.ContainerAdjustment{
			Linux: &api.LinuxContainerAdjustment{
				Namespaces: []*api.LinuxNamespace{},
			},
		}

		require.Nil(t, adjust.Strip(), "empty Linux namespaces should be reduced to nil")
	})

	t.Run("Linux sysctl adjustment", func(t *testing.T) {
		adjust := &api.ContainerAdjustment{
			Linux: &api.LinuxContainerAdjustment{
				Sysctl: map[string]string{},
			},
		}

		require.Nil(t, adjust.Strip(), "empty Linux sysctl should be reduced to nil")
	})

	t.Run("Linux net devices adjustment", func(t *testing.T) {
		adjust := &api.ContainerAdjustment{
			Linux: &api.LinuxContainerAdjustment{
				NetDevices: map[string]*api.LinuxNetDevice{},
			},
		}

		require.Nil(t, adjust.Strip(), "empty Linux net devices should be reduced to nil")
	})

	t.Run("Linux scheduler policy adjustment", func(t *testing.T) {
		adjust := &api.ContainerAdjustment{
			Linux: &api.LinuxContainerAdjustment{
				Scheduler: &api.LinuxScheduler{},
			},
		}

		require.Nil(t, adjust.Strip(), "empty Linux scheduler policy should be reduced to nil")
	})

	t.Run("empty container adjustment", func(t *testing.T) {
		adjust := &api.ContainerAdjustment{
			Annotations: map[string]string{},
			Mounts:      []*api.Mount{},
			Env:         []*api.KeyValue{},
			Hooks:       &api.Hooks{},
			Rlimits:     []*api.POSIXRlimit{},
			CDIDevices:  []*api.CDIDevice{},
			Linux: &api.LinuxContainerAdjustment{
				Devices: []*api.LinuxDevice{},
				Resources: &api.LinuxResources{
					Memory:         &api.LinuxMemory{},
					Cpu:            &api.LinuxCPU{},
					HugepageLimits: []*api.HugepageLimit{},
					Unified:        map[string]string{},
				},
				Namespaces: []*api.LinuxNamespace{},
				NetDevices: map[string]*api.LinuxNetDevice{},
				Sysctl:     map[string]string{},
			},
		}

		require.Nil(t, adjust.Strip(), "empty container adjustment should be reduced to nil")
	})
}

func init() {
	// Make sure gofakeit properly generates test conversion data for
	// our deeply nested structs, slices of pointers to structs, etc.
	// The default is 10 which is not enough for some of our data types.
	faker.RecursiveDepth = 25
}
