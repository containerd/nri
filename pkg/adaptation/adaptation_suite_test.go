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

package adaptation_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	nri "github.com/containerd/nri/pkg/adaptation"
	"github.com/containerd/nri/pkg/api"
	"github.com/containerd/nri/pkg/plugin"
	validator "github.com/containerd/nri/plugins/default-validator/builtin"
	rspec "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("Configuration", func() {
	var (
		s = &Suite{}
	)

	AfterEach(func() {
		s.Cleanup()
	})

	When("no (extra) options given", func() {
		BeforeEach(func() {
			s.Prepare(&mockRuntime{}, &mockPlugin{idx: "00", name: "test"})
		})

		It("should allow startup", func() {
			Expect(s.runtime.Start(s.dir)).To(Succeed())
		})

		It("should allow external plugins to connect", func() {
			var (
				runtime = s.runtime
				plugin  = s.plugins[0]
				timeout = time.After(startupTimeout)
			)
			Expect(runtime.Start(s.dir)).To(Succeed())
			Expect(plugin.Start(s.dir)).To(Succeed())
			Expect(plugin.Wait(PluginSynchronized, timeout)).To(Succeed())
		})
	})

	When("external connections are explicitly disabled", func() {
		var ()

		BeforeEach(func() {
			s.Prepare(
				&mockRuntime{
					options: []nri.Option{
						nri.WithDisabledExternalConnections(),
					},
				},
				&mockPlugin{idx: "00", name: "test"},
			)
		})

		It("should prevent plugins from connecting", func() {
			var (
				runtime = s.runtime
				plugin  = s.plugins[0]
			)
			Expect(runtime.Start(s.dir)).To(Succeed())
			Expect(plugin.Start(s.dir)).ToNot(Succeed())
		})
	})
})

var _ = Describe("Adaptation", func() {
	When("SyncFn is nil", func() {
		var (
			syncFn   func(ctx context.Context, cb nri.SyncCB) error
			updateFn = func(_ context.Context, _ []*nri.ContainerUpdate) ([]*nri.ContainerUpdate, error) {
				return nil, nil
			}
		)

		It("should prevent Adaptation creation with an error", func() {
			var (
				dir = GinkgoT().TempDir()
				etc = filepath.Join(dir, "etc", "nri")
			)

			Expect(os.MkdirAll(etc, 0o755)).To(Succeed())

			r, err := nri.New("mockRuntime", "0.0.1", syncFn, updateFn,
				nri.WithPluginPath(filepath.Join(dir, "opt", "nri", "plugins")),
				nri.WithPluginConfigPath(filepath.Join(dir, "etc", "nri", "conf.d")),
				nri.WithSocketPath(filepath.Join(dir, "nri.sock")),
			)

			Expect(r).To(BeNil())
			Expect(err).ToNot(BeNil())
		})
	})

	When("UpdateFn is nil", func() {
		var (
			updateFn func(ctx context.Context, updates []*nri.ContainerUpdate) ([]*nri.ContainerUpdate, error)
			syncFn   = func(_ context.Context, _ nri.SyncCB) error {
				return nil
			}
		)

		It("should prevent Adaptation creation with an error", func() {
			var (
				dir = GinkgoT().TempDir()
				etc = filepath.Join(dir, "etc", "nri")
			)

			Expect(os.MkdirAll(etc, 0o755)).To(Succeed())

			r, err := nri.New("mockRuntime", "0.0.1", syncFn, updateFn,
				nri.WithPluginPath(filepath.Join(dir, "opt", "nri", "plugins")),
				nri.WithPluginConfigPath(filepath.Join(dir, "etc", "nri", "conf.d")),
				nri.WithSocketPath(filepath.Join(dir, "nri.sock")),
			)

			Expect(r).To(BeNil())
			Expect(err).ToNot(BeNil())
		})
	})
})

var _ = Describe("Plugin connection", func() {
	var (
		s = &Suite{}
	)

	BeforeEach(func() {
		s.Prepare(
			&mockRuntime{
				pods: map[string]*api.PodSandbox{
					"pod0": {
						Id:        "pod0",
						Name:      "pod0",
						Uid:       "uid0",
						Namespace: "default",
					},
					"pod1": {
						Id:        "pod1",
						Name:      "pod1",
						Uid:       "uid1",
						Namespace: "default",
					},
				},
				ctrs: map[string]*api.Container{
					"ctr0": {
						Id:           "ctr0",
						PodSandboxId: "pod0",
						Name:         "ctr0",
						State:        api.ContainerState_CONTAINER_CREATED,
					},
					"ctr1": {
						Id:           "ctr1",
						PodSandboxId: "pod1",
						Name:         "ctr1",
						State:        api.ContainerState_CONTAINER_CREATED,
					},
				},
			},
			&mockPlugin{
				name: "test",
				idx:  "00",
			},
		)
	})

	AfterEach(func() {
		s.Cleanup()
	})

	It("should configure the plugin", func() {
		var (
			plugin = s.plugins[0]
		)

		s.Startup()

		Expect(plugin.Events()).Should(
			ContainElement(
				PluginConfigured,
			),
		)
	})

	It("should synchronize the plugin after configuration", func() {
		var (
			runtime = s.runtime
			plugin  = s.plugins[0]
		)

		s.Startup()

		Expect(plugin.Events()).Should(
			ConsistOf(
				PluginConfigured,
				PluginSynchronized,
			),
		)

		Expect(protoEqual(plugin.pods["pod0"], runtime.pods["pod0"])).Should(BeTrue(),
			protoDiff(plugin.pods["pod0"], runtime.pods["pod0"]))
		Expect(protoEqual(plugin.pods["pod1"], runtime.pods["pod1"])).Should(BeTrue(),
			protoDiff(plugin.pods["pod1"], runtime.pods["pod1"]))
		Expect(protoEqual(plugin.ctrs["ctr0"], runtime.ctrs["ctr0"])).Should(BeTrue(),
			protoDiff(plugin.ctrs["ctr0"], runtime.ctrs["ctr0"]))
		Expect(protoEqual(plugin.ctrs["ctr1"], runtime.ctrs["ctr1"])).Should(BeTrue(),
			protoDiff(plugin.ctrs["ctr1"], runtime.ctrs["ctr1"]))
	})
})

var _ = Describe("Pod and container requests and events", func() {
	var (
		s = &Suite{}
	)

	AfterEach(func() {
		s.Cleanup()
	})

	When("there are no plugins", func() {
		BeforeEach(func() {
			s.Prepare(&mockRuntime{})
		})

		It("should always succeed", func() {
			var (
				ctx  = context.Background()
				pod0 = &api.PodSandbox{
					Id:        "pod0",
					Name:      "pod0",
					Uid:       "uid0",
					Namespace: "default",
				}
				pod1 = &api.PodSandbox{
					Id:        "pod1",
					Name:      "pod1",
					Uid:       "uid1",
					Namespace: "default",
				}
				ctr0 = &api.Container{
					Id:           "ctr0",
					PodSandboxId: "pod0",
					Name:         "ctr0",
					State:        api.ContainerState_CONTAINER_CREATED, // XXX FIXME-kludge
				}
				ctr1 = &api.Container{
					Id:           "ctr1",
					PodSandboxId: "pod1",
					Name:         "ctr1",
					State:        api.ContainerState_CONTAINER_CREATED, // XXXX FIXME-kludge
				}
			)

			s.Startup()

			Expect(s.runtime.startStopPodAndContainer(ctx, pod0, ctr0)).To(Succeed())
			Expect(s.runtime.startStopPodAndContainer(ctx, pod1, ctr1)).To(Succeed())
		})
	})

	When("when there are plugins", func() {
		BeforeEach(func() {
			s.Prepare(&mockRuntime{}, &mockPlugin{idx: "00", name: "test"})
		})

		DescribeTable("should honor plugins' event subscriptions",
			func(subscriptions ...string) {
				var (
					runtime = s.runtime
					plugin  = s.plugins[0]
					ctx     = context.Background()

					pod = &api.PodSandbox{
						Id:        "pod0",
						Name:      "pod0",
						Uid:       "uid0",
						Namespace: "default",
					}
					ctr = &api.Container{
						Id:           "ctr0",
						PodSandboxId: "pod0",
						Name:         "ctr0",
						State:        api.ContainerState_CONTAINER_CREATED, // XXX FIXME-kludge
					}
				)

				plugin.mask = api.MustParseEventMask(subscriptions...)

				s.Startup()

				Expect(runtime.startStopPodAndContainer(ctx, pod, ctr)).To(Succeed())
				for _, events := range subscriptions {
					for _, event := range strings.Split(events, ",") {
						match := &Event{Type: EventType(event)}
						Expect(plugin.EventQ().Has(match)).To(BeTrue())
					}
				}
			},
			Entry("with RunPodSandbox", "RunPodSandbox"),
			Entry("with UpdatePodSandbox", "UpdatePodSandbox"),
			Entry("with PostUpdatePodSandbox", "PostUpdatePodSandbox"),
			Entry("with StopPodSandbox", "StopPodSandbox"),
			Entry("with RemovePodSandbox", "RemovePodSandbox"),

			Entry("with CreateContainer", "CreateContainer"),
			Entry("with PostCreateContainer", "PostCreateContainer"),
			Entry("with StartContainer", "StartContainer"),
			Entry("with PostStartContainer", "PostStartContainer"),
			Entry("with UpdateContainer", "UpdateContainer"),
			Entry("with PostUpdateContainer", "PostUpdateContainer"),
			Entry("with StopContainer", "StopContainer"),
			Entry("with RemoveContainer", "RemoveContainer"),

			Entry("with all pod events", "RunPodSandbox,StopPodSandbox,RemovePodSandbox"),
			Entry("with all container requests", "CreateContainer,UpdateContainer,StopContainer"),
			Entry("with all container requests and events",
				"CreateContainer,PostCreateContainer",
				"StartContainer,PostStartContainer",
				"UpdateContainer,PostUpdateContainer",
				"StopContainer",
				"RemoveContainer",
			),
			Entry("with all pod and container requests and events",
				"RunPodSandbox,UpdatePodSandbox,PostUpdatePodSandbox,StopPodSandbox,RemovePodSandbox",
				"CreateContainer,PostCreateContainer",
				"StartContainer,PostStartContainer",
				"UpdateContainer,PostUpdateContainer",
				"StopContainer",
				"RemoveContainer",
			),
		)
	})

	When("when there are multiple plugins", func() {
		BeforeEach(func() {
			s.Prepare(
				&mockRuntime{},
				&mockPlugin{idx: "20", name: "test"},
				&mockPlugin{idx: "99", name: "foo"},
				&mockPlugin{idx: "00", name: "bar"},
			)

		})

		DescribeTable("should honor plugins' event subscriptions",
			func(subscriptions ...string) {
				var (
					runtime = s.runtime
					plugins = s.plugins
					ctx     = context.Background()

					pod = &api.PodSandbox{
						Id:        "pod0",
						Name:      "pod0",
						Uid:       "uid0",
						Namespace: "default",
					}
					ctr = &api.Container{
						Id:           "ctr0",
						PodSandboxId: "pod0",
						Name:         "ctr0",
						State:        api.ContainerState_CONTAINER_CREATED, // XXX FIXME-kludge
					}

					order       []*mockPlugin
					recordOrder = func(p *mockPlugin, _ *api.PodSandbox, _ *api.Container) error {
						order = append(order, p)
						return nil
					}
				)

				for _, p := range plugins {
					p.mask = api.MustParseEventMask(subscriptions...)
					p.startContainer = recordOrder
				}

				s.Startup()

				Expect(runtime.startStopPodAndContainer(ctx, pod, ctr)).To(Succeed())
				Expect(order).Should(
					ConsistOf(
						plugins[2],
						plugins[0],
						plugins[1],
					),
				)
			},

			Entry("with StartContainer", "StartContainer"),
			Entry("with all container CRI requests",
				"CreateContainer,StartContainer,UpdateContainer,StopContainer,RemoveContainer"),
			Entry("with all container requests and events",
				"CreateContainer,PostCreateContainer",
				"StartContainer,PostStartContainer",
				"UpdateContainer,PostUpdateContainer",
				"StopContainer",
				"RemoveContainer",
			),
			Entry("with all pod and container requests and events",
				"RunPodSandbox,UpdatePodSandbox,PostUpdatePodSandbox,StopPodSandbox,RemovePodSandbox",
				"CreateContainer,PostCreateContainer",
				"StartContainer,PostStartContainer",
				"UpdateContainer,PostUpdateContainer",
				"StopContainer",
				"RemoveContainer",
			),
		)
	})
})

