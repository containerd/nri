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
	stdnet "net"
	"sync"
	"testing"
	"time"

	"github.com/containerd/ttrpc"
	"github.com/stretchr/testify/require"

	nri "github.com/containerd/nri/pkg/adaptation"
	"github.com/containerd/nri/pkg/api"
	"github.com/containerd/nri/pkg/stub"
	validator "github.com/containerd/nri/plugins/default-validator/builtin"
)

// These tests run the failure policy through the full stack: stub, ttrpc
// and adaptation. They check the facts that the unit tests in plugin_test.go
// cannot check:
//
//   - the stub sends the policy and the adaptation applies it,
//   - a real request timeout and a real lost connection produce the errors
//     that the adaptation expects,
//   - the adaptation does not call the remaining plugins for a failed
//     request,
//   - a plugin that is kept connected receives the next request,
//   - a deadline set by the runtime does not count as a plugin failure.
//
// The unit tests cover the full matrix of errors and policies.

// requestTimeout is the plugin request timeout for these tests. It is short
// to keep the tests fast. A plugin that must exceed it blocks until released.
const requestTimeout = 100 * time.Millisecond

// blocker is a request handler that blocks until released.
type blocker struct {
	arrived chan struct{}
	release chan struct{}
	done    chan struct{}
	first   sync.Once
	once    sync.Once
}

func newBlocker(t *testing.T) *blocker {
	t.Helper()
	b := &blocker{
		arrived: make(chan struct{}),
		release: make(chan struct{}),
		done:    make(chan struct{}),
	}
	// Do not leave a plugin blocked after the test.
	t.Cleanup(b.Release)
	return b
}

// handle blocks until released, then marks the blocked request as done.
func (b *blocker) handle() error {
	b.first.Do(func() { close(b.arrived) })
	<-b.release
	b.once.Do(func() { close(b.done) })
	return nil
}

// WaitArrived waits until the first request has reached the handler.
func (b *blocker) WaitArrived(t *testing.T) {
	t.Helper()
	select {
	case <-b.arrived:
	case <-time.After(startupTimeout):
		t.Fatal("timed out waiting for a request to reach the blocked handler")
	}
}

// Release unblocks the handler.
func (b *blocker) Release() {
	select {
	case <-b.release:
	default:
		close(b.release)
	}
}

// WaitDone waits until the blocked handler has returned. After this, the
// test can send a new request to the plugin.
func (b *blocker) WaitDone(t *testing.T) {
	t.Helper()
	select {
	case <-b.done:
	case <-time.After(startupTimeout):
		t.Fatal("timed out waiting for the blocked request handler to return")
	}
}

