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

func TestIgnoredUpdateDoesNotRetainOwnership(t *testing.T) {
	for _, target := range []string{"ctr0", "ctr1"} {
		for _, rejectedMemory := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/rejectedMemory=%t", target, rejectedMemory), func(t *testing.T) {
				first := &api.ContainerUpdate{ContainerId: target}
				first.SetLinuxCPUShares(256)
				ignored := &api.ContainerUpdate{ContainerId: target}
				if rejectedMemory {
					ignored.SetLinuxMemoryLimit(1024)
				}
				ignored.SetLinuxCPUShares(512)
				ignored.SetIgnoreFailure()
				last := &api.ContainerUpdate{ContainerId: target}
				last.SetLinuxMemoryLimit(4096)

				var observed *api.LinuxResources
				plugins := []*mockPlugin{
					{idx: "00", name: "cpu"},
					{idx: "10", name: "ignored"},
					{idx: "20", name: "memory"},
				}
				plugins[0].updateContainer = func(*mockPlugin, *api.PodSandbox, *api.Container, *api.LinuxResources) ([]*api.ContainerUpdate, error) {
					return []*api.ContainerUpdate{first}, nil
				}
				plugins[1].updateContainer = func(*mockPlugin, *api.PodSandbox, *api.Container, *api.LinuxResources) ([]*api.ContainerUpdate, error) {
					return []*api.ContainerUpdate{ignored}, nil
				}
				plugins[2].updateContainer = func(_ *mockPlugin, _ *api.PodSandbox, _ *api.Container, resources *api.LinuxResources) ([]*api.ContainerUpdate, error) {
					observed = resources.Copy()
					return []*api.ContainerUpdate{last}, nil
				}
				s := &Suite{}
				s.Prepare(t, &mockRuntime{}, plugins...)
				s.Startup()
				response, err := s.runtime.UpdateContainer(context.Background(), &api.UpdateContainerRequest{
					Pod:            &api.PodSandbox{Id: "pod0"},
					Container:      &api.Container{Id: "ctr0"},
					LinuxResources: &api.LinuxResources{},
				})
				if target == "ctr0" {
					require.Equal(t, uint64(256), observed.GetCpu().GetShares().GetValue())
					require.Nil(t, observed.GetMemory().GetLimit(), "rejected update must not change resources")
				}
				require.NoError(t, err, "ignored update must not block a later memory update")
				for _, update := range response.Update {
					if update.ContainerId == target {
						require.Equal(t, uint64(256), update.Linux.Resources.Cpu.Shares.Value)
						require.Equal(t, int64(4096), update.Linux.Resources.Memory.Limit.Value)
						return
					}
				}
				t.Fatalf("missing update for %s", target)
			})
		}
	}
}