var _ = Describe("Plugin container creation adjustments", func() {
	var (
		s = &Suite{}
	)

	adjust := func(subject string, p *mockPlugin, _ *api.PodSandbox, c *api.Container, overwrite bool) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
		plugin := p.idx + "-" + p.name
		a := &api.ContainerAdjustment{}
		switch subject {
		case "annotation":
			if overwrite {
				a.RemoveAnnotation("key")
			}
			a.AddAnnotation("key", plugin)

		case "mount":
			mnt := &api.Mount{
				Source:      "/dev/" + plugin,
				Destination: "/mnt/test",
			}
			if overwrite {
				a.RemoveMount(mnt.Destination)
			}
			a.AddMount(mnt)

		case "remove mount":
			a.RemoveMount("/remove/test/destination")

		case "environment":
			if overwrite {
				a.RemoveEnv("key")
			}
			a.AddEnv("key", plugin)

		case "arguments":
			if !overwrite {
				a.SetArgs([]string{"echo", "updated", "argument", "list"})
			} else {
				a.UpdateArgs(append(slices.Clone(c.Args), "twice..."))
			}

		case "hooks":
			a.AddHooks(
				&api.Hooks{
					Prestart: []*api.Hook{
						{
							Path: "/bin/" + plugin,
						},
					},
				},
			)

		case "device":
			idx, _ := strconv.ParseInt(p.idx, 10, 64)
			dev := &api.LinuxDevice{
				Path:  "/dev/test",
				Type:  "c",
				Major: 313,
				Minor: 100 + idx,
			}
			if overwrite {
				a.RemoveDevice(dev.Path)
			}
			a.AddDevice(dev)

		case "namespace":
			ns := &api.LinuxNamespace{
				Type: "cgroup",
			}
			a.AddOrReplaceNamespace(ns)

		case "rlimit":
			a.AddRlimit("nofile", 456, 123)

		case "CDI-device":
			a.AddCDIDevice(
				&api.CDIDevice{
					Name: "vendor0.com/dev=dev0",
				},
			)

		case "I/O priority":
			a.SetLinuxIOPriority(&nri.LinuxIOPriority{
				Class:    api.IOPrioClass_IOPRIO_CLASS_RT,
				Priority: 5,
			})

		case "clear I/O priority":
			a.SetLinuxIOPriority(&nri.LinuxIOPriority{
				Class: api.IOPrioClass_IOPRIO_CLASS_NONE,
			})

		case "resources/cpu":
			a.SetLinuxCPUShares(123)
			a.SetLinuxCPUQuota(456)
			a.SetLinuxCPUPeriod(789)
			a.SetLinuxCPURealtimeRuntime(321)
			a.SetLinuxCPURealtimePeriod(654)
			a.SetLinuxCPUSetCPUs("0-1")
			a.SetLinuxCPUSetMems("2-3")

		case "resources/mem":
			a.SetLinuxMemoryLimit(1234000)
			a.SetLinuxMemoryReservation(4000)
			a.SetLinuxMemorySwap(34000)
			a.SetLinuxMemoryKernel(30000)
			a.SetLinuxMemoryKernelTCP(2000)
			a.SetLinuxMemorySwappiness(987)
			a.SetLinuxMemoryDisableOomKiller()
			a.SetLinuxMemoryUseHierarchy()

		case "resources/classes":
			a.SetLinuxRDTClass(plugin)
			a.SetLinuxBlockIOClass(plugin)

		case "resources/hugepagelimits":
			a.AddLinuxHugepageLimit("1M", 4096)
			a.AddLinuxHugepageLimit("4M", 1024)

		case "resources/unified":
			a.AddLinuxUnified("resource.1", "value1")
			a.AddLinuxUnified("resource.2", "value2")

		case "cgroupspath":
			a.SetLinuxCgroupsPath("/" + plugin)

		case "seccomp":
			a.SetLinuxSeccompPolicy(
				func() *api.LinuxSeccomp {
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
					return api.FromOCILinuxSeccomp(&seccomp)
				}(),
			)
		case "rdt":
			if overwrite {
				a.RemoveLinuxRDT()
			}
			a.SetLinuxRDTClosID(p.name)
			a.SetLinuxRDTSchemata([]string{"L3:0=ff", "MB:0=50"})
			a.SetLinuxRDTEnableMonitoring(true)
		}

		return a, nil, nil
	}

	AfterEach(func() {
		s.Cleanup()
	})

	When("there is a single plugin", func() {
		BeforeEach(func() {
			s.Prepare(&mockRuntime{}, &mockPlugin{idx: "00", name: "test"})
		})

		DescribeTable("should be successfully collected without conflicts",
			func(subject string, expected *api.ContainerAdjustment) {
				var (
					runtime = s.runtime
					plugin  = s.plugins[0]
					ctx     = context.Background()

					pod = &api.PodSandbox{
						Id:        "pod0",
						Name:      "pod0",
						Uid:       "uid0",
						Namespace: "default",
					}
					ctr = &api.Container{
						Id:           "ctr0",
						PodSandboxId: "pod0",
						Name:         "ctr0",
						State:        api.ContainerState_CONTAINER_CREATED, // XXX FIXME-kludge
						Mounts: []*api.Mount{
							{
								Type:        "bind",
								Source:      "/remove/test",
								Destination: "/remove/test/destination",
							},
						},
						Args: []string{
							"echo",
							"original",
							"argument",
							"list",
						},
					}
				)

				create := func(p *mockPlugin, pod *api.PodSandbox, ctr *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
					return adjust(subject, p, pod, ctr, false)
				}

				plugin.createContainer = create

				s.Startup()

				podReq := &api.RunPodSandboxRequest{Pod: pod}
				Expect(runtime.RunPodSandbox(ctx, podReq)).To(Succeed())
				ctrReq := &api.CreateContainerRequest{
					Pod:       pod,
					Container: ctr,
				}
				reply, err := runtime.CreateContainer(ctx, ctrReq)
				Expect(err).To(BeNil())
				Expect(protoEqual(reply.Adjust.Strip(), expected.Strip())).Should(BeTrue(),
					protoDiff(reply.Adjust, expected))
			},

			Entry("adjust annotations", "annotation",
				&api.ContainerAdjustment{
					Annotations: map[string]string{
						"key": "00-test",
					},
				},
			),
			Entry("adjust mounts", "mount",
				&api.ContainerAdjustment{
					Mounts: []*api.Mount{
						{
							Source:      "/dev/00-test",
							Destination: "/mnt/test",
						},
					},
				},
			),
			Entry("remove a mount", "remove mount",
				&api.ContainerAdjustment{
					Mounts: []*api.Mount{
						{
							Destination: api.MarkForRemoval("/remove/test/destination"),
						},
					},
				},
			),
			Entry("adjust environment", "environment",
				&api.ContainerAdjustment{
					Env: []*api.KeyValue{
						{
							Key:   "key",
							Value: "00-test",
						},
					},
				},
			),
			Entry("adjust arguments", "arguments",
				&api.ContainerAdjustment{
					Args: []string{
						"echo",
						"updated",
						"argument",
						"list",
					},
				},
			),
			Entry("adjust hooks", "hooks",
				&api.ContainerAdjustment{
					Hooks: &api.Hooks{
						Prestart: []*api.Hook{
							{
								Path: "/bin/00-test",
							},
						},
					},
				},
			),
			Entry("adjust devices", "device",
				&api.ContainerAdjustment{
					Linux: &api.LinuxContainerAdjustment{
						Devices: []*api.LinuxDevice{
							{
								Path:  "/dev/test",
								Type:  "c",
								Major: 313,
								Minor: 100,
							},
						},
					},
				},
			),
			Entry("adjust namespace", "namespace",
				&api.ContainerAdjustment{
					Linux: &api.LinuxContainerAdjustment{
						Namespaces: []*api.LinuxNamespace{
							{
								Type: "cgroup",
							},
						},
					},
				},
			),
			Entry("adjust rlimits", "rlimit",
				&api.ContainerAdjustment{
					Rlimits: []*api.POSIXRlimit{{Type: "nofile", Soft: 123, Hard: 456}},
				},
			),
			Entry("adjust CDI Devices", "CDI-device",
				&api.ContainerAdjustment{
					CDIDevices: []*api.CDIDevice{
						{
							Name: "vendor0.com/dev=dev0",
						},
					},
				},
			),

			Entry("adjust I/O priority", "I/O priority",
				&api.ContainerAdjustment{
					Linux: &api.LinuxContainerAdjustment{
						IoPriority: &api.LinuxIOPriority{
							Class:    api.IOPrioClass_IOPRIO_CLASS_RT,
							Priority: 5,
						},
					},
				},
			),
			Entry("clear I/O priority", "clear I/O priority",
				&api.ContainerAdjustment{
					Linux: &api.LinuxContainerAdjustment{
						IoPriority: &api.LinuxIOPriority{},
					},
				},
			),

			Entry("adjust CPU resources", "resources/cpu",
				&api.ContainerAdjustment{
					Linux: &api.LinuxContainerAdjustment{
						Resources: &api.LinuxResources{
							Cpu: &api.LinuxCPU{
								Shares:          api.UInt64(123),
								Quota:           api.Int64(456),
								Period:          api.UInt64(789),
								RealtimeRuntime: api.Int64(321),
								RealtimePeriod:  api.UInt64(654),
								Cpus:            "0-1",
								Mems:            "2-3",
							},
						},
					},
				},
			),
			Entry("adjust memory resources", "resources/mem",
				&api.ContainerAdjustment{
					Linux: &api.LinuxContainerAdjustment{
						Resources: &api.LinuxResources{
							Memory: &api.LinuxMemory{
								Limit:            api.Int64(1234000),
								Reservation:      api.Int64(4000),
								Swap:             api.Int64(34000),
								Kernel:           api.Int64(30000),
								KernelTcp:        api.Int64(2000),
								Swappiness:       api.UInt64(987),
								DisableOomKiller: api.Bool(true),
								UseHierarchy:     api.Bool(true),
							},
						},
					},
				},
			),
			Entry("adjust class-based resources", "resources/classes",
				&api.ContainerAdjustment{
					Linux: &api.LinuxContainerAdjustment{
						Resources: &api.LinuxResources{
							RdtClass:     api.String("00-test"),
							BlockioClass: api.String("00-test"),
						},
					},
				},
			),
			Entry("adjust hugepage limits", "resources/hugepagelimits",
				&api.ContainerAdjustment{
					Linux: &api.LinuxContainerAdjustment{
						Resources: &api.LinuxResources{
							HugepageLimits: []*api.HugepageLimit{
								{
									PageSize: "1M",
									Limit:    4096,
								},
								{
									PageSize: "4M",
									Limit:    1024,
								},
							},
						},
					},
				},
			),
			Entry("adjust cgroupv2 unified resources", "resources/unified",
				&api.ContainerAdjustment{
					Linux: &api.LinuxContainerAdjustment{
						Resources: &api.LinuxResources{
							Unified: map[string]string{
								"resource.1": "value1",
								"resource.2": "value2",
							},
						},
					},
				},
			),
			Entry("adjust cgroups path", "cgroupspath",
				&api.ContainerAdjustment{
					Linux: &api.LinuxContainerAdjustment{
						CgroupsPath: "/00-test",
					},
				},
			),
			Entry("adjust seccomp policy", "seccomp",
				&api.ContainerAdjustment{
					Linux: &api.LinuxContainerAdjustment{
						SeccompPolicy: func() *api.LinuxSeccomp {
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
							return api.FromOCILinuxSeccomp(&seccomp)
						}(),
					},
				},
			),
			Entry("adjust RDT", "rdt",
				&api.ContainerAdjustment{
					Linux: &api.LinuxContainerAdjustment{
						Rdt: &api.LinuxRdt{
							ClosId:           api.String("test"),
							Schemata:         api.RepeatedString([]string{"L3:0=ff", "MB:0=50"}),
							EnableMonitoring: api.Bool(true),
						},
					},
				},
			),
		)
	})

	When("there are multiple plugins", func() {
		BeforeEach(func() {
			s.Prepare(
				&mockRuntime{},
				&mockPlugin{idx: "10", name: "foo"},
				&mockPlugin{idx: "00", name: "bar"},
			)
		})

		DescribeTable("should be successfully combined if there are no conflicts",
			func(subject string, remove, shouldFail bool, expected *api.ContainerAdjustment) {
				var (
					runtime = s.runtime
					plugins = s.plugins
					ctx     = context.Background()

					pod = &api.PodSandbox{
						Id:        "pod0",
						Name:      "pod0",
						Uid:       "uid0",
						Namespace: "default",
					}
					ctr = &api.Container{
						Id:           "ctr0",
						PodSandboxId: "pod0",
						Name:         "ctr0",
						State:        api.ContainerState_CONTAINER_CREATED, // XXX FIXME-kludge
						Args: []string{
							"echo",
							"original",
							"argument",
							"list",
						},
					}
				)

				create := func(p *mockPlugin, pod *api.PodSandbox, ctr *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
					return adjust(subject, p, pod, ctr, p == plugins[0] && remove)
				}

				plugins[0].createContainer = create
				plugins[1].createContainer = create

				s.Startup()

				podReq := &api.RunPodSandboxRequest{Pod: pod}
				Expect(runtime.RunPodSandbox(ctx, podReq)).To(Succeed())
				ctrReq := &api.CreateContainerRequest{
					Pod:       pod,
					Container: ctr,
				}
				reply, err := runtime.CreateContainer(ctx, ctrReq)
				if shouldFail {
					Expect(err).ToNot(BeNil())
				} else {
					Expect(err).To(BeNil())
					Expect(protoEqual(reply.Adjust.Strip(), expected.Strip())).Should(BeTrue(),
						protoDiff(reply.Adjust, expected))
				}
			},

			Entry("adjust annotations (conflicts)", "annotation", false, true, nil),
			Entry("adjust annotations", "annotation", true, false,
				&api.ContainerAdjustment{
					Annotations: map[string]string{
						"-key": "",
						"key":  "10-foo",
					},
				},
			),
			Entry("adjust mounts (conflicts)", "mount", false, true, nil),
			Entry("adjust mounts", "mount", true, false,
				&api.ContainerAdjustment{
					Mounts: []*api.Mount{
						{
							Source:      "/dev/10-foo",
							Destination: "/mnt/test",
						},
					},
				},
			),
			Entry("adjust environment (conflicts)", "environment", false, true, nil),
			Entry("adjust environment", "environment", true, false,
				&api.ContainerAdjustment{
					Env: []*api.KeyValue{
						{
							Key:   "key",
							Value: "10-foo",
						},
					},
				},
			),

			Entry("adjust arguments (conflicts)", "arguments", false, true, nil),
			Entry("adjust arguments", "arguments", true, false,
				&api.ContainerAdjustment{
					Args: []string{
						"echo",
						"updated",
						"argument",
						"list",
						"twice...",
					},
				},
			),

			Entry("adjust hooks", "hooks", false, false,
				&api.ContainerAdjustment{
					Hooks: &api.Hooks{
						Prestart: []*api.Hook{
							{
								Path: "/bin/00-bar",
							},
							{
								Path: "/bin/10-foo",
							},
						},
					},
				},
			),
			Entry("adjust devices", "device", false, true, nil),
			Entry("adjust devices", "device", true, false,
				&api.ContainerAdjustment{
					Linux: &api.LinuxContainerAdjustment{
						Devices: []*api.LinuxDevice{
							{
								Path:  "/dev/test",
								Type:  "c",
								Major: 313,
								Minor: 110,
							},
						},
					},
				},
			),
			Entry("adjust resources", "resources/classes", false, true, nil),
			Entry("adjust I/O priority (conflicts)", "I/O priority", false, true, nil),
			Entry("adjust RDT (conflicts)", "rdt", false, true, nil),
			Entry("adjust RDT", "rdt", true, false,
				&api.ContainerAdjustment{
					Linux: &api.LinuxContainerAdjustment{
						Rdt: &api.LinuxRdt{
							ClosId:           api.String("foo"),
							Schemata:         api.RepeatedString([]string{"L3:0=ff", "MB:0=50"}),
							EnableMonitoring: api.Bool(true),
						},
					},
				},
			),
		)
	})

	When("there are validating plugins", func() {
		BeforeEach(func() {
			s.Prepare(
				&mockRuntime{},
				&mockPlugin{idx: "00", name: "foo"},
				&mockPlugin{idx: "00", name: "validator"},
			)
		})

		DescribeTable("validation result should be honored",
			func(subject string, shouldFail bool, expected *api.ContainerAdjustment) {
				var (
					runtime = s.runtime
					plugins = s.plugins
					ctx     = context.Background()

					pod = &api.PodSandbox{
						Id:        "pod0",
						Name:      "pod0",
						Uid:       "uid0",
						Namespace: "default",
					}
					ctr = &api.Container{
						Id:           "ctr0",
						PodSandboxId: "pod0",
						Name:         "ctr0",
						State:        api.ContainerState_CONTAINER_CREATED, // XXX FIXME-kludge
						Args: []string{
							"echo",
							"original",
							"argument",
							"list",
						},
					}

					forbidden = "no-no"
				)

				create := func(p *mockPlugin, _ *api.PodSandbox, _ *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
					plugin := p.idx + "-" + p.name
					a := &api.ContainerAdjustment{}
					switch subject {
					case "annotation":
						key := "key"
						if shouldFail {
							key = forbidden
						}
						a.AddAnnotation(key, plugin)
					}

					return a, nil, nil
				}

				validate := func(_ *mockPlugin, req *api.ValidateContainerAdjustmentRequest) error {
					_, ok := req.Owners.AnnotationOwner(req.Container.Id, forbidden)
					if ok {
						return fmt.Errorf("forbidden annotation %q adjusted", forbidden)
					}
					return nil
				}

				plugins[0].createContainer = create
				plugins[1].validateAdjustment = validate
				s.Startup()

				podReq := &api.RunPodSandboxRequest{Pod: pod}
				Expect(runtime.RunPodSandbox(ctx, podReq)).To(Succeed())
				ctrReq := &api.CreateContainerRequest{
					Pod:       pod,
					Container: ctr,
				}
				reply, err := runtime.CreateContainer(ctx, ctrReq)
				if shouldFail {
					Expect(err).ToNot(BeNil())
				} else {
					Expect(err).To(BeNil())
					Expect(protoEqual(reply.Adjust.Strip(), expected.Strip())).Should(BeTrue(),
						protoDiff(reply.Adjust, expected))
				}
			},

			Entry("adjust allowed annotation", "annotation", false,
				&api.ContainerAdjustment{
					Annotations: map[string]string{
						"key": "00-foo",
					},
				},
			),

			Entry("adjust forbidden annotation", "annotation", true, nil),
		)
	})

	When("the default validator is enabled and OCI Hook injection is disabled", func() {
		BeforeEach(func() {
			s.Prepare(
				&mockRuntime{
					options: []nri.Option{
						nri.WithDefaultValidator(
							&validator.DefaultValidatorConfig{
								Enable:                  true,
								RejectOCIHookAdjustment: true,
							},
						),
					},
				},
				&mockPlugin{idx: "00", name: "foo"},
				&mockPlugin{idx: "10", name: "validator1"},
				&mockPlugin{idx: "20", name: "validator2"},
			)
		})

		It("should reject OCI Hook injection", func() {
			var (
				create = func(_ *mockPlugin, _ *api.PodSandbox, ctr *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
					a := &api.ContainerAdjustment{}
					if ctr.GetName() == "ctr1" {
						a.AddHooks(
							&api.Hooks{
								Prestart: []*api.Hook{
									{
										Path: "/bin/sh",
										Args: []string{"/bin/sh", "-c", "true"},
									},
								},
							},
						)
					}

					return a, nil, nil
				}

				validate = func(_ *mockPlugin, _ *api.ValidateContainerAdjustmentRequest) error {
					return nil
				}

				runtime = s.runtime
				plugins = s.plugins
				ctx     = context.Background()

				pod = &api.PodSandbox{
					Id:        "pod0",
					Name:      "pod0",
					Uid:       "uid0",
					Namespace: "default",
				}
				ctr0 = &api.Container{
					Id:           "ctr0",
					PodSandboxId: "pod0",
					Name:         "ctr0",
					State:        api.ContainerState_CONTAINER_CREATED,
				}
				ctr1 = &api.Container{
					Id:           "ctr1",
					PodSandboxId: "pod0",
					Name:         "ctr1",
					State:        api.ContainerState_CONTAINER_CREATED,
				}
			)

			plugins[0].createContainer = create
			plugins[1].validateAdjustment = validate
			plugins[2].validateAdjustment = validate

			s.Startup()
			podReq := &api.RunPodSandboxRequest{Pod: pod}
			Expect(runtime.RunPodSandbox(ctx, podReq)).To(Succeed())

			ctrReq := &api.CreateContainerRequest{
				Pod:       pod,
				Container: ctr0,
			}
			reply, err := runtime.CreateContainer(ctx, ctrReq)
			Expect(reply).ToNot(BeNil())
			Expect(err).To(BeNil())

			ctrReq = &api.CreateContainerRequest{
				Pod:       pod,
				Container: ctr1,
			}
			reply, err = runtime.CreateContainer(ctx, ctrReq)
			Expect(err).ToNot(BeNil())
			Expect(reply).To(BeNil())
		})
	})

	When("default validator disallows runtime default seccomp policy adjustment", func() {
		BeforeEach(func() {
			s.Prepare(
				&mockRuntime{
					options: []nri.Option{
						nri.WithDefaultValidator(
							&validator.DefaultValidatorConfig{
								Enable:                                true,
								RejectRuntimeDefaultSeccompAdjustment: true,
							},
						),
					},
				},
				&mockPlugin{idx: "00", name: "foo"},
				&mockPlugin{idx: "10", name: "validator1"},
				&mockPlugin{idx: "20", name: "validator2"},
			)
		})

		It("should reject runtime default seccomp policy adjustment", func() {
			var (
				create = func(_ *mockPlugin, _ *api.PodSandbox, ctr *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
					a := &api.ContainerAdjustment{}
					if ctr.GetName() == "ctr1" {
						a.SetLinuxSeccompPolicy(
							func() *api.LinuxSeccomp {
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
								return api.FromOCILinuxSeccomp(&seccomp)
							}(),
						)
					}
					return a, nil, nil
				}

				validate = func(_ *mockPlugin, _ *api.ValidateContainerAdjustmentRequest) error {
					return nil
				}

				runtime = s.runtime
				plugins = s.plugins
				ctx     = context.Background()

				pod = &api.PodSandbox{
					Id:        "pod0",
					Name:      "pod0",
					Uid:       "uid0",
					Namespace: "default",
				}
				ctr0 = &api.Container{
					Id:           "ctr0",
					PodSandboxId: "pod0",
					Name:         "ctr0",
					State:        api.ContainerState_CONTAINER_CREATED,
				}
				ctr1 = &api.Container{
					Id:           "ctr1",
					PodSandboxId: "pod0",
					Name:         "ctr1",
					State:        api.ContainerState_CONTAINER_CREATED,
					Linux: &api.LinuxContainer{
						SeccompProfile: &api.SecurityProfile{
							ProfileType: api.SecurityProfile_RUNTIME_DEFAULT,
						},
					},
				}
			)

			plugins[0].createContainer = create
			plugins[1].validateAdjustment = validate
			plugins[2].validateAdjustment = validate

			s.Startup()
			podReq := &api.RunPodSandboxRequest{Pod: pod}
			Expect(runtime.RunPodSandbox(ctx, podReq)).To(Succeed())

			ctrReq := &api.CreateContainerRequest{
				Pod:       pod,
				Container: ctr0,
			}
			reply, err := runtime.CreateContainer(ctx, ctrReq)
			Expect(reply).ToNot(BeNil())
			Expect(err).To(BeNil())

			ctrReq = &api.CreateContainerRequest{
				Pod:       pod,
				Container: ctr1,
			}
			reply, err = runtime.CreateContainer(ctx, ctrReq)
			Expect(err).ToNot(BeNil())
			Expect(reply).To(BeNil())
		})
	})

	When("default validator allows runtime default seccomp policy adjustment", func() {
		BeforeEach(func() {
			s.Prepare(
				&mockRuntime{
					options: []nri.Option{
						nri.WithDefaultValidator(
							&validator.DefaultValidatorConfig{
								Enable:                                true,
								RejectRuntimeDefaultSeccompAdjustment: false,
							},
						),
					},
				},
				&mockPlugin{idx: "00", name: "foo"},
				&mockPlugin{idx: "10", name: "validator1"},
				&mockPlugin{idx: "20", name: "validator2"},
			)
		})

		It("should not reject runtime default seccomp policy adjustment", func() {
			var (
				create = func(_ *mockPlugin, _ *api.PodSandbox, ctr *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
					a := &api.ContainerAdjustment{}
					if ctr.GetName() == "ctr1" {
						a.SetLinuxSeccompPolicy(
							func() *api.LinuxSeccomp {
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
								return api.FromOCILinuxSeccomp(&seccomp)
							}(),
						)
					}
					return a, nil, nil
				}

				validate = func(_ *mockPlugin, _ *api.ValidateContainerAdjustmentRequest) error {
					return nil
				}

				runtime = s.runtime
				plugins = s.plugins
				ctx     = context.Background()

				pod = &api.PodSandbox{
					Id:        "pod0",
					Name:      "pod0",
					Uid:       "uid0",
					Namespace: "default",
				}
				ctr0 = &api.Container{
					Id:           "ctr0",
					PodSandboxId: "pod0",
					Name:         "ctr0",
					State:        api.ContainerState_CONTAINER_CREATED,
				}
				ctr1 = &api.Container{
					Id:           "ctr1",
					PodSandboxId: "pod0",
					Name:         "ctr1",
					State:        api.ContainerState_CONTAINER_CREATED,
					Linux: &api.LinuxContainer{
						SeccompProfile: &api.SecurityProfile{
							ProfileType: api.SecurityProfile_RUNTIME_DEFAULT,
						},
					},
				}
			)

			plugins[0].createContainer = create
			plugins[1].validateAdjustment = validate
			plugins[2].validateAdjustment = validate

			s.Startup()
			podReq := &api.RunPodSandboxRequest{Pod: pod}
			Expect(runtime.RunPodSandbox(ctx, podReq)).To(Succeed())

			ctrReq := &api.CreateContainerRequest{
				Pod:       pod,
				Container: ctr0,
			}
			reply, err := runtime.CreateContainer(ctx, ctrReq)
			Expect(reply).ToNot(BeNil())
			Expect(err).To(BeNil())

			ctrReq = &api.CreateContainerRequest{
				Pod:       pod,
				Container: ctr1,
			}
			reply, err = runtime.CreateContainer(ctx, ctrReq)
			Expect(reply).ToNot(BeNil())
			Expect(err).To(BeNil())
		})
	})

	When("default validator disallows custom seccomp policy adjustment", func() {
		BeforeEach(func() {
			s.Prepare(
				&mockRuntime{
					options: []nri.Option{
						nri.WithDefaultValidator(
							&validator.DefaultValidatorConfig{
								Enable:                        true,
								RejectCustomSeccompAdjustment: true,
							},
						),
					},
				},
				&mockPlugin{idx: "00", name: "foo"},
				&mockPlugin{idx: "10", name: "validator1"},
				&mockPlugin{idx: "20", name: "validator2"},
			)
		})

		It("should reject custom seccomp policy adjustment", func() {
			var (
				create = func(_ *mockPlugin, _ *api.PodSandbox, ctr *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
					a := &api.ContainerAdjustment{}
					if ctr.GetName() == "ctr1" {
						a.SetLinuxSeccompPolicy(
							func() *api.LinuxSeccomp {
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
								return api.FromOCILinuxSeccomp(&seccomp)
							}(),
						)
					}
					return a, nil, nil
				}

				validate = func(_ *mockPlugin, _ *api.ValidateContainerAdjustmentRequest) error {
					return nil
				}

				runtime = s.runtime
				plugins = s.plugins
				ctx     = context.Background()

				pod = &api.PodSandbox{
					Id:        "pod0",
					Name:      "pod0",
					Uid:       "uid0",
					Namespace: "default",
				}
				ctr0 = &api.Container{
					Id:           "ctr0",
					PodSandboxId: "pod0",
					Name:         "ctr0",
					State:        api.ContainerState_CONTAINER_CREATED,
				}
				ctr1 = &api.Container{
					Id:           "ctr1",
					PodSandboxId: "pod0",
					Name:         "ctr1",
					State:        api.ContainerState_CONTAINER_CREATED,
					Linux: &api.LinuxContainer{
						SeccompProfile: &api.SecurityProfile{
							ProfileType:  api.SecurityProfile_LOCALHOST,
							LocalhostRef: "/xyzzy/foobar",
						},
					},
				}
			)

			plugins[0].createContainer = create
			plugins[1].validateAdjustment = validate
			plugins[2].validateAdjustment = validate

			s.Startup()
			podReq := &api.RunPodSandboxRequest{Pod: pod}
			Expect(runtime.RunPodSandbox(ctx, podReq)).To(Succeed())

			ctrReq := &api.CreateContainerRequest{
				Pod:       pod,
				Container: ctr0,
			}
			reply, err := runtime.CreateContainer(ctx, ctrReq)
			Expect(reply).ToNot(BeNil())
			Expect(err).To(BeNil())

			ctrReq = &api.CreateContainerRequest{
				Pod:       pod,
				Container: ctr1,
			}
			reply, err = runtime.CreateContainer(ctx, ctrReq)
			Expect(err).ToNot(BeNil())
			Expect(reply).To(BeNil())
		})
	})

	When("default validator allows custom seccomp policy adjustment", func() {
		BeforeEach(func() {
			s.Prepare(
				&mockRuntime{
					options: []nri.Option{
						nri.WithDefaultValidator(
							&validator.DefaultValidatorConfig{
								Enable:                        true,
								RejectCustomSeccompAdjustment: false,
							},
						),
					},
				},
				&mockPlugin{idx: "00", name: "foo"},
				&mockPlugin{idx: "10", name: "validator1"},
				&mockPlugin{idx: "20", name: "validator2"},
			)
		})

		It("should not reject custom seccomp policy adjustment", func() {
			var (
				create = func(_ *mockPlugin, _ *api.PodSandbox, ctr *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
					a := &api.ContainerAdjustment{}
					if ctr.GetName() == "ctr1" {
						a.SetLinuxSeccompPolicy(
							func() *api.LinuxSeccomp {
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
								return api.FromOCILinuxSeccomp(&seccomp)
							}(),
						)
					}
					return a, nil, nil
				}

				validate = func(_ *mockPlugin, _ *api.ValidateContainerAdjustmentRequest) error {
					return nil
				}

				runtime = s.runtime
				plugins = s.plugins
				ctx     = context.Background()

				pod = &api.PodSandbox{
					Id:        "pod0",
					Name:      "pod0",
					Uid:       "uid0",
					Namespace: "default",
				}
				ctr0 = &api.Container{
					Id:           "ctr0",
					PodSandboxId: "pod0",
					Name:         "ctr0",
					State:        api.ContainerState_CONTAINER_CREATED,
				}
				ctr1 = &api.Container{
					Id:           "ctr1",
					PodSandboxId: "pod0",
					Name:         "ctr1",
					State:        api.ContainerState_CONTAINER_CREATED,
					Linux: &api.LinuxContainer{
						SeccompProfile: &api.SecurityProfile{
							ProfileType:  api.SecurityProfile_LOCALHOST,
							LocalhostRef: "/xyzzy/foobar",
						},
					},
				}
			)

			plugins[0].createContainer = create
			plugins[1].validateAdjustment = validate
			plugins[2].validateAdjustment = validate

			s.Startup()
			podReq := &api.RunPodSandboxRequest{Pod: pod}
			Expect(runtime.RunPodSandbox(ctx, podReq)).To(Succeed())

			ctrReq := &api.CreateContainerRequest{
				Pod:       pod,
				Container: ctr0,
			}
			reply, err := runtime.CreateContainer(ctx, ctrReq)
			Expect(reply).ToNot(BeNil())
			Expect(err).To(BeNil())

			ctrReq = &api.CreateContainerRequest{
				Pod:       pod,
				Container: ctr1,
			}
			reply, err = runtime.CreateContainer(ctx, ctrReq)
			Expect(reply).ToNot(BeNil())
			Expect(err).To(BeNil())
		})
	})

	When("default validator disallows unconfined seccomp policy adjustment", func() {
		BeforeEach(func() {
			s.Prepare(
				&mockRuntime{
					options: []nri.Option{
						nri.WithDefaultValidator(
							&validator.DefaultValidatorConfig{
								Enable:                            true,
								RejectUnconfinedSeccompAdjustment: true,
							},
						),
					},
				},
				&mockPlugin{idx: "00", name: "foo"},
				&mockPlugin{idx: "10", name: "validator1"},
				&mockPlugin{idx: "20", name: "validator2"},
			)
		})

		It("should reject unconfined seccomp policy adjustment", func() {
			var (
				create = func(_ *mockPlugin, _ *api.PodSandbox, ctr *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
					a := &api.ContainerAdjustment{}
					if ctr.GetName() == "ctr1" {
						a.SetLinuxSeccompPolicy(
							func() *api.LinuxSeccomp {
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
								return api.FromOCILinuxSeccomp(&seccomp)
							}(),
						)
					}
					return a, nil, nil
				}

				validate = func(_ *mockPlugin, _ *api.ValidateContainerAdjustmentRequest) error {
					return nil
				}

				runtime = s.runtime
				plugins = s.plugins
				ctx     = context.Background()

				pod = &api.PodSandbox{
					Id:        "pod0",
					Name:      "pod0",
					Uid:       "uid0",
					Namespace: "default",
				}
				ctr0 = &api.Container{
					Id:           "ctr0",
					PodSandboxId: "pod0",
					Name:         "ctr0",
					State:        api.ContainerState_CONTAINER_CREATED,
				}
				ctr1 = &api.Container{
					Id:           "ctr1",
					PodSandboxId: "pod0",
					Name:         "ctr1",
					State:        api.ContainerState_CONTAINER_CREATED,
					Linux: &api.LinuxContainer{
						SeccompProfile: &api.SecurityProfile{
							ProfileType: api.SecurityProfile_UNCONFINED,
						},
					},
				}
			)

			plugins[0].createContainer = create
			plugins[1].validateAdjustment = validate
			plugins[2].validateAdjustment = validate

			s.Startup()
			podReq := &api.RunPodSandboxRequest{Pod: pod}
			Expect(runtime.RunPodSandbox(ctx, podReq)).To(Succeed())

			ctrReq := &api.CreateContainerRequest{
				Pod:       pod,
				Container: ctr0,
			}
			reply, err := runtime.CreateContainer(ctx, ctrReq)
			Expect(reply).ToNot(BeNil())
			Expect(err).To(BeNil())

			ctrReq = &api.CreateContainerRequest{
				Pod:       pod,
				Container: ctr1,
			}
			reply, err = runtime.CreateContainer(ctx, ctrReq)
			Expect(err).ToNot(BeNil())
			Expect(reply).To(BeNil())
		})
	})

	When("default validator allows unconfined seccomp policy adjustment", func() {
		BeforeEach(func() {
			s.Prepare(
				&mockRuntime{
					options: []nri.Option{
						nri.WithDefaultValidator(
							&validator.DefaultValidatorConfig{
								Enable:                            true,
								RejectUnconfinedSeccompAdjustment: false,
							},
						),
					},
				},
				&mockPlugin{idx: "00", name: "foo"},
				&mockPlugin{idx: "10", name: "validator1"},
				&mockPlugin{idx: "20", name: "validator2"},
			)
		})

		It("should not reject unconfined seccomp policy adjustment", func() {
			var (
				create = func(_ *mockPlugin, _ *api.PodSandbox, ctr *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
					a := &api.ContainerAdjustment{}
					if ctr.GetName() == "ctr1" {
						a.SetLinuxSeccompPolicy(
							func() *api.LinuxSeccomp {
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
								return api.FromOCILinuxSeccomp(&seccomp)
							}(),
						)
					}
					return a, nil, nil
				}

				validate = func(_ *mockPlugin, _ *api.ValidateContainerAdjustmentRequest) error {
					return nil
				}

				runtime = s.runtime
				plugins = s.plugins
				ctx     = context.Background()

				pod = &api.PodSandbox{
					Id:        "pod0",
					Name:      "pod0",
					Uid:       "uid0",
					Namespace: "default",
				}
				ctr0 = &api.Container{
					Id:           "ctr0",
					PodSandboxId: "pod0",
					Name:         "ctr0",
					State:        api.ContainerState_CONTAINER_CREATED,
				}
				ctr1 = &api.Container{
					Id:           "ctr1",
					PodSandboxId: "pod0",
					Name:         "ctr1",
					State:        api.ContainerState_CONTAINER_CREATED,
					Linux: &api.LinuxContainer{
						SeccompProfile: &api.SecurityProfile{
							ProfileType: api.SecurityProfile_UNCONFINED,
						},
					},
				}
			)

			plugins[0].createContainer = create
			plugins[1].validateAdjustment = validate
			plugins[2].validateAdjustment = validate

			s.Startup()
			podReq := &api.RunPodSandboxRequest{Pod: pod}
			Expect(runtime.RunPodSandbox(ctx, podReq)).To(Succeed())

			ctrReq := &api.CreateContainerRequest{
				Pod:       pod,
				Container: ctr0,
			}
			reply, err := runtime.CreateContainer(ctx, ctrReq)
			Expect(reply).ToNot(BeNil())
			Expect(err).To(BeNil())

			ctrReq = &api.CreateContainerRequest{
				Pod:       pod,
				Container: ctr1,
			}
			reply, err = runtime.CreateContainer(ctx, ctrReq)
			Expect(reply).ToNot(BeNil())
			Expect(err).To(BeNil())
		})
	})

	When("the default validator is enabled and namespace adjustment is disabled", func() {
		BeforeEach(func() {
			s.Prepare(
				&mockRuntime{
					options: []nri.Option{
						nri.WithDefaultValidator(
							&validator.DefaultValidatorConfig{
								Enable:                    true,
								RejectNamespaceAdjustment: true,
							},
						),
					},
				},
				&mockPlugin{idx: "00", name: "foo"},
				&mockPlugin{idx: "10", name: "validator1"},
				&mockPlugin{idx: "20", name: "validator2"},
			)
		})

		It("should reject namespace adjustment", func() {
			var (
				create = func(_ *mockPlugin, _ *api.PodSandbox, ctr *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
					a := &api.ContainerAdjustment{}
					if ctr.GetName() == "ctr1" {
						a.AddOrReplaceNamespace(
							&api.LinuxNamespace{
								Type: "cgroup",
								Path: "/",
							},
						)
					}
					return a, nil, nil
				}

				validate = func(_ *mockPlugin, _ *api.ValidateContainerAdjustmentRequest) error {
					return nil
				}

				runtime = s.runtime
				plugins = s.plugins
				ctx     = context.Background()

				pod = &api.PodSandbox{
					Id:        "pod0",
					Name:      "pod0",
					Uid:       "uid0",
					Namespace: "default",
				}
				ctr0 = &api.Container{
					Id:           "ctr0",
					PodSandboxId: "pod0",
					Name:         "ctr0",
					State:        api.ContainerState_CONTAINER_CREATED,
				}
				ctr1 = &api.Container{
					Id:           "ctr1",
					PodSandboxId: "pod0",
					Name:         "ctr1",
					State:        api.ContainerState_CONTAINER_CREATED,
				}
			)

			plugins[0].createContainer = create
			plugins[1].validateAdjustment = validate
			plugins[2].validateAdjustment = validate

			s.Startup()
			podReq := &api.RunPodSandboxRequest{Pod: pod}
			Expect(runtime.RunPodSandbox(ctx, podReq)).To(Succeed())

			ctrReq := &api.CreateContainerRequest{
				Pod:       pod,
				Container: ctr0,
			}
			reply, err := runtime.CreateContainer(ctx, ctrReq)
			Expect(reply).ToNot(BeNil())
			Expect(err).To(BeNil())

			ctrReq = &api.CreateContainerRequest{
				Pod:       pod,
				Container: ctr1,
			}
			reply, err = runtime.CreateContainer(ctx, ctrReq)
			Expect(err).ToNot(BeNil())
			Expect(reply).To(BeNil())
		})
	})

	When("the default validator is enabled with some required plugins", func() {
		const AnnotationDomain = plugin.AnnotationDomain
		BeforeEach(func() {
			s.Prepare(
				&mockRuntime{
					options: []nri.Option{
						nri.WithDefaultValidator(
							&validator.DefaultValidatorConfig{
								Enable: true,
								RequiredPlugins: []string{
									"foo",
									"bar",
								},
								TolerateMissingAnnotation: "tolerate-missing-plugins." + AnnotationDomain,
							},
						),
					},
				},
				&mockPlugin{idx: "00", name: "foo"},
			)
		})

		It("should not allow container creation if required plugins are missing", func() {
			var (
				runtime = s.runtime
				ctx     = context.Background()

				pod = &api.PodSandbox{
					Id:        "pod0",
					Name:      "pod0",
					Uid:       "uid0",
					Namespace: "default",
				}
			)

			s.Startup()
			podReq := &api.RunPodSandboxRequest{Pod: pod}
			Expect(runtime.RunPodSandbox(ctx, podReq)).To(Succeed())

			ctrReq := &api.CreateContainerRequest{
				Pod: pod,
				Container: &api.Container{
					Id:           "ctr0",
					PodSandboxId: "pod0",
					Name:         "ctr0",
					State:        api.ContainerState_CONTAINER_CREATED,
				},
			}
			reply, err := runtime.CreateContainer(ctx, ctrReq)
			Expect(reply).To(BeNil())
			Expect(err).ToNot(BeNil())
		})

		It("should allow container creation, if missing plugins are tolerated", func() {
			var (
				runtime = s.runtime
				ctx     = context.Background()

				pod = &api.PodSandbox{
					Id:        "pod0",
					Name:      "pod0",
					Uid:       "uid0",
					Namespace: "default",
					Annotations: map[string]string{
						"tolerate-missing-plugins." + AnnotationDomain + "/container.ctr0": "true",
					},
				}
			)

			s.Startup()
			podReq := &api.RunPodSandboxRequest{Pod: pod}
			Expect(runtime.RunPodSandbox(ctx, podReq)).To(Succeed())

			ctrReq := &api.CreateContainerRequest{
				Pod: pod,
				Container: &api.Container{
					Id:           "ctr0",
					PodSandboxId: "pod0",
					Name:         "ctr0",
					State:        api.ContainerState_CONTAINER_CREATED,
				},
			}
			reply, err := runtime.CreateContainer(ctx, ctrReq)
			Expect(reply).ToNot(BeNil())
			Expect(err).To(BeNil())
		})

		It("should allow container creation if all required plugins are present", func() {
			var (
				runtime = s.runtime
				ctx     = context.Background()

				pod = &api.PodSandbox{
					Id:        "pod0",
					Name:      "pod0",
					Uid:       "uid0",
					Namespace: "default",
				}
			)

			s.Startup()
			podReq := &api.RunPodSandboxRequest{Pod: pod}
			Expect(runtime.RunPodSandbox(ctx, podReq)).To(Succeed())

			s.StartPlugins(&mockPlugin{idx: "10", name: "bar"})
			s.WaitForPluginsToSync(s.plugin("10-bar"))

			ctrReq := &api.CreateContainerRequest{
				Pod: pod,
				Container: &api.Container{
					Id:           "ctr0",
					PodSandboxId: "pod0",
					Name:         "ctr0",
					State:        api.ContainerState_CONTAINER_CREATED,
				},
			}
			reply, err := runtime.CreateContainer(ctx, ctrReq)
			Expect(reply).ToNot(BeNil())
			Expect(err).To(BeNil())
		})

		It("should not allow container creation if annotated required plugins are missing", func() {
			var (
				runtime = s.runtime
				ctx     = context.Background()

				pod = &api.PodSandbox{
					Id:        "pod0",
					Name:      "pod0",
					Uid:       "uid0",
					Namespace: "default",
					Annotations: map[string]string{
						"required-plugins." + AnnotationDomain + "/container.ctr0": "[ \"xyzzy\" ]",
					},
				}
			)

			s.Startup()
			podReq := &api.RunPodSandboxRequest{Pod: pod}
			Expect(runtime.RunPodSandbox(ctx, podReq)).To(Succeed())

			s.StartPlugins(&mockPlugin{idx: "10", name: "bar"})
			s.WaitForPluginsToSync(s.plugin("10-bar"))

			ctrReq := &api.CreateContainerRequest{
				Pod: pod,
				Container: &api.Container{
					Id:           "ctr0",
					PodSandboxId: "pod0",
					Name:         "ctr0",
					State:        api.ContainerState_CONTAINER_CREATED,
				},
			}
			reply, err := runtime.CreateContainer(ctx, ctrReq)
			Expect(reply).To(BeNil())
			Expect(err).ToNot(BeNil())

			s.StartPlugins(&mockPlugin{idx: "20", name: "xyzzy"})
			s.WaitForPluginsToSync(s.plugin("20-xyzzy"))

			ctrReq = &api.CreateContainerRequest{
				Pod: pod,
				Container: &api.Container{
					Id:           "ctr0",
					PodSandboxId: "pod0",
					Name:         "ctr0",
					State:        api.ContainerState_CONTAINER_CREATED,
				},
			}
			reply, err = runtime.CreateContainer(ctx, ctrReq)
			Expect(reply).ToNot(BeNil())
			Expect(err).To(BeNil())
		})

	})

})

