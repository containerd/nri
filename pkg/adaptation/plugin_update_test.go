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
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	nri "github.com/containerd/nri/pkg/adaptation"
	"github.com/containerd/nri/pkg/adaptation/builtin"
	"github.com/containerd/nri/pkg/api"

	"github.com/stretchr/testify/require"
)

// A plugin which has registered, but which the runtime does not consider
// live, must not be able to request unsolicited container updates.
func TestUpdateContainersFromPluginWhichIsNotLive(t *testing.T) {
	var (
		s      = &Suite{}
		plugin = &mockPlugin{idx: "10", name: "test"}
	)

	s.Prepare(t, &mockRuntime{})
	s.StartRuntime()

	// Hold off plugin synchronization. The accept loop registers and
	// configures the plugin, then blocks before synchronizing it, so the
	// plugin is connected and registered but not among the runtime's
	// plugins.
	block := s.runtime.runtime.BlockPluginSync()
	defer block.Unblock()

	s.StartPlugins(plugin)
	require.NoError(t, plugin.Wait(PluginConfigured, time.After(startupTimeout)))

	_, err := plugin.stub.UpdateContainers([]*api.ContainerUpdate{
		{ContainerId: "ctr0"},
	})
	require.Error(t, err, "unsolicited update from a plugin which is not live")
}

// A second instance of a plugin takes over the name once it has
// synchronized. The instance it replaces stops receiving events, but is
// not disconnected.
func TestPluginUpdateTakesOverTheName(t *testing.T) {
	var (
		s         = &Suite{}
		incumbent = &mockPlugin{idx: "10", name: "test"}
		newcomer  = &mockPlugin{idx: "10", name: "test"}
		ctx       = context.Background()
		pod       = &api.PodSandbox{Id: "pod0", Name: "pod0", Namespace: "default"}
	)

	s.Prepare(t, &mockRuntime{}, incumbent)
	s.Startup()

	s.StartPlugins(newcomer)
	s.WaitForPluginsToSync(newcomer)

	incumbent.EventQ().Reset()
	newcomer.EventQ().Reset()

	require.NoError(t, s.runtime.RunPodSandbox(ctx, &api.RunPodSandboxRequest{Pod: pod}))

	require.True(t, newcomer.EventQ().Has(PodSandboxEvent(pod, RunPodSandbox)),
		"the newcomer instance should receive the event")
	require.False(t, incumbent.EventQ().Has(PodSandboxEvent(pod, RunPodSandbox)),
		"the replaced instance should not receive the event")
	require.False(t, incumbent.EventQ().Has(PluginDisconnected),
		"the replaced instance should not be disconnected")
}

// A failed update leaves the instance it would have replaced live, and
// lets a later instance of the name register.
func TestFailedPluginUpdateKeepsTheLiveInstance(t *testing.T) {
	var (
		s       = &Suite{}
		live    = &mockPlugin{idx: "10", name: "test"}
		failing = &mockPlugin{
			idx:  "10",
			name: "test",
			synchronize: func(*mockPlugin, []*api.PodSandbox, []*api.Container) ([]*api.ContainerUpdate, error) {
				return nil, errors.New("mock plugin forced synchronize failure")
			},
		}
		retry = &mockPlugin{idx: "10", name: "test"}
		ctx   = context.Background()
		pod   = &api.PodSandbox{Id: "pod0", Name: "pod0", Namespace: "default"}
	)

	s.Prepare(t, &mockRuntime{}, live)
	s.Startup()

	s.StartPlugins(failing)
	require.NoError(t, failing.Wait(PluginDisconnected, time.After(startupTimeout)))

	live.EventQ().Reset()
	require.NoError(t, s.runtime.RunPodSandbox(ctx, &api.RunPodSandboxRequest{Pod: pod}))
	require.True(t, live.EventQ().Has(PodSandboxEvent(pod, RunPodSandbox)),
		"the live instance should still receive events")

	s.StartPlugins(retry)
	s.WaitForPluginsToSync(retry)

	live.EventQ().Reset()
	retry.EventQ().Reset()
	pod = &api.PodSandbox{Id: "pod1", Name: "pod1", Namespace: "default"}
	require.NoError(t, s.runtime.RunPodSandbox(ctx, &api.RunPodSandboxRequest{Pod: pod}))

	require.True(t, retry.EventQ().Has(PodSandboxEvent(pod, RunPodSandbox)),
		"a later instance of the name should take over")
	require.False(t, live.EventQ().Has(PodSandboxEvent(pod, RunPodSandbox)),
		"the replaced instance should not receive the event")
}

