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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	rspec "github.com/opencontainers/runtime-spec/specs-go"
	rgen "github.com/opencontainers/runtime-tools/generate"

	"github.com/containerd/nri/pkg/api"
	xgen "github.com/containerd/nri/pkg/runtime-tools/generate"
)

func TestGenerate(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Generate Suite")
}

var _ = Describe("Adjustment", func() {
	When("nil", func() {
		It("does not modify the Spec", func() {
			var (
				spec   = makeSpec()
				adjust *api.ContainerAdjustment
			)

			rg := &rgen.Generator{Config: spec}
			xg := xgen.SpecGenerator(rg)

			Expect(xg).ToNot(BeNil())
			Expect(xg.Adjust(adjust)).To(Succeed())
			Expect(spec).To(Equal(makeSpec()))
		})
	})

	When("empty", func() {
		It("does not modify the Spec", func() {
			var (
				spec   = makeSpec()
				adjust = &api.ContainerAdjustment{
					Linux: &api.LinuxContainerAdjustment{},
				}
			)

			rg := &rgen.Generator{Config: spec}
			xg := xgen.SpecGenerator(rg)

			Expect(xg).ToNot(BeNil())
			Expect(xg.Adjust(adjust)).To(Succeed())
			Expect(spec).To(Equal(makeSpec()))
		})
	})

	When("has args", func() {
		It("adjusts Spec correctly", func() {
			var (
				spec   = makeSpec()
				adjust = &api.ContainerAdjustment{
					Args: []string{
						"arg0",
						"arg1",
						"arg2",
					},
				}
			)

			rg := &rgen.Generator{Config: spec}
			xg := xgen.SpecGenerator(rg)

			Expect(xg).ToNot(BeNil())
			Expect(xg.Adjust(adjust)).To(Succeed())
			Expect(spec).To(Equal(makeSpec(withArgs("arg0", "arg1", "arg2"))))
		})
	})

	When("has rlimits", func() {
		It("adjusts Spec correctly", func() {
			var (
				spec   = makeSpec()
				adjust = &api.ContainerAdjustment{
					Rlimits: []*api.POSIXRlimit{{
						Type: "nofile",
						Hard: 456,
						Soft: 123,
					}},
				}
			)

			rg := &rgen.Generator{Config: spec}
			xg := xgen.SpecGenerator(rg)

			Expect(xg).ToNot(BeNil())
			Expect(xg.Adjust(adjust)).To(Succeed())
			Expect(spec).To(Equal(makeSpec(withRlimit("nofile", 456, 123))))
		})
	})

	When("has memory limit", func() {
		It("adjusts Spec correctly", func() {
			var (
				spec   = makeSpec()
				adjust = &api.ContainerAdjustment{
					Linux: &api.LinuxContainerAdjustment{
						Resources: &api.LinuxResources{
							Memory: &api.LinuxMemory{
								Limit: api.Int64(11111),
							},
						},
					},
				}
			)

			rg := &rgen.Generator{Config: spec}
			xg := xgen.SpecGenerator(rg)

			Expect(xg).ToNot(BeNil())
			Expect(xg.Adjust(adjust)).To(Succeed())
			Expect(spec).To(Equal(makeSpec(withMemoryLimit(11111), withMemorySwap(11111))))
		})
	})

	When("has oom score adj", func() {
		It("adjusts Spec correctly", func() {
			var (
				oomScoreAdj = 123
				spec        = makeSpec()
				adjust      = &api.ContainerAdjustment{
					Linux: &api.LinuxContainerAdjustment{
						OomScoreAdj: &api.OptionalInt{
							Value: int64(oomScoreAdj),
						},
					},
				}
			)

			rg := &rgen.Generator{Config: spec}
			xg := xgen.SpecGenerator(rg)

			Expect(xg).ToNot(BeNil())
			Expect(xg.Adjust(adjust)).To(Succeed())
			Expect(spec).To(Equal(makeSpec(withOomScoreAdj(&oomScoreAdj))))
		})
	})

	When("unset oom score adj", func() {
		It("adjusts Spec correctly", func() {
			var (
				spec   = makeSpec()
				adjust = &api.ContainerAdjustment{
					Linux: &api.LinuxContainerAdjustment{
						OomScoreAdj: nil,
					},
				}
			)

			rg := &rgen.Generator{Config: spec}
			xg := xgen.SpecGenerator(rg)

			Expect(xg).ToNot(BeNil())
			Expect(xg.Adjust(adjust)).To(Succeed())
			Expect(spec).To(Equal(makeSpec(withOomScoreAdj(nil))))
		})
	})

	When("existing oom score adj", func() {
		It("does not adjust Spec", func() {
			var (
				spec         = makeSpec()
				expectedSpec = makeSpec()
				adjust       = &api.ContainerAdjustment{}
			)
			oomScoreAdj := 123
			spec.Process.OOMScoreAdj = &oomScoreAdj
			expectedSpec.Process.OOMScoreAdj = &oomScoreAdj

			rg := &rgen.Generator{Config: spec}
			xg := xgen.SpecGenerator(rg)

			Expect(xg).ToNot(BeNil())
			Expect(xg.Adjust(adjust)).To(Succeed())
			Expect(spec).To(Equal(expectedSpec))
		})
	})

	When("has CPU shares", func() {
		It("adjusts Spec correctly", func() {
			var (
				spec   = makeSpec()
				adjust = &api.ContainerAdjustment{
					Linux: &api.LinuxContainerAdjustment{
						Resources: &api.LinuxResources{
							Cpu: &api.LinuxCPU{
								Shares: api.UInt64(11111),
							},
						},
					},
				}
			)

			rg := &rgen.Generator{Config: spec}
			xg := xgen.SpecGenerator(rg)

			Expect(xg).ToNot(BeNil())
			Expect(xg.Adjust(adjust)).To(Succeed())
			Expect(spec).To(Equal(makeSpec(withCPUShares(11111))))
		})
	})

	When("has CPU quota", func() {
		It("adjusts Spec correctly", func() {
			var (
				spec   = makeSpec()
				adjust = &api.ContainerAdjustment{
					Linux: &api.LinuxContainerAdjustment{
						Resources: &api.LinuxResources{
							Cpu: &api.LinuxCPU{
								Quota: api.Int64(11111),
							},
						},
					},
				}
			)

			rg := &rgen.Generator{Config: spec}
			xg := xgen.SpecGenerator(rg)

			Expect(xg).ToNot(BeNil())
			Expect(xg.Adjust(adjust)).To(Succeed())
			Expect(spec).To(Equal(makeSpec(withCPUQuota(11111))))
		})
	})

	When("has CPU period", func() {
		It("adjusts Spec correctly", func() {
			var (
				spec   = makeSpec()
				adjust = &api.ContainerAdjustment{
					Linux: &api.LinuxContainerAdjustment{
						Resources: &api.LinuxResources{
							Cpu: &api.LinuxCPU{
								Period: api.UInt64(11111),
							},
						},
					},
				}
			)

			rg := &rgen.Generator{Config: spec}
			xg := xgen.SpecGenerator(rg)

			Expect(xg).ToNot(BeNil())
			Expect(xg.Adjust(adjust)).To(Succeed())
			Expect(spec).To(Equal(makeSpec(withCPUPeriod(11111))))
		})
	})

	When("has cpuset CPUs", func() {
		It("adjusts Spec correctly", func() {
			var (
				spec   = makeSpec()
				adjust = &api.ContainerAdjustment{
					Linux: &api.LinuxContainerAdjustment{
						Resources: &api.LinuxResources{
							Cpu: &api.LinuxCPU{
								Cpus: "5,6",
							},
						},
					},
				}
			)

			rg := &rgen.Generator{Config: spec}
			xg := xgen.SpecGenerator(rg)

			Expect(xg).ToNot(BeNil())
			Expect(xg.Adjust(adjust)).To(Succeed())
			Expect(spec).To(Equal(makeSpec(withCPUSetCPUs("5,6"))))
		})
	})

	When("has cpuset mems", func() {
		It("adjusts Spec correctly", func() {
			var (
				spec   = makeSpec()
				adjust = &api.ContainerAdjustment{
					Linux: &api.LinuxContainerAdjustment{
						Resources: &api.LinuxResources{
							Cpu: &api.LinuxCPU{
								Mems: "5,6",
							},
						},
					},
				}
			)

			rg := &rgen.Generator{Config: spec}
			xg := xgen.SpecGenerator(rg)

			Expect(xg).ToNot(BeNil())
			Expect(xg.Adjust(adjust)).To(Succeed())
			Expect(spec).To(Equal(makeSpec(withCPUSetMems("5,6"))))
		})
	})

	When("has pids limit", func() {
		It("adjusts Spec correctly", func() {
			var (
				spec   = makeSpec()
				adjust = &api.ContainerAdjustment{
					Linux: &api.LinuxContainerAdjustment{
						Resources: &api.LinuxResources{
							Pids: &api.LinuxPids{
								Limit: 123,
							},
						},
					},
				}
			)

			rg := &rgen.Generator{Config: spec}
			xg := xgen.SpecGenerator(rg)

			Expect(xg).ToNot(BeNil())
			Expect(xg.Adjust(adjust)).To(Succeed())
			Expect(spec).To(Equal(makeSpec(withPidsLimit(123))))
		})
	})

	When("has mounts", func() {
		It("it sorts the Spec mount slice", func() {
			var (
				spec   = makeSpec()
				adjust = &api.ContainerAdjustment{
					Mounts: []*api.Mount{
						{
							Destination: "/a/b/c/d/e",
							Source:      "/host/e",
						},
						{
							Destination: "/a/b/c",
							Source:      "/host/c",
						},
						{
							Destination: "/a/b",
							Source:      "/host/b",
						},
						{
							Destination: "/a",
							Source:      "/host/a",
						},
					},
				}
			)

			rg := &rgen.Generator{Config: spec}
			xg := xgen.SpecGenerator(rg)

			Expect(xg).ToNot(BeNil())
			Expect(xg.Adjust(adjust)).To(Succeed())
			Expect(spec).To(Equal(makeSpec(
				withMounts([]rspec.Mount{
					{
						Destination: "/a",
						Source:      "/host/a",
					},
					{
						Destination: "/a/b",
						Source:      "/host/b",
					},
					{
						Destination: "/a/b/c",
						Source:      "/host/c",
					},
					{
						Destination: "/a/b/c/d/e",
						Source:      "/host/e",
					},
				}),
			)))
		})
	})

	When("has a seccomp policy adjustment", func() {
		It("adjusts Spec correctly", func() {
			var (
				spec    = makeSpec()
				seccomp = rspec.LinuxSeccomp{
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
				adjust = &api.ContainerAdjustment{
					Linux: &api.LinuxContainerAdjustment{
						SeccompPolicy: api.FromOCILinuxSeccomp(&seccomp),
					},
				}
			)

			rg := &rgen.Generator{Config: spec}
			xg := xgen.SpecGenerator(rg)

			Expect(xg).ToNot(BeNil())
			Expect(xg.Adjust(adjust)).To(Succeed())
			Expect(*spec.Linux.Seccomp).To(Equal(seccomp))
		})
	})

	When("has a RDT adjustment", func() {
		It("adjusts Spec correctly", func() {
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

			rg := &rgen.Generator{Config: spec}
			xg := xgen.SpecGenerator(rg)

			Expect(xg).ToNot(BeNil())
			Expect(xg.Adjust(adjust)).To(Succeed())
			Expect(spec.Linux.IntelRdt).To(Equal(&rspec.LinuxIntelRdt{
				ClosID:           "foo",
				Schemata:         []string{"L2:0=ff", "L3:0=f"},
				EnableMonitoring: true,
			}))
		})
	})
})

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
	return func(spec *rspec.Spec) {
		if spec.Linux == nil {
			spec.Linux = &rspec.Linux{}
		}
		if spec.Linux.Resources == nil {
			spec.Linux.Resources = &rspec.LinuxResources{}
		}
		if spec.Linux.Resources.CPU == nil {
			spec.Linux.Resources.CPU = &rspec.LinuxCPU{}
		}
		spec.Linux.Resources.CPU.Shares = &v
	}
}

