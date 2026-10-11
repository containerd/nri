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

package generate_test

import (
	"testing"

	rspec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/require"

	"github.com/containerd/nri/pkg/api"
	xgen "github.com/containerd/nri/pkg/runtime-tools/generate"
	"github.com/containerd/nri/pkg/runtime-tools/internal/ocigen"
)

func TestMemoryAdjustment(t *testing.T) {
	tests := []struct {
		name   string
		memory *api.LinuxMemory
		change func(*rspec.LinuxMemory)
	}{
		{
			name: "limit retains the default swap limit",
			memory: &api.LinuxMemory{
				Limit: api.Int64(4096),
			},
			change: func(m *rspec.LinuxMemory) {
				m.Limit = api.Int64(4096).Get()
				m.Swap = api.Int64(4096).Get()
			},
		},
		{
			name: "explicit swap overrides the limit default",
			memory: &api.LinuxMemory{
				Limit: api.Int64(4096),
				Swap:  api.Int64(8192),
			},
			change: func(m *rspec.LinuxMemory) {
				m.Limit = api.Int64(4096).Get()
				m.Swap = api.Int64(8192).Get()
			},
		},
		{
			name:   "swap without a limit",
			memory: &api.LinuxMemory{Swap: api.Int64(-1)},
			change: func(m *rspec.LinuxMemory) { m.Swap = api.Int64(-1).Get() },
		},
		{
			name:   "reservation",
			memory: &api.LinuxMemory{Reservation: api.Int64(2048)},
			change: func(m *rspec.LinuxMemory) { m.Reservation = api.Int64(2048).Get() },
		},
		{
			name:   "kernel limit",
			memory: &api.LinuxMemory{Kernel: api.Int64(1024)},
			change: func(m *rspec.LinuxMemory) { m.Kernel = api.Int64(1024).Get() }, //nolint:staticcheck // preserve the supported legacy field
		},
		{
			name:   "kernel TCP limit",
			memory: &api.LinuxMemory{KernelTcp: api.Int64(512)},
			change: func(m *rspec.LinuxMemory) { m.KernelTCP = api.Int64(512).Get() },
		},
		{
			name:   "zero swappiness",
			memory: &api.LinuxMemory{Swappiness: api.UInt64(0)},
			change: func(m *rspec.LinuxMemory) { m.Swappiness = api.UInt64(0).Get() },
		},
		{
			name:   "enable OOM killer",
			memory: &api.LinuxMemory{DisableOomKiller: api.Bool(false)},
			change: func(m *rspec.LinuxMemory) { m.DisableOOMKiller = api.Bool(false).Get() },
		},
		{
			name:   "disable hierarchy",
			memory: &api.LinuxMemory{UseHierarchy: api.Bool(false)},
			change: func(m *rspec.LinuxMemory) { m.UseHierarchy = api.Bool(false).Get() },
		},
		{
			name:   "zero limit",
			memory: &api.LinuxMemory{Limit: api.Int64(0)},
			change: func(m *rspec.LinuxMemory) {
				m.Limit = api.Int64(0).Get()
				m.Swap = api.Int64(0).Get()
			},
		},
		{
			name:   "empty memory preserves existing fields",
			memory: &api.LinuxMemory{},
		},
		{
			name: "nil memory preserves existing fields",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, populated := range []bool{false, true} {
				t.Run(map[bool]string{false: "empty spec", true: "existing fields"}[populated], func(t *testing.T) {
					spec := &rspec.Spec{
						Process: &rspec.Process{},
						Linux:   &rspec.Linux{Resources: &rspec.LinuxResources{}},
					}
					var expected *rspec.LinuxMemory
					if populated {
						spec.Linux.Resources = &rspec.LinuxResources{Memory: &rspec.LinuxMemory{
							Limit:            api.Int64(256).Get(),
							Reservation:      api.Int64(128).Get(),
							Swap:             api.Int64(512).Get(),
							Kernel:           api.Int64(64).Get(),
							KernelTCP:        api.Int64(32).Get(),
							Swappiness:       api.UInt64(60).Get(),
							DisableOOMKiller: api.Bool(true).Get(),
							UseHierarchy:     api.Bool(true).Get(),
						}}
						original := *spec.Linux.Resources.Memory
						expected = &original
					}
					if tt.change != nil {
						if expected == nil {
							expected = &rspec.LinuxMemory{}
						}
						tt.change(expected)
					}
					g := xgen.SpecGenerator(ocigen.New(spec))
					adjust := &api.ContainerAdjustment{Linux: &api.LinuxContainerAdjustment{
						Resources: &api.LinuxResources{Memory: tt.memory},
					}}
					require.NoError(t, g.Adjust(adjust))
					require.Equal(t, expected, spec.Linux.Resources.Memory)
				})
			}
		})
	}
}
