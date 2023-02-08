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
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"sigs.k8s.io/yaml"

	nri "github.com/containerd/nri/pkg/adaptation"
	"github.com/containerd/nri/pkg/api"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Configuration", func() {
	var (
		s = &Suite{}
	)

	AfterEach(func() {
		s.Cleanup()
	})

	When("invalid", func() {
		var (
			invalidConfig = "xyzzy gibberish foobar"
		)

		BeforeEach(func() {
			s.Prepare(invalidConfig, &mockRuntime{})
		})

		It("should prevent startup with an error", func() {
			Expect(s.runtime.Start(s.dir)).ToNot(Succeed())
		})
	})

	When("empty", func() {
		var (
			emptyConfig = ""
		)

		BeforeEach(func() {
			s.Prepare(emptyConfig, &mockRuntime{}, &mockPlugin{idx: "00", name: "test"})
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
		var (
			config = "disableConnections: true"
		)

		BeforeEach(func() {
			s.Prepare(config, &mockRuntime{}, &mockPlugin{idx: "00", name: "test"})
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

	When("external connections are explicitly enabled", func() {
		var (
			config = "disableConnections: false"
		)

		BeforeEach(func() {
			s.Prepare(config, &mockRuntime{}, &mockPlugin{idx: "00", name: "test"})
		})

		It("should allow plugins to connect", func() {
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

})

var _ = Describe("Adaptation", func() {
	When("SyncFn is nil", func() {
		var (
			syncFn   func(ctx context.Context, cb nri.SyncCB) error
			updateFn = func(ctx context.Context, updates []*nri.ContainerUpdate) ([]*nri.ContainerUpdate, error) {
				return nil, nil
			}
		)

		It("should prevent Adaptation creation with an error", func() {
			var (
				dir = GinkgoT().TempDir()
				etc = filepath.Join(dir, "etc", "nri")
				cfg = filepath.Join(etc, "nri.conf")
			)

			Expect(os.MkdirAll(etc, 0o755)).To(Succeed())
			Expect(os.WriteFile(cfg, []byte(""), 0o644)).To(Succeed())

			r, err := nri.New("mockRuntime", "0.0.1", syncFn, updateFn,
				nri.WithConfigPath(filepath.Join(dir, "etc", "nri", "nri.conf")),
				nri.WithPluginPath(filepath.Join(dir, "opt", "nri", "plugins")),
				nri.WithSocketPath(filepath.Join(dir, "nri.sock")),
			)

			Expect(r).To(BeNil())
			Expect(err).ToNot(BeNil())
		})
	})

	When("UpdateFn is nil", func() {
		var (
			updateFn func(ctx context.Context, updates []*nri.ContainerUpdate) ([]*nri.ContainerUpdate, error)
			syncFn   = func(ctx context.Context, cb nri.SyncCB) error {
				return nil
			}
		)

		It("should prevent Adaptation creation with an error", func() {
			var (
				dir = GinkgoT().TempDir()
				etc = filepath.Join(dir, "etc", "nri")
				cfg = filepath.Join(etc, "nri.conf")
			)

			Expect(os.MkdirAll(etc, 0o755)).To(Succeed())
			Expect(os.WriteFile(cfg, []byte(""), 0o644)).To(Succeed())

			r, err := nri.New("mockRuntime", "0.0.1", syncFn, updateFn,
				nri.WithConfigPath(filepath.Join(dir, "etc", "nri", "nri.conf")),
				nri.WithPluginPath(filepath.Join(dir, "opt", "nri", "plugins")),
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
		var (
			config = ""
		)

		s.Prepare(config,
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

		runtime.pods = strip(runtime.pods, map[string]*api.PodSandbox{}).(map[string]*api.PodSandbox)
		runtime.ctrs = strip(runtime.ctrs, map[string]*api.Container{}).(map[string]*api.Container)
		plugin.pods = strip(plugin.pods, map[string]*api.PodSandbox{}).(map[string]*api.PodSandbox)
		plugin.ctrs = strip(plugin.ctrs, map[string]*api.Container{}).(map[string]*api.Container)
		Expect(plugin.pods["pod0"]).To(Equal(runtime.pods["pod0"]))
		Expect(plugin.pods["pod1"]).To(Equal(runtime.pods["pod1"]))
		Expect(plugin.ctrs["ctr0"]).To(Equal(runtime.ctrs["ctr0"]))
		Expect(plugin.ctrs["ctr1"]).To(Equal(runtime.ctrs["ctr1"]))
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
			var (
				config = ""
			)

			s.Prepare(config, &mockRuntime{})
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
			var (
				config = ""
			)

			s.Prepare(config, &mockRuntime{}, &mockPlugin{idx: "00", name: "test"})
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
				"RunPodSandbox,StopPodSandbox,RemovePodSandbox",
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
			var (
				config = ""
			)

			s.Prepare(
				config,
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
					recordOrder = func(p *mockPlugin, pod *api.PodSandbox, ctr *api.Container) error {
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
				"RunPodSandbox,StopPodSandbox,RemovePodSandbox",
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

	adjust := func(subject string, p *mockPlugin, pod *api.PodSandbox, ctr *api.Container, overwrite bool) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
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

		case "environment":
			if overwrite {
				a.RemoveEnv("key")
			}
			a.AddEnv("key", plugin)

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
		}

		return a, nil, nil
	}

	AfterEach(func() {
		s.Cleanup()
	})

	When("there is a single plugin", func() {
		BeforeEach(func() {
			var (
				config = ""
			)

			s.Prepare(config, &mockRuntime{}, &mockPlugin{idx: "00", name: "test"})
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
					}
				)

				create := func(p *mockPlugin, pod *api.PodSandbox, ctr *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
					return adjust(subject, p, pod, ctr, false)
				}

				plugin.createContainer = create

				s.Startup()

				podReq := &api.RunPodSandboxRequest{Pod: pod}
				Expect(runtime.runtime.RunPodSandbox(ctx, podReq)).To(Succeed())
				ctrReq := &api.CreateContainerRequest{
					Pod:       pod,
					Container: ctr,
				}
				reply, err := runtime.runtime.CreateContainer(ctx, ctrReq)
				Expect(err).To(BeNil())
				Expect(stripAdjustment(reply.Adjust)).Should(Equal(stripAdjustment(expected)))
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
		)
	})

	When("there are multiple plugins", func() {
		BeforeEach(func() {
			var (
				config = ""
			)

			s.Prepare(
				config,
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
					}
				)

				create := func(p *mockPlugin, pod *api.PodSandbox, ctr *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
					return adjust(subject, p, pod, ctr, p == plugins[0] && remove)
				}

				plugins[0].createContainer = create
				plugins[1].createContainer = create

				s.Startup()

				podReq := &api.RunPodSandboxRequest{Pod: pod}
				Expect(runtime.runtime.RunPodSandbox(ctx, podReq)).To(Succeed())
				ctrReq := &api.CreateContainerRequest{
					Pod:       pod,
					Container: ctr,
				}
				reply, err := runtime.runtime.CreateContainer(ctx, ctrReq)
				if shouldFail {
					Expect(err).ToNot(BeNil())
				} else {
					Expect(err).To(BeNil())
					reply.Adjust = strip(reply.Adjust, &api.ContainerAdjustment{}).(*api.ContainerAdjustment)
					Expect(stripAdjustment(reply.Adjust)).Should(Equal(stripAdjustment(expected)))
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
		)
	})

})

// --------------------------------------------

var _ = Describe("Plugin container updates during creation", func() {
	var (
		s = &Suite{}
	)

	update := func(subject, which string, p *mockPlugin, pod *api.PodSandbox, ctr *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
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
			var (
				config = ""
			)

			s.Prepare(config, &mockRuntime{}, &mockPlugin{idx: "00", name: "test"})
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
				Expect(runtime.runtime.RunPodSandbox(ctx, podReq)).To(Succeed())
				ctrReq := &api.CreateContainerRequest{
					Pod:       pod0,
					Container: ctr0,
				}
				_, err := runtime.runtime.CreateContainer(ctx, ctrReq)
				Expect(err).To(BeNil())

				podReq = &api.RunPodSandboxRequest{Pod: pod1}
				Expect(runtime.runtime.RunPodSandbox(ctx, podReq)).To(Succeed())
				ctrReq = &api.CreateContainerRequest{
					Pod:       pod1,
					Container: ctr1,
				}
				reply, err = runtime.runtime.CreateContainer(ctx, ctrReq)
				Expect(err).To(BeNil())

				Expect(len(reply.Update)).To(Equal(1))
				expected.ContainerId = reply.Update[0].ContainerId
				Expect(stripUpdate(reply.Update[0])).Should(Equal(stripUpdate(expected)))
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
			var (
				config = ""
			)

			s.Prepare(
				config,
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
				Expect(runtime.runtime.RunPodSandbox(ctx, podReq)).To(Succeed())
				ctrReq := &api.CreateContainerRequest{
					Pod:       pod0,
					Container: ctr0,
				}
				_, err := runtime.runtime.CreateContainer(ctx, ctrReq)
				Expect(err).To(BeNil())

				podReq = &api.RunPodSandboxRequest{Pod: pod1}
				Expect(runtime.runtime.RunPodSandbox(ctx, podReq)).To(Succeed())
				ctrReq = &api.CreateContainerRequest{
					Pod:       pod1,
					Container: ctr1,
				}
				reply, err = runtime.runtime.CreateContainer(ctx, ctrReq)
				if which == "both" {
					Expect(err).ToNot(BeNil())
				} else {
					Expect(err).To(BeNil())
					Expect(len(reply.Update)).To(Equal(1))
					expected.ContainerId = reply.Update[0].ContainerId
					Expect(stripUpdate(reply.Update[0])).Should(Equal(stripUpdate(expected)))
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

var _ = Describe("Unsolicited container update requests", func() {
	var (
		s = &Suite{}
	)

	AfterEach(func() {
		s.Cleanup()
	})

	When("there are plugins", func() {
		BeforeEach(func() {
			var (
				config = ""
			)

			s.Prepare(config, &mockRuntime{}, &mockPlugin{idx: "00", name: "test"})
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

			runtime.updateFn = func(ctx context.Context, updates []*nri.ContainerUpdate) ([]*nri.ContainerUpdate, error) {
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

// Notes:
//
//	XXX FIXME KLUDGE
//	Ever since we had to switch from gogo/protobuf to google.golang.org/protobuf
//	(from the gogo one), we can't turn off sizeCache and a few other generated
//	unexported fields (or if we can, I failed to figure out so far how). In some
//	of our test cases this gives a false positive in ginkgo.Expect/Equal combos,
//	since that also checks unexported fields for equality. As a band-aid work-
//	around we marshal then unmarshal compared objects in offending test cases to
//	clear those unexported fields.
func strip(obj interface{}, ptr interface{}) interface{} {
	bytes, err := yaml.Marshal(obj)
	Expect(err).To(BeNil())
	Expect(yaml.Unmarshal(bytes, ptr)).To(Succeed())
	return ptr
}

func stripAdjustment(a *api.ContainerAdjustment) *api.ContainerAdjustment {
	stripAnnotations(a)
	stripMounts(a)
	stripEnv(a)
	stripHooks(a)
	stripLinuxAdjustment(a)
	return a
}

func stripAnnotations(a *api.ContainerAdjustment) {
	if len(a.Annotations) == 0 {
		a.Annotations = nil
	}
}

func stripMounts(a *api.ContainerAdjustment) {
	if len(a.Mounts) == 0 {
		a.Mounts = nil
	}
}

func stripEnv(a *api.ContainerAdjustment) {
	if len(a.Env) == 0 {
		a.Env = nil
	}
}

func stripHooks(a *api.ContainerAdjustment) {
	if a.Hooks == nil {
		return
	}
	switch {
	case len(a.Hooks.Prestart) > 0:
	case len(a.Hooks.CreateRuntime) > 0:
	case len(a.Hooks.CreateContainer) > 0:
	case len(a.Hooks.StartContainer) > 0:
	case len(a.Hooks.Poststart) > 0:
	case len(a.Hooks.Poststop) > 0:
	default:
		a.Hooks = nil
	}
}

func stripLinuxAdjustment(a *api.ContainerAdjustment) {
	if a.Linux == nil {
		return
	}
	stripLinuxDevices(a)
	a.Linux.Resources = stripLinuxResources(a.Linux.Resources)
	if a.Linux.Devices == nil && a.Linux.Resources == nil && a.Linux.CgroupsPath == "" {
		a.Linux = nil
	}
}

func stripLinuxDevices(a *api.ContainerAdjustment) {
	if len(a.Linux.Devices) == 0 {
		a.Linux.Devices = nil
	}
}

func stripLinuxResources(r *api.LinuxResources) *api.LinuxResources {
	if r == nil {
		return nil
	}

	r.Memory = stripLinuxResourcesMemory(r.Memory)
	r.Cpu = stripLinuxResourcesCpu(r.Cpu)
	r.HugepageLimits = stripLinuxResourcesHugepageLimits(r.HugepageLimits)
	r.Unified = stripLinuxResourcesUnified(r.Unified)

	switch {
	case r.Memory != nil:
		return r
	case r.Cpu != nil:
		return r
	case r.HugepageLimits != nil:
		return r
	case r.Unified != nil:
		return r
	case r.BlockioClass.GetValue() != "":
		return r
	case r.RdtClass.GetValue() != "":
		return r
	}

	return nil
}

func stripLinuxResourcesMemory(m *api.LinuxMemory) *api.LinuxMemory {
	if m == nil {
		return nil
	}
	switch {
	case m.Limit.GetValue() != 0:
		return m
	case m.Reservation.GetValue() != 0:
		return m
	case m.Swap.GetValue() != 0:
		return m
	case m.Kernel.GetValue() != 0:
		return m
	case m.KernelTcp.GetValue() != 0:
		return m
	case m.Swappiness.GetValue() != 0:
		return m
	case m.DisableOomKiller.GetValue():
		return m
	case m.UseHierarchy.GetValue():
		return m
	}
	return nil
}

func stripLinuxResourcesCpu(c *api.LinuxCPU) *api.LinuxCPU {
	if c == nil {
		return nil
	}
	switch {
	case c.Shares.GetValue() != 0:
		return c
	case c.Quota.GetValue() != 0:
		return c
	case c.Period.GetValue() != 0:
		return c
	case c.RealtimeRuntime.GetValue() != 0:
		return c
	case c.RealtimePeriod.GetValue() != 0:
		return c
	case c.Cpus != "":
		return c
	case c.Mems != "":
		return c
	}
	return nil
}

func stripLinuxResourcesHugepageLimits(l []*api.HugepageLimit) []*api.HugepageLimit {
	if len(l) == 0 {
		return nil
	}
	return l
}

func stripLinuxResourcesUnified(u map[string]string) map[string]string {
	if len(u) == 0 {
		return nil
	}
	return u
}

func stripUpdate(u *api.ContainerUpdate) *api.ContainerUpdate {
	if u == nil {
		return nil
	}

	u.Linux = stripUpdateLinux(u.Linux)
	if u.ContainerId == "" && u.Linux == nil && !u.IgnoreFailure {
		return nil
	}

	return u
}

func stripUpdateLinux(l *api.LinuxContainerUpdate) *api.LinuxContainerUpdate {
	if l == nil {
		return nil
	}

	r := stripLinuxResources(l.Resources)
	if r == nil {
		return l
	}
	l.Resources = r

	return l
}