// --------------------------------------------

var _ = Describe("Plugin container updates during creation", func() {
	var (
		s = &Suite{}
	)

	update := func(subject, which string, p *mockPlugin, _ *api.PodSandbox, ctr *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
		plugin := p.idx + "-" + p.name

		if which != plugin && which != "*" && which != "both" {
			return nil, nil, nil
		}
		if ctr.Name != "ctr1" {
			return nil, nil, nil
		}

		u := &api.ContainerUpdate{}
		u.SetContainerId("ctr0")

		switch subject {
		case "resources/cpu":
			u.SetLinuxCPUShares(123)
			u.SetLinuxCPUQuota(456)
			u.SetLinuxCPUPeriod(789)
			u.SetLinuxCPURealtimeRuntime(321)
			u.SetLinuxCPURealtimePeriod(654)
			u.SetLinuxCPUSetCPUs("0-1")
			u.SetLinuxCPUSetMems("2-3")

		case "resources/memory":
			u.SetLinuxMemoryLimit(1234000)
			u.SetLinuxMemoryReservation(4000)
			u.SetLinuxMemorySwap(34000)
			u.SetLinuxMemoryKernel(30000)
			u.SetLinuxMemoryKernelTCP(2000)
			u.SetLinuxMemorySwappiness(987)
			u.SetLinuxMemoryDisableOomKiller()
			u.SetLinuxMemoryUseHierarchy()

		case "resources/classes":
			u.SetLinuxRDTClass(plugin)
			u.SetLinuxBlockIOClass(plugin)

		case "resources/hugepagelimits":
			u.AddLinuxHugepageLimit("1M", 4096)
			u.AddLinuxHugepageLimit("4M", 1024)

		case "resources/unified":
			u.AddLinuxUnified("resource.1", "value1")
			u.AddLinuxUnified("resource.2", "value2")
		}

		return nil, []*api.ContainerUpdate{u}, nil
	}

	AfterEach(func() {
		s.Cleanup()
	})

	When("there is a single plugin", func() {
		BeforeEach(func() {
			s.Prepare(&mockRuntime{}, &mockPlugin{idx: "00", name: "test"})
		})

		DescribeTable("should be successfully collected without conflicts",
			func(subject string, expected *api.ContainerUpdate) {
				var (
					runtime = s.runtime
					plugin  = s.plugins[0]
					ctx     = context.Background()

					pod0 = &api.PodSandbox{
						Id:        "pod0",
						Name:      "pod0",
						Uid:       "uid0",
						Namespace: "default",
					}
					ctr0 = &api.Container{
						Id:           "ctr0",
						PodSandboxId: "pod0",
						Name:         "ctr0",
						State:        api.ContainerState_CONTAINER_CREATED, // XXX FIXME-kludge
					}
					pod1 = &api.PodSandbox{
						Id:        "pod1",
						Name:      "pod1",
						Uid:       "uid1",
						Namespace: "default",
					}
					ctr1 = &api.Container{
						Id:           "ctr1",
						PodSandboxId: "pod1",
						Name:         "ctr1",
						State:        api.ContainerState_CONTAINER_CREATED, // XXX FIXME-kludge
					}

					reply *api.CreateContainerResponse
				)

				create := func(p *mockPlugin, pod *api.PodSandbox, ctr *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
					plugin := p.idx + "-" + p.name
					return update(subject, plugin, p, pod, ctr)
				}

				plugin.createContainer = create

				s.Startup()

				podReq := &api.RunPodSandboxRequest{Pod: pod0}
				Expect(runtime.RunPodSandbox(ctx, podReq)).To(Succeed())
				ctrReq := &api.CreateContainerRequest{
					Pod:       pod0,
					Container: ctr0,
				}
				_, err := runtime.CreateContainer(ctx, ctrReq)
				Expect(err).To(BeNil())

				podReq = &api.RunPodSandboxRequest{Pod: pod1}
				Expect(runtime.RunPodSandbox(ctx, podReq)).To(Succeed())
				ctrReq = &api.CreateContainerRequest{
					Pod:       pod1,
					Container: ctr1,
				}
				reply, err = runtime.CreateContainer(ctx, ctrReq)
				Expect(err).To(BeNil())

				Expect(len(reply.Update)).To(Equal(1))
				expected.ContainerId = reply.Update[0].ContainerId
				Expect(protoEqual(reply.Update[0].Strip(), expected.Strip())).Should(BeTrue(),
					protoDiff(reply.Update[0], expected))
			},

			Entry("update CPU resources", "resources/cpu",
				&api.ContainerUpdate{
					Linux: &api.LinuxContainerUpdate{
						Resources: &api.LinuxResources{
							Cpu: &api.LinuxCPU{
								Shares:          api.UInt64(123),
								Quota:           api.Int64(456),
								Period:          api.UInt64(789),
								RealtimeRuntime: api.Int64(321),
								RealtimePeriod:  api.UInt64(654),
								Cpus:            "0-1",
								Mems:            "2-3",
							},
						},
					},
				},
			),
			Entry("update memory resources", "resources/memory",
				&api.ContainerUpdate{
					Linux: &api.LinuxContainerUpdate{
						Resources: &api.LinuxResources{
							Memory: &api.LinuxMemory{
								Limit:            api.Int64(1234000),
								Reservation:      api.Int64(4000),
								Swap:             api.Int64(34000),
								Kernel:           api.Int64(30000),
								KernelTcp:        api.Int64(2000),
								Swappiness:       api.UInt64(987),
								DisableOomKiller: api.Bool(true),
								UseHierarchy:     api.Bool(true),
							},
						},
					},
				},
			),
			Entry("update class-based resources", "resources/classes",
				&api.ContainerUpdate{
					Linux: &api.LinuxContainerUpdate{
						Resources: &api.LinuxResources{
							RdtClass:     api.String("00-test"),
							BlockioClass: api.String("00-test"),
						},
					},
				},
			),
			Entry("update hugepage limits", "resources/hugepagelimits",
				&api.ContainerUpdate{
					Linux: &api.LinuxContainerUpdate{
						Resources: &api.LinuxResources{
							HugepageLimits: []*api.HugepageLimit{
								{
									PageSize: "1M",
									Limit:    4096,
								},
								{
									PageSize: "4M",
									Limit:    1024,
								},
							},
						},
					},
				},
			),
			Entry("update cgroupv2 unified resources", "resources/unified",
				&api.ContainerUpdate{
					Linux: &api.LinuxContainerUpdate{
						Resources: &api.LinuxResources{
							Unified: map[string]string{
								"resource.1": "value1",
								"resource.2": "value2",
							},
						},
					},
				},
			),
		)
	})

	When("there are multiple plugins", func() {
		BeforeEach(func() {
			s.Prepare(
				&mockRuntime{},
				&mockPlugin{idx: "10", name: "foo"},
				&mockPlugin{idx: "00", name: "bar"},
			)
		})

		DescribeTable("should fail with conflicts, successfully collected otherwise",
			func(subject string, which string, expected *api.ContainerUpdate) {
				var (
					runtime = s.runtime
					plugins = s.plugins
					ctx     = context.Background()

					pod0 = &api.PodSandbox{
						Id:        "pod0",
						Name:      "pod0",
						Uid:       "uid0",
						Namespace: "default",
					}
					ctr0 = &api.Container{
						Id:           "ctr0",
						PodSandboxId: "pod0",
						Name:         "ctr0",
						State:        api.ContainerState_CONTAINER_CREATED, // XXX FIXME-kludge
					}
					pod1 = &api.PodSandbox{
						Id:        "pod1",
						Name:      "pod1",
						Uid:       "uid1",
						Namespace: "default",
					}
					ctr1 = &api.Container{
						Id:           "ctr1",
						PodSandboxId: "pod1",
						Name:         "ctr1",
						State:        api.ContainerState_CONTAINER_CREATED, // XXX FIXME-kludge
					}

					reply *api.CreateContainerResponse
				)

				create := func(p *mockPlugin, pod *api.PodSandbox, ctr *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
					return update(subject, which, p, pod, ctr)
				}

				plugins[0].createContainer = create
				plugins[1].createContainer = create

				s.Startup()

				podReq := &api.RunPodSandboxRequest{Pod: pod0}
				Expect(runtime.RunPodSandbox(ctx, podReq)).To(Succeed())
				ctrReq := &api.CreateContainerRequest{
					Pod:       pod0,
					Container: ctr0,
				}
				_, err := runtime.CreateContainer(ctx, ctrReq)
				Expect(err).To(BeNil())

				podReq = &api.RunPodSandboxRequest{Pod: pod1}
				Expect(runtime.RunPodSandbox(ctx, podReq)).To(Succeed())
				ctrReq = &api.CreateContainerRequest{
					Pod:       pod1,
					Container: ctr1,
				}
				reply, err = runtime.CreateContainer(ctx, ctrReq)
				if which == "both" {
					Expect(err).ToNot(BeNil())
				} else {
					Expect(err).To(BeNil())
					Expect(len(reply.Update)).To(Equal(1))
					expected.ContainerId = reply.Update[0].ContainerId
					Expect(protoEqual(reply.Update[0].Strip(), expected.Strip())).Should(BeTrue(),
						protoDiff(reply.Update[0], expected))
				}
			},

			Entry("update CPU resources", "resources/cpu", "both", nil),
			Entry("update CPU resources", "resources/cpu", "10-foo",
				&api.ContainerUpdate{
					Linux: &api.LinuxContainerUpdate{
						Resources: &api.LinuxResources{
							Cpu: &api.LinuxCPU{
								Shares:          api.UInt64(123),
								Quota:           api.Int64(456),
								Period:          api.UInt64(789),
								RealtimeRuntime: api.Int64(321),
								RealtimePeriod:  api.UInt64(654),
								Cpus:            "0-1",
								Mems:            "2-3",
							},
						},
					},
				},
			),
			Entry("update memory resources", "resources/memory", "both", nil),
			Entry("update memory resources", "resources/memory", "10-foo",
				&api.ContainerUpdate{
					Linux: &api.LinuxContainerUpdate{
						Resources: &api.LinuxResources{
							Memory: &api.LinuxMemory{
								Limit:            api.Int64(1234000),
								Reservation:      api.Int64(4000),
								Swap:             api.Int64(34000),
								Kernel:           api.Int64(30000),
								KernelTcp:        api.Int64(2000),
								Swappiness:       api.UInt64(987),
								DisableOomKiller: api.Bool(true),
								UseHierarchy:     api.Bool(true),
							},
						},
					},
				},
			),
			Entry("update class-based resources", "resources/classes", "both", nil),
			Entry("update class-based resources", "resources/classes", "10-foo",
				&api.ContainerUpdate{
					Linux: &api.LinuxContainerUpdate{
						Resources: &api.LinuxResources{
							RdtClass:     api.String("10-foo"),
							BlockioClass: api.String("10-foo"),
						},
					},
				},
			),
			Entry("update hugepage limits", "resources/hugepagelimits", "both", nil),
			Entry("update hugepage limits", "resources/hugepagelimits", "10-foo",
				&api.ContainerUpdate{
					Linux: &api.LinuxContainerUpdate{
						Resources: &api.LinuxResources{
							HugepageLimits: []*api.HugepageLimit{
								{
									PageSize: "1M",
									Limit:    4096,
								},
								{
									PageSize: "4M",
									Limit:    1024,
								},
							},
						},
					},
				},
			),
			Entry("update cgroupv2 unified resources", "resources/unified", "both", nil),
			Entry("update cgroupv2 unified resources", "resources/unified", "10-foo",
				&api.ContainerUpdate{
					Linux: &api.LinuxContainerUpdate{
						Resources: &api.LinuxResources{
							Unified: map[string]string{
								"resource.1": "value1",
								"resource.2": "value2",
							},
						},
					},
				},
			),
		)
	})
})