func TestPluginFailurePolicy(t *testing.T) {
	s := &Suite{}

	setup := func(t *testing.T, plugins ...*mockPlugin) {
		t.Helper()
		nri.SetPluginRequestTimeout(requestTimeout)
		t.Cleanup(func() {
			nri.SetPluginRequestTimeout(nri.DefaultPluginRequestTimeout)
		})
		s.Prepare(t, &mockRuntime{}, plugins...)
	}

	var (
		ctx  = context.Background()
		pod0 = &api.PodSandbox{Id: "pod0", Name: "pod0", Uid: "uid0", Namespace: "default"}
		pod1 = &api.PodSandbox{Id: "pod1", Name: "pod1", Uid: "uid1", Namespace: "default"}
		ctr0 = &api.Container{Id: "ctr0", PodSandboxId: "pod0", Name: "ctr0", State: api.ContainerState_CONTAINER_CREATED}
		ctr1 = &api.Container{Id: "ctr1", PodSandboxId: "pod0", Name: "ctr1", State: api.ContainerState_CONTAINER_CREATED}

		keepAndFail = &api.FailurePolicy{FailRequest: true, KeepConnected: true}
		dropAndFail = &api.FailurePolicy{FailRequest: true}

		runPod = func(ctx context.Context, pod *api.PodSandbox) error {
			return s.runtime.RunPodSandbox(ctx, &api.RunPodSandboxRequest{Pod: pod})
		}

		// newPlugin returns a plugin subscribed to RunPodSandbox only. A nil
		// policy leaves the plugin without a policy.
		newPlugin = func(idx, name string, policy *api.FailurePolicy) *mockPlugin {
			p := &mockPlugin{
				idx:  idx,
				name: name,
				mask: api.MustParseEventMask("RunPodSandbox"),
			}
			if policy != nil {
				p.options = []stub.Option{stub.WithFailurePolicy(policy)}
			}
			return p
		}

		// blockPlugin makes the plugin block in RunPodSandbox until released.
		blockPlugin = func(t *testing.T, p *mockPlugin) *blocker {
			b := newBlocker(t)
			p.runPodSandbox = func(*mockPlugin, *api.PodSandbox, *api.Container) error {
				return b.handle()
			}
			return b
		}
	)

	// A plugin without a policy keeps the old behavior. On a timeout, the
	// adaptation drops the plugin and the request proceeds. The runtime
	// does not see the timeout (#316).
	t.Run("no policy: a timed out plugin is dropped and the request proceeds", func(t *testing.T) {
		slow := newPlugin("00", "slow", nil)
		next := newPlugin("10", "next", nil)
		blockPlugin(t, slow)
		setup(t, slow, next)
		s.Startup()

		require.NoError(t, runPod(ctx, pod0))
		require.NoError(t, slow.Wait(PluginDisconnected, time.After(startupTimeout)))
		require.NoError(t, next.Wait(PodSandboxEvent(pod0, RunPodSandbox), time.After(startupTimeout)))
	})

	// With both fields set, a timeout fails the request and the plugin
	// stays. The error names the plugin and the request. The remaining
	// plugins do not receive the request. The plugin receives the next
	// request.
	t.Run("fail_request+keep_connected: a timed out plugin fails the request and stays", func(t *testing.T) {
		slow := newPlugin("00", "slow", keepAndFail)
		next := newPlugin("10", "next", nil)
		b := blockPlugin(t, slow)
		setup(t, slow, next)
		s.Startup()

		err := runPod(ctx, pod0)
		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.ErrorContains(t, err, slow.FullName())
		require.ErrorContains(t, err, "RunPodSandbox")
		require.False(t, next.EventQ().Has(PodSandboxEvent(pod0, RunPodSandbox)))

		b.Release()
		b.WaitDone(t)
		require.NoError(t, runPod(ctx, pod1))
		require.NoError(t, slow.Wait(PodSandboxEvent(pod1, RunPodSandbox), time.After(startupTimeout)))
		require.NoError(t, next.Wait(PodSandboxEvent(pod1, RunPodSandbox), time.After(startupTimeout)))
		require.False(t, slow.EventQ().Has(PluginDisconnected))
	})

	// A validator fails the request on any error. keep_connected still
	// decides if a timed out validator is dropped.
	t.Run("keep_connected: a timed out validator fails the request and stays", func(t *testing.T) {
		b := newBlocker(t)
		validator := &mockPlugin{
			idx:     "00",
			name:    "validator",
			options: []stub.Option{stub.WithFailurePolicy(&api.FailurePolicy{KeepConnected: true})},
		}
		validator.validateAdjustment = func(*mockPlugin, *api.ValidateContainerAdjustmentRequest) error {
			return b.handle()
		}
		setup(t, validator)
		s.Startup()

		require.NoError(t, runPod(ctx, pod0))

		_, err := s.runtime.CreateContainer(ctx, &api.CreateContainerRequest{Pod: pod0, Container: ctr0})
		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.ErrorContains(t, err, validator.FullName())

		b.Release()
		b.WaitDone(t)
		_, err = s.runtime.CreateContainer(ctx, &api.CreateContainerRequest{Pod: pod0, Container: ctr1})
		require.NoError(t, err)
		require.NoError(t, validator.Wait(ContainerEvent(ctr1, ValidateContainerAdjustment), time.After(startupTimeout)))
		require.False(t, validator.EventQ().Has(PluginDisconnected))
	})

	// A lost connection always drops the plugin. fail_request decides if the
	// request fails.
	t.Run("fail_request: a plugin that loses its connection fails the request and is dropped", func(t *testing.T) {
		var conn stdnet.Conn
		crash := newPlugin("00", "crash", dropAndFail)
		next := newPlugin("10", "next", nil)
		// Keep the plugin connection so that the handler can close it. This
		// emulates a plugin that dies during a request.
		crash.options = append(crash.options,
			stub.WithDialer(func(path string) (stdnet.Conn, error) {
				c, err := stdnet.Dial("unix", path)
				conn = c
				return c, err
			}),
		)
		crash.runPodSandbox = func(*mockPlugin, *api.PodSandbox, *api.Container) error {
			return conn.Close()
		}
		setup(t, crash, next)
		s.Startup()

		err := runPod(ctx, pod0)
		require.ErrorIs(t, err, ttrpc.ErrClosed)
		require.ErrorContains(t, err, crash.FullName())
		require.NoError(t, crash.Wait(PluginDisconnected, time.After(startupTimeout)))
		require.False(t, next.EventQ().Has(PodSandboxEvent(pod0, RunPodSandbox)))
	})

	// A deadline set by the runtime is not a plugin failure. The plugin is
	// not dropped and the runtime gets its own error back.
	t.Run("a runtime deadline is not a plugin failure", func(t *testing.T) {
		slow := newPlugin("00", "slow", keepAndFail)
		b := blockPlugin(t, slow)
		setup(t, slow)
		s.Startup()

		// The runtime deadline is shorter than the request timeout.
		rctx, cancel := context.WithTimeout(ctx, requestTimeout/4)
		defer cancel()
		err := runPod(rctx, pod0)
		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.NotContains(t, err.Error(), slow.FullName())

		b.Release()
		b.WaitDone(t)
		require.NoError(t, runPod(ctx, pod1))
		require.NoError(t, slow.Wait(PodSandboxEvent(pod1, RunPodSandbox), time.After(startupTimeout)))
		require.False(t, slow.EventQ().Has(PluginDisconnected))
	})

	t.Run("the stub rejects a nil policy", func(t *testing.T) {
		_, err := stub.New(&mockPlugin{},
			stub.WithPluginName("test"),
			stub.WithPluginIdx("00"),
			stub.WithFailurePolicy(nil),
		)
		require.Error(t, err)
	})
}