// A retired plugin cannot request unsolicited container updates.
func TestUpdateContainersFromRetiredPlugin(t *testing.T) {
	var (
		s        = &Suite{}
		retired  = &mockPlugin{idx: "10", name: "test"}
		newcomer = &mockPlugin{idx: "10", name: "test"}
	)

	s.Prepare(t, &mockRuntime{}, retired)
	s.Startup()

	s.StartPlugins(newcomer)
	s.WaitForPluginsToSync(newcomer)

	_, err := retired.stub.UpdateContainers([]*api.ContainerUpdate{
		{ContainerId: "ctr0"},
	})
	require.Error(t, err, "unsolicited update from a retired plugin")
}

// A retired plugin stays retired when the plugin which replaced it goes
// away.
func TestRetiredPluginIsNotPromoted(t *testing.T) {
	var (
		s        = &Suite{}
		retired  = &mockPlugin{idx: "10", name: "test"}
		newcomer = &mockPlugin{idx: "10", name: "test"}
		ctx      = context.Background()
		pod      = &api.PodSandbox{Id: "pod0", Name: "pod0", Namespace: "default"}
	)

	s.Prepare(t, &mockRuntime{}, retired)
	s.Startup()

	s.StartPlugins(newcomer)
	s.WaitForPluginsToSync(newcomer)

	newcomer.Stop()
	require.NoError(t, newcomer.Wait(PluginStopped, time.After(startupTimeout)))

	retired.EventQ().Reset()

	// The first relay may fail for the plugin which just went away. It
	// drops it, so the next one has no plugins left to relay to.
	_ = s.runtime.RunPodSandbox(ctx, &api.RunPodSandboxRequest{Pod: pod})
	require.NoError(t, s.runtime.RunPodSandbox(ctx, &api.RunPodSandboxRequest{Pod: pod}))

	require.False(t, retired.EventQ().Has(PodSandboxEvent(pod, RunPodSandbox)),
		"a retired plugin should not be promoted")
}

// A runtime is allowed to return no error from SyncFn although the
// synchronization callback failed. A plugin which failed to synchronize
// must not take over the name even then.
func TestFailedPluginUpdateWithSwallowedSyncError(t *testing.T) {
	var (
		s       = &Suite{}
		live    = &mockPlugin{idx: "10", name: "test"}
		failing = &mockPlugin{
			idx:  "10",
			name: "test",
			synchronize: func(*mockPlugin, []*api.PodSandbox, []*api.Container) ([]*api.ContainerUpdate, error) {
				return nil, errors.New("mock plugin forced synchronize failure")
			},
		}
		ctx = context.Background()
		pod = &api.PodSandbox{Id: "pod0", Name: "pod0", Namespace: "default"}
	)

	s.Prepare(t, &mockRuntime{swallowSyncErr: true}, live)
	s.Startup()

	s.StartPlugins(failing)
	require.NoError(t, failing.Wait(PluginDisconnected, time.After(startupTimeout)))

	live.EventQ().Reset()
	require.NoError(t, s.runtime.RunPodSandbox(ctx, &api.RunPodSandboxRequest{Pod: pod}))
	require.True(t, live.EventQ().Has(PodSandboxEvent(pod, RunPodSandbox)),
		"the live instance should still receive events")
}

// An external plugin cannot displace a builtin plugin of the same name.
func TestExternalPluginCannotDisplaceBuiltinPlugin(t *testing.T) {
	var (
		s        = &Suite{}
		runtime  = &mockRuntime{}
		external = &mockPlugin{idx: "10", name: "builtin-test"}
	)

	runtime.options = append(runtime.options,
		nri.WithBuiltinPlugins(&builtin.BuiltinPlugin{
			Index: "10",
			Base:  "builtin-test",
		}),
	)

	s.Prepare(t, runtime)
	s.StartRuntime()

	require.NoError(t, external.Init(s.Dir()))
	require.Error(t, external.Start(s.Dir()),
		"registration of an external plugin taking a builtin plugin's name")
}

