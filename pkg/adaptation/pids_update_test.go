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
	"testing"

	"github.com/containerd/nri/pkg/api"
	"github.com/stretchr/testify/require"
)

func TestPluginPidsUpdates(t *testing.T) {
	for _, target := range []string{"ctr0", "ctr1"} {
		for _, initial := range []*api.LinuxPids{nil, {Limit: 64}} {
			for _, limit := range []int64{128, 0, -1} {
				t.Run(fmt.Sprintf("%s/initial=%v/limit=%d", target, initial, limit), func(t *testing.T) {
					pidUpdate := &api.ContainerUpdate{ContainerId: target}
					pidUpdate.SetLinuxPidLimits(limit)
					cpuUpdate := &api.ContainerUpdate{ContainerId: target}
					cpuUpdate.SetLinuxCPUShares(256)

					var observed *api.LinuxPids
					plugins := []*mockPlugin{
						{idx: "00", name: "pids"},
						{idx: "10", name: "cpu"},
					}
					plugins[0].updateContainer = func(*mockPlugin, *api.PodSandbox, *api.Container, *api.LinuxResources) ([]*api.ContainerUpdate, error) {
						return []*api.ContainerUpdate{pidUpdate}, nil
					}
					plugins[1].updateContainer = func(_ *mockPlugin, _ *api.PodSandbox, _ *api.Container, resources *api.LinuxResources) ([]*api.ContainerUpdate, error) {
						observed = resources.GetPids()
						return []*api.ContainerUpdate{cpuUpdate}, nil
					}
					s := &Suite{}
					s.Prepare(t, &mockRuntime{}, plugins...)
					s.Startup()
					response, err := s.runtime.UpdateContainer(context.Background(), &api.UpdateContainerRequest{
						Pod:            &api.PodSandbox{Id: "pod0"},
						Container:      &api.Container{Id: "ctr0"},
						LinuxResources: &api.LinuxResources{Pids: initial},
					})
					require.NoError(t, err)
					for _, update := range response.Update {
						if update.ContainerId == target {
							require.Equal(t, &api.LinuxPids{Limit: limit}, update.Linux.Resources.Pids)
							require.Equal(t, uint64(256), update.Linux.Resources.Cpu.Shares.Value)
							if target == "ctr0" {
								require.Equal(t, &api.LinuxPids{Limit: limit}, observed)
							}
							return
						}
					}
					t.Fatalf("missing update for %s", target)
				})
			}
		}
	}
}

func TestPluginPidsUpdateOwnership(t *testing.T) {
	for _, test := range []struct {
		name     string
		first    *api.ContainerUpdate
		conflict bool
		ignore   bool
	}{
		{name: "nil resources", first: &api.ContainerUpdate{ContainerId: "ctr0"}},
		{name: "empty resources", first: &api.ContainerUpdate{ContainerId: "ctr0", Linux: &api.LinuxContainerUpdate{Resources: &api.LinuxResources{}}}},
		{name: "CPU resources", first: &api.ContainerUpdate{ContainerId: "ctr0", Linux: &api.LinuxContainerUpdate{Resources: &api.LinuxResources{Cpu: &api.LinuxCPU{Shares: api.UInt64(256)}}}}},
		{name: "conflicting PID limits", first: &api.ContainerUpdate{ContainerId: "ctr0", Linux: &api.LinuxContainerUpdate{Resources: &api.LinuxResources{Pids: &api.LinuxPids{Limit: 32}}}}, conflict: true},
		{name: "ignored conflicting PID limits", first: &api.ContainerUpdate{ContainerId: "ctr0", Linux: &api.LinuxContainerUpdate{Resources: &api.LinuxResources{Pids: &api.LinuxPids{Limit: 32}}}}, ignore: true},
	} {
		for _, initial := range []*api.LinuxPids{nil, {Limit: 64}} {
			t.Run(fmt.Sprintf("%s/initial=%v", test.name, initial), func(t *testing.T) {
				plugins := []*mockPlugin{{idx: "00", name: "first"}, {idx: "10", name: "pids"}}
				plugins[0].updateContainer = func(*mockPlugin, *api.PodSandbox, *api.Container, *api.LinuxResources) ([]*api.ContainerUpdate, error) {
					return []*api.ContainerUpdate{test.first}, nil
				}
				plugins[1].updateContainer = func(*mockPlugin, *api.PodSandbox, *api.Container, *api.LinuxResources) ([]*api.ContainerUpdate, error) {
					update := &api.ContainerUpdate{ContainerId: "ctr0"}
					update.SetLinuxPidLimits(128)
					if test.ignore {
						update.SetIgnoreFailure()
						update.SetLinuxCPUShares(256)
					}
					return []*api.ContainerUpdate{update}, nil
				}
				s := &Suite{}
				s.Prepare(t, &mockRuntime{}, plugins...)
				s.Startup()
				response, err := s.runtime.UpdateContainer(context.Background(), &api.UpdateContainerRequest{
					Pod:            &api.PodSandbox{Id: "pod0"},
					Container:      &api.Container{Id: "ctr0"},
					LinuxResources: &api.LinuxResources{Pids: initial},
				})
				if test.conflict {
					require.ErrorContains(t, err, "both tried to set PidsLimit")
					return
				}
				require.NoError(t, err)
				require.Len(t, response.Update, 1)
				if test.ignore {
					require.Equal(t, &api.LinuxPids{Limit: 32}, response.Update[0].Linux.Resources.Pids)
					require.Nil(t, response.Update[0].Linux.Resources.Cpu.GetShares())
					return
				}
				require.Equal(t, &api.LinuxPids{Limit: 128}, response.Update[0].Linux.Resources.Pids)
			})
		}
	}
}
