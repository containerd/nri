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
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

	rspec "github.com/opencontainers/runtime-spec/specs-go"

	nri "github.com/containerd/nri/pkg/adaptation"
	"github.com/containerd/nri/pkg/api"
	nriplugin "github.com/containerd/nri/pkg/plugin"
	validator "github.com/containerd/nri/plugins/default-validator/builtin"
)

func TestConfiguration(t *testing.T) {
	s := &Suite{}

	t.Run("no (extra) options given", func(t *testing.T) {
		setup := func(t *testing.T) {
			s.Prepare(t, &mockRuntime{}, &mockPlugin{idx: "00", name: "test"})
		}

		t.Run("should allow startup", func(t *testing.T) {
			setup(t)
			require.NoError(t, s.runtime.Start(s.dir))
		})

		t.Run("should allow external plugins to connect", func(t *testing.T) {
			setup(t)

			var (
				runtime = s.runtime
				plugin  = s.plugins[0]
				timeout = time.After(startupTimeout)
			)
			require.NoError(t, runtime.Start(s.dir))
			require.NoError(t, plugin.Start(s.dir))
			require.NoError(t, plugin.Wait(PluginSynchronized, timeout))
		})
	})

	t.Run("external connections are explicitly disabled", func(t *testing.T) {
		setup := func(t *testing.T) {
			s.Prepare(t,
				&mockRuntime{
					options: []nri.Option{
						nri.WithDisabledExternalConnections(),
					},
				},
				&mockPlugin{idx: "00", name: "test"},
			)
		}

		t.Run("should prevent plugins from connecting", func(t *testing.T) {
			setup(t)

			var (
				runtime = s.runtime
				plugin  = s.plugins[0]
			)
			require.NoError(t, runtime.Start(s.dir))
			require.Error(t, plugin.Start(s.dir))
		})
	})
}

func TestAdaptation(t *testing.T) {
	t.Run("SyncFn is nil", func(t *testing.T) {
		var (
			syncFn   func(ctx context.Context, cb nri.SyncCB) error
			updateFn = func(_ context.Context, _ []*nri.ContainerUpdate) ([]*nri.ContainerUpdate, error) {
				return nil, nil
			}
		)

		t.Run("should prevent Adaptation creation with an error", func(t *testing.T) {
			var (
				dir = t.TempDir()
				etc = filepath.Join(dir, "etc", "nri")
			)

			require.NoError(t, os.MkdirAll(etc, 0o755))

			r, err := nri.New("mockRuntime", "0.0.1", syncFn, updateFn,
				nri.WithPluginPath(filepath.Join(dir, "opt", "nri", "plugins")),
				nri.WithPluginConfigPath(filepath.Join(dir, "etc", "nri", "conf.d")),
				nri.WithSocketPath(filepath.Join(dir, "nri.sock")),
			)

			require.Nil(t, r)
			require.NotNil(t, err)
		})
	})

	t.Run("UpdateFn is nil", func(t *testing.T) {
		var (
			updateFn func(ctx context.Context, updates []*nri.ContainerUpdate) ([]*nri.ContainerUpdate, error)
			syncFn   = func(_ context.Context, _ nri.SyncCB) error {
				return nil
			}
		)

		t.Run("should prevent Adaptation creation with an error", func(t *testing.T) {
			var (
				dir = t.TempDir()
				etc = filepath.Join(dir, "etc", "nri")
			)

			require.NoError(t, os.MkdirAll(etc, 0o755))

			r, err := nri.New("mockRuntime", "0.0.1", syncFn, updateFn,
				nri.WithPluginPath(filepath.Join(dir, "opt", "nri", "plugins")),
				nri.WithPluginConfigPath(filepath.Join(dir, "etc", "nri", "conf.d")),
				nri.WithSocketPath(filepath.Join(dir, "nri.sock")),
			)

			require.Nil(t, r)
			require.NotNil(t, err)
		})
	})
}