func withCPUQuota(v int64) specOption {
	return func(spec *rspec.Spec) {
		if spec.Linux == nil {
			spec.Linux = &rspec.Linux{}
		}
		if spec.Linux.Resources == nil {
			spec.Linux.Resources = &rspec.LinuxResources{}
		}
		if spec.Linux.Resources.CPU == nil {
			spec.Linux.Resources.CPU = &rspec.LinuxCPU{}
		}
		spec.Linux.Resources.CPU.Quota = &v
	}
}

func withCPUPeriod(v uint64) specOption {
	return func(spec *rspec.Spec) {
		if spec.Linux == nil {
			spec.Linux = &rspec.Linux{}
		}
		if spec.Linux.Resources == nil {
			spec.Linux.Resources = &rspec.LinuxResources{}
		}
		if spec.Linux.Resources.CPU == nil {
			spec.Linux.Resources.CPU = &rspec.LinuxCPU{}
		}
		spec.Linux.Resources.CPU.Period = &v
	}
}

func withCPUSetCPUs(v string) specOption {
	return func(spec *rspec.Spec) {
		if spec.Linux == nil {
			spec.Linux = &rspec.Linux{}
		}
		if spec.Linux.Resources == nil {
			spec.Linux.Resources = &rspec.LinuxResources{}
		}
		if spec.Linux.Resources.CPU == nil {
			spec.Linux.Resources.CPU = &rspec.LinuxCPU{}
		}
		spec.Linux.Resources.CPU.Cpus = v
	}
}

