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

package adaptation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/containerd/ttrpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/containerd/nri/pkg/adaptation/builtin"
)

// The four possible failure policies. The zero value is the old NRI
// behavior: drop the plugin and let the request proceed without it.
var (
	dropAndProceed = &FailurePolicy{}
	dropAndFail    = &FailurePolicy{FailRequest: true}
	keepAndProceed = &FailurePolicy{KeepConnected: true}
	keepAndFail    = &FailurePolicy{FailRequest: true, KeepConnected: true}

	allPolicies = []*FailurePolicy{nil, dropAndProceed, dropAndFail, keepAndProceed, keepAndFail}
)

func policyName(policy *FailurePolicy) string {
	if policy == nil {
		return "unset"
	}
	return fmt.Sprintf("fail_request=%v,keep_connected=%v",
		policy.GetFailRequest(), policy.GetKeepConnected())
}

// testPlugin returns a builtin plugin with the given failure policy. A
// builtin plugin has no connection, so close() is safe to call.
func testPlugin(policy *FailurePolicy) *plugin {
	return &plugin{
		idx:    "00",
		base:   "test",
		policy: policy,
		impl:   &pluginType{builtinImpl: &builtin.BuiltinPlugin{}},
	}
}

// expiredContext returns a context with an expired deadline. This is what
// the runtime context looks like when the runtime timed out, not NRI.
func expiredContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	t.Cleanup(cancel)
	<-ctx.Done()
	return ctx
}

var (
	errTimeout        = context.DeadlineExceeded
	errWrappedTimeout = fmt.Errorf("call failed: %w", context.DeadlineExceeded)
	errProtocol       = fmt.Errorf("unexpected message: %w", ttrpc.ErrProtocol)
	errPlugin         = errors.New("plugin rejected the request")
	// A plugin that returns context.DeadlineExceeded. ttrpc delivers it
	// as a status error, so it does not match the context error.
	errPluginTimeout = status.Error(codes.DeadlineExceeded, "too slow")
)

func TestDropsPlugin(t *testing.T) {
	for _, policy := range allPolicies {
		for _, tc := range []struct {
			name string
			err  error
			want bool
		}{
			// A lost connection always drops the plugin.
			{name: "closed connection", err: ttrpc.ErrClosed, want: true},
			{name: "closed server", err: ttrpc.ErrServerClosed, want: true},
			{name: "protocol error", err: errProtocol, want: true},
			// A timeout drops the plugin unless KeepConnected is set.
			{name: "timeout", err: errTimeout, want: !policy.GetKeepConnected()},
			{name: "wrapped timeout", err: errWrappedTimeout, want: !policy.GetKeepConnected()},
			// Anything else is not fatal.
			{name: "cancelled", err: context.Canceled, want: false},
			{name: "plugin error", err: errPlugin, want: false},
			{name: "plugin returned deadline exceeded", err: errPluginTimeout, want: false},
		} {
			t.Run(tc.name+", "+policyName(policy), func(t *testing.T) {
				if got := testPlugin(policy).dropsPlugin(tc.err); got != tc.want {
					t.Errorf("dropsPlugin(%v) with policy %s = %v, want %v",
						tc.err, policyName(policy), got, tc.want)
				}
			})
		}
	}
}

func TestHandleRequestError(t *testing.T) {
	var (
		ctx   = context.Background()
		event = Event_RUN_POD_SANDBOX
	)

	// requireMentions checks that err wraps cause and names the plugin and the request.
	requireMentions := func(t *testing.T, p *plugin, err, cause error) {
		t.Helper()
		if !errors.Is(err, cause) {
			t.Fatalf("expected an error wrapping %v, got %v", cause, err)
		}
		for _, want := range []string{p.name(), event.PrettyName()} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("expected error %q to mention %q", err, want)
			}
		}
	}

	// The full matrix of fatal errors and policies. FailRequest alone
	// decides if the request fails. KeepConnected decides if a timeout
	// drops the plugin. A lost connection or a protocol error always
	// drops the plugin.
	for _, tc := range []struct {
		name     string
		err      error
		policy   *FailurePolicy
		wantErr  bool
		wantKept bool
	}{
		{name: "lost connection, unset", err: ttrpc.ErrClosed, policy: nil},
		{name: "lost connection, drop+proceed", err: ttrpc.ErrClosed, policy: dropAndProceed},
		{name: "lost connection, drop+fail", err: ttrpc.ErrClosed, policy: dropAndFail, wantErr: true},
		{name: "lost connection, keep+proceed", err: ttrpc.ErrClosed, policy: keepAndProceed},
		{name: "lost connection, keep+fail", err: ttrpc.ErrClosed, policy: keepAndFail, wantErr: true},

		{name: "protocol error, unset", err: errProtocol, policy: nil},
		{name: "protocol error, drop+proceed", err: errProtocol, policy: dropAndProceed},
		{name: "protocol error, drop+fail", err: errProtocol, policy: dropAndFail, wantErr: true},
		{name: "protocol error, keep+proceed", err: errProtocol, policy: keepAndProceed},
		{name: "protocol error, keep+fail", err: errProtocol, policy: keepAndFail, wantErr: true},

		{name: "timeout, unset", err: errTimeout, policy: nil},
		{name: "timeout, drop+proceed", err: errTimeout, policy: dropAndProceed},
		{name: "timeout, drop+fail", err: errTimeout, policy: dropAndFail, wantErr: true},
		{name: "timeout, keep+proceed", err: errTimeout, policy: keepAndProceed, wantKept: true},
		{name: "timeout, keep+fail", err: errTimeout, policy: keepAndFail, wantErr: true, wantKept: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := testPlugin(tc.policy)
			err := p.handleRequestError(ctx, event, tc.err)
			if tc.wantErr {
				requireMentions(t, p, err, tc.err)
			} else if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if kept := !p.isClosed(); kept != tc.wantKept {
				t.Fatalf("expected plugin kept=%v, got kept=%v", tc.wantKept, kept)
			}
		})
	}

	// Errors that are not plugin failures are returned unchanged. They
	// never affect the plugin, whatever the policy. This covers errors
	// returned by the plugin, cancelled requests, and any error after the
	// runtime context is done.
	for _, policy := range allPolicies {
		for _, tc := range []struct {
			name string
			ctx  context.Context
			err  error
		}{
			{name: "plugin error", ctx: ctx, err: errPlugin},
			{name: "plugin returned deadline exceeded", ctx: ctx, err: errPluginTimeout},
			{name: "cancelled", ctx: ctx, err: context.Canceled},
			{name: "runtime timeout", ctx: expiredContext(t), err: errTimeout},
		} {
			t.Run(tc.name+", "+policyName(policy)+": propagated unchanged", func(t *testing.T) {
				p := testPlugin(policy)
				if got := p.handleRequestError(tc.ctx, event, tc.err); got != tc.err {
					t.Fatalf("expected %v unchanged, got %v", tc.err, got)
				}
				if p.isClosed() {
					t.Fatal("expected the plugin to stay open")
				}
			})
		}
	}
}