func TestPluginConnection(t *testing.T) {
	s := &Suite{}

	setup := func(t *testing.T) {
		s.Prepare(t,
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
	}

	t.Run("should reject plugins with an invalid name", func(t *testing.T) {
		setup(t)

		var (
			validPlugin = &mockPlugin{
				name: "abcd-0123+EFGH_4567.ijkl",
				idx:  "05",
			}
			invalidPlugin = &mockPlugin{
				name: "foo,bar",
				idx:  "10",
			}
		)

		s.Startup()

		require.NoError(t, validPlugin.Start(s.dir))
		require.Error(t, invalidPlugin.Start(s.dir))
	})

	t.Run("should configure the plugin", func(t *testing.T) {
		setup(t)

		plugin := s.plugins[0]

		s.Startup()

		require.Contains(t, plugin.Events(), PluginConfigured)
	})

	t.Run("should synchronize the plugin after configuration", func(t *testing.T) {
		setup(t)

		var (
			runtime = s.runtime
			plugin  = s.plugins[0]
		)

		s.Startup()

		require.ElementsMatch(t, []*Event{
			PluginConfigured,
			PluginSynchronized,
		}, plugin.Events())

		require.True(t, protoEqual(plugin.pods["pod0"], runtime.pods["pod0"]), protoDiff(plugin.pods["pod0"], runtime.pods["pod0"]))
		require.True(t, protoEqual(plugin.pods["pod1"], runtime.pods["pod1"]), protoDiff(plugin.pods["pod1"], runtime.pods["pod1"]))
		require.True(t, protoEqual(plugin.ctrs["ctr0"], runtime.ctrs["ctr0"]), protoDiff(plugin.ctrs["ctr0"], runtime.ctrs["ctr0"]))
		require.True(t, protoEqual(plugin.ctrs["ctr1"], runtime.ctrs["ctr1"]), protoDiff(plugin.ctrs["ctr1"], runtime.ctrs["ctr1"]))
	})

	t.Run("close plugins on failed synchronization", func(t *testing.T) {
		setup(t)

		var (
			runtime = s.runtime
			plugin0 = s.plugins[0]
			plugin1 = &mockPlugin{idx: "10", name: "bar"}
			timeout = time.After(startupTimeout)
		)

		s.Startup()

		require.ElementsMatch(t, []*Event{
			PluginConfigured,
			PluginSynchronized,
		}, plugin0.Events())

		runtime.failSync = true

		s.StartPlugins(plugin1)
		require.NoError(t, plugin1.Wait(PluginDisconnected, timeout))
	})
}

func TestPodAndContainerRequestsAndEvents(t *testing.T) {
	s := &Suite{}

	t.Run("there are no plugins", func(t *testing.T) {
		setup := func(t *testing.T) {
			s.Prepare(t, &mockRuntime{})
		}

		t.Run("should always succeed", func(t *testing.T) {
			setup(t)

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

			require.NoError(t, s.runtime.startStopPodAndContainer(ctx, pod0, ctr0))
			require.NoError(t, s.runtime.startStopPodAndContainer(ctx, pod1, ctr1))
		})
	})

	t.Run("when there are plugins", func(t *testing.T) {
		runTable := func(subscriptions ...string) func(t *testing.T) {
			return func(t *testing.T) {
				s.Prepare(t, &mockRuntime{}, &mockPlugin{idx: "00", name: "test"})
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

				require.NoError(t, runtime.startStopPodAndContainer(ctx, pod, ctr))
				for _, events := range subscriptions {
					for _, event := range strings.Split(events, ",") {
						match := &Event{Type: EventType(event)}
						require.True(t, plugin.EventQ().Has(match))
					}
				}
			}
		}
		t.Run("should honor plugins' event subscriptions", func(t *testing.T) {
			t.Run("with RunPodSandbox", runTable("RunPodSandbox"))
			t.Run("with UpdatePodSandbox", runTable("UpdatePodSandbox"))
			t.Run("with PostUpdatePodSandbox", runTable("PostUpdatePodSandbox"))
			t.Run("with StopPodSandbox", runTable("StopPodSandbox"))
			t.Run("with RemovePodSandbox", runTable("RemovePodSandbox"))
			t.Run("with CreateContainer", runTable("CreateContainer"))
			t.Run("with PostCreateContainer", runTable("PostCreateContainer"))
			t.Run("with StartContainer", runTable("StartContainer"))
			t.Run("with PostStartContainer", runTable("PostStartContainer"))
			t.Run("with UpdateContainer", runTable("UpdateContainer"))
			t.Run("with PostUpdateContainer", runTable("PostUpdateContainer"))
			t.Run("with StopContainer", runTable("StopContainer"))
			t.Run("with RemoveContainer", runTable("RemoveContainer"))
			t.Run("with all pod events", runTable("RunPodSandbox,StopPodSandbox,RemovePodSandbox"))
			t.Run("with all container requests", runTable("CreateContainer,UpdateContainer,StopContainer"))
			t.Run("with all container requests and events", runTable(
				"CreateContainer,PostCreateContainer",
				"StartContainer,PostStartContainer",
				"UpdateContainer,PostUpdateContainer",
				"StopContainer",
				"RemoveContainer",
			))
			t.Run("with all pod and container requests and events", runTable(
				"RunPodSandbox,UpdatePodSandbox,PostUpdatePodSandbox,StopPodSandbox,RemovePodSandbox",
				"CreateContainer,PostCreateContainer",
				"StartContainer,PostStartContainer",
				"UpdateContainer,PostUpdateContainer",
				"StopContainer",
				"RemoveContainer",
			))
		})
	})

	t.Run("when there are multiple plugins", func(t *testing.T) {
		runTable := func(subscriptions ...string) func(t *testing.T) {
			return func(t *testing.T) {
				s.Prepare(t,
					&mockRuntime{},
					&mockPlugin{idx: "20", name: "test"},
					&mockPlugin{idx: "99", name: "foo"},
					&mockPlugin{idx: "00", name: "bar"},
				)
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

				require.NoError(t, runtime.startStopPodAndContainer(ctx, pod, ctr))
				require.ElementsMatch(t, []*mockPlugin{
					plugins[2],
					plugins[0],
					plugins[1],
				}, order)
			}
		}
		t.Run("should honor plugins' event subscriptions", func(t *testing.T) {
			t.Run("with StartContainer", runTable("StartContainer"))
			t.Run("with all container CRI requests", runTable("CreateContainer,StartContainer,UpdateContainer,StopContainer,RemoveContainer"))
			t.Run("with all container requests and events", runTable(
				"CreateContainer,PostCreateContainer",
				"StartContainer,PostStartContainer",
				"UpdateContainer,PostUpdateContainer",
				"StopContainer",
				"RemoveContainer",
			))
			t.Run("with all pod and container requests and events", runTable(
				"RunPodSandbox,UpdatePodSandbox,PostUpdatePodSandbox,StopPodSandbox,RemovePodSandbox",
				"CreateContainer,PostCreateContainer",
				"StartContainer,PostStartContainer",
				"UpdateContainer,PostUpdateContainer",
				"StopContainer",
				"RemoveContainer",
			))
		})
	})
}

func TestPluginContainerCreationAdjustments(t *testing.T) {
	s := &Suite{}

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
				Path: "/var/run/cgroupns/replaced",
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

		case "linux net device":
			if overwrite {
				a.RemoveLinuxNetDevice("hostIf")
			}
			a.AddLinuxNetDevice(
				"hostIf",
				&api.LinuxNetDevice{
					Name: "containerIf",
				},
			)

		case "linux scheduler":
			a.SetLinuxScheduler(&api.LinuxScheduler{
				Policy:   api.LinuxSchedulerPolicy_SCHED_FIFO,
				Priority: 10,
				Flags: []api.LinuxSchedulerFlag{
					api.LinuxSchedulerFlag_SCHED_FLAG_RESET_ON_FORK,
				},
			})

		case "linux sysctl":
			a.SetLinuxSysctl("net.core.somaxconn", "256")

		case "linux memory policy":
			a.SetLinuxMemoryPolicy(
				api.MpolMode_MPOL_INTERLEAVE, "0,1", api.MpolFlag_MPOL_F_STATIC_NODES,
			)

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

	t.Run("there is a single plugin", func(t *testing.T) {
		runTable := func(subject string, expected *api.ContainerAdjustment) func(*testing.T) {
			return func(t *testing.T) {
				s.Prepare(t, &mockRuntime{}, &mockPlugin{idx: "00", name: "test"})
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
						Linux: &api.LinuxContainer{
							Resources: &api.LinuxResources{
								Cpu: &api.LinuxCPU{
									Shares:          api.UInt64(111),
									Quota:           api.Int64(222),
									Period:          api.UInt64(333),
									RealtimeRuntime: api.Int64(444),
									RealtimePeriod:  api.UInt64(555),
								},
								Memory: &api.LinuxMemory{
									Limit:            api.Int64(11111),
									Reservation:      api.Int64(22222),
									Swap:             api.Int64(33333),
									Swappiness:       api.UInt64(44444),
									DisableOomKiller: api.Bool(false),
									UseHierarchy:     api.Bool(false),
								},
							},
							Namespaces: []*api.LinuxNamespace{
								{
									Type: "cgroup",
									Path: "/var/run/cgroupns/original",
								},
							},
						},
					}
				)

				create := func(p *mockPlugin, pod *api.PodSandbox, ctr *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
					return adjust(subject, p, pod, ctr, false)
				}

				plugin.createContainer = create

				s.Startup()

				podReq := &api.RunPodSandboxRequest{Pod: pod}
				require.NoError(t, runtime.RunPodSandbox(ctx, podReq))
				ctrReq := &api.CreateContainerRequest{
					Pod:       pod,
					Container: ctr,
				}
				reply, err := runtime.CreateContainer(ctx, ctrReq)
				require.Nil(t, err)
				require.True(t, protoEqual(reply.Adjust.Strip(), expected), protoDiff(reply.Adjust, expected))
			}
		}
		t.Run("should be successfully collected without conflicts", func(t *testing.T) {
			t.Run("adjust annotations", runTable("annotation", &api.ContainerAdjustment{
				Annotations: map[string]string{
					"key": "00-test",
				},
			}))
			t.Run("adjust mounts", runTable("mount", &api.ContainerAdjustment{
				Mounts: []*api.Mount{
					{
						Source:      "/dev/00-test",
						Destination: "/mnt/test",
					},
				},
			}))
			t.Run("remove a mount", runTable("remove mount", &api.ContainerAdjustment{
				Mounts: []*api.Mount{
					{
						Destination: api.MarkForRemoval("/remove/test/destination"),
					},
				},
			}))
			t.Run("adjust environment", runTable("environment", &api.ContainerAdjustment{
				Env: []*api.KeyValue{
					{
						Key:   "key",
						Value: "00-test",
					},
				},
			}))
			t.Run("adjust arguments", runTable("arguments", &api.ContainerAdjustment{
				Args: []string{
					"echo",
					"updated",
					"argument",
					"list",
				},
			}))
			t.Run("adjust hooks", runTable("hooks", &api.ContainerAdjustment{
				Hooks: &api.Hooks{
					Prestart: []*api.Hook{
						{
							Path: "/bin/00-test",
						},
					},
				},
			}))
			t.Run("adjust devices", runTable("device", &api.ContainerAdjustment{
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
			}))
			t.Run("adjust namespace", runTable("namespace", &api.ContainerAdjustment{
				Linux: &api.LinuxContainerAdjustment{
					Namespaces: []*api.LinuxNamespace{
						{
							Type: "cgroup",
							Path: "/var/run/cgroupns/replaced",
						},
					},
				},
			}))
			t.Run("adjust rlimits", runTable("rlimit", &api.ContainerAdjustment{
				Rlimits: []*api.POSIXRlimit{{Type: "nofile", Soft: 123, Hard: 456}},
			}))
			t.Run("adjust CDI Devices", runTable("CDI-device", &api.ContainerAdjustment{
				CDIDevices: []*api.CDIDevice{
					{
						Name: "vendor0.com/dev=dev0",
					},
				},
			}))
			t.Run("adjust I/O priority", runTable("I/O priority", &api.ContainerAdjustment{
				Linux: &api.LinuxContainerAdjustment{
					IoPriority: &api.LinuxIOPriority{
						Class:    api.IOPrioClass_IOPRIO_CLASS_RT,
						Priority: 5,
					},
				},
			}))
			t.Run("adjust linux net devices", runTable("linux net device", &api.ContainerAdjustment{
				Linux: &api.LinuxContainerAdjustment{
					NetDevices: map[string]*api.LinuxNetDevice{
						"hostIf": {
							Name: "containerIf",
						},
					},
				},
			}))
			t.Run("adjust linux scheduler", runTable("linux scheduler", &api.ContainerAdjustment{
				Linux: &api.LinuxContainerAdjustment{
					Scheduler: &api.LinuxScheduler{
						Policy:   api.LinuxSchedulerPolicy_SCHED_FIFO,
						Priority: 10,
						Flags: []api.LinuxSchedulerFlag{
							api.LinuxSchedulerFlag_SCHED_FLAG_RESET_ON_FORK,
						},
					},
				},
			}))
			t.Run("adjust linux sysctl settings", runTable("linux sysctl", &api.ContainerAdjustment{
				Linux: &api.LinuxContainerAdjustment{
					Sysctl: map[string]string{
						"net.core.somaxconn": "256",
					},
				},
			}))
			t.Run("adjust linux memory policy", runTable("linux memory policy", &api.ContainerAdjustment{
				Linux: &api.LinuxContainerAdjustment{
					MemoryPolicy: &api.LinuxMemoryPolicy{
						Mode:  api.MpolMode_MPOL_INTERLEAVE,
						Nodes: "0,1",
						Flags: []api.MpolFlag{
							api.MpolFlag_MPOL_F_STATIC_NODES,
						},
					},
				},
			}))
			t.Run("adjust CPU resources", runTable("resources/cpu", &api.ContainerAdjustment{
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
			}))
			t.Run("adjust memory resources", runTable("resources/mem", &api.ContainerAdjustment{
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
			}))
			t.Run("adjust class-based resources", runTable("resources/classes", &api.ContainerAdjustment{
				Linux: &api.LinuxContainerAdjustment{
					Resources: &api.LinuxResources{
						RdtClass:     api.String("00-test"),
						BlockioClass: api.String("00-test"),
					},
				},
			}))
			t.Run("adjust hugepage limits", runTable("resources/hugepagelimits", &api.ContainerAdjustment{
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
			}))
			t.Run("adjust cgroupv2 unified resources", runTable("resources/unified", &api.ContainerAdjustment{
				Linux: &api.LinuxContainerAdjustment{
					Resources: &api.LinuxResources{
						Unified: map[string]string{
							"resource.1": "value1",
							"resource.2": "value2",
						},
					},
				},
			}))
			t.Run("adjust cgroups path", runTable("cgroupspath", &api.ContainerAdjustment{
				Linux: &api.LinuxContainerAdjustment{
					CgroupsPath: "/00-test",
				},
			}))
			t.Run("adjust seccomp policy", runTable("seccomp", &api.ContainerAdjustment{
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
			}))
			t.Run("adjust RDT", runTable("rdt", &api.ContainerAdjustment{
				Linux: &api.LinuxContainerAdjustment{
					Rdt: &api.LinuxRdt{
						ClosId:           api.String("test"),
						Schemata:         api.RepeatedString([]string{"L3:0=ff", "MB:0=50"}),
						EnableMonitoring: api.Bool(true),
					},
				},
			}))
		})
	})

	t.Run("there are multiple plugins", func(t *testing.T) {
		runTable := func(subject string, remove, shouldFail bool, expected *api.ContainerAdjustment) func(*testing.T) {
			return func(t *testing.T) {
				s.Prepare(t,
					&mockRuntime{},
					&mockPlugin{idx: "10", name: "foo"},
					&mockPlugin{idx: "00", name: "bar"},
				)
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
						Linux: &api.LinuxContainer{
							Resources: &api.LinuxResources{
								Cpu: &api.LinuxCPU{
									Shares:          api.UInt64(111),
									Quota:           api.Int64(222),
									Period:          api.UInt64(333),
									RealtimeRuntime: api.Int64(444),
									RealtimePeriod:  api.UInt64(555),
								},
								Memory: &api.LinuxMemory{
									Limit:            api.Int64(11111),
									Reservation:      api.Int64(22222),
									Swap:             api.Int64(33333),
									Swappiness:       api.UInt64(44444),
									DisableOomKiller: api.Bool(false),
									UseHierarchy:     api.Bool(false),
								},
							},
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
				require.NoError(t, runtime.RunPodSandbox(ctx, podReq))
				ctrReq := &api.CreateContainerRequest{
					Pod:       pod,
					Container: ctr,
				}
				reply, err := runtime.CreateContainer(ctx, ctrReq)
				if shouldFail {
					require.NotNil(t, err)
				} else {
					require.Nil(t, err)
					require.True(t, protoEqual(reply.Adjust.Strip(), expected), protoDiff(reply.Adjust, expected))
				}
			}
		}
		t.Run("should be successfully combined if there are no conflicts", func(t *testing.T) {
			t.Run("adjust annotations (conflicts)", runTable("annotation", false, true, nil))
			t.Run("adjust annotations", runTable("annotation", true, false, &api.ContainerAdjustment{
				Annotations: map[string]string{
					"-key": "",
					"key":  "10-foo",
				},
			}))
			t.Run("adjust mounts (conflicts)", runTable("mount", false, true, nil))
			t.Run("adjust mounts", runTable("mount", true, false, &api.ContainerAdjustment{
				Mounts: []*api.Mount{
					{
						Source:      "/dev/10-foo",
						Destination: "/mnt/test",
					},
				},
			}))
			t.Run("adjust environment (conflicts)", runTable("environment", false, true, nil))
			t.Run("adjust environment", runTable("environment", true, false, &api.ContainerAdjustment{
				Env: []*api.KeyValue{
					{
						Key:   "key",
						Value: "10-foo",
					},
				},
			}))
			t.Run("adjust arguments (conflicts)", runTable("arguments", false, true, nil))
			t.Run("adjust arguments", runTable("arguments", true, false, &api.ContainerAdjustment{
				Args: []string{
					"echo",
					"updated",
					"argument",
					"list",
					"twice...",
				},
			}))
			t.Run("adjust hooks", runTable("hooks", false, false, &api.ContainerAdjustment{
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
			}))
			t.Run("adjust devices", runTable("device", false, true, nil))
			t.Run("adjust devices", runTable("device", true, false, &api.ContainerAdjustment{
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
			}))
			t.Run("adjust resources", runTable("resources/classes", false, true, nil))
			t.Run("adjust I/O priority (conflicts)", runTable("I/O priority", false, true, nil))
			t.Run("adjust linux net devices", runTable("linux net device", true, false, &api.ContainerAdjustment{
				Linux: &api.LinuxContainerAdjustment{
					NetDevices: map[string]*api.LinuxNetDevice{
						"-hostIf": nil,
						"hostIf": {
							Name: "containerIf",
						},
					},
				},
			}))
			t.Run("adjust linux net devices (conflicts)", runTable("linux net device", false, true, nil))
			t.Run("adjust linux scheduler (conflicts)", runTable("linux scheduler", false, true, nil))
			t.Run("adjust RDT (conflicts)", runTable("rdt", false, true, nil))
			t.Run("adjust RDT", runTable("rdt", true, false, &api.ContainerAdjustment{
				Linux: &api.LinuxContainerAdjustment{
					Rdt: &api.LinuxRdt{
						ClosId:           api.String("foo"),
						Schemata:         api.RepeatedString([]string{"L3:0=ff", "MB:0=50"}),
						EnableMonitoring: api.Bool(true),
					},
				},
			}))
		})
	})

	t.Run("there are validating plugins", func(t *testing.T) {
		runTable := func(subject string, shouldFail bool, expected *api.ContainerAdjustment) func(*testing.T) {
			return func(t *testing.T) {
				s.Prepare(t,
					&mockRuntime{},
					&mockPlugin{idx: "00", name: "foo"},
					&mockPlugin{idx: "00", name: "validator"},
				)
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
				require.NoError(t, runtime.RunPodSandbox(ctx, podReq))
				ctrReq := &api.CreateContainerRequest{
					Pod:       pod,
					Container: ctr,
				}
				reply, err := runtime.CreateContainer(ctx, ctrReq)
				if shouldFail {
					require.NotNil(t, err)
				} else {
					require.Nil(t, err)
					require.True(t, protoEqual(reply.Adjust.Strip(), expected), protoDiff(reply.Adjust, expected))
				}
			}
		}
		t.Run("validation result should be honored", func(t *testing.T) {
			t.Run("adjust allowed annotation", runTable("annotation", false, &api.ContainerAdjustment{
				Annotations: map[string]string{
					"key": "00-foo",
				},
			}))
			t.Run("adjust forbidden annotation", runTable("annotation", true, nil))
		})
	})

	t.Run("the default validator is enabled and OCI Hook injection is disabled", func(t *testing.T) {
		setup := func(t *testing.T) {
			s.Prepare(t,
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
		}

		t.Run("should reject OCI Hook injection", func(t *testing.T) {
			setup(t)
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
			require.NoError(t, runtime.RunPodSandbox(ctx, podReq))

			ctrReq := &api.CreateContainerRequest{
				Pod:       pod,
				Container: ctr0,
			}
			reply, err := runtime.CreateContainer(ctx, ctrReq)
			require.NotNil(t, reply)
			require.Nil(t, err)

			ctrReq = &api.CreateContainerRequest{
				Pod:       pod,
				Container: ctr1,
			}
			reply, err = runtime.CreateContainer(ctx, ctrReq)
			require.NotNil(t, err)
			require.Nil(t, reply)
		})
	})

	t.Run("default validator disallows runtime default seccomp policy adjustment", func(t *testing.T) {
		setup := func(t *testing.T) {
			s.Prepare(t,
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
		}

		t.Run("should reject runtime default seccomp policy adjustment", func(t *testing.T) {
			setup(t)

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
			require.NoError(t, runtime.RunPodSandbox(ctx, podReq))

			ctrReq := &api.CreateContainerRequest{
				Pod:       pod,
				Container: ctr0,
			}
			reply, err := runtime.CreateContainer(ctx, ctrReq)
			require.NotNil(t, reply)
			require.Nil(t, err)

			ctrReq = &api.CreateContainerRequest{
				Pod:       pod,
				Container: ctr1,
			}
			reply, err = runtime.CreateContainer(ctx, ctrReq)
			require.NotNil(t, err)
			require.Nil(t, reply)
		})
	})

	t.Run("default validator allows runtime default seccomp policy adjustment", func(t *testing.T) {
		setup := func(t *testing.T) {
			s.Prepare(t,
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
		}

		t.Run("should not reject runtime default seccomp policy adjustment", func(t *testing.T) {
			setup(t)

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
			require.NoError(t, runtime.RunPodSandbox(ctx, podReq))

			ctrReq := &api.CreateContainerRequest{
				Pod:       pod,
				Container: ctr0,
			}
			reply, err := runtime.CreateContainer(ctx, ctrReq)
			require.NotNil(t, reply)
			require.Nil(t, err)

			ctrReq = &api.CreateContainerRequest{
				Pod:       pod,
				Container: ctr1,
			}
			reply, err = runtime.CreateContainer(ctx, ctrReq)
			require.NotNil(t, reply)
			require.Nil(t, err)
		})
	})

	t.Run("default validator disallows custom seccomp policy adjustment", func(t *testing.T) {
		setup := func(t *testing.T) {
			s.Prepare(t,
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
		}

		t.Run("should reject custom seccomp policy adjustment", func(t *testing.T) {
			setup(t)

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
			require.NoError(t, runtime.RunPodSandbox(ctx, podReq))

			ctrReq := &api.CreateContainerRequest{
				Pod:       pod,
				Container: ctr0,
			}
			reply, err := runtime.CreateContainer(ctx, ctrReq)
			require.NotNil(t, reply)
			require.Nil(t, err)

			ctrReq = &api.CreateContainerRequest{
				Pod:       pod,
				Container: ctr1,
			}
			reply, err = runtime.CreateContainer(ctx, ctrReq)
			require.NotNil(t, err)
			require.Nil(t, reply)
		})
	})

	t.Run("default validator allows custom seccomp policy adjustment", func(t *testing.T) {
		setup := func(t *testing.T) {
			s.Prepare(t,
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
		}

		t.Run("should not reject custom seccomp policy adjustment", func(t *testing.T) {
			setup(t)

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
			require.NoError(t, runtime.RunPodSandbox(ctx, podReq))

			ctrReq := &api.CreateContainerRequest{
				Pod:       pod,
				Container: ctr0,
			}
			reply, err := runtime.CreateContainer(ctx, ctrReq)
			require.NotNil(t, reply)
			require.Nil(t, err)

			ctrReq = &api.CreateContainerRequest{
				Pod:       pod,
				Container: ctr1,
			}
			reply, err = runtime.CreateContainer(ctx, ctrReq)
			require.NotNil(t, reply)
			require.Nil(t, err)
		})
	})

	t.Run("default validator disallows unconfined seccomp policy adjustment", func(t *testing.T) {
		setup := func(t *testing.T) {
			s.Prepare(t,
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
		}

		t.Run("should reject unconfined seccomp policy adjustment", func(t *testing.T) {
			setup(t)

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
			require.NoError(t, runtime.RunPodSandbox(ctx, podReq))

			ctrReq := &api.CreateContainerRequest{
				Pod:       pod,
				Container: ctr0,
			}
			reply, err := runtime.CreateContainer(ctx, ctrReq)
			require.NotNil(t, reply)
			require.Nil(t, err)

			ctrReq = &api.CreateContainerRequest{
				Pod:       pod,
				Container: ctr1,
			}
			reply, err = runtime.CreateContainer(ctx, ctrReq)
			require.NotNil(t, err)
			require.Nil(t, reply)
		})
	})

	t.Run("default validator allows unconfined seccomp policy adjustment", func(t *testing.T) {
		setup := func(t *testing.T) {
			s.Prepare(t,
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
		}

		t.Run("should not reject unconfined seccomp policy adjustment", func(t *testing.T) {
			setup(t)

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
			require.NoError(t, runtime.RunPodSandbox(ctx, podReq))

			ctrReq := &api.CreateContainerRequest{
				Pod:       pod,
				Container: ctr0,
			}
			reply, err := runtime.CreateContainer(ctx, ctrReq)
			require.NotNil(t, reply)
			require.Nil(t, err)

			ctrReq = &api.CreateContainerRequest{
				Pod:       pod,
				Container: ctr1,
			}
			reply, err = runtime.CreateContainer(ctx, ctrReq)
			require.NotNil(t, reply)
			require.Nil(t, err)
		})
	})

	t.Run("the default validator is enabled and namespace adjustment is disabled", func(t *testing.T) {
		setup := func(t *testing.T) {
			s.Prepare(t,
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
		}

		t.Run("should reject namespace adjustment", func(t *testing.T) {
			setup(t)

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
			require.NoError(t, runtime.RunPodSandbox(ctx, podReq))

			ctrReq := &api.CreateContainerRequest{
				Pod:       pod,
				Container: ctr0,
			}
			reply, err := runtime.CreateContainer(ctx, ctrReq)
			require.NotNil(t, reply)
			require.Nil(t, err)

			ctrReq = &api.CreateContainerRequest{
				Pod:       pod,
				Container: ctr1,
			}
			reply, err = runtime.CreateContainer(ctx, ctrReq)
			require.NotNil(t, err)
			require.Nil(t, reply)
		})
	})

	t.Run("the default validator is enabled with some required plugins", func(t *testing.T) {
		const AnnotationDomain = nriplugin.AnnotationDomain

		setup := func(t *testing.T) {
			s.Prepare(t,
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
		}

		t.Run("should not allow container creation if required plugins are missing", func(t *testing.T) {
			setup(t)

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
			require.NoError(t, runtime.RunPodSandbox(ctx, podReq))

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
			require.Nil(t, reply)
			require.NotNil(t, err)
		})

		t.Run("should allow container creation, if missing plugins are tolerated", func(t *testing.T) {
			setup(t)

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
			require.NoError(t, runtime.RunPodSandbox(ctx, podReq))

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
			require.NotNil(t, reply)
			require.Nil(t, err)
		})

		t.Run("should allow container creation if all required plugins are present", func(t *testing.T) {
			setup(t)

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
			require.NoError(t, runtime.RunPodSandbox(ctx, podReq))

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
			require.NotNil(t, reply)
			require.Nil(t, err)
		})

		t.Run("should not allow container creation if annotated required plugins are missing", func(t *testing.T) {
			setup(t)

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
			require.NoError(t, runtime.RunPodSandbox(ctx, podReq))

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
			require.Nil(t, reply)
			require.NotNil(t, err)

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
			require.NotNil(t, reply)
			require.Nil(t, err)
		})
	})
}

// --------------------------------------------

func TestPluginContainerUpdatesDuringCreation(t *testing.T) {
	s := &Suite{}

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

	t.Run("there is a single plugin", func(t *testing.T) {
		runTable := func(subject string, expected *api.ContainerUpdate) func(*testing.T) {
			return func(t *testing.T) {
				s.Prepare(t, &mockRuntime{}, &mockPlugin{idx: "00", name: "test"})
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
				require.NoError(t, runtime.RunPodSandbox(ctx, podReq))
				ctrReq := &api.CreateContainerRequest{
					Pod:       pod0,
					Container: ctr0,
				}
				_, err := runtime.CreateContainer(ctx, ctrReq)
				require.Nil(t, err)

				podReq = &api.RunPodSandboxRequest{Pod: pod1}
				require.NoError(t, runtime.RunPodSandbox(ctx, podReq))
				ctrReq = &api.CreateContainerRequest{
					Pod:       pod1,
					Container: ctr1,
				}
				reply, err = runtime.CreateContainer(ctx, ctrReq)
				require.Nil(t, err)

				require.Equal(t, 1, len(reply.Update))
				expected.ContainerId = reply.Update[0].ContainerId
				require.True(t, protoEqual(reply.Update[0].Strip(), expected), protoDiff(reply.Update[0], expected))
			}
		}
		t.Run("should be successfully collected without conflicts", func(t *testing.T) {
			t.Run("update CPU resources", runTable("resources/cpu", &api.ContainerUpdate{
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
			}))
			t.Run("update memory resources", runTable("resources/memory", &api.ContainerUpdate{
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
			}))
			t.Run("update class-based resources", runTable("resources/classes", &api.ContainerUpdate{
				Linux: &api.LinuxContainerUpdate{
					Resources: &api.LinuxResources{
						RdtClass:     api.String("00-test"),
						BlockioClass: api.String("00-test"),
					},
				},
			}))
			t.Run("update hugepage limits", runTable("resources/hugepagelimits", &api.ContainerUpdate{
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
			}))
			t.Run("update cgroupv2 unified resources", runTable("resources/unified", &api.ContainerUpdate{
				Linux: &api.LinuxContainerUpdate{
					Resources: &api.LinuxResources{
						Unified: map[string]string{
							"resource.1": "value1",
							"resource.2": "value2",
						},
					},
				},
			}))
		})
	})

	t.Run("there are multiple plugins", func(t *testing.T) {
		runTable := func(subject string, which string, expected *api.ContainerUpdate) func(*testing.T) {
			return func(t *testing.T) {
				s.Prepare(t,
					&mockRuntime{},
					&mockPlugin{idx: "10", name: "foo"},
					&mockPlugin{idx: "00", name: "bar"},
				)
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
				require.NoError(t, runtime.RunPodSandbox(ctx, podReq))
				ctrReq := &api.CreateContainerRequest{
					Pod:       pod0,
					Container: ctr0,
				}
				_, err := runtime.CreateContainer(ctx, ctrReq)
				require.Nil(t, err)

				podReq = &api.RunPodSandboxRequest{Pod: pod1}
				require.NoError(t, runtime.RunPodSandbox(ctx, podReq))
				ctrReq = &api.CreateContainerRequest{
					Pod:       pod1,
					Container: ctr1,
				}
				reply, err = runtime.CreateContainer(ctx, ctrReq)
				if which == "both" {
					require.NotNil(t, err)
				} else {
					require.Nil(t, err)
					require.Equal(t, 1, len(reply.Update))
					expected.ContainerId = reply.Update[0].ContainerId
					require.True(t, protoEqual(reply.Update[0].Strip(), expected), protoDiff(reply.Update[0], expected))
				}
			}
		}
		t.Run("should fail with conflicts, successfully collected otherwise", func(t *testing.T) {
			t.Run("update CPU resources", runTable("resources/cpu", "both", nil))
			t.Run("update CPU resources", runTable("resources/cpu", "10-foo", &api.ContainerUpdate{
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
			}))
			t.Run("update memory resources", runTable("resources/memory", "both", nil))
			t.Run("update memory resources", runTable("resources/memory", "10-foo", &api.ContainerUpdate{
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
			}))
			t.Run("update class-based resources", runTable("resources/classes", "both", nil))
			t.Run("update class-based resources", runTable("resources/classes", "10-foo", &api.ContainerUpdate{
				Linux: &api.LinuxContainerUpdate{
					Resources: &api.LinuxResources{
						RdtClass:     api.String("10-foo"),
						BlockioClass: api.String("10-foo"),
					},
				},
			}))
			t.Run("update hugepage limits", runTable("resources/hugepagelimits", "both", nil))
			t.Run("update hugepage limits", runTable("resources/hugepagelimits", "10-foo", &api.ContainerUpdate{
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
			}))
			t.Run("update cgroupv2 unified resources", runTable("resources/unified", "both", nil))
			t.Run("update cgroupv2 unified resources", runTable("resources/unified", "10-foo", &api.ContainerUpdate{
				Linux: &api.LinuxContainerUpdate{
					Resources: &api.LinuxResources{
						Unified: map[string]string{
							"resource.1": "value1",
							"resource.2": "value2",
						},
					},
				},
			}))
		})
	})
}

func TestSolicitedContainerUpdatesByPlugins(t *testing.T) {
	s := &Suite{}

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

	t.Run("there is a single plugin", func(t *testing.T) {
		runTable := func(subject string, expected *api.ContainerUpdate) func(*testing.T) {
			return func(t *testing.T) {
				s.Prepare(t, &mockRuntime{}, &mockPlugin{idx: "00", name: "test"})
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
				require.NoError(t, runtime.RunPodSandbox(ctx, podReq))
				ctrReq := &api.CreateContainerRequest{
					Pod:       pod0,
					Container: ctr0,
				}
				_, err := runtime.CreateContainer(ctx, ctrReq)
				require.Nil(t, err)

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

				require.Equal(t, 1, len(reply.Update))
				require.Nil(t, err)
				expected.ContainerId = reply.Update[0].ContainerId
				require.True(t, protoEqual(reply.Update[0].Strip(), expected), protoDiff(reply.Update[0], expected))
			}
		}
		t.Run("should be successfully collected without conflicts", func(t *testing.T) {
			t.Run("update CPU resources", runTable("resources/cpu", &api.ContainerUpdate{
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
			}))
			t.Run("update memory resources", runTable("resources/memory", &api.ContainerUpdate{
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
			}))
			t.Run("update class-based resources", runTable("resources/classes", &api.ContainerUpdate{
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
			}))
			t.Run("update hugepage limits", runTable("resources/hugepagelimits", &api.ContainerUpdate{
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
			}))
			t.Run("update cgroupv2 unified resources", runTable("resources/unified", &api.ContainerUpdate{
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
			}))
		})
	})

	t.Run("there are multiple plugins", func(t *testing.T) {
		runTable := func(subject string, which string, expected *api.ContainerUpdate) func(*testing.T) {
			return func(t *testing.T) {
				s.Prepare(t,
					&mockRuntime{},
					&mockPlugin{idx: "10", name: "foo"},
					&mockPlugin{idx: "00", name: "bar"},
				)
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
				require.NoError(t, runtime.RunPodSandbox(ctx, podReq))
				ctrReq := &api.CreateContainerRequest{
					Pod:       pod0,
					Container: ctr0,
				}
				_, err := runtime.CreateContainer(ctx, ctrReq)
				require.Nil(t, err)

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
					require.NotNil(t, err)
				} else {
					require.Nil(t, err)
					require.Equal(t, 1, len(reply.Update))
					expected.ContainerId = reply.Update[0].ContainerId
					require.True(t, protoEqual(reply.Update[0].Strip(), expected), protoDiff(reply.Update[0], expected))

				}
			}
		}
		t.Run("should fail with conflicts, successfully collected otherwise", func(t *testing.T) {
			t.Run("update CPU resources", runTable("resources/cpu", "both", nil))
			t.Run("update CPU resources", runTable("resources/cpu", "10-foo", &api.ContainerUpdate{
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
			}))
			t.Run("update memory resources", runTable("resources/memory", "both", nil))
			t.Run("update memory resources", runTable("resources/memory", "10-foo", &api.ContainerUpdate{
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
			}))
			t.Run("update class-based resources", runTable("resources/classes", "both", nil))
			t.Run("update class-based resources", runTable("resources/classes", "10-foo", &api.ContainerUpdate{
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
			}))
			t.Run("update hugepage limits", runTable("resources/hugepagelimits", "both", nil))
			t.Run("update hugepage limits", runTable("resources/hugepagelimits", "10-foo", &api.ContainerUpdate{
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
			}))
			t.Run("update cgroupv2 unified resources", runTable("resources/unified", "both", nil))
			t.Run("update cgroupv2 unified resources", runTable("resources/unified", "10-foo", &api.ContainerUpdate{
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
			}))
		})
	})
}

func TestUnsolicitedContainerUpdateRequests(t *testing.T) {
	s := &Suite{}

	t.Run("there are plugins", func(t *testing.T) {
		setup := func(t *testing.T) {
			s.Prepare(t, &mockRuntime{}, &mockPlugin{idx: "00", name: "test"})
		}

		t.Run("should fail gracefully without unstarted plugins", func(t *testing.T) {
			setup(t)
			plugin := s.plugins[0]

			s.StartRuntime()
			require.NoError(t, plugin.Init(s.Dir()))

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
			require.NotNil(t, err)
		})

		t.Run("should be delivered, without crash/panic", func(t *testing.T) {
			setup(t)

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
			require.NoError(t, runtime.startStopPodAndContainer(ctx, pod, ctr))

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

			require.Nil(t, failed)
			require.Nil(t, err)
			require.NotEqual(t, requestedUpdates, recordedUpdates)
		})
	})
}

func TestPluginConfigurationRequest(t *testing.T) {
	s := &Suite{}

	setup := func(t *testing.T) {
		s.Prepare(t, &mockRuntime{}, &mockPlugin{idx: "00", name: "test"})
	}

	t.Run("should pass runtime version information to plugins", func(t *testing.T) {
		setup(t)

		var (
			runtimeName    = "test-runtime"
			runtimeVersion = "1.2.3"
		)

		s.runtime.name = runtimeName
		s.runtime.version = runtimeVersion

		s.Startup()

		require.Equal(t, runtimeName, s.plugins[0].RuntimeName())
		require.Equal(t, runtimeVersion, s.plugins[0].RuntimeVersion())
	})

	t.Run("unchanged", func(t *testing.T) {
		t.Run("should pass default timeout information to plugins", func(t *testing.T) {
			setup(t)

			var (
				registerTimeout = nri.DefaultPluginRegistrationTimeout
				requestTimeout  = nri.DefaultPluginRequestTimeout
			)

			s.Startup()
			require.Equal(t, registerTimeout, s.plugins[0].stub.RegistrationTimeout())
			require.Equal(t, requestTimeout, s.plugins[0].stub.RequestTimeout())
		})
	})

	t.Run("reconfigured", func(t *testing.T) {
		var (
			registerTimeout = nri.DefaultPluginRegistrationTimeout + 5*time.Millisecond
			requestTimeout  = nri.DefaultPluginRequestTimeout + 7*time.Millisecond
		)

		setup := func(t *testing.T) {
			t.Helper()

			nri.SetPluginRegistrationTimeout(registerTimeout)
			nri.SetPluginRequestTimeout(requestTimeout)
			s.Prepare(t, &mockRuntime{}, &mockPlugin{idx: "00", name: "test"})

			t.Cleanup(func() {
				nri.SetPluginRegistrationTimeout(nri.DefaultPluginRegistrationTimeout)
				nri.SetPluginRequestTimeout(nri.DefaultPluginRequestTimeout)
			})
		}

		t.Run("should pass configured timeout information to plugins", func(t *testing.T) {
			setup(t)
			s.Startup()
			require.Equal(t, registerTimeout, s.plugins[0].stub.RegistrationTimeout())
			require.Equal(t, requestTimeout, s.plugins[0].stub.RequestTimeout())
		})
	})
}

func TestNRIVersionExchange(t *testing.T) {
	s := &Suite{}

	setup := func(t *testing.T) {
		s.Prepare(t, &mockRuntime{}, &mockPlugin{idx: "00", name: "test"})
	}

	t.Run("should pass runtime version information to plugins", func(t *testing.T) {
		setup(t)

		var (
			runtimeName    = "test-runtime"
			runtimeVersion = "1.2.3"
			nriVersion     = "v9.8.7"
		)

		s.runtime.name = runtimeName
		s.runtime.version = runtimeVersion
		s.runtime.options = append(s.runtime.options, nri.WithTestNRIVersion(nriVersion))

		s.Startup()

		require.Equal(t, runtimeName, s.plugins[0].RuntimeName())
		require.Equal(t, runtimeVersion, s.plugins[0].RuntimeVersion())
		require.Equal(t, nriVersion, s.plugins[0].RuntimeNRIVersion())
	})
}

func protoDiff(a, b proto.Message) string {
	return cmp.Diff(a, b, protocmp.Transform())
}

func protoEqual(a, b proto.Message) bool {
	return cmp.Equal(a, b, cmpopts.EquateEmpty(), protocmp.Transform())
}
