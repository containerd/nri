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

func TestAdjustment(t *testing.T) {
	oomScoreAdj := 123

	tests := []struct {
		doc      string
		adjust   *api.ContainerAdjustment
		prepare  func(*rspec.Spec)
		expected func() *rspec.Spec
	}{
		{
			doc:      "nil",
			expected: func() *rspec.Spec { return makeSpec() },
		},
		{
			doc: "empty",
			adjust: &api.ContainerAdjustment{
				Linux: &api.LinuxContainerAdjustment{},
			},
			expected: func() *rspec.Spec { return makeSpec() },
		},
		{
			doc: "args",
			adjust: &api.ContainerAdjustment{
				Args: []string{"arg0", "arg1", "arg2"},
			},
			expected: func() *rspec.Spec {
				return makeSpec(withArgs("arg0", "arg1", "arg2"))
			},
		},
		{
			doc: "rlimits",
			adjust: &api.ContainerAdjustment{
				Rlimits: []*api.POSIXRlimit{{
					Type: "nofile",
					Hard: 456,
					Soft: 123,
				}},
			},
			expected: func() *rspec.Spec {
				return makeSpec(withRlimit("nofile", 456, 123))
			},
		},
		{
			doc: "memory limit",
			adjust: &api.ContainerAdjustment{
				Linux: &api.LinuxContainerAdjustment{
					Resources: &api.LinuxResources{
						Memory: &api.LinuxMemory{
							Limit: api.Int64(11111),
						},
					},
				},
			},
			expected: func() *rspec.Spec {
				return makeSpec(withMemoryLimit(11111), withMemorySwap(11111))
			},
		},
		{
			doc: "oom score adj",
			adjust: &api.ContainerAdjustment{
				Linux: &api.LinuxContainerAdjustment{
					OomScoreAdj: &api.OptionalInt{Value: int64(oomScoreAdj)},
				},
			},
			expected: func() *rspec.Spec {
				return makeSpec(withOomScoreAdj(&oomScoreAdj))
			},
		},
		{
			doc: "unset oom score adj",
			adjust: &api.ContainerAdjustment{
				Linux: &api.LinuxContainerAdjustment{},
			},
			expected: func() *rspec.Spec {
				return makeSpec(withOomScoreAdj(nil))
			},
		},
		{
			doc:    "existing oom score adj",
			adjust: &api.ContainerAdjustment{},
			prepare: func(spec *rspec.Spec) {
				spec.Process.OOMScoreAdj = &oomScoreAdj
			},
			expected: func() *rspec.Spec {
				return makeSpec(withOomScoreAdj(&oomScoreAdj))
			},
		},
		{
			doc: "CPU shares",
			adjust: &api.ContainerAdjustment{
				Linux: &api.LinuxContainerAdjustment{
					Resources: &api.LinuxResources{
						Cpu: &api.LinuxCPU{Shares: api.UInt64(11111)},
					},
				},
			},
			expected: func() *rspec.Spec {
				return makeSpec(withCPUShares(11111))
			},
		},
		{
			doc: "CPU quota",
			adjust: &api.ContainerAdjustment{
				Linux: &api.LinuxContainerAdjustment{
					Resources: &api.LinuxResources{
						Cpu: &api.LinuxCPU{Quota: api.Int64(11111)},
					},
				},
			},
			expected: func() *rspec.Spec {
				return makeSpec(withCPUQuota(11111))
			},
		},
		{
			doc: "CPU period",
			adjust: &api.ContainerAdjustment{
				Linux: &api.LinuxContainerAdjustment{
					Resources: &api.LinuxResources{
						Cpu: &api.LinuxCPU{Period: api.UInt64(11111)},
					},
				},
			},
			expected: func() *rspec.Spec {
				return makeSpec(withCPUPeriod(11111))
			},
		},
		{
			doc: "cpuset CPUs",
			adjust: &api.ContainerAdjustment{
				Linux: &api.LinuxContainerAdjustment{
					Resources: &api.LinuxResources{
						Cpu: &api.LinuxCPU{Cpus: "5,6"},
					},
				},
			},
			expected: func() *rspec.Spec {
				return makeSpec(withCPUSetCPUs("5,6"))
			},
		},
		{
			doc: "cpuset mems",
			adjust: &api.ContainerAdjustment{
				Linux: &api.LinuxContainerAdjustment{
					Resources: &api.LinuxResources{
						Cpu: &api.LinuxCPU{Mems: "5,6"},
					},
				},
			},
			expected: func() *rspec.Spec {
				return makeSpec(withCPUSetMems("5,6"))
			},
		},
		{
			doc: "pids limit",
			adjust: &api.ContainerAdjustment{
				Linux: &api.LinuxContainerAdjustment{
					Resources: &api.LinuxResources{
						Pids: &api.LinuxPids{Limit: 123},
					},
				},
			},
			expected: func() *rspec.Spec {
				return makeSpec(withPidsLimit(123))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.doc, func(t *testing.T) {
			spec := makeSpec()
			if tc.prepare != nil {
				tc.prepare(spec)
			}

			xg := xgen.SpecGenerator(ocigen.New(spec))
			require.NotNil(t, xg)
			require.NoError(t, xg.Adjust(tc.adjust))
			require.Equal(t, tc.expected(), spec)
		})
	}

	t.Run("mounts", func(t *testing.T) {
		spec := makeSpec()
		adjust := &api.ContainerAdjustment{
			Mounts: []*api.Mount{
				{Destination: "/a/b/c/d/e", Source: "/host/e"},
				{Destination: "/a/b/c", Source: "/host/c"},
				{Destination: "/a/b", Source: "/host/b"},
				{Destination: "/a", Source: "/host/a"},
			},
		}

		xg := xgen.SpecGenerator(ocigen.New(spec))
		require.NotNil(t, xg)
		require.NoError(t, xg.Adjust(adjust))
		require.Equal(t, makeSpec(withMounts([]rspec.Mount{
			{Destination: "/a", Source: "/host/a"},
			{Destination: "/a/b", Source: "/host/b"},
			{Destination: "/a/b/c", Source: "/host/c"},
			{Destination: "/a/b/c/d/e", Source: "/host/e"},
		})), spec)
	})

	t.Run("seccomp policy", func(t *testing.T) {
		spec := makeSpec()
		seccomp := rspec.LinuxSeccomp{
			DefaultAction: rspec.ActAllow,
			ListenerPath:  "/run/meshuggah-rocks.sock",
			Architectures: []rspec.Arch{},
			Flags:         []rspec.LinuxSeccompFlag{},
			Syscalls: []rspec.LinuxSyscall{{
				Names:  []string{"sched_getaffinity"},
				Action: rspec.ActNotify,
				Args:   []rspec.LinuxSeccompArg{},
			}},
		}
		adjust := &api.ContainerAdjustment{
			Linux: &api.LinuxContainerAdjustment{
				SeccompPolicy: api.FromOCILinuxSeccomp(&seccomp),
			},
		}

		xg := xgen.SpecGenerator(ocigen.New(spec))
		require.NotNil(t, xg)
		require.NoError(t, xg.Adjust(adjust))
		require.Equal(t, seccomp, *spec.Linux.Seccomp)
	})

	t.Run("sysctl", func(t *testing.T) {
		spec := makeSpec()
		spec.Linux.Sysctl = map[string]string{"delete.me": "foobar"}
		adjust := &api.ContainerAdjustment{
			Linux: &api.LinuxContainerAdjustment{
				Sysctl: map[string]string{
					"net.ipv4.ip_forward":           "1",
					api.MarkForRemoval("delete.me"): "",
				},
			},
		}

		xg := xgen.SpecGenerator(ocigen.New(spec))
		require.NotNil(t, xg)
		require.NoError(t, xg.Adjust(adjust))
		require.Equal(t, map[string]string{"net.ipv4.ip_forward": "1"}, spec.Linux.Sysctl)
	})

	t.Run("RDT", func(t *testing.T) {
		spec := makeSpec()
		adjust := &api.ContainerAdjustment{
			Linux: &api.LinuxContainerAdjustment{
				Rdt: &api.LinuxRdt{
					ClosId:           api.String("foo"),
					Schemata:         api.RepeatedString([]string{"L2:0=ff", "L3:0=f"}),
					EnableMonitoring: api.Bool(true),
				},
			},
		}

		xg := xgen.SpecGenerator(ocigen.New(spec))
		require.NotNil(t, xg)
		require.NoError(t, xg.Adjust(adjust))
		require.Equal(t, &rspec.LinuxIntelRdt{
			ClosID:           "foo",
			Schemata:         []string{"L2:0=ff", "L3:0=f"},
			EnableMonitoring: true,
		}, spec.Linux.IntelRdt)
	})

	t.Run("remove RDT", func(t *testing.T) {
		spec := makeSpec()
		spec.Linux.IntelRdt = &rspec.LinuxIntelRdt{ClosID: "bar"}
		adjust := &api.ContainerAdjustment{
			Linux: &api.LinuxContainerAdjustment{
				Rdt: &api.LinuxRdt{Remove: true},
			},
		}

		xg := xgen.SpecGenerator(ocigen.New(spec))
		require.NotNil(t, xg)
		require.NoError(t, xg.Adjust(adjust))
		require.Nil(t, spec.Linux.IntelRdt)
	})
}

type specOption func(*rspec.Spec)

func withArgs(args ...string) specOption {
	return func(spec *rspec.Spec) {
		if spec.Process == nil {
			spec.Process = &rspec.Process{}
		}
		spec.Process.Args = args
	}
}

func withMemoryLimit(v int64) specOption {
	return func(spec *rspec.Spec) {
		if spec.Linux == nil {
			spec.Linux = &rspec.Linux{}
		}
		if spec.Linux.Resources == nil {
			spec.Linux.Resources = &rspec.LinuxResources{}
		}
		if spec.Linux.Resources.Memory == nil {
			spec.Linux.Resources.Memory = &rspec.LinuxMemory{}
		}
		spec.Linux.Resources.Memory.Limit = &v
	}
}

func withMemorySwap(v int64) specOption {
	return func(spec *rspec.Spec) {
		if spec.Linux == nil {
			spec.Linux = &rspec.Linux{}
		}
		if spec.Linux.Resources == nil {
			spec.Linux.Resources = &rspec.LinuxResources{}
		}
		if spec.Linux.Resources.Memory == nil {
			spec.Linux.Resources.Memory = &rspec.LinuxMemory{}
		}
		spec.Linux.Resources.Memory.Swap = &v
	}
}

func withOomScoreAdj(v *int) specOption {
	return func(spec *rspec.Spec) {
		if spec.Process == nil {
			spec.Process = &rspec.Process{}
		}
		spec.Process.OOMScoreAdj = v
	}
}

func withCPUShares(v uint64) specOption {
	return func(spec *rspec.Spec) { ensureCPU(spec).Shares = &v }
}

func withCPUQuota(v int64) specOption {
	return func(spec *rspec.Spec) { ensureCPU(spec).Quota = &v }
}

func withCPUPeriod(v uint64) specOption {
	return func(spec *rspec.Spec) { ensureCPU(spec).Period = &v }
}

func withCPUSetCPUs(v string) specOption {
	return func(spec *rspec.Spec) { ensureCPU(spec).Cpus = v }
}

func withCPUSetMems(v string) specOption {
	return func(spec *rspec.Spec) { ensureCPU(spec).Mems = v }
}

func ensureCPU(spec *rspec.Spec) *rspec.LinuxCPU {
	if spec.Linux == nil {
		spec.Linux = &rspec.Linux{}
	}
	if spec.Linux.Resources == nil {
		spec.Linux.Resources = &rspec.LinuxResources{}
	}
	if spec.Linux.Resources.CPU == nil {
		spec.Linux.Resources.CPU = &rspec.LinuxCPU{}
	}
	return spec.Linux.Resources.CPU
}

func withPidsLimit(v int64) specOption {
	return func(spec *rspec.Spec) {
		if spec.Linux == nil {
			spec.Linux = &rspec.Linux{}
		}
		if spec.Linux.Resources == nil {
			spec.Linux.Resources = &rspec.LinuxResources{}
		}
		spec.Linux.Resources.Pids = &rspec.LinuxPids{Limit: &v}
	}
}

func withMounts(mounts []rspec.Mount) specOption {
	return func(spec *rspec.Spec) { spec.Mounts = append(spec.Mounts, mounts...) }
}

func withRlimit(typ string, hard, soft uint64) specOption {
	return func(spec *rspec.Spec) {
		if spec.Process == nil {
			return
		}
		spec.Process.Rlimits = append(spec.Process.Rlimits, rspec.POSIXRlimit{
			Type: typ,
			Hard: hard,
			Soft: soft,
		})
	}
}

func makeSpec(options ...specOption) *rspec.Spec {
	spec := &rspec.Spec{
		Process: &rspec.Process{},
		Linux: &rspec.Linux{
			Resources: &rspec.LinuxResources{
				Memory: &rspec.LinuxMemory{
					Limit: ptr(int64(12345)),
				},
				CPU: &rspec.LinuxCPU{
					Shares: ptr(uint64(45678)),
					Quota:  ptr(int64(87654)),
					Period: ptr(uint64(54321)),
					Cpus:   "0-111",
					Mems:   "0-4",
				},
				Pids: &rspec.LinuxPids{
					Limit: ptr(int64(1)),
				},
			},
		},
	}
	for _, option := range options {
		option(spec)
	}
	return spec
}

func ptr[T any](v T) *T {
	return &v
}