func TestResourceUpdateOwnershipTransactions(t *testing.T) {
	cpu := func(shares uint64) *api.LinuxResources {
		return &api.LinuxResources{Cpu: &api.LinuxCPU{Shares: api.UInt64(shares)}}
	}
	memory := func(limit int64) *api.LinuxResources {
		return &api.LinuxResources{Memory: &api.LinuxMemory{Limit: api.Int64(limit)}}
	}
	memoryCPU := func(limit int64, shares uint64) *api.LinuxResources {
		resources := memory(limit)
		resources.Cpu = cpu(shares).Cpu
		return resources
	}
	hugepages := func(limits ...*api.HugepageLimit) *api.LinuxResources {
		return &api.LinuxResources{HugepageLimits: limits}
	}
	for _, test := range []struct {
		name       string
		first      *api.LinuxResources
		rejected   *api.LinuxResources
		retry      *api.LinuxResources
		last       *api.LinuxResources
		expected   *api.LinuxResources
		conflict   string
		nonIgnored bool
		other      bool
		otherOwner bool
	}{
		{
			name: "discard simple claim", first: cpu(256),
			rejected: memoryCPU(1024, 512), last: memory(4096), expected: memoryCPU(4096, 256),
		},
		{
			name: "preserve prior simple owner", first: cpu(256),
			rejected: memoryCPU(1024, 512), last: cpu(1024), conflict: "CPUShares",
		},
		{
			name: "discard compound claim", first: hugepages(&api.HugepageLimit{PageSize: "2M", Limit: 256}),
			rejected: hugepages(&api.HugepageLimit{PageSize: "1G", Limit: 512}, &api.HugepageLimit{PageSize: "2M", Limit: 512}),
			last:     hugepages(&api.HugepageLimit{PageSize: "1G", Limit: 1024}),
			expected: hugepages(&api.HugepageLimit{PageSize: "2M", Limit: 256}, &api.HugepageLimit{PageSize: "1G", Limit: 1024}),
		},
		{
			name: "preserve prior compound owner", first: hugepages(&api.HugepageLimit{PageSize: "2M", Limit: 256}),
			rejected: hugepages(&api.HugepageLimit{PageSize: "1G", Limit: 512}, &api.HugepageLimit{PageSize: "2M", Limit: 512}),
			last:     hugepages(&api.HugepageLimit{PageSize: "2M", Limit: 1024}), conflict: "HugepageLimits 2M",
		},
		{
			name:     "discard new container owner",
			rejected: hugepages(&api.HugepageLimit{PageSize: "2M", Limit: 256}, &api.HugepageLimit{PageSize: "2M", Limit: 512}),
			last:     hugepages(&api.HugepageLimit{PageSize: "2M", Limit: 1024}),
			expected: hugepages(&api.HugepageLimit{PageSize: "2M", Limit: 1024}),
		},
		{
			name: "retry after rejected update", first: cpu(256), rejected: memoryCPU(1024, 512), retry: memory(2048),
			last:     &api.LinuxResources{Memory: &api.LinuxMemory{Reservation: api.Int64(8192)}},
			expected: &api.LinuxResources{Cpu: cpu(256).Cpu, Memory: &api.LinuxMemory{Limit: api.Int64(2048), Reservation: api.Int64(8192)}},
		},
		{
			name: "preserve other successful updates", first: cpu(256),
			rejected: memoryCPU(1024, 512), last: memory(4096), expected: memoryCPU(4096, 256), other: true,
		},
		{
			name: "preserve other container owner", first: cpu(256),
			rejected: memoryCPU(1024, 512), last: memory(4096), conflict: "CPUShares", other: true, otherOwner: true,
		},
		{
			name: "non-ignored conflict still fails", first: cpu(256),
			rejected: memoryCPU(1024, 512), last: memory(4096), conflict: "CPUShares", nonIgnored: true,
		},
	} {
		for _, path := range []struct {
			name   string
			target string
			create bool
		}{
			{name: "update current", target: "ctr0"},
			{name: "update other", target: "ctr1"},
			{name: "create related", target: "ctr0", create: true},
		} {
			t.Run(test.name+"/"+path.name, func(t *testing.T) {
				update := func(id string, resources *api.LinuxResources) *api.ContainerUpdate {
					return &api.ContainerUpdate{ContainerId: id, Linux: &api.LinuxContainerUpdate{Resources: resources}}
				}
				var first []*api.ContainerUpdate
				if test.first != nil {
					first = []*api.ContainerUpdate{update(path.target, test.first)}
				}
				rejected := update(path.target, test.rejected)
				rejected.IgnoreFailure = !test.nonIgnored
				middle := []*api.ContainerUpdate{rejected}
				if test.retry != nil {
					middle = append(middle, update(path.target, test.retry))
				}
				last := []*api.ContainerUpdate{update(path.target, test.last)}
				if test.other {
					middle = append([]*api.ContainerUpdate{update("ctr2", cpu(128))}, middle...)
					middle = append(middle, update("ctr2", memory(16384)))
					if test.otherOwner {
						last = append(last, update("ctr2", cpu(512)))
					} else {
						last = append(last, update("ctr2", &api.LinuxResources{Memory: &api.LinuxMemory{Reservation: api.Int64(8192)}}))
					}
				}
				plugins := []*mockPlugin{{idx: "00", name: "first"}, {idx: "10", name: "ignored"}, {idx: "20", name: "last"}}
				for i, updates := range [][]*api.ContainerUpdate{first, middle, last} {
					plugins[i].updateContainer = func(*mockPlugin, *api.PodSandbox, *api.Container, *api.LinuxResources) ([]*api.ContainerUpdate, error) {
						return updates, nil
					}
					plugins[i].createContainer = func(*mockPlugin, *api.PodSandbox, *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
						var adjust *api.ContainerAdjustment
						if i == 0 {
							adjust = &api.ContainerAdjustment{}
							adjust.SetLinuxCPUQuota(1000)
						}
						return adjust, updates, nil
					}
				}
				s := &Suite{}
				s.Prepare(t, &mockRuntime{}, plugins...)
				s.Startup()
				var (
					updates []*api.ContainerUpdate
					err     error
				)
				if path.create {
					response, createErr := s.runtime.CreateContainer(context.Background(), &api.CreateContainerRequest{
						Pod: &api.PodSandbox{Id: "pod0"}, Container: &api.Container{Id: "created"},
					})
					err = createErr
					if err == nil {
						updates = response.Update
						require.Equal(t, int64(1000), response.Adjust.Linux.Resources.Cpu.Quota.Value)
					}
				} else {
					response, updateErr := s.runtime.UpdateContainer(context.Background(), &api.UpdateContainerRequest{
						Pod: &api.PodSandbox{Id: "pod0"}, Container: &api.Container{Id: "ctr0"}, LinuxResources: &api.LinuxResources{},
					})
					err = updateErr
					if err == nil {
						updates = response.Update
					}
				}
				if test.conflict != "" {
					require.ErrorContains(t, err, "both tried to set "+test.conflict)
					caller, owner := "20-last", "00-first"
					if test.nonIgnored {
						caller = "10-ignored"
					}
					if test.otherOwner {
						owner = "10-ignored"
					}
					require.ErrorContains(t, err, fmt.Sprintf("plugins %q and %q", caller, owner))
					return
				}
				require.NoError(t, err)
				found, foundOther := false, !test.other
				for _, u := range updates {
					switch u.GetContainerId() {
					case path.target:
						expected := update(path.target, test.expected)
						require.True(t, protoEqual(u.Strip(), expected), protoDiff(u.Strip(), expected))
						found = true
					case "ctr2":
						foundOther = true
						expected := update("ctr2", &api.LinuxResources{
							Cpu: cpu(128).Cpu, Memory: &api.LinuxMemory{Limit: api.Int64(16384), Reservation: api.Int64(8192)},
						})
						require.True(t, protoEqual(u.Strip(), expected), protoDiff(u.Strip(), expected))
					}
				}
				require.True(t, found, "missing update for %s", path.target)
				require.True(t, foundOther, "missing successful updates for ctr2")
			})
		}
	}
}