var _ = Describe("Solicited container updates by plugins", func() {
	var (
		s = &Suite{}
	)

	update := func(subject, which string, p *mockPlugin, _ *api.PodSandbox, ctr *api.Container, _, _ *api.LinuxResources) ([]*api.ContainerUpdate, error) {
		plugin := p.idx + "-" + p.name

		if which != plugin && which != "*" && which != "both" {
			return nil, nil
		}
		if ctr.Name != "ctr0" {
			return nil, nil
		}

		u := &api.ContainerUpdate{}
		u.SetContainerId(ctr.Id)

		switch subject {
		case "resources/cpu":
			u.SetLinuxCPUShares(123)
			u.SetLinuxCPUQuota(456)
			u.SetLinuxCPUPeriod(789)
			u.SetLinuxCPURealtimeRuntime(321)
			u.SetLinuxCPURealtimePeriod(654)
			u.SetLinuxCPUSetCPUs("0-1")
			u.SetLinuxCPUSetMems("2-3")

		case "resources/memory":
			u.SetLinuxMemoryLimit(1234000)
			u.SetLinuxMemoryReservation(4000)
			u.SetLinuxMemorySwap(34000)
			u.SetLinuxMemoryKernel(30000)
			u.SetLinuxMemoryKernelTCP(2000)
			u.SetLinuxMemorySwappiness(987)
			u.SetLinuxMemoryDisableOomKiller()
			u.SetLinuxMemoryUseHierarchy()

		case "resources/classes":
			u.SetLinuxRDTClass(plugin)
			u.SetLinuxBlockIOClass(plugin)

		case "resources/hugepagelimits":
			u.AddLinuxHugepageLimit("1M", 4096)
			u.AddLinuxHugepageLimit("4M", 1024)

		case "resources/unified":
			u.AddLinuxUnified("resource.1", "value1")
			u.AddLinuxUnified("resource.2", "value2")
		}

		return []*api.ContainerUpdate{u}, nil
	}

	AfterEach(func() {
		s.Cleanup()
	})

	When("there is a single plugin", func() {
		BeforeEach(func() {
			s.Prepare(&mockRuntime{}, &mockPlugin{idx: "00", name: "test"})
		})

		DescribeTable("should be successfully collected without conflicts",
			func(subject string, expected *api.ContainerUpdate) {
				var (
					runtime = s.runtime
					plugin  = s.plugins[0]
					ctx     = context.Background()

					pod0 = &api.PodSandbox{
						Id:        "pod0",
						Name:      "pod0",
						Uid:       "uid0",
						Namespace: "default",
					}
					ctr0 = &api.Container{
						Id:           "ctr0",
						PodSandboxId: "pod0",
						Name:         "ctr0",
						State:        api.ContainerState_CONTAINER_CREATED, // XXX FIXME-kludge
					}

					reply *api.UpdateContainerResponse
				)

				updateContainer := func(p *mockPlugin, pod *api.PodSandbox, ctr *api.Container, r *api.LinuxResources) ([]*api.ContainerUpdate, error) {
					plugin := p.idx + "-" + p.name
					return update(subject, plugin, p, pod, ctr, r, nil)
				}
				plugin.updateContainer = updateContainer

				s.Startup()

				podReq := &api.RunPodSandboxRequest{Pod: pod0}
				Expect(runtime.RunPodSandbox(ctx, podReq)).To(Succeed())
				ctrReq := &api.CreateContainerRequest{
					Pod:       pod0,
					Container: ctr0,
				}
				_, err := runtime.CreateContainer(ctx, ctrReq)
				Expect(err).To(BeNil())

				updReq := &api.UpdateContainerRequest{
					Pod:       pod0,
					Container: ctr0,
					LinuxResources: &api.LinuxResources{
						Cpu: &api.LinuxCPU{
							Shares:          api.UInt64(999),
							Quota:           api.Int64(888),
							Period:          api.UInt64(777),
							RealtimeRuntime: api.Int64(666),
							RealtimePeriod:  api.UInt64(555),
							Cpus:            "444",
							Mems:            "333",
						},
						Memory: &api.LinuxMemory{
							Limit:            api.Int64(9999),
							Reservation:      api.Int64(8888),
							Swap:             api.Int64(7777),
							Kernel:           api.Int64(6666),
							KernelTcp:        api.Int64(5555),
							Swappiness:       api.UInt64(444),
							DisableOomKiller: api.Bool(false),
							UseHierarchy:     api.Bool(false),
						},
					},
				}
				reply, err = runtime.UpdateContainer(ctx, updReq)

				Expect(len(reply.Update)).To(Equal(1))
				Expect(err).To(BeNil())
				expected.ContainerId = reply.Update[0].ContainerId
				Expect(protoEqual(reply.Update[0].Strip(), expected.Strip())).Should(BeTrue(),
					protoDiff(reply.Update[0], expected))
			},

			Entry("update CPU resources", "resources/cpu",
				&api.ContainerUpdate{
					Linux: &api.LinuxContainerUpdate{
						Resources: &api.LinuxResources{
							Cpu: &api.LinuxCPU{
								Shares:          api.UInt64(123),
								Quota:           api.Int64(456),
								Period:          api.UInt64(789),
								RealtimeRuntime: api.Int64(321),
								RealtimePeriod:  api.UInt64(654),
								Cpus:            "0-1",
								Mems:            "2-3",
							},
							Memory: &api.LinuxMemory{
								Limit:            api.Int64(9999),
								Reservation:      api.Int64(8888),
								Swap:             api.Int64(7777),
								Kernel:           api.Int64(6666),
								KernelTcp:        api.Int64(5555),
								Swappiness:       api.UInt64(444),
								DisableOomKiller: api.Bool(false),
								UseHierarchy:     api.Bool(false),
							},
						},
					},
				},
			),
			Entry("update memory resources", "resources/memory",
				&api.ContainerUpdate{
					Linux: &api.LinuxContainerUpdate{
						Resources: &api.LinuxResources{
							Cpu: &api.LinuxCPU{
								Shares:          api.UInt64(999),
								Quota:           api.Int64(888),
								Period:          api.UInt64(777),
								RealtimeRuntime: api.Int64(666),
								RealtimePeriod:  api.UInt64(555),
								Cpus:            "444",
								Mems:            "333",
							},
							Memory: &api.LinuxMemory{
								Limit:            api.Int64(1234000),
								Reservation:      api.Int64(4000),
								Swap:             api.Int64(34000),
								Kernel:           api.Int64(30000),
								KernelTcp:        api.Int64(2000),
								Swappiness:       api.UInt64(987),
								DisableOomKiller: api.Bool(true),
								UseHierarchy:     api.Bool(true),
							},
						},
					},
				},
			),
			Entry("update class-based resources", "resources/classes",
				&api.ContainerUpdate{
					Linux: &api.LinuxContainerUpdate{
						Resources: &api.LinuxResources{
							Cpu: &api.LinuxCPU{
								Shares:          api.UInt64(999),
								Quota:           api.Int64(888),
								Period:          api.UInt64(777),
								RealtimeRuntime: api.Int64(666),
								RealtimePeriod:  api.UInt64(555),
								Cpus:            "444",
								Mems:            "333",
							},
							Memory: &api.LinuxMemory{
								Limit:            api.Int64(9999),
								Reservation:      api.Int64(8888),
								Swap:             api.Int64(7777),
								Kernel:           api.Int64(6666),
								KernelTcp:        api.Int64(5555),
								Swappiness:       api.UInt64(444),
								DisableOomKiller: api.Bool(false),
								UseHierarchy:     api.Bool(false),
							},

							RdtClass:     api.String("00-test"),
							BlockioClass: api.String("00-test"),
						},
					},
				},
			),
			Entry("update hugepage limits", "resources/hugepagelimits",
				&api.ContainerUpdate{
					Linux: &api.LinuxContainerUpdate{
						Resources: &api.LinuxResources{
							Cpu: &api.LinuxCPU{
								Shares:          api.UInt64(999),
								Quota:           api.Int64(888),
								Period:          api.UInt64(777),
								RealtimeRuntime: api.Int64(666),
								RealtimePeriod:  api.UInt64(555),
								Cpus:            "444",
								Mems:            "333",
							},
							Memory: &api.LinuxMemory{
								Limit:            api.Int64(9999),
								Reservation:      api.Int64(8888),
								Swap:             api.Int64(7777),
								Kernel:           api.Int64(6666),
								KernelTcp:        api.Int64(5555),
								Swappiness:       api.UInt64(444),
								DisableOomKiller: api.Bool(false),
								UseHierarchy:     api.Bool(false),
							},
							HugepageLimits: []*api.HugepageLimit{
								{
									PageSize: "1M",
									Limit:    4096,
								},
								{
									PageSize: "4M",
									Limit:    1024,
								},
							},
						},
					},
				},
			),
			Entry("update cgroupv2 unified resources", "resources/unified",
				&api.ContainerUpdate{
					Linux: &api.LinuxContainerUpdate{
						Resources: &api.LinuxResources{
							Cpu: &api.LinuxCPU{
								Shares:          api.UInt64(999),
								Quota:           api.Int64(888),
								Period:          api.UInt64(777),
								RealtimeRuntime: api.Int64(666),
								RealtimePeriod:  api.UInt64(555),
								Cpus:            "444",
								Mems:            "333",
							},
							Memory: &api.LinuxMemory{
								Limit:            api.Int64(9999),
								Reservation:      api.Int64(8888),
								Swap:             api.Int64(7777),
								Kernel:           api.Int64(6666),
								KernelTcp:        api.Int64(5555),
								Swappiness:       api.UInt64(444),
								DisableOomKiller: api.Bool(false),
								UseHierarchy:     api.Bool(false),
							},

							Unified: map[string]string{
								"resource.1": "value1",
								"resource.2": "value2",
							},
						},
					},
				},
			),
		)
	})

	When("there are multiple plugins", func() {
		BeforeEach(func() {
			s.Prepare(
				&mockRuntime{},
				&mockPlugin{idx: "10", name: "foo"},
				&mockPlugin{idx: "00", name: "bar"},
			)
		})

		DescribeTable("should fail with conflicts, successfully collected otherwise",
			func(subject string, which string, expected *api.ContainerUpdate) {
				var (
					runtime = s.runtime
					plugins = s.plugins
					ctx     = context.Background()

					pod0 = &api.PodSandbox{
						Id:        "pod0",
						Name:      "pod0",
						Uid:       "uid0",
						Namespace: "default",
					}
					ctr0 = &api.Container{
						Id:           "ctr0",
						PodSandboxId: "pod0",
						Name:         "ctr0",
						State:        api.ContainerState_CONTAINER_CREATED, // XXX FIXME-kludge
					}

					reply *api.UpdateContainerResponse
				)

				updateContainer := func(p *mockPlugin, pod *api.PodSandbox, ctr *api.Container, r *api.LinuxResources) ([]*api.ContainerUpdate, error) {
					return update(subject, which, p, pod, ctr, r, nil)
				}

				plugins[0].updateContainer = updateContainer
				plugins[1].updateContainer = updateContainer

				s.Startup()

				podReq := &api.RunPodSandboxRequest{Pod: pod0}
				Expect(runtime.RunPodSandbox(ctx, podReq)).To(Succeed())
				ctrReq := &api.CreateContainerRequest{
					Pod:       pod0,
					Container: ctr0,
				}
				_, err := runtime.CreateContainer(ctx, ctrReq)
				Expect(err).To(BeNil())

				updReq := &api.UpdateContainerRequest{
					Pod:       pod0,
					Container: ctr0,
					LinuxResources: &api.LinuxResources{
						Cpu: &api.LinuxCPU{
							Shares:          api.UInt64(999),
							Quota:           api.Int64(888),
							Period:          api.UInt64(777),
							RealtimeRuntime: api.Int64(666),
							RealtimePeriod:  api.UInt64(555),
							Cpus:            "444",
							Mems:            "333",
						},
						Memory: &api.LinuxMemory{
							Limit:            api.Int64(9999),
							Reservation:      api.Int64(8888),
							Swap:             api.Int64(7777),
							Kernel:           api.Int64(6666),
							KernelTcp:        api.Int64(5555),
							Swappiness:       api.UInt64(444),
							DisableOomKiller: api.Bool(false),
							UseHierarchy:     api.Bool(false),
						},
					},
				}
				reply, err = runtime.UpdateContainer(ctx, updReq)
				if which == "both" {
					Expect(err).ToNot(BeNil())
				} else {
					Expect(err).To(BeNil())
					Expect(len(reply.Update)).To(Equal(1))
					expected.ContainerId = reply.Update[0].ContainerId
					Expect(protoEqual(reply.Update[0].Strip(), expected.Strip())).Should(BeTrue(),
						protoDiff(reply.Update[0], expected))

				}
			},

			Entry("update CPU resources", "resources/cpu", "both", nil),
			Entry("update CPU resources", "resources/cpu", "10-foo",
				&api.ContainerUpdate{
					Linux: &api.LinuxContainerUpdate{
						Resources: &api.LinuxResources{
							Cpu: &api.LinuxCPU{
								Shares:          api.UInt64(123),
								Quota:           api.Int64(456),
								Period:          api.UInt64(789),
								RealtimeRuntime: api.Int64(321),
								RealtimePeriod:  api.UInt64(654),
								Cpus:            "0-1",
								Mems:            "2-3",
							},
							Memory: &api.LinuxMemory{
								Limit:            api.Int64(9999),
								Reservation:      api.Int64(8888),
								Swap:             api.Int64(7777),
								Kernel:           api.Int64(6666),
								KernelTcp:        api.Int64(5555),
								Swappiness:       api.UInt64(444),
								DisableOomKiller: api.Bool(false),
								UseHierarchy:     api.Bool(false),
							},
						},
					},
				},
			),
			Entry("update memory resources", "resources/memory", "both", nil),
			Entry("update memory resources", "resources/memory", "10-foo",
				&api.ContainerUpdate{
					Linux: &api.LinuxContainerUpdate{
						Resources: &api.LinuxResources{
							Cpu: &api.LinuxCPU{
								Shares:          api.UInt64(999),
								Quota:           api.Int64(888),
								Period:          api.UInt64(777),
								RealtimeRuntime: api.Int64(666),
								RealtimePeriod:  api.UInt64(555),
								Cpus:            "444",
								Mems:            "333",
							},
							Memory: &api.LinuxMemory{
								Limit:            api.Int64(1234000),
								Reservation:      api.Int64(4000),
								Swap:             api.Int64(34000),
								Kernel:           api.Int64(30000),
								KernelTcp:        api.Int64(2000),
								Swappiness:       api.UInt64(987),
								DisableOomKiller: api.Bool(true),
								UseHierarchy:     api.Bool(true),
							},
						},
					},
				},
			),
			Entry("update class-based resources", "resources/classes", "both", nil),
			Entry("update class-based resources", "resources/classes", "10-foo",
				&api.ContainerUpdate{
					Linux: &api.LinuxContainerUpdate{
						Resources: &api.LinuxResources{
							Cpu: &api.LinuxCPU{
								Shares:          api.UInt64(999),
								Quota:           api.Int64(888),
								Period:          api.UInt64(777),
								RealtimeRuntime: api.Int64(666),
								RealtimePeriod:  api.UInt64(555),
								Cpus:            "444",
								Mems:            "333",
							},
							Memory: &api.LinuxMemory{
								Limit:            api.Int64(9999),
								Reservation:      api.Int64(8888),
								Swap:             api.Int64(7777),
								Kernel:           api.Int64(6666),
								KernelTcp:        api.Int64(5555),
								Swappiness:       api.UInt64(444),
								DisableOomKiller: api.Bool(false),
								UseHierarchy:     api.Bool(false),
							},
							RdtClass:     api.String("10-foo"),
							BlockioClass: api.String("10-foo"),
						},
					},
				},
			),
			Entry("update hugepage limits", "resources/hugepagelimits", "both", nil),
			Entry("update hugepage limits", "resources/hugepagelimits", "10-foo",
				&api.ContainerUpdate{
					Linux: &api.LinuxContainerUpdate{
						Resources: &api.LinuxResources{
							Cpu: &api.LinuxCPU{
								Shares:          api.UInt64(999),
								Quota:           api.Int64(888),
								Period:          api.UInt64(777),
								RealtimeRuntime: api.Int64(666),
								RealtimePeriod:  api.UInt64(555),
								Cpus:            "444",
								Mems:            "333",
							},
							Memory: &api.LinuxMemory{
								Limit:            api.Int64(9999),
								Reservation:      api.Int64(8888),
								Swap:             api.Int64(7777),
								Kernel:           api.Int64(6666),
								KernelTcp:        api.Int64(5555),
								Swappiness:       api.UInt64(444),
								DisableOomKiller: api.Bool(false),
								UseHierarchy:     api.Bool(false),
							},
							HugepageLimits: []*api.HugepageLimit{
								{
									PageSize: "1M",
									Limit:    4096,
								},
								{
									PageSize: "4M",
									Limit:    1024,
								},
							},
						},
					},
				},
			),
			Entry("update cgroupv2 unified resources", "resources/unified", "both", nil),
			Entry("update cgroupv2 unified resources", "resources/unified", "10-foo",
				&api.ContainerUpdate{
					Linux: &api.LinuxContainerUpdate{
						Resources: &api.LinuxResources{
							Cpu: &api.LinuxCPU{
								Shares:          api.UInt64(999),
								Quota:           api.Int64(888),
								Period:          api.UInt64(777),
								RealtimeRuntime: api.Int64(666),
								RealtimePeriod:  api.UInt64(555),
								Cpus:            "444",
								Mems:            "333",
							},
							Memory: &api.LinuxMemory{
								Limit:            api.Int64(9999),
								Reservation:      api.Int64(8888),
								Swap:             api.Int64(7777),
								Kernel:           api.Int64(6666),
								KernelTcp:        api.Int64(5555),
								Swappiness:       api.UInt64(444),
								DisableOomKiller: api.Bool(false),
								UseHierarchy:     api.Bool(false),
							},
							Unified: map[string]string{
								"resource.1": "value1",
								"resource.2": "value2",
							},
						},
					},
				},
			),
		)
	})
})

