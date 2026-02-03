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
	"bytes"
	"fmt"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"

	rspec "github.com/opencontainers/runtime-spec/specs-go"
	rgen "github.com/opencontainers/runtime-tools/generate"

	"github.com/containerd/nri/pkg/api"
	xgen "github.com/containerd/nri/pkg/runtime-tools/generate"
)

func TestGenerate(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Generate Suite")
}

type logger struct {
	messages []string
}

func newLogger() *logger {
	return &logger{
		messages: []string{},
	}
}

func (l *logger) Log(event string, fields xgen.Fields) {
	var (
		buf = &bytes.Buffer{}
		log = &logrus.Logger{
			Out: buf,
			Formatter: &logrus.TextFormatter{
				DisableTimestamp: true,
				DisableQuote:     true,
			},
			Level: logrus.InfoLevel,
		}
	)
	log.WithFields(fields).Info(event)
	l.messages = append(l.messages, buf.String())
}

func (l *logger) MessageCount() int {
	return len(l.messages)
}

func (l *logger) Has(event string, fields xgen.Fields) bool {
	for _, entry := range l.messages {
		if !strings.Contains(entry, event) {
			continue
		}
		found := true
		for k, v := range fields {
			field := fmt.Sprintf("%s=%v", k, v)
			if !strings.Contains(entry, field) {
				found = false
				break
			}
		}
		if found {
			return true
		}
	}
	return false
}