// Two builtin plugins cannot share an indexed name.
func TestDuplicateBuiltinPluginNameIsRefused(t *testing.T) {
	var (
		s       = &Suite{}
		runtime = &mockRuntime{}
	)

	runtime.options = append(runtime.options,
		nri.WithBuiltinPlugins(
			&builtin.BuiltinPlugin{Index: "10", Base: "builtin-test"},
			&builtin.BuiltinPlugin{Index: "10", Base: "builtin-test"},
		),
	)

	s.Prepare(t, runtime)
	require.Error(t, runtime.Start(s.Dir()),
		"startup with two builtin plugins of the same name")
}

// childPids returns the pids of this process' live children.
func childPids(t *testing.T) []int {
	t.Helper()

	entries, err := os.ReadDir("/proc")
	require.NoError(t, err)

	var (
		pids []int
		me   = os.Getpid()
	)
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		stat, err := os.ReadFile(filepath.Join("/proc", e.Name(), "stat"))
		if err != nil {
			continue // the process is gone, it is not a leftover
		}
		// The comm field can contain spaces and parentheses, the fields
		// after the last ')' are the state and the parent pid.
		fields := strings.Fields(string(stat[strings.LastIndex(string(stat), ")")+1:]))
		if len(fields) < 2 {
			continue
		}
		if ppid, err := strconv.Atoi(fields[1]); err == nil && ppid == me {
			pids = append(pids, pid)
		}
	}

	return pids
}

// The runtime launches a pre-installed plugin before it can claim the
// name. If the registry refuses the claim, the runtime must stop the
// plugin it launched.
func TestRefusedPreinstalledPluginIsStopped(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the test needs /proc to find leftover plugin processes")
	}

	var (
		s  = &Suite{}
		rt = &mockRuntime{}
	)

	rt.options = append(rt.options,
		nri.WithBuiltinPlugins(
			&builtin.BuiltinPlugin{Index: "10", Base: "collide"},
		),
	)

	dir := s.Prepare(t, rt)

	// A pre-installed plugin with the indexed name of the builtin one. It
	// blocks reading its socket, so it stays alive until it is stopped.
	plugins := filepath.Join(dir, "opt", "nri", "plugins")
	require.NoError(t, os.MkdirAll(plugins, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(plugins, "10-collide"),
		[]byte("#!/bin/sh\nread line <&3\n"), 0o755))

	before := childPids(t)
	t.Cleanup(func() {
		for _, pid := range childPids(t) {
			if !slices.Contains(before, pid) {
				syscall.Kill(pid, syscall.SIGKILL)
			}
		}
	})

	require.NoError(t, rt.Start(dir),
		"startup with a refused pre-installed plugin")
	require.Equal(t, before, childPids(t),
		"plugin processes left behind by a refused registration")
}

// Every plugin is told its status in the runtime: a plugin which
// registers is told it is live, and one replaced by another instance is
// told it is retired.
func TestPluginsAreToldTheirStatus(t *testing.T) {
	var (
		s         = &Suite{}
		incumbent = &mockPlugin{idx: "10", name: "test"}
		newcomer  = &mockPlugin{idx: "10", name: "test"}
	)

	s.Prepare(t, &mockRuntime{}, incumbent)
	s.Startup()

	require.NoError(t, incumbent.Wait(PluginLive, time.After(startupTimeout)),
		"a plugin which registered should be told it is live")

	s.StartPlugins(newcomer)
	s.WaitForPluginsToSync(newcomer)

	require.NoError(t, newcomer.Wait(PluginLive, time.After(startupTimeout)),
		"a plugin which took over a name should be told it is live")
	require.NoError(t, incumbent.Wait(PluginRetired, time.After(startupTimeout)),
		"a replaced plugin should be told it is retired")

	require.True(t, newcomer.stub.IsLive(), "the live plugin")
	require.False(t, incumbent.stub.IsLive(), "the retired plugin")
}