var _ = Describe("Unsolicited container update requests", func() {
	var (
		s = &Suite{}
	)

	AfterEach(func() {
		s.Cleanup()
	})

	When("there are plugins", func() {
		BeforeEach(func() {
			s.Prepare(&mockRuntime{}, &mockPlugin{idx: "00", name: "test"})
		})

		It("should fail gracefully without unstarted plugins", func() {
			var (
				plugin = s.plugins[0]
			)

			s.StartRuntime()
			Expect(plugin.Init(s.Dir())).To(Succeed())

			updates := []*api.ContainerUpdate{
				{
					ContainerId: "pod0",
					Linux: &api.LinuxContainerUpdate{
						Resources: &api.LinuxResources{
							RdtClass: api.String("test"),
						},
					},
				},
			}
			_, err := plugin.stub.UpdateContainers(updates)
			Expect(err).ToNot(BeNil())
		})

		It("should be delivered, without crash/panic", func() {
			var (
				runtime = s.runtime
				plugin  = s.plugins[0]
				ctx     = context.Background()

				pod = &api.PodSandbox{
					Id:        "pod0",
					Name:      "pod0",
					Uid:       "uid0",
					Namespace: "default",
				}
				ctr = &api.Container{
					Id:           "ctr0",
					PodSandboxId: "pod0",
					Name:         "ctr0",
					State:        api.ContainerState_CONTAINER_CREATED,
				}

				recordedUpdates []*nri.ContainerUpdate
			)

			runtime.updateFn = func(_ context.Context, updates []*nri.ContainerUpdate) ([]*nri.ContainerUpdate, error) {
				recordedUpdates = updates
				return nil, nil
			}

			s.Startup()
			Expect(runtime.startStopPodAndContainer(ctx, pod, ctr)).To(Succeed())

			requestedUpdates := []*api.ContainerUpdate{
				{
					ContainerId: "pod0",
					Linux: &api.LinuxContainerUpdate{
						Resources: &api.LinuxResources{
							RdtClass: api.String("test"),
						},
					},
				},
			}
			failed, err := plugin.stub.UpdateContainers(requestedUpdates)

			Expect(failed).To(BeNil())
			Expect(err).To(BeNil())
			Expect(recordedUpdates).ToNot(Equal(requestedUpdates))
		})
	})
})

