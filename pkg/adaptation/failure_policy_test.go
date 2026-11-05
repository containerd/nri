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

// These tests run the failure policy through the stub, ttrpc, and adaptation:
//
//   - the stub sends the policy and the adaptation applies it,
//   - request timeouts and lost connections produce the expected errors,
//   - the adaptation skips remaining plugins when a request fails,
//   - a plugin kept connected receives subsequent requests,
//   - a deadline set by the runtime does not count as a plugin failure, and
//   - the adaptation skips closed plugins.

// requestTimeout is the plugin request timeout for these tests. A plugin
// that must exceed it blocks until released.
const requestTimeout = 100 * time.Millisecond

// lifecycleRequests sends each pod and container request to the adaptation.
// The event is the name that the mock plugin records for the request.
var lifecycleRequests = []struct {
	event string
	send  func(context.Context, *nri.Adaptation, *api.PodSandbox, *api.Container) error
}{
	{RunPodSandbox, func(ctx context.Context, r *nri.Adaptation, pod *api.PodSandbox, _ *api.Container) error {
		return r.RunPodSandbox(ctx, &api.RunPodSandboxRequest{Pod: pod})
	}},
	{UpdatePodSandbox, func(ctx context.Context, r *nri.Adaptation, pod *api.PodSandbox, _ *api.Container) error {
		_, err := r.UpdatePodSandbox(ctx, &api.UpdatePodSandboxRequest{
			Pod:                    pod,
			OverheadLinuxResources: &api.LinuxResources{},
			LinuxResources:         &api.LinuxResources{},
		})
		return err
	}},
	{PostUpdatePodSandbox, func(ctx context.Context, r *nri.Adaptation, pod *api.PodSandbox, _ *api.Container) error {
		return r.PostUpdatePodSandbox(ctx, &api.PostUpdatePodSandboxRequest{Pod: pod})
	}},
	{StopPodSandbox, func(ctx context.Context, r *nri.Adaptation, pod *api.PodSandbox, _ *api.Container) error {
		return r.StopPodSandbox(ctx, &api.StopPodSandboxRequest{Pod: pod})
	}},
	{RemovePodSandbox, func(ctx context.Context, r *nri.Adaptation, pod *api.PodSandbox, _ *api.Container) error {
		return r.RemovePodSandbox(ctx, &api.RemovePodSandboxRequest{Pod: pod})
	}},
	{CreateContainer, func(ctx context.Context, r *nri.Adaptation, pod *api.PodSandbox, ctr *api.Container) error {
		_, err := r.CreateContainer(ctx, &api.CreateContainerRequest{Pod: pod, Container: ctr})
		return err
	}},
	{PostCreateContainer, func(ctx context.Context, r *nri.Adaptation, pod *api.PodSandbox, ctr *api.Container) error {
		return r.PostCreateContainer(ctx, &api.PostCreateContainerRequest{Pod: pod, Container: ctr})
	}},
	{StartContainer, func(ctx context.Context, r *nri.Adaptation, pod *api.PodSandbox, ctr *api.Container) error {
		return r.StartContainer(ctx, &api.StartContainerRequest{Pod: pod, Container: ctr})
	}},
	{PostStartContainer, func(ctx context.Context, r *nri.Adaptation, pod *api.PodSandbox, ctr *api.Container) error {
		return r.PostStartContainer(ctx, &api.PostStartContainerRequest{Pod: pod, Container: ctr})
	}},
	{UpdateContainer, func(ctx context.Context, r *nri.Adaptation, pod *api.PodSandbox, ctr *api.Container) error {
		_, err := r.UpdateContainer(ctx, &api.UpdateContainerRequest{
			Pod:            pod,
			Container:      ctr,
			LinuxResources: &api.LinuxResources{},
		})
		return err
	}},
	{PostUpdateContainer, func(ctx context.Context, r *nri.Adaptation, pod *api.PodSandbox, ctr *api.Container) error {
		return r.PostUpdateContainer(ctx, &api.PostUpdateContainerRequest{Pod: pod, Container: ctr})
	}},
	{StopContainer, func(ctx context.Context, r *nri.Adaptation, pod *api.PodSandbox, ctr *api.Container) error {
		_, err := r.StopContainer(ctx, &api.StopContainerRequest{Pod: pod, Container: ctr})
		return err
	}},
	{RemoveContainer, func(ctx context.Context, r *nri.Adaptation, pod *api.PodSandbox, ctr *api.Container) error {
		return r.RemoveContainer(ctx, &api.RemoveContainerRequest{Pod: pod, Container: ctr})
	}},
}

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

	// For a plugin without a policy, a timeout drops the plugin and the
	// request proceeds. The runtime does not see the timeout (#316).
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
		// Retain the connection so the handler can close it, simulating
		// a plugin dying during a request.
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

	// The failure policy only applies to connected plugins. A plugin that
	// disconnects while idle stays registered until the next request
	// removes it. That request must not relay to it: with fail_request,
	// the request would fail with ttrpc.ErrClosed for a plugin that is
	// already gone. The request proceeds with the remaining plugins.
	//
	// Each request type checks the connection itself, and the first request
	// after the disconnect removes the plugin. Each request type is tested
	// as the first request after the disconnect.
	t.Run("fail_request: a plugin that disconnects while idle does not fail the next request", func(t *testing.T) {
		for _, tc := range lifecycleRequests {
			t.Run(tc.event, func(t *testing.T) {
				gone := newPlugin("00", "gone", keepAndFail)
				next := newPlugin("10", "next", nil)
				gone.mask = api.MustParseEventMask(tc.event)
				next.mask = api.MustParseEventMask(tc.event)
				setup(t, gone, next)
				s.Startup()
				rt := s.runtime.runtime

				before := &Event{Type: EventType(tc.event), Pod: pod0, Ctr: ctr0}
				require.NoError(t, tc.send(ctx, rt, pod0, ctr0))
				require.NoError(t, gone.Wait(before, time.After(startupTimeout)))
				require.NoError(t, next.Wait(before, time.After(startupTimeout)))

				// The plugin disconnects between two requests. Wait until
				// the adaptation has noticed the closed connection.
				closed := rt.PluginClosed(gone.FullName())
				gone.Stop()
				require.Eventually(t, closed, startupTimeout, 5*time.Millisecond)

				after := &Event{Type: EventType(tc.event), Pod: pod1, Ctr: ctr1}
				require.NoError(t, tc.send(ctx, rt, pod1, ctr1))
				require.NoError(t, next.Wait(after, time.After(startupTimeout)))
			})
		}
	})

	// Validators are called from their own list, so they check the
	// connection separately. A validator that disconnects while idle does
	// not fail the next CreateContainer. Before this check, exactly one
	// request failed with ttrpc.ErrClosed and the same request removed the
	// validator; the requests after it proceeded without it. No plugin can
	// use that as a gate. A validator that must be present is declared in
	// the required plugins of the default validator (see scenario E).
	t.Run("a validator that disconnects while idle does not fail the next request", func(t *testing.T) {
		validator := &mockPlugin{
			idx:  "00",
			name: "validator",
			mask: api.MustParseEventMask("ValidateContainerAdjustment"),
		}
		validator.validateAdjustment = func(*mockPlugin, *api.ValidateContainerAdjustmentRequest) error {
			return nil
		}
		setup(t, validator)
		s.Startup()
		rt := s.runtime.runtime

		require.NoError(t, runPod(ctx, pod0))
		_, err := s.runtime.CreateContainer(ctx, &api.CreateContainerRequest{Pod: pod0, Container: ctr0})
		require.NoError(t, err)
		require.NoError(t, validator.Wait(ContainerEvent(ctr0, ValidateContainerAdjustment), time.After(startupTimeout)))

		closed := rt.PluginClosed(validator.FullName())
		validator.Stop()
		require.Eventually(t, closed, startupTimeout, 5*time.Millisecond)

		_, err = s.runtime.CreateContainer(ctx, &api.CreateContainerRequest{Pod: pod0, Container: ctr1})
		require.NoError(t, err)
	})

	// NRI checks the connection of each plugin when its turn comes. A
	// plugin that disconnects during a request, while an earlier plugin
	// handles it, is skipped like a plugin that disconnected before the
	// request. The request proceeds without it.
	t.Run("fail_request: a plugin that disconnects before its turn is skipped", func(t *testing.T) {
		slow := newPlugin("00", "slow", nil)
		gone := newPlugin("10", "gone", keepAndFail)
		b := blockPlugin(t, slow)
		// The default request timeout leaves the blocked plugin enough time.
		s.Prepare(t, &mockRuntime{}, slow, gone)
		s.Startup()
		rt := s.runtime.runtime

		// Look up the plugin before the request: the lookup takes the lock
		// that the request holds.
		closed := rt.PluginClosed(gone.FullName())
		errC := make(chan error, 1)
		go func() { errC <- runPod(ctx, pod0) }()
		b.WaitArrived(t)

		gone.Stop()
		require.Eventually(t, closed, startupTimeout, 5*time.Millisecond)
		b.Release()

		select {
		case err := <-errC:
			require.NoError(t, err)
		case <-time.After(startupTimeout):
			t.Fatal("timed out waiting for the request to return")
		}
		require.False(t, gone.EventQ().Has(PodSandboxEvent(pod0, RunPodSandbox)))
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

	// Scenario B: a plugin that never answers. NRI does not perform health
	// checks or count failures.
	//
	// Without a policy, each request drops the plugin. When the plugin
	// reconnects while still hung, the next request times out and drops
	// it again. Each request waits the full timeout.
	//
	// With keep_connected, each request waits the full timeout and the
	// plugin stays registered. With fail_request, each request also fails.
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
			// registers and remains hung.
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
	// 00 does not have a policy and times out in CreateContainer.
	//
	// NRI drops plugin 00 and calls plugin 10 with a container that lacks
	// the adjustments of plugin 00. Nothing in the request tells plugin 10
	// that plugin 00 did not run. The container is created without the
	// adjustments.
	t.Run("C: a plugin runs after a dropped plugin without knowing it", func(t *testing.T) {
		var (
			b       = newBlocker(t)
			seenEnv []string
			slow    = newPlugin("00", "slow", nil)
			next    = newPlugin("10", "next", nil)
		)
		// Plugin 00 blocks in CreateContainer before returning an adjustment.
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

	// Scenario D: the required plugins check of the default validator
	// guards plugins that must be present.
	//
	// The check uses the list of plugins consulted for the container. NRI
	// adds a plugin to the list before relaying the request. A plugin that
	// times out and is dropped remains in the list. The check does not
	// catch it. The container is created without the required plugin.
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

	// Scenario E: the required plugins check catches a plugin that
	// disconnected while idle. NRI skips a closed plugin before it reports
	// the plugin to the validators as consulted. The container is rejected
	// instead of being created without the required plugin.
	t.Run("E: the required plugins check catches a plugin that disconnected while idle", func(t *testing.T) {
		gone := newPlugin("00", "gone", nil)
		setup(t, &mockRuntime{
			options: []nri.Option{
				nri.WithDefaultValidator(&validator.DefaultValidatorConfig{
					Enable:          true,
					RequiredPlugins: []string{"gone"},
				}),
			},
		}, gone)
		s.Startup()

		require.NoError(t, runPod(ctx, pod0))

		closed := s.runtime.runtime.PluginClosed(gone.FullName())
		gone.Stop()
		require.Eventually(t, closed, startupTimeout, 5*time.Millisecond)

		_, err := s.runtime.CreateContainer(ctx, &api.CreateContainerRequest{Pod: pod0, Container: ctr0})
		require.ErrorContains(t, err, "required plugin")
	})
}