// A plugin which the runtime drops stops reporting itself live, without
// any status update telling it so.
func TestDroppedPluginIsNotLive(t *testing.T) {
	var (
		s    = &Suite{}
		slow = &mockPlugin{
			idx:  "10",
			name: "test",
			mask: api.MustParseEventMask("RunPodSandbox"),
		}
		ctx = context.Background()
		pod = &api.PodSandbox{Id: "pod0", Name: "pod0", Namespace: "default"}
	)

	b := newBlocker(t)
	slow.runPodSandbox = func(*mockPlugin, *api.PodSandbox, *api.Container) error {
		return b.handle()
	}

	s.Prepare(t, &mockRuntime{}, slow)
	s.Startup()

	require.NoError(t, slow.Wait(PluginLive, time.After(startupTimeout)))
	require.True(t, slow.stub.IsLive())

	// The plugin blocks past the request timeout, so the runtime drops it.
	require.NoError(t, s.runtime.RunPodSandbox(ctx, &api.RunPodSandboxRequest{Pod: pod}))
	require.NoError(t, slow.Wait(PluginDisconnected, time.After(startupTimeout)))

	require.False(t, slow.stub.IsLive(), "a plugin dropped by the runtime")
}

// A plugin stops reporting itself live when the runtime goes away.
func TestPluginIsNotLiveAfterRuntimeStops(t *testing.T) {
	var (
		s      = &Suite{}
		plugin = &mockPlugin{idx: "10", name: "test"}
	)

	s.Prepare(t, &mockRuntime{}, plugin)
	s.Startup()

	require.NoError(t, plugin.Wait(PluginLive, time.After(startupTimeout)))
	require.True(t, plugin.stub.IsLive())

	s.runtime.Stop()

	require.Eventually(t, func() bool { return !plugin.stub.IsLive() },
		startupTimeout, 20*time.Millisecond, "a plugin after the runtime stopped")
}

// The status a plugin reads tells the live instance, a retired one, and a
// runtime which reported nothing apart from each other.
func TestPluginReadsItsStatus(t *testing.T) {
	var (
		s         = &Suite{}
		incumbent = &mockPlugin{idx: "10", name: "test"}
		newcomer  = &mockPlugin{idx: "10", name: "test"}
	)

	s.Prepare(t, &mockRuntime{}, incumbent)
	s.Startup()
	require.NoError(t, incumbent.Wait(PluginLive, time.After(startupTimeout)))

	require.True(t, incumbent.stub.IsStarted(), "a registered plugin")
	require.Equal(t, api.PluginStatus_PLUGIN_STATUS_LIVE, incumbent.stub.Status(),
		"a registered plugin")

	s.StartPlugins(newcomer)
	s.WaitForPluginsToSync(newcomer)
	require.NoError(t, incumbent.Wait(PluginRetired, time.After(startupTimeout)))

	require.True(t, incumbent.stub.IsStarted(), "a retired plugin is still connected")
	require.Equal(t, api.PluginStatus_PLUGIN_STATUS_RETIRED, incumbent.stub.Status(),
		"a retired plugin")
	require.False(t, incumbent.stub.IsLive(), "a retired plugin")

	s.runtime.Stop()

	require.Eventually(t, func() bool { return !newcomer.stub.IsStarted() },
		startupTimeout, 20*time.Millisecond, "a plugin after the runtime stopped")
	require.Equal(t, api.PluginStatus_PLUGIN_STATUS_UNKNOWN, newcomer.stub.Status(),
		"a plugin which is not connected")
}

// A plugin which is still registering when the runtime stops must not
// become the live instance afterwards.
func TestRegisteringPluginIsStoppedWithTheRuntime(t *testing.T) {
	var (
		s       = &Suite{}
		pending = &mockPlugin{idx: "10", name: "test"}
	)

	s.Prepare(t, &mockRuntime{})
	s.Startup()

	// Hold plugin synchronization. The plugin registers and gets
	// configured, then waits for the block to be lifted.
	block := s.runtime.runtime.BlockPluginSync()

	s.StartPlugins(pending)

	s.runtime.Stop()
	block.Unblock()

	require.NoError(t, pending.Wait(PluginDisconnected, time.After(startupTimeout)),
		"a plugin still registering when the runtime stopped")
	require.False(t, pending.EventQ().Has(PluginLive),
		"such a plugin must not be told that it is live")
}