func TestPluginFailureScenarios(t *testing.T) {
	s := &Suite{}

	setup := func(t *testing.T, runtime *mockRuntime, plugins ...*mockPlugin) {
		t.Helper()
		nri.SetPluginRequestTimeout(requestTimeout)
		t.Cleanup(func() {
			nri.SetPluginRequestTimeout(nri.DefaultPluginRequestTimeout)
		})
		s.Prepare(t, runtime, plugins...)
	}

	var (
		ctx  = context.Background()
		pod0 = &api.PodSandbox{Id: "pod0", Name: "pod0", Uid: "uid0", Namespace: "default"}
		pod1 = &api.PodSandbox{Id: "pod1", Name: "pod1", Uid: "uid1", Namespace: "default"}
		pod2 = &api.PodSandbox{Id: "pod2", Name: "pod2", Uid: "uid2", Namespace: "default"}
		ctr0 = &api.Container{Id: "ctr0", PodSandboxId: "pod0", Name: "ctr0", State: api.ContainerState_CONTAINER_CREATED}

		keepAndFail = &api.FailurePolicy{FailRequest: true, KeepConnected: true}

		runPod = func(ctx context.Context, pod *api.PodSandbox) error {
			return s.runtime.RunPodSandbox(ctx, &api.RunPodSandboxRequest{Pod: pod})
		}

		newPlugin = func(idx, name string, policy *api.FailurePolicy) *mockPlugin {
			p := &mockPlugin{
				idx:  idx,
				name: name,
				mask: api.MustParseEventMask("RunPodSandbox", "CreateContainer"),
			}
			if policy != nil {
				p.options = []stub.Option{stub.WithFailurePolicy(policy)}
			}
			return p
		}

		blockPlugin = func(t *testing.T, p *mockPlugin) *blocker {
			b := newBlocker(t)
			p.runPodSandbox = func(*mockPlugin, *api.PodSandbox, *api.Container) error {
				return b.handle()
			}
			return b
		}
	)

	// Scenario A: the runtime sets a deadline on the operation. NRI calls
	// the plugins one after the other. Each plugin has its own request
	// timeout, but all of them share the runtime deadline. A slow plugin
	// consumes the budget of the plugins after it.
	t.Run("A: a slow plugin consumes the runtime deadline of the plugins after it", func(t *testing.T) {
		slow00 := newPlugin("00", "slow00", nil)
		slow10 := newPlugin("10", "slow10", nil)
		blockPlugin(t, slow00)
		b10 := blockPlugin(t, slow10)
		setup(t, &mockRuntime{}, slow00, slow10)
		s.Startup()

		// Cancel the runtime context once the request reaches plugin 10.
		// This emulates a runtime deadline that expires at that moment.
		rctx, cancel := context.WithCancel(ctx)
		defer cancel()
		go func() {
			b10.WaitArrived(t)
			cancel()
		}()

		start := time.Now()
		err := runPod(rctx, pod0)
		elapsed := time.Since(start)

		// The runtime gets its own error. No plugin is named.
		require.ErrorIs(t, err, context.Canceled)
		require.NotContains(t, err.Error(), slow10.FullName())
		// Plugin 10 received the request only after plugin 00 timed out.
		require.GreaterOrEqual(t, elapsed, requestTimeout)
		// Plugin 00 is dropped for its timeout. Plugin 10 is not.
		require.NoError(t, slow00.Wait(PluginDisconnected, time.After(startupTimeout)))
		require.False(t, slow10.EventQ().Has(PluginDisconnected))
	})

	// Scenario B: a plugin that never answers. There is no health check and
	// no counter of failures.
	//
	// Today, without a policy: each request drops the plugin. The plugin
	// reconnects, is still hung, and the next request times out and drops
	// it again. Each request waits the full timeout.
	//
	// With keep_connected: the same, without the reconnect. Each request
	// waits the full timeout and the plugin stays registered. With
	// fail_request too, each request fails.
	t.Run("B: a plugin that never answers delays each request by the full timeout", func(t *testing.T) {
		t.Run("no policy: dropped, reconnects, dropped again", func(t *testing.T) {
			hung := newPlugin("00", "hung", nil)
			blockPlugin(t, hung)
			setup(t, &mockRuntime{}, hung)
			s.Startup()

			start := time.Now()
			require.NoError(t, runPod(ctx, pod0))
			require.GreaterOrEqual(t, time.Since(start), requestTimeout)
			require.NoError(t, hung.Wait(PluginDisconnected, time.After(startupTimeout)))

			// The plugin restarts. A new instance of the same plugin
			// registers, and it is still hung.
			restarted := newPlugin("00", "hung", nil)
			blockPlugin(t, restarted)
			s.StartPlugins(restarted)
			s.WaitForPluginsToSync(restarted)

			start = time.Now()
			require.NoError(t, runPod(ctx, pod1))
			require.GreaterOrEqual(t, time.Since(start), requestTimeout)
			require.NoError(t, restarted.Wait(PluginDisconnected, time.After(startupTimeout)))
		})

		t.Run("fail_request+keep_connected: each request fails, the plugin stays", func(t *testing.T) {
			hung := newPlugin("00", "hung", keepAndFail)
			blockPlugin(t, hung)
			setup(t, &mockRuntime{}, hung)
			s.Startup()

			for _, pod := range []*api.PodSandbox{pod0, pod1, pod2} {
				start := time.Now()
				err := runPod(ctx, pod)
				require.ErrorIs(t, err, context.DeadlineExceeded)
				require.ErrorContains(t, err, hung.FullName())
				require.GreaterOrEqual(t, time.Since(start), requestTimeout)
			}
			require.False(t, hung.EventQ().Has(PluginDisconnected))
		})
	})

	// Scenario C: plugin 10 depends on the adjustments of plugin 00. Plugin
	// 00 has no policy and times out in CreateContainer.
	//
	// Today: NRI drops plugin 00 and calls plugin 10 with a container that
	// lacks the adjustments of plugin 00. Nothing in the request tells
	// plugin 10 that plugin 00 did not run. The container is created
	// without the adjustments.
	t.Run("C: a plugin runs after a dropped plugin without knowing it", func(t *testing.T) {
		var (
			b       = newBlocker(t)
			seenEnv []string
			slow    = newPlugin("00", "slow", nil)
			next    = newPlugin("10", "next", nil)
		)
		// Plugin 00 would add an environment variable, but it blocks.
		slow.createContainer = func(*mockPlugin, *api.PodSandbox, *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
			if err := b.handle(); err != nil {
				return nil, nil, err
			}
			a := &api.ContainerAdjustment{}
			a.AddEnv("FROM_SLOW", "1")
			return a, nil, nil
		}
		// Plugin 10 records the environment it sees.
		next.createContainer = func(_ *mockPlugin, _ *api.PodSandbox, ctr *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
			seenEnv = ctr.GetEnv()
			return nil, nil, nil
		}
		setup(t, &mockRuntime{}, slow, next)
		s.Startup()

		require.NoError(t, runPod(ctx, pod0))
		rpl, err := s.runtime.CreateContainer(ctx, &api.CreateContainerRequest{Pod: pod0, Container: ctr0})
		require.NoError(t, err)
		require.NoError(t, slow.Wait(PluginDisconnected, time.After(startupTimeout)))
		require.NoError(t, next.Wait(ContainerEvent(ctr0, CreateContainer), time.After(startupTimeout)))

		// Plugin 10 ran, and saw no sign of plugin 00.
		require.Empty(t, seenEnv)
		// The container has no adjustment from plugin 00.
		require.Empty(t, rpl.GetAdjust().GetEnv())
	})

	// Scenario D: the required plugins check of the default validator is
	// the existing guard for plugins that must be present.
	//
	// Today: the check uses the list of plugins consulted for the
	// container. NRI adds a plugin to the list before it relays the
	// request. A plugin that times out and is dropped is still in the
	// list. The check does not catch it, and the container is created
	// without the required plugin being present.
	t.Run("D: the required plugins check does not catch a dropped plugin", func(t *testing.T) {
		b := newBlocker(t)
		slow := newPlugin("00", "slow", nil)
		slow.createContainer = func(*mockPlugin, *api.PodSandbox, *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
			return nil, nil, b.handle()
		}
		setup(t, &mockRuntime{
			options: []nri.Option{
				nri.WithDefaultValidator(&validator.DefaultValidatorConfig{
					Enable:          true,
					RequiredPlugins: []string{"slow"},
				}),
			},
		}, slow)
		s.Startup()

		require.NoError(t, runPod(ctx, pod0))
		_, err := s.runtime.CreateContainer(ctx, &api.CreateContainerRequest{Pod: pod0, Container: ctr0})
		require.NoError(t, err)
		require.NoError(t, slow.Wait(PluginDisconnected, time.After(startupTimeout)))
	})
}
