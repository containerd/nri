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
	"strconv"
	"strings"
	"testing"

	"github.com/containerd/ttrpc"

	"github.com/containerd/nri/pkg/adaptation/builtin"
	"github.com/containerd/nri/pkg/api"
)

// syncTestPlugin returns a builtin plugin that synchronizes with fn.
func syncTestPlugin(fn func(context.Context, *api.SynchronizeRequest) (*api.SynchronizeResponse, error)) *plugin {
	return &plugin{
		idx:  "00",
		base: "sync-test",
		r:    &Adaptation{metrics: &noopMetrics{}},
		impl: &pluginType{
			builtinImpl: &builtin.BuiltinPlugin{
				Base:  "sync-test",
				Index: "00",
				Handlers: builtin.BuiltinHandlers{
					Synchronize: fn,
				},
			},
		},
	}
}

// syncTestObjects returns pods and containers for a sync test.
func syncTestObjects(pods, containers int) ([]*PodSandbox, []*Container) {
	ps := make([]*PodSandbox, pods)
	for i := range ps {
		ps[i] = &PodSandbox{Id: "pod-" + strconv.Itoa(i)}
	}
	cs := make([]*Container, containers)
	for i := range cs {
		cs[i] = &Container{Id: "ctr-" + strconv.Itoa(i)}
	}
	return ps, cs
}

// oversizedMessageError is the error a ttrpc connection returns for a
// message that exceeds the maximum message size.
func oversizedMessageError() error {
	return ttrpc.OversizedMessageError(12 << 20)
}

// TestSynchronizeOversizedSyncMsg verifies that synchronizing with a
// plugin using messages that don't fit the maximum message size fails
// cleanly when the state cannot be split any further. In particular; it
// must not panic if the recalculated minimum number of objects per
// message exceeds the number of objects left.
func TestSynchronizeOversizedSyncMsg(t *testing.T) {
	// A plugin that refuses every message as oversized. The first
	// message (1 pod, 20 containers) triggers a recalculation that
	// bumps the pod count up to 4 although only a single pod is left.
	p := syncTestPlugin(func(context.Context, *api.SynchronizeRequest) (*api.SynchronizeResponse, error) {
		return nil, oversizedMessageError()
	})

	pods, ctrs := syncTestObjects(1, 20)

	updates, err := p.synchronize(context.Background(), pods, ctrs)
	if err == nil {
		t.Fatalf("expected synchronization to fail, got updates %v", updates)
	}
	if !strings.Contains(err.Error(), "split messages") {
		t.Errorf("expected a split message failure, got %v", err)
	}
}

// TestSynchronizeOversizedSplitMsgs verifies that synchronization
// recovers from an oversized message by splitting the remaining state
// into smaller chunks.
func TestSynchronizeOversizedSplitMsgs(t *testing.T) {
	const maxObjsPerMsg = 10

	var requests []*api.SynchronizeRequest
	p := syncTestPlugin(func(_ context.Context, req *api.SynchronizeRequest) (*api.SynchronizeResponse, error) {
		requests = append(requests, req)
		if len(req.Pods)+len(req.Containers) > maxObjsPerMsg {
			return nil, oversizedMessageError()
		}
		return &api.SynchronizeResponse{More: req.More}, nil
	})

	pods, ctrs := syncTestObjects(1, 20)

	updates, err := p.synchronize(context.Background(), pods, ctrs)
	if err != nil {
		t.Fatalf("expected synchronization to succeed, got %v", err)
	}
	if len(updates) != 0 {
		t.Fatalf("expected no updates, got %v", updates)
	}

	if len(requests) < 2 {
		t.Fatalf("expected the state to be split into several messages, got %d", len(requests))
	}
	if first := requests[0]; len(first.Pods)+len(first.Containers) != len(pods)+len(ctrs) {
		t.Errorf("expected the first message to carry all %d objects, got %d",
			len(pods)+len(ctrs), len(first.Pods)+len(first.Containers))
	}
	for i, req := range requests[1:] {
		if got := len(req.Pods) + len(req.Containers); got > maxObjsPerMsg {
			t.Errorf("split message #%d carries %d objects, max is %d", i+2, got, maxObjsPerMsg)
		}
	}
}
