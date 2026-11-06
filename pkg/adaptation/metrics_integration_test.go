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
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	nri "github.com/containerd/nri/pkg/adaptation"
	"github.com/containerd/nri/pkg/api"
)

// integrationMetrics is a thread-safe Metrics implementation for integration tests.
// It records every call made to it so tests can assert on what the adaptation layer
// reported after real plugin invocations over the socket.
type integrationMetrics struct {
	mu          sync.Mutex
	pluginCount int
	invocations []metricsInvocation
	latencies   []metricsLatency
	adjustments []metricsAdjustment
}

type metricsInvocation struct {
	pluginName string
	operation  string
	err        error
}

type metricsLatency struct {
	pluginName string
	operation  string
	latency    time.Duration
}

type metricsAdjustment struct {
	pluginName string
	operation  string
	updates    int
	evicts     int
}

func (m *integrationMetrics) RecordPluginInvocation(pluginName, operation string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.invocations = append(m.invocations, metricsInvocation{
		pluginName: pluginName,
		operation:  operation,
		err:        err,
	})
}

func (m *integrationMetrics) RecordPluginLatency(pluginName, operation string, latency time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.latencies = append(m.latencies, metricsLatency{
		pluginName: pluginName,
		operation:  operation,
		latency:    latency,
	})
}

func (m *integrationMetrics) RecordPluginAdjustments(pluginName, operation string, _ *nri.ContainerAdjustment, updates, evicts int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.adjustments = append(m.adjustments, metricsAdjustment{
		pluginName: pluginName,
		operation:  operation,
		updates:    updates,
		evicts:     evicts,
	})
}

func (m *integrationMetrics) UpdatePluginCount(count int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pluginCount = count
}

// snapshot returns a consistent copy of all recorded data. The lock is held
// while copying, so callers can inspect the result without holding the lock.
func (m *integrationMetrics) snapshot() (int, []metricsInvocation, []metricsLatency, []metricsAdjustment) {
	m.mu.Lock()
	defer m.mu.Unlock()
	invs := make([]metricsInvocation, len(m.invocations))
	copy(invs, m.invocations)
	lats := make([]metricsLatency, len(m.latencies))
	copy(lats, m.latencies)
	adjs := make([]metricsAdjustment, len(m.adjustments))
	copy(adjs, m.adjustments)
	return m.pluginCount, invs, lats, adjs
}

// operationsFrom returns the operation names from a slice of invocations.
func operationsFrom(invs []metricsInvocation) []string {
	ops := make([]string, len(invs))
	for i, inv := range invs {
		ops[i] = inv.operation
	}
	return ops
}

// TestMetricsIntegration verifies that the Metrics interface is called correctly
// when a real plugin connects over the socket and the runtime drives a full
// pod-and-container lifecycle.
func TestMetricsIntegration(t *testing.T) {
	s := &Suite{}
	m := &integrationMetrics{}

	plugin := &mockPlugin{
		idx:  "00",
		name: "metrics-test",
		mask: api.MustParseEventMask(
			"RunPodSandbox", "UpdatePodSandbox", "PostUpdatePodSandbox",
			"StopPodSandbox", "RemovePodSandbox",
			"CreateContainer", "PostCreateContainer",
			"StartContainer", "PostStartContainer",
			"UpdateContainer", "PostUpdateContainer",
			"StopContainer", "RemoveContainer",
		),
	}

	s.Prepare(t,
		&mockRuntime{options: []nri.Option{nri.WithMetrics(m)}},
		plugin,
	)
	s.Startup()

	var (
		ctx = context.Background()
		pod = &api.PodSandbox{
			Id:        "pod0",
			Name:      "pod0",
			Uid:       "uid0",
			Namespace: "default",
		}
		ctr = &api.Container{
			Id:           "ctr0",
			PodSandboxId: "pod0",
			Name:         "ctr0",
			State:        api.ContainerState_CONTAINER_CREATED,
		}
	)

	require.NoError(t, s.runtime.startStopPodAndContainer(ctx, pod, ctr))

	pluginCount, invocations, latencies, adjustments := m.snapshot()

	// The plugin connected and was registered, so the count should be 1.
	assert.Equal(t, 1, pluginCount, "plugin count should be 1 after one plugin connects")

	// Every invocation must name the plugin and have a positive latency.
	pluginName := plugin.FullName()
	for _, inv := range invocations {
		assert.Equal(t, pluginName, inv.pluginName, "invocation should name the connected plugin")
		assert.NoError(t, inv.err, "all operations should succeed")
	}
	for _, lat := range latencies {
		assert.Equal(t, pluginName, lat.pluginName, "latency should name the connected plugin")
		assert.Positive(t, lat.latency, "latency should be positive")
	}

	// Every invocation must also have a corresponding latency record.
	assert.Equal(t, len(invocations), len(latencies),
		"every invocation should have a matching latency record")

	// The full lifecycle drives 13 events plus the initial Synchronize that fires
	// when the plugin connects. Build the expected operation list in the same order
	// they are issued.
	expectedOps := []string{
		"Synchronize",
		api.Event_RUN_POD_SANDBOX.PrettyName(),
		api.Event_UPDATE_POD_SANDBOX.PrettyName(),
		api.Event_POST_UPDATE_POD_SANDBOX.PrettyName(),
		api.Event_CREATE_CONTAINER.PrettyName(),
		api.Event_POST_CREATE_CONTAINER.PrettyName(),
		api.Event_START_CONTAINER.PrettyName(),
		api.Event_POST_START_CONTAINER.PrettyName(),
		api.Event_UPDATE_CONTAINER.PrettyName(),
		api.Event_POST_UPDATE_CONTAINER.PrettyName(),
		api.Event_STOP_CONTAINER.PrettyName(),
		api.Event_REMOVE_CONTAINER.PrettyName(),
		api.Event_STOP_POD_SANDBOX.PrettyName(),
		api.Event_REMOVE_POD_SANDBOX.PrettyName(),
	}
	assert.Equal(t, expectedOps, operationsFrom(invocations),
		"operations should be recorded in lifecycle order")

	// Operations that return adjustments: Synchronize (at startup), CreateContainer.
	// Synchronize is fired when the plugin connects; CreateContainer returns an
	// empty adjustment with no updates/evicts by default.
	for _, adj := range adjustments {
		assert.Equal(t, pluginName, adj.pluginName)
	}
	// Ensure at least CreateContainer produced an adjustment record.
	foundCreateContainer := false
	for _, adj := range adjustments {
		if adj.operation == api.Event_CREATE_CONTAINER.PrettyName() {
			foundCreateContainer = true
		}
	}
	assert.True(t, foundCreateContainer, "CreateContainer should produce an adjustment record")
}