var _ = Describe("Adjustment", func() {
	When("nil", func() {
		It("does not modify the Spec", func() {
			var (
				spec   = makeSpec()
				adjust *api.ContainerAdjustment
			)

			al := newLogger()
			rg := &rgen.Generator{Config: spec}
			xg := xgen.SpecGenerator(rg, xgen.WithLogger(al.Log, nil))

			Expect(xg).ToNot(BeNil())
			Expect(xg.Adjust(adjust)).To(Succeed())
			Expect(spec).To(Equal(makeSpec()))
			Expect(al.MessageCount()).To(Equal(0))
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

			al := newLogger()
			rg := &rgen.Generator{Config: spec}
			xg := xgen.SpecGenerator(rg, xgen.WithLogger(al.Log, nil))

			Expect(xg).ToNot(BeNil())
			Expect(xg.Adjust(adjust)).To(Succeed())
			Expect(spec).To(Equal(makeSpec()))
			Expect(al.MessageCount()).To(Equal(0))
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

			al := newLogger()
			rg := &rgen.Generator{Config: spec}
			xg := xgen.SpecGenerator(rg, xgen.WithLogger(al.Log, nil))

			Expect(xg).ToNot(BeNil())
			Expect(xg.Adjust(adjust)).To(Succeed())
			Expect(spec).To(Equal(makeSpec(withArgs("arg0", "arg1", "arg2"))))
			Expect(al.Has(xgen.AuditSetProcessArgs, nil)).To(BeTrue())
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

			al := newLogger()
			rg := &rgen.Generator{Config: spec}
			xg := xgen.SpecGenerator(rg, xgen.WithLogger(al.Log, nil))

			Expect(xg).ToNot(BeNil())
			Expect(xg.Adjust(adjust)).To(Succeed())
			Expect(spec).To(Equal(makeSpec(withRlimit("nofile", 456, 123))))
			Expect(al.Has(xgen.AuditAddProcessRlimits,
				xgen.Fields{
					"value.type": "nofile",
					"value.soft": 123,
					"value.hard": 456,
				})).To(BeTrue())
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

			al := newLogger()
			rg := &rgen.Generator{Config: spec}
			xg := xgen.SpecGenerator(rg, xgen.WithLogger(al.Log, nil))

			Expect(xg).ToNot(BeNil())
			Expect(xg.Adjust(adjust)).To(Succeed())
			Expect(spec).To(Equal(makeSpec(withMemoryLimit(11111), withMemorySwap(11111))))
			Expect(al.Has(xgen.AuditSetLinuxMemLimit,
				xgen.Fields{
					"value": "11111",
				})).To(BeTrue())
			Expect(al.Has(xgen.AuditSetLinuxMemSwapLimit,
				xgen.Fields{
					"value": "11111",
				})).To(BeTrue())
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

			al := newLogger()
			rg := &rgen.Generator{Config: spec}
			xg := xgen.SpecGenerator(rg, xgen.WithLogger(al.Log, nil))

			Expect(xg).ToNot(BeNil())
			Expect(xg.Adjust(adjust)).To(Succeed())
			Expect(spec).To(Equal(makeSpec(withOomScoreAdj(&oomScoreAdj))))
			Expect(al.Has(xgen.AuditSetProcessOOMScoreAdj,
				xgen.Fields{
					"value": oomScoreAdj,
				})).To(BeTrue())
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

			al := newLogger()
			rg := &rgen.Generator{Config: spec}
			xg := xgen.SpecGenerator(rg, xgen.WithLogger(al.Log, nil))

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

			al := newLogger()
			rg := &rgen.Generator{Config: spec}
			xg := xgen.SpecGenerator(rg, xgen.WithLogger(al.Log, nil))

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

			al := newLogger()
			rg := &rgen.Generator{Config: spec}
			xg := xgen.SpecGenerator(rg, xgen.WithLogger(al.Log, nil))

			Expect(xg).ToNot(BeNil())
			Expect(xg.Adjust(adjust)).To(Succeed())
			Expect(spec).To(Equal(makeSpec(withCPUShares(11111))))
			Expect(al.Has(xgen.AuditSetLinuxCPUShares,
				xgen.Fields{
					"value": 11111,
				})).To(BeTrue())
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

			al := newLogger()
			rg := &rgen.Generator{Config: spec}
			xg := xgen.SpecGenerator(rg, xgen.WithLogger(al.Log, nil))

			Expect(xg).ToNot(BeNil())
			Expect(xg.Adjust(adjust)).To(Succeed())
			Expect(spec).To(Equal(makeSpec(withCPUQuota(11111))))
			Expect(al.Has(xgen.AuditSetLinuxCPUQuota,
				xgen.Fields{
					"value": 11111,
				})).To(BeTrue())
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

			al := newLogger()
			rg := &rgen.Generator{Config: spec}
			xg := xgen.SpecGenerator(rg, xgen.WithLogger(al.Log, nil))

			Expect(xg).ToNot(BeNil())
			Expect(xg.Adjust(adjust)).To(Succeed())
			Expect(spec).To(Equal(makeSpec(withCPUPeriod(11111))))
			Expect(al.Has(xgen.AuditSetLinuxCPUPeriod,
				xgen.Fields{
					"value": 11111,
				})).To(BeTrue())
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

			al := newLogger()
			rg := &rgen.Generator{Config: spec}
			xg := xgen.SpecGenerator(rg, xgen.WithLogger(al.Log, nil))

			Expect(xg).ToNot(BeNil())
			Expect(xg.Adjust(adjust)).To(Succeed())
			Expect(spec).To(Equal(makeSpec(withCPUSetCPUs("5,6"))))
			Expect(al.Has(xgen.AuditSetLinuxCPUSetCPUs,
				xgen.Fields{
					"value": "5,6",
				})).To(BeTrue())
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

			al := newLogger()
			rg := &rgen.Generator{Config: spec}
			xg := xgen.SpecGenerator(rg, xgen.WithLogger(al.Log, nil))

			Expect(xg).ToNot(BeNil())
			Expect(xg.Adjust(adjust)).To(Succeed())
			Expect(spec).To(Equal(makeSpec(withCPUSetMems("5,6"))))
			Expect(al.Has(xgen.AuditSetLinuxCPUSetMems,
				xgen.Fields{
					"value": "5,6",
				})).To(BeTrue())
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

			al := newLogger()
			rg := &rgen.Generator{Config: spec}
			xg := xgen.SpecGenerator(rg, xgen.WithLogger(al.Log, nil))

			Expect(xg).ToNot(BeNil())
			Expect(xg.Adjust(adjust)).To(Succeed())
			Expect(spec).To(Equal(makeSpec(withPidsLimit(123))))
			Expect(al.Has(xgen.AuditSetLinuxPidsLimit,
				xgen.Fields{
					"value": 123,
				})).To(BeTrue())
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

			al := newLogger()
			rg := &rgen.Generator{Config: spec}
			xg := xgen.SpecGenerator(rg, xgen.WithLogger(al.Log, nil))

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
			Expect(al.Has(xgen.AuditAddMount,
				xgen.Fields{
					"value.destination": "/a",
				})).To(BeTrue())
			Expect(al.Has(xgen.AuditAddMount,
				xgen.Fields{
					"value.destination": "/a/b",
				})).To(BeTrue())
			Expect(al.Has(xgen.AuditAddMount,
				xgen.Fields{
					"value.destination": "/a/b/c",
				})).To(BeTrue())
			Expect(al.Has(xgen.AuditAddMount,
				xgen.Fields{
					"value.destination": "/a/b/c/d/e",
				})).To(BeTrue())
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

			al := newLogger()
			rg := &rgen.Generator{Config: spec}
			xg := xgen.SpecGenerator(rg, xgen.WithLogger(al.Log, nil))

			Expect(xg).ToNot(BeNil())
			Expect(xg.Adjust(adjust)).To(Succeed())
			Expect(*spec.Linux.Seccomp).To(Equal(seccomp))
			Expect(al.Has(xgen.AuditSetLinuxSeccompPolicy, nil)).To(BeTrue())
		})
	})

	When("has a sysctl adjustment", func() {
		It("adjusts Spec correctly", func() {
			var (
				spec   = makeSpec()
				adjust = &api.ContainerAdjustment{
					Linux: &api.LinuxContainerAdjustment{
						Sysctl: map[string]string{
							"net.ipv4.ip_forward":           "1",
							api.MarkForRemoval("delete.me"): "",
						},
					},
				}
			)
			spec.Linux.Sysctl = map[string]string{
				"delete.me": "foobar",
			}

			al := newLogger()
			rg := &rgen.Generator{Config: spec}
			xg := xgen.SpecGenerator(rg, xgen.WithLogger(al.Log, nil))

			Expect(xg).ToNot(BeNil())
			Expect(xg.Adjust(adjust)).To(Succeed())
			Expect(spec.Linux.Sysctl).To(Equal(map[string]string{
				"net.ipv4.ip_forward": "1",
			}))
			Expect(al.Has(xgen.AuditSetLinuxSysctl,
				xgen.Fields{
					"value.key":   "net.ipv4.ip_forward",
					"value.value": "1",
				})).To(BeTrue())
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

			al := newLogger()
			rg := &rgen.Generator{Config: spec}
			xg := xgen.SpecGenerator(rg, xgen.WithLogger(al.Log, nil))

			Expect(xg).ToNot(BeNil())
			Expect(xg.Adjust(adjust)).To(Succeed())
			Expect(spec.Linux.IntelRdt).To(Equal(&rspec.LinuxIntelRdt{
				ClosID:           "foo",
				Schemata:         []string{"L2:0=ff", "L3:0=f"},
				EnableMonitoring: true,
			}))
			Expect(al.Has(xgen.AuditSetLinuxRdtClosID,
				xgen.Fields{
					"value": "foo",
				})).To(BeTrue())
			Expect(al.Has(xgen.AuditSetLinuxRdtSchemata,
				xgen.Fields{
					"value": "[L2:0=ff L3:0=f]",
				})).To(BeTrue())
			Expect(al.Has(xgen.AuditSetLinuxRdtMonitoring,
				xgen.Fields{
					"value": true,
				})).To(BeTrue())
		})
	})
	When("has a RDT remove adjustment", func() {
		It("removes the IntelRdt config", func() {
			spec := makeSpec()
			spec.Linux.IntelRdt = &rspec.LinuxIntelRdt{ClosID: "bar"}
			adjust := &api.ContainerAdjustment{
				Linux: &api.LinuxContainerAdjustment{
					Rdt: &api.LinuxRdt{
						Remove: true,
					},
				},
			}

			al := newLogger()
			rg := &rgen.Generator{Config: spec}
			xg := xgen.SpecGenerator(rg, xgen.WithLogger(al.Log, nil))

			Expect(xg).ToNot(BeNil())
			Expect(xg.Adjust(adjust)).To(Succeed())
			Expect(spec.Linux.IntelRdt).To(BeNil())
			Expect(al.Has(xgen.AuditClearLinuxRdt, nil)).To(BeTrue())
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
			Limit: &v,
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
					Limit: func(v int64) *int64 { return &v }(1),
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
