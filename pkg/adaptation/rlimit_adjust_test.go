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

// Regression tests for rlimit adjustments carrying duplicate types.
//
// adjustRlimits() used to blindly append every adjusted rlimit to both
// the create request and the adjustment returned to the runtime. The
// OCI spec allows at most one rlimit entry per type, so a plugin
// adjusting a type already present in the create request left a
// duplicate in the view that later plugins see. Rlimits are now merged
// by type in both views, the last adjustment of a type winning.
// Re-adjusting a type the plugin itself already owns, on the other
// hand, is deliberately still a conflict: a plugin is expected to
// resolve that internally and send a single final entry.

func TestPluginRlimitAdjustments(t *testing.T) {
	pod := func() *api.PodSandbox {
		return &api.PodSandbox{
			Id:        "pod0",
			Name:      "pod0",
			Uid:       "uid0",
			Namespace: "default",
		}
	}
	ctr := func(id string, rlimits ...*api.POSIXRlimit) *api.Container {
		return &api.Container{
			Id:           id,
			PodSandboxId: "pod0",
			Name:         id,
			State:        api.ContainerState_CONTAINER_CREATED,
			Rlimits:      rlimits,
		}
	}

	// Per review, this stays a conflict: a plugin sending the same type
	// twice is expected to resolve that internally and send a single
	// final entry, rather than having the adaptation merge it away.
	t.Run("should fail if one plugin sets the same rlimit type twice", func(t *testing.T) {
		s := &Suite{}
		s.Prepare(t, &mockRuntime{}, &mockPlugin{idx: "00", name: "rlimit"})
		runtime, ctx := s.runtime, context.Background()

		s.plugins[0].createContainer = func(_ *mockPlugin, _ *api.PodSandbox, _ *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
			a := &api.ContainerAdjustment{}
			a.AddRlimit("nofile", 100, 50)
			a.AddRlimit("nofile", 200, 150)
			return a, nil, nil
		}
		s.Startup()

		require.NoError(t, runtime.RunPodSandbox(ctx, &api.RunPodSandboxRequest{Pod: pod()}))
		_, err := runtime.CreateContainer(ctx, &api.CreateContainerRequest{Pod: pod(), Container: ctr("ctr0")})
		require.Error(t, err)
		require.Contains(t, err.Error(), "Rlimits")
	})

	t.Run("should merge an adjusted rlimit into the view seen by later plugins", func(t *testing.T) {
		s := &Suite{}
		s.Prepare(t,
			&mockRuntime{},
			&mockPlugin{idx: "00", name: "first"},
			&mockPlugin{idx: "10", name: "second"},
		)
		runtime, ctx := s.runtime, context.Background()

		s.plugins[0].createContainer = func(_ *mockPlugin, _ *api.PodSandbox, _ *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
			a := &api.ContainerAdjustment{}
			a.AddRlimit("nofile", 200, 150)
			return a, nil, nil
		}
		var seen []*api.POSIXRlimit
		s.plugins[1].createContainer = func(_ *mockPlugin, _ *api.PodSandbox, c *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
			seen = c.Rlimits
			return nil, nil, nil
		}
		s.Startup()

		require.NoError(t, runtime.RunPodSandbox(ctx, &api.RunPodSandboxRequest{Pod: pod()}))
		ctrReq := &api.CreateContainerRequest{
			Pod:       pod(),
			Container: ctr("ctr0", &api.POSIXRlimit{Type: "nofile", Hard: 5, Soft: 5}),
		}
		_, err := runtime.CreateContainer(ctx, ctrReq)
		require.NoError(t, err)

		// The second plugin sees the adjustment merged into the entry
		// already in the create request, instead of a duplicate appended.
		requireRlimits(t, []*api.POSIXRlimit{
			{Type: "nofile", Hard: 200, Soft: 150},
		}, seen)
	})

	t.Run("should merge an adjusted rlimit into one already in the create request", func(t *testing.T) {
		s := &Suite{}
		s.Prepare(t,
			&mockRuntime{},
			&mockPlugin{idx: "00", name: "rlimit"},
			&mockPlugin{idx: "10", name: "observer"},
		)
		runtime, ctx := s.runtime, context.Background()

		s.plugins[0].createContainer = func(_ *mockPlugin, _ *api.PodSandbox, _ *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
			a := &api.ContainerAdjustment{}
			a.AddRlimit("nofile", 200, 150)
			return a, nil, nil
		}
		s.Startup()

		require.NoError(t, runtime.RunPodSandbox(ctx, &api.RunPodSandboxRequest{Pod: pod()}))
		ctrReq := &api.CreateContainerRequest{
			Pod:       pod(),
			Container: ctr("ctr0", &api.POSIXRlimit{Type: "nofile", Hard: 5, Soft: 5}),
		}
		reply, err := runtime.CreateContainer(ctx, ctrReq)
		require.NoError(t, err)

		// The adjustment the runtime applies carries the plugin's entry.
		requireRlimits(t, []*api.POSIXRlimit{
			{Type: "nofile", Hard: 200, Soft: 150},
		}, reply.GetAdjust().GetRlimits())
		// The create request has the plugin's entry merged into the one
		// already there, instead of a duplicate appended.
		requireRlimits(t, []*api.POSIXRlimit{
			{Type: "nofile", Hard: 200, Soft: 150},
		}, ctrReq.GetContainer().GetRlimits())
	})

	t.Run("should merge equivalent rlimit types in different forms", func(t *testing.T) {
		s := &Suite{}
		s.Prepare(t, &mockRuntime{}, &mockPlugin{idx: "00", name: "rlimit"})
		runtime, ctx := s.runtime, context.Background()

		s.plugins[0].createContainer = func(_ *mockPlugin, _ *api.PodSandbox, _ *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
			a := &api.ContainerAdjustment{}
			a.AddRlimit("nofile", 200, 150)
			return a, nil, nil
		}
		s.Startup()

		require.NoError(t, runtime.RunPodSandbox(ctx, &api.RunPodSandboxRequest{Pod: pod()}))
		ctrReq := &api.CreateContainerRequest{
			Pod:       pod(),
			Container: ctr("ctr0", &api.POSIXRlimit{Type: "RLIMIT_NOFILE", Hard: 5, Soft: 5}),
		}
		reply, err := runtime.CreateContainer(ctx, ctrReq)
		require.NoError(t, err)

		// "nofile" and "RLIMIT_NOFILE" are the same rlimit type, so the
		// adjustment merges into the existing entry, keeping the type
		// spelling of the adjustment.
		requireRlimits(t, []*api.POSIXRlimit{
			{Type: "nofile", Hard: 200, Soft: 150},
		}, reply.GetAdjust().GetRlimits())
		requireRlimits(t, []*api.POSIXRlimit{
			{Type: "nofile", Hard: 200, Soft: 150},
		}, ctrReq.GetContainer().GetRlimits())
	})

	t.Run("should conflict if two plugins set the same rlimit type", func(t *testing.T) {
		s := &Suite{}
		s.Prepare(t,
			&mockRuntime{},
			&mockPlugin{idx: "00", name: "first"},
			&mockPlugin{idx: "10", name: "second"},
		)
		runtime, ctx := s.runtime, context.Background()

		setRlimits := func(p *mockPlugin, hard, soft uint64) {
			p.createContainer = func(_ *mockPlugin, _ *api.PodSandbox, _ *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
				a := &api.ContainerAdjustment{}
				a.AddRlimit("nofile", hard, soft)
				return a, nil, nil
			}
		}
		setRlimits(s.plugins[0], 100, 50)
		setRlimits(s.plugins[1], 200, 150)
		s.Startup()

		require.NoError(t, runtime.RunPodSandbox(ctx, &api.RunPodSandboxRequest{Pod: pod()}))
		_, err := runtime.CreateContainer(ctx, &api.CreateContainerRequest{Pod: pod(), Container: ctr("ctr0")})
		require.Error(t, err)
		require.Contains(t, err.Error(), "Rlimits")
	})

	t.Run("should conflict if two plugins set equivalent rlimit types", func(t *testing.T) {
		s := &Suite{}
		s.Prepare(t,
			&mockRuntime{},
			&mockPlugin{idx: "00", name: "first"},
			&mockPlugin{idx: "10", name: "second"},
		)
		runtime, ctx := s.runtime, context.Background()

		s.plugins[0].createContainer = func(_ *mockPlugin, _ *api.PodSandbox, _ *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
			a := &api.ContainerAdjustment{}
			a.AddRlimit("nofile", 100, 50)
			return a, nil, nil
		}
		s.plugins[1].createContainer = func(_ *mockPlugin, _ *api.PodSandbox, _ *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
			a := &api.ContainerAdjustment{}
			a.AddRlimit("RLIMIT_NOFILE", 200, 150)
			return a, nil, nil
		}
		s.Startup()

		require.NoError(t, runtime.RunPodSandbox(ctx, &api.RunPodSandboxRequest{Pod: pod()}))
		_, err := runtime.CreateContainer(ctx, &api.CreateContainerRequest{Pod: pod(), Container: ctr("ctr0")})
		require.Error(t, err)
		require.Contains(t, err.Error(), "Rlimits")
	})
}

// requireRlimits compares rlimits with proto semantics, since some of
// the actual entries have crossed the plugin connection and therefore
// carry internal proto state that deep equality would trip over.
func requireRlimits(t *testing.T, expected, actual []*api.POSIXRlimit) {
	t.Helper()
	require.Equal(t, len(expected), len(actual), "unexpected rlimits: %v", actual)
	for i := range expected {
		require.True(t, protoEqual(expected[i], actual[i]),
			"rlimit[%d] differs, got %v, want %v", i, actual[i], expected[i])
	}
}