func withCPUSetMems(v string) specOption {
	return func(spec *rspec.Spec) {
		if spec.Linux == nil {
			spec.Linux = &rspec.Linux{}
		}
		if spec.Linux.Resources == nil {
			spec.Linux.Resources = &rspec.LinuxResources{}
		}
		if spec.Linux.Resources.CPU == nil {
			spec.Linux.Resources.CPU = &rspec.LinuxCPU{}
		}
		spec.Linux.Resources.CPU.Mems = v
	}
}

func withPidsLimit(v int64) specOption {
	return func(spec *rspec.Spec) {
		if spec.Linux == nil {
			spec.Linux = &rspec.Linux{}
		}
		if spec.Linux.Resources == nil {
			spec.Linux.Resources = &rspec.LinuxResources{}
		}
		spec.Linux.Resources.Pids = &rspec.LinuxPids{
			Limit: v,
		}
	}
}

func withMounts(mounts []rspec.Mount) specOption {
	return func(spec *rspec.Spec) {
		spec.Mounts = append(spec.Mounts, mounts...)
	}
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
					Limit: Int64(12345),
				},
				CPU: &rspec.LinuxCPU{
					Shares: Uint64(45678),
					Quota:  Int64(87654),
					Period: Uint64(54321),
					Cpus:   "0-111",
					Mems:   "0-4",
				},
				Pids: &rspec.LinuxPids{
					Limit: 1,
				},
			},
		},
	}
	for _, o := range options {
		o(spec)
	}
	return spec
}

func Int64(v int64) *int64 {
	return &v
}

func Uint64(v uint64) *uint64 {
	return &v
}