var _ = Describe("Plugin configuration request", func() {
	var (
		s = &Suite{}
	)

	AfterEach(func() {
		s.Cleanup()
	})

	BeforeEach(func() {
		s.Prepare(&mockRuntime{}, &mockPlugin{idx: "00", name: "test"})
	})

	It("should pass runtime version information to plugins", func() {
		var (
			runtimeName    = "test-runtime"
			runtimeVersion = "1.2.3"
		)

		s.runtime.name = runtimeName
		s.runtime.version = runtimeVersion

		s.Startup()

		Expect(s.plugins[0].RuntimeName()).To(Equal(runtimeName))
		Expect(s.plugins[0].RuntimeVersion()).To(Equal(runtimeVersion))
	})

	When("unchanged", func() {
		It("should pass default timeout information to plugins", func() {
			var (
				registerTimeout = nri.DefaultPluginRegistrationTimeout
				requestTimeout  = nri.DefaultPluginRequestTimeout
			)

			s.Startup()
			Expect(s.plugins[0].stub.RegistrationTimeout()).To(Equal(registerTimeout))
			Expect(s.plugins[0].stub.RequestTimeout()).To(Equal(requestTimeout))
		})
	})

	When("reconfigured", func() {
		var (
			registerTimeout = nri.DefaultPluginRegistrationTimeout + 5*time.Millisecond
			requestTimeout  = nri.DefaultPluginRequestTimeout + 7*time.Millisecond
		)

		BeforeEach(func() {
			nri.SetPluginRegistrationTimeout(registerTimeout)
			nri.SetPluginRequestTimeout(requestTimeout)
			s.Prepare(&mockRuntime{}, &mockPlugin{idx: "00", name: "test"})
		})

		AfterEach(func() {
			nri.SetPluginRegistrationTimeout(nri.DefaultPluginRegistrationTimeout)
			nri.SetPluginRequestTimeout(nri.DefaultPluginRequestTimeout)
		})

		It("should pass configured timeout information to plugins", func() {
			s.Startup()
			Expect(s.plugins[0].stub.RegistrationTimeout()).To(Equal(registerTimeout))
			Expect(s.plugins[0].stub.RequestTimeout()).To(Equal(requestTimeout))
		})
	})
})

func protoDiff(a, b proto.Message) string {
	return cmp.Diff(a, b, protocmp.Transform())
}

func protoEqual(a, b proto.Message) bool {
	return cmp.Equal(a, b, cmpopts.EquateEmpty(), protocmp.Transform())
}
