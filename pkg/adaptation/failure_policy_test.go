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
	release chan struct{}
	done    chan struct{}
	once    sync.Once
}

func newBlocker(t *testing.T) *blocker {
	t.Helper()
	b := &blocker{
		release: make(chan struct{}),
		done:    make(chan struct{}),
	}
	// Do not leave a plugin blocked after the test.
	t.Cleanup(b.Release)
	return b
}

// handle blocks until released, then marks the blocked request as done.
func (b *blocker) handle() error {
	<-b.release
	b.once.Do(func() { close(b.done) })
	return nil
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