func TestIgnoredRelatedUpdatePreservesCreationOwner(t *testing.T) {
	plugins := []*mockPlugin{{idx: "00", name: "first"}, {idx: "10", name: "ignored"}, {idx: "20", name: "last"}}
	plugins[0].createContainer = func(*mockPlugin, *api.PodSandbox, *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
		adjust := &api.ContainerAdjustment{}
		adjust.SetLinuxCPUShares(256)
		update := &api.ContainerUpdate{ContainerId: "existing"}
		update.SetLinuxCPUShares(128)
		return adjust, []*api.ContainerUpdate{update}, nil
	}
	plugins[1].createContainer = func(*mockPlugin, *api.PodSandbox, *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
		update := &api.ContainerUpdate{ContainerId: "existing"}
		update.SetLinuxMemoryLimit(1024)
		update.SetLinuxCPUShares(512)
		update.SetIgnoreFailure()
		return nil, []*api.ContainerUpdate{update}, nil
	}
	plugins[2].createContainer = func(*mockPlugin, *api.PodSandbox, *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
		adjust := &api.ContainerAdjustment{}
		adjust.SetLinuxCPUShares(512)
		return adjust, nil, nil
	}
	s := &Suite{}
	s.Prepare(t, &mockRuntime{}, plugins...)
	s.Startup()
	_, err := s.runtime.CreateContainer(context.Background(), &api.CreateContainerRequest{
		Pod: &api.PodSandbox{Id: "pod0"}, Container: &api.Container{Id: "created"},
	})
	require.ErrorContains(t, err, "plugins \"20-last\" and \"00-first\" both tried to set CPUShares")
}
