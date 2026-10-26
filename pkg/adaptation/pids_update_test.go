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
	"testing"

	"github.com/containerd/nri/pkg/api"
	"github.com/stretchr/testify/require"
)

// Regression tests for plugin-requested pids limit updates.
//
// updateResources() used to read the pids limit to apply from the merged
// copy of the container's current resources instead of the plugin's
// update (u.Linux.Resources), unlike every other resource field in the
// function. Consequently a pids limit requested by a plugin was silently
// dropped if the update request carried none, and silently ignored (the
// old value kept) if the request carried one.

func TestPluginPidsLimitUpdates(t *testing.T) {
	pod := func() *api.PodSandbox {
		return &api.PodSandbox{
			Id:        "pod0",
			Name:      "pod0",
			Uid:       "uid0",
			Namespace: "default",
		}
	}
	ctr := func(id string) *api.Container {
		return &api.Container{
			Id:           id,
			PodSandboxId: "pod0",
			Name:         id,
			State:        api.ContainerState_CONTAINER_CREATED,
		}
	}
	// CRI-style update request resources, without a pids limit.
	resources := func() *api.LinuxResources {
		return &api.LinuxResources{
			Cpu:    &api.LinuxCPU{},
			Memory: &api.LinuxMemory{},
		}
	}
	setPidsLimit := func(p *mockPlugin, limit int64) {
		p.updateContainer = func(_ *mockPlugin, _ *api.PodSandbox, c *api.Container, _ *api.LinuxResources) ([]*api.ContainerUpdate, error) {
			u := &api.ContainerUpdate{}
			u.SetContainerId(c.Id)
			u.SetLinuxPidLimits(limit)
			return []*api.ContainerUpdate{u}, nil
		}
	}

	t.Run("should apply a pids limit the request does not carry", func(t *testing.T) {
		s := &Suite{}
		s.Prepare(t, &mockRuntime{}, &mockPlugin{idx: "00", name: "pidcap"})
		runtime, ctx := s.runtime, context.Background()

		setPidsLimit(s.plugins[0], 100)
		s.Startup()

		require.NoError(t, runtime.RunPodSandbox(ctx, &api.RunPodSandboxRequest{Pod: pod()}))
		_, err := runtime.CreateContainer(ctx, &api.CreateContainerRequest{Pod: pod(), Container: ctr("ctr0")})
		require.NoError(t, err)

		reply, err := runtime.UpdateContainer(ctx, &api.UpdateContainerRequest{
			Pod:            pod(),
			Container:      ctr("ctr0"),
			LinuxResources: resources(),
		})
		require.NoError(t, err)
		require.Equal(t, 1, len(reply.Update))

		pids := reply.Update[0].GetLinux().GetResources().GetPids()
		require.NotNil(t, pids, "plugin's pids limit update was dropped")
		require.Equal(t, int64(100), pids.GetLimit())
	})

	t.Run("should override the pids limit carried by the request", func(t *testing.T) {
		s := &Suite{}
		s.Prepare(t, &mockRuntime{}, &mockPlugin{idx: "00", name: "pidcap"})
		runtime, ctx := s.runtime, context.Background()

		setPidsLimit(s.plugins[0], 100)
		s.Startup()

		require.NoError(t, runtime.RunPodSandbox(ctx, &api.RunPodSandboxRequest{Pod: pod()}))
		_, err := runtime.CreateContainer(ctx, &api.CreateContainerRequest{Pod: pod(), Container: ctr("ctr0")})
		require.NoError(t, err)

		res := resources()
		res.Pids = &api.LinuxPids{Limit: 200}
		reply, err := runtime.UpdateContainer(ctx, &api.UpdateContainerRequest{
			Pod:            pod(),
			Container:      ctr("ctr0"),
			LinuxResources: res,
		})
		require.NoError(t, err)
		require.Equal(t, 1, len(reply.Update))

		pids := reply.Update[0].GetLinux().GetResources().GetPids()
		require.NotNil(t, pids, "pids limit disappeared from the merged update")
		require.Equal(t, int64(100), pids.GetLimit())
	})

	t.Run("should conflict if two plugins set the pids limit", func(t *testing.T) {
		s := &Suite{}
		s.Prepare(t,
			&mockRuntime{},
			&mockPlugin{idx: "00", name: "pidcap1"},
			&mockPlugin{idx: "10", name: "pidcap2"},
		)
		runtime, ctx := s.runtime, context.Background()

		setPidsLimit(s.plugins[0], 100)
		setPidsLimit(s.plugins[1], 100)
		s.Startup()

		require.NoError(t, runtime.RunPodSandbox(ctx, &api.RunPodSandboxRequest{Pod: pod()}))
		_, err := runtime.CreateContainer(ctx, &api.CreateContainerRequest{Pod: pod(), Container: ctr("ctr0")})
		require.NoError(t, err)

		_, err = runtime.UpdateContainer(ctx, &api.UpdateContainerRequest{
			Pod:            pod(),
			Container:      ctr("ctr0"),
			LinuxResources: resources(),
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "PidsLimit")
	})

	t.Run("should propagate a pids limit in create container update section", func(t *testing.T) {
		s := &Suite{}
		s.Prepare(t, &mockRuntime{}, &mockPlugin{idx: "00", name: "pidcap"})
		runtime, ctx := s.runtime, context.Background()

		s.plugins[0].createContainer = func(_ *mockPlugin, _ *api.PodSandbox, c *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
			if c.Id != "ctr1" {
				return nil, nil, nil
			}
			u := &api.ContainerUpdate{}
			u.SetContainerId("ctr0")
			u.SetLinuxPidLimits(300)
			return nil, []*api.ContainerUpdate{u}, nil
		}
		s.Startup()

		require.NoError(t, runtime.RunPodSandbox(ctx, &api.RunPodSandboxRequest{Pod: pod()}))
		_, err := runtime.CreateContainer(ctx, &api.CreateContainerRequest{Pod: pod(), Container: ctr("ctr0")})
		require.NoError(t, err)

		resp, err := runtime.CreateContainer(ctx, &api.CreateContainerRequest{Pod: pod(), Container: ctr("ctr1")})
		require.NoError(t, err)

		var upd *api.ContainerUpdate
		for _, u := range resp.GetUpdate() {
			if u.GetContainerId() == "ctr0" {
				upd = u
			}
		}
		require.NotNil(t, upd, "update for ctr0 missing from create container response")

		pids := upd.GetLinux().GetResources().GetPids()
		require.NotNil(t, pids, "pids limit update in create container update section was dropped")
		require.Equal(t, int64(300), pids.GetLimit())
	})
}
