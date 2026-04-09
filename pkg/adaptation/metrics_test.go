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
	"testing"
	"time"

	"github.com/containerd/nri/pkg/api"
	"github.com/stretchr/testify/assert"
)

// mockMetrics provides a simple implementation of the Metrics interface for testing.
type mockMetrics struct {
	pluginCount int
	invocations []mockInvocation
	latencies   []mockLatency
	adjustments []mockAdjustment
}

type mockInvocation struct {
	pluginName string
	operation  string
	err        error
}

type mockLatency struct {
	pluginName string
	operation  string
	latency    time.Duration
}

type mockAdjustment struct {
	pluginName string
	operation  string
	adjust     *ContainerAdjustment
	updates    int
	evicts     int
}

func (m *mockMetrics) RecordPluginInvocation(pluginName, operation string, err error) {
	m.invocations = append(m.invocations, mockInvocation{
		pluginName: pluginName,
		operation:  operation,
		err:        err,
	})
}

func (m *mockMetrics) RecordPluginLatency(pluginName, operation string, latency time.Duration) {
	m.latencies = append(m.latencies, mockLatency{
		pluginName: pluginName,
		operation:  operation,
		latency:    latency,
	})
}

func (m *mockMetrics) RecordPluginAdjustments(pluginName, operation string, adjust *ContainerAdjustment, updates, evicts int) {
	m.adjustments = append(m.adjustments, mockAdjustment{
		pluginName: pluginName,
		operation:  operation,
		adjust:     adjust,
		updates:    updates,
		evicts:     evicts,
	})
}

func (m *mockMetrics) UpdatePluginCount(count int) {
	m.pluginCount = count
}

type dummyPlugin struct {
	api.PluginService
}

func (d *dummyPlugin) Synchronize(_ context.Context, _ *api.SynchronizeRequest) (*api.SynchronizeResponse, error) {
	return &api.SynchronizeResponse{
		Update: []*api.ContainerUpdate{
			{ContainerId: "test-container"},
		},
	}, nil
}

func (d *dummyPlugin) RunPodSandbox(_ context.Context, _ *api.RunPodSandboxRequest) (*api.RunPodSandboxResponse, error) {
	return &api.RunPodSandboxResponse{}, nil
}

func (d *dummyPlugin) UpdatePodSandbox(_ context.Context, _ *api.UpdatePodSandboxRequest) (*api.UpdatePodSandboxResponse, error) {
	return &api.UpdatePodSandboxResponse{}, nil
}

func (d *dummyPlugin) PostUpdatePodSandbox(_ context.Context, _ *api.PostUpdatePodSandboxRequest) (*api.PostUpdatePodSandboxResponse, error) {
	return &api.PostUpdatePodSandboxResponse{}, nil
}

func (d *dummyPlugin) StopPodSandbox(_ context.Context, _ *api.StopPodSandboxRequest) (*api.StopPodSandboxResponse, error) {
	return &api.StopPodSandboxResponse{}, nil
}

func (d *dummyPlugin) RemovePodSandbox(_ context.Context, _ *api.RemovePodSandboxRequest) (*api.RemovePodSandboxResponse, error) {
	return &api.RemovePodSandboxResponse{}, nil
}

func (d *dummyPlugin) CreateContainer(_ context.Context, _ *api.CreateContainerRequest) (*api.CreateContainerResponse, error) {
	return &api.CreateContainerResponse{
		Adjust: &api.ContainerAdjustment{},
		Update: []*api.ContainerUpdate{
			{ContainerId: "test-container"},
		},
		Evict: []*api.ContainerEviction{
			{ContainerId: "test-container"},
		},
	}, nil
}

func (d *dummyPlugin) PostCreateContainer(_ context.Context, _ *api.PostCreateContainerRequest) (*api.PostCreateContainerResponse, error) {
	return &api.PostCreateContainerResponse{}, nil
}

func (d *dummyPlugin) StartContainer(_ context.Context, _ *api.StartContainerRequest) (*api.StartContainerResponse, error) {
	return &api.StartContainerResponse{}, nil
}

func (d *dummyPlugin) PostStartContainer(_ context.Context, _ *api.PostStartContainerRequest) (*api.PostStartContainerResponse, error) {
	return &api.PostStartContainerResponse{}, nil
}

func (d *dummyPlugin) UpdateContainer(_ context.Context, _ *api.UpdateContainerRequest) (*api.UpdateContainerResponse, error) {
	return &api.UpdateContainerResponse{
		Update: []*api.ContainerUpdate{
			{ContainerId: "test-container"},
		},
		Evict: []*api.ContainerEviction{
			{ContainerId: "test-container"},
			{ContainerId: "test-container-2"},
		},
	}, nil
}

func (d *dummyPlugin) PostUpdateContainer(_ context.Context, _ *api.PostUpdateContainerRequest) (*api.PostUpdateContainerResponse, error) {
	return &api.PostUpdateContainerResponse{}, nil
}

func (d *dummyPlugin) StopContainer(_ context.Context, _ *api.StopContainerRequest) (*api.StopContainerResponse, error) {
	return &api.StopContainerResponse{
		Update: []*api.ContainerUpdate{
			{ContainerId: "test-container"},
		},
	}, nil
}

func (d *dummyPlugin) RemoveContainer(_ context.Context, _ *api.RemoveContainerRequest) (*api.RemoveContainerResponse, error) {
	return &api.RemoveContainerResponse{}, nil
}

func (d *dummyPlugin) StateChange(_ context.Context, _ *api.StateChangeEvent) (*api.Empty, error) {
	return &api.Empty{}, nil
}

func (d *dummyPlugin) ValidateContainerAdjustment(_ context.Context, _ *api.ValidateContainerAdjustmentRequest) (*api.ValidateContainerAdjustmentResponse, error) {
	return &api.ValidateContainerAdjustmentResponse{}, nil
}

func setupTestPlugin() (*mockMetrics, *plugin) {
	m := &mockMetrics{}
	adapt := &Adaptation{metrics: m}

	impl := &pluginType{builtinImpl: &dummyPlugin{}}

	var events api.EventMask
	events.Set(api.Event_RUN_POD_SANDBOX)
	events.Set(api.Event_UPDATE_POD_SANDBOX)
	events.Set(api.Event_POST_UPDATE_POD_SANDBOX)
	events.Set(api.Event_STOP_POD_SANDBOX)
	events.Set(api.Event_REMOVE_POD_SANDBOX)
	events.Set(api.Event_CREATE_CONTAINER)
	events.Set(api.Event_POST_CREATE_CONTAINER)
	events.Set(api.Event_START_CONTAINER)
	events.Set(api.Event_POST_START_CONTAINER)
	events.Set(api.Event_UPDATE_CONTAINER)
	events.Set(api.Event_POST_UPDATE_CONTAINER)
	events.Set(api.Event_STOP_CONTAINER)
	events.Set(api.Event_REMOVE_CONTAINER)
	events.Set(api.Event_VALIDATE_CONTAINER_ADJUSTMENT)

	p := &plugin{
		r:      adapt,
		events: events,
		impl:   impl,
		idx:    "00",
		base:   "test-plugin",
	}

	return m, p
}

func TestPluginSynchronizeMetrics(t *testing.T) {
	m, p := setupTestPlugin()

	_, err := p.synchronize(context.Background(), nil, nil)
	assert.NoError(t, err)

	assert.Len(t, m.invocations, 1)
	assert.Equal(t, p.name(), m.invocations[0].pluginName)
	assert.Equal(t, "Synchronize", m.invocations[0].operation)
	assert.Nil(t, m.invocations[0].err)

	assert.Len(t, m.latencies, 1)
	assert.Equal(t, p.name(), m.latencies[0].pluginName)
	assert.Equal(t, "Synchronize", m.latencies[0].operation)
	assert.NotZero(t, m.latencies[0].latency)

	assert.Len(t, m.adjustments, 1)
	assert.Equal(t, p.name(), m.adjustments[0].pluginName)
	assert.Equal(t, "Synchronize", m.adjustments[0].operation)
	assert.Equal(t, 1, m.adjustments[0].updates)
	assert.Equal(t, 0, m.adjustments[0].evicts)
	assert.Nil(t, m.adjustments[0].adjust)
}

func TestPluginRunPodSandboxMetrics(t *testing.T) {
	m, p := setupTestPlugin()

	evt := api.Event_RUN_POD_SANDBOX
	req := &api.RunPodSandboxRequest{}

	_, err := p.runPodSandbox(context.Background(), req)
	assert.NoError(t, err)

	assert.Len(t, m.invocations, 1)
	assert.Equal(t, p.name(), m.invocations[0].pluginName)
	assert.Equal(t, evt.PrettyName(), m.invocations[0].operation)
	assert.Nil(t, m.invocations[0].err)

	assert.Len(t, m.latencies, 1)
	assert.Equal(t, p.name(), m.latencies[0].pluginName)
	assert.Equal(t, evt.PrettyName(), m.latencies[0].operation)
	assert.NotZero(t, m.latencies[0].latency)
}

func TestPluginUpdatePodSandboxMetrics(t *testing.T) {
	m, p := setupTestPlugin()

	evt := api.Event_UPDATE_POD_SANDBOX
	req := &api.UpdatePodSandboxRequest{}
	_, err := p.updatePodSandbox(context.Background(), req)
	assert.NoError(t, err)

	assert.Len(t, m.invocations, 1)
	assert.Equal(t, p.name(), m.invocations[0].pluginName)
	assert.Equal(t, evt.PrettyName(), m.invocations[0].operation)
	assert.Nil(t, m.invocations[0].err)

	assert.Len(t, m.latencies, 1)
	assert.Equal(t, p.name(), m.latencies[0].pluginName)
	assert.Equal(t, evt.PrettyName(), m.latencies[0].operation)
	assert.NotZero(t, m.latencies[0].latency)

	assert.Len(t, m.adjustments, 0)
}

func TestPluginPostUpdatePodSandboxMetrics(t *testing.T) {
	m, p := setupTestPlugin()

	evt := api.Event_POST_UPDATE_POD_SANDBOX
	req := &api.PostUpdatePodSandboxRequest{}
	_, err := p.postUpdatePodSandbox(context.Background(), req)
	assert.NoError(t, err)

	assert.Len(t, m.invocations, 1)
	assert.Equal(t, p.name(), m.invocations[0].pluginName)
	assert.Equal(t, evt.PrettyName(), m.invocations[0].operation)
	assert.Nil(t, m.invocations[0].err)

	assert.Len(t, m.latencies, 1)
	assert.Equal(t, p.name(), m.latencies[0].pluginName)
	assert.Equal(t, evt.PrettyName(), m.latencies[0].operation)
	assert.NotZero(t, m.latencies[0].latency)

	assert.Len(t, m.adjustments, 0)
}

func TestPluginStopPodSandboxMetrics(t *testing.T) {
	m, p := setupTestPlugin()

	evt := api.Event_STOP_POD_SANDBOX
	req := &api.StopPodSandboxRequest{}
	_, err := p.stopPodSandbox(context.Background(), req)
	assert.NoError(t, err)

	assert.Len(t, m.invocations, 1)
	assert.Equal(t, p.name(), m.invocations[0].pluginName)
	assert.Equal(t, evt.PrettyName(), m.invocations[0].operation)
	assert.Nil(t, m.invocations[0].err)

	assert.Len(t, m.latencies, 1)
	assert.Equal(t, p.name(), m.latencies[0].pluginName)
	assert.Equal(t, evt.PrettyName(), m.latencies[0].operation)
	assert.NotZero(t, m.latencies[0].latency)

	assert.Len(t, m.adjustments, 0)
}

func TestPluginRemovePodSandboxMetrics(t *testing.T) {
	m, p := setupTestPlugin()

	evt := api.Event_REMOVE_POD_SANDBOX
	req := &api.RemovePodSandboxRequest{}
	_, err := p.removePodSandbox(context.Background(), req)
	assert.NoError(t, err)

	assert.Len(t, m.invocations, 1)
	assert.Equal(t, p.name(), m.invocations[0].pluginName)
	assert.Equal(t, evt.PrettyName(), m.invocations[0].operation)
	assert.Nil(t, m.invocations[0].err)

	assert.Len(t, m.latencies, 1)
	assert.Equal(t, p.name(), m.latencies[0].pluginName)
	assert.Equal(t, evt.PrettyName(), m.latencies[0].operation)
	assert.NotZero(t, m.latencies[0].latency)

	assert.Len(t, m.adjustments, 0)
}

func TestPluginCreateContainerMetrics(t *testing.T) {
	m, p := setupTestPlugin()

	evt := api.Event_CREATE_CONTAINER
	req := &api.CreateContainerRequest{}
	_, err := p.createContainer(context.Background(), req)
	assert.NoError(t, err)

	assert.Len(t, m.invocations, 1)
	assert.Equal(t, p.name(), m.invocations[0].pluginName)
	assert.Equal(t, evt.PrettyName(), m.invocations[0].operation)
	assert.Nil(t, m.invocations[0].err)

	assert.Len(t, m.latencies, 1)
	assert.Equal(t, p.name(), m.latencies[0].pluginName)
	assert.Equal(t, evt.PrettyName(), m.latencies[0].operation)
	assert.NotZero(t, m.latencies[0].latency)

	assert.Len(t, m.adjustments, 1)
	assert.Equal(t, p.name(), m.adjustments[0].pluginName)
	assert.Equal(t, evt.PrettyName(), m.adjustments[0].operation)
	assert.Equal(t, 1, m.adjustments[0].updates)
	assert.Equal(t, 1, m.adjustments[0].evicts)
	assert.NotNil(t, m.adjustments[0].adjust)
}

func TestPluginPostCreateContainerMetrics(t *testing.T) {
	m, p := setupTestPlugin()

	evt := api.Event_POST_CREATE_CONTAINER
	req := &api.PostCreateContainerRequest{}
	_, err := p.postCreateContainer(context.Background(), req)
	assert.NoError(t, err)

	assert.Len(t, m.invocations, 1)
	assert.Equal(t, p.name(), m.invocations[0].pluginName)
	assert.Equal(t, evt.PrettyName(), m.invocations[0].operation)
	assert.Nil(t, m.invocations[0].err)

	assert.Len(t, m.latencies, 1)
	assert.Equal(t, p.name(), m.latencies[0].pluginName)
	assert.Equal(t, evt.PrettyName(), m.latencies[0].operation)
	assert.NotZero(t, m.latencies[0].latency)
}

func TestPluginStartContainerMetrics(t *testing.T) {
	m, p := setupTestPlugin()

	evt := api.Event_START_CONTAINER
	req := &api.StartContainerRequest{}
	_, err := p.startContainer(context.Background(), req)
	assert.NoError(t, err)

	assert.Len(t, m.invocations, 1)
	assert.Equal(t, p.name(), m.invocations[0].pluginName)
	assert.Equal(t, evt.PrettyName(), m.invocations[0].operation)
	assert.Nil(t, m.invocations[0].err)

	assert.Len(t, m.latencies, 1)
	assert.Equal(t, p.name(), m.latencies[0].pluginName)
	assert.Equal(t, evt.PrettyName(), m.latencies[0].operation)
	assert.NotZero(t, m.latencies[0].latency)
}

func TestPluginPostStartContainerMetrics(t *testing.T) {
	m, p := setupTestPlugin()

	evt := api.Event_POST_START_CONTAINER
	req := &api.PostStartContainerRequest{}
	_, err := p.postStartContainer(context.Background(), req)
	assert.NoError(t, err)

	assert.Len(t, m.invocations, 1)
	assert.Equal(t, p.name(), m.invocations[0].pluginName)
	assert.Equal(t, evt.PrettyName(), m.invocations[0].operation)
	assert.Nil(t, m.invocations[0].err)

	assert.Len(t, m.latencies, 1)
	assert.Equal(t, p.name(), m.latencies[0].pluginName)
	assert.Equal(t, evt.PrettyName(), m.latencies[0].operation)
	assert.NotZero(t, m.latencies[0].latency)
}

func TestPluginUpdateContainerMetrics(t *testing.T) {
	m, p := setupTestPlugin()

	evt := api.Event_UPDATE_CONTAINER
	req := &api.UpdateContainerRequest{}
	_, err := p.updateContainer(context.Background(), req)
	assert.NoError(t, err)

	assert.Len(t, m.invocations, 1)
	assert.Equal(t, p.name(), m.invocations[0].pluginName)
	assert.Equal(t, evt.PrettyName(), m.invocations[0].operation)
	assert.Nil(t, m.invocations[0].err)

	assert.Len(t, m.latencies, 1)
	assert.Equal(t, p.name(), m.latencies[0].pluginName)
	assert.Equal(t, evt.PrettyName(), m.latencies[0].operation)
	assert.NotZero(t, m.latencies[0].latency)

	assert.Len(t, m.adjustments, 1)
	assert.Equal(t, p.name(), m.adjustments[0].pluginName)
	assert.Equal(t, evt.PrettyName(), m.adjustments[0].operation)
	assert.Equal(t, 1, m.adjustments[0].updates)
	assert.Equal(t, 2, m.adjustments[0].evicts)
	assert.Nil(t, m.adjustments[0].adjust)
}

func TestPluginPostUpdateContainerMetrics(t *testing.T) {
	m, p := setupTestPlugin()

	evt := api.Event_POST_UPDATE_CONTAINER
	req := &api.PostUpdateContainerRequest{}
	_, err := p.postUpdateContainer(context.Background(), req)
	assert.NoError(t, err)

	assert.Len(t, m.invocations, 1)
	assert.Equal(t, p.name(), m.invocations[0].pluginName)
	assert.Equal(t, evt.PrettyName(), m.invocations[0].operation)
	assert.Nil(t, m.invocations[0].err)

	assert.Len(t, m.latencies, 1)
	assert.Equal(t, p.name(), m.latencies[0].pluginName)
	assert.Equal(t, evt.PrettyName(), m.latencies[0].operation)
	assert.NotZero(t, m.latencies[0].latency)
}

func TestPluginStopContainerMetrics(t *testing.T) {
	m, p := setupTestPlugin()

	evt := api.Event_STOP_CONTAINER
	req := &api.StopContainerRequest{}
	_, err := p.stopContainer(context.Background(), req)
	assert.NoError(t, err)

	assert.Len(t, m.invocations, 1)
	assert.Equal(t, p.name(), m.invocations[0].pluginName)
	assert.Equal(t, evt.PrettyName(), m.invocations[0].operation)
	assert.Nil(t, m.invocations[0].err)

	assert.Len(t, m.latencies, 1)
	assert.Equal(t, p.name(), m.latencies[0].pluginName)
	assert.Equal(t, evt.PrettyName(), m.latencies[0].operation)
	assert.NotZero(t, m.latencies[0].latency)

	assert.Len(t, m.adjustments, 1)
	assert.Equal(t, p.name(), m.adjustments[0].pluginName)
	assert.Equal(t, evt.PrettyName(), m.adjustments[0].operation)
	assert.Equal(t, 1, m.adjustments[0].updates)
	assert.Equal(t, 0, m.adjustments[0].evicts)
	assert.Nil(t, m.adjustments[0].adjust)
}

func TestPluginRemoveContainerMetrics(t *testing.T) {
	m, p := setupTestPlugin()

	evt := api.Event_REMOVE_CONTAINER
	req := &api.RemoveContainerRequest{}
	_, err := p.removeContainer(context.Background(), req)
	assert.NoError(t, err)

	assert.Len(t, m.invocations, 1)
	assert.Equal(t, p.name(), m.invocations[0].pluginName)
	assert.Equal(t, evt.PrettyName(), m.invocations[0].operation)
	assert.Nil(t, m.invocations[0].err)

	assert.Len(t, m.latencies, 1)
	assert.Equal(t, p.name(), m.latencies[0].pluginName)
	assert.Equal(t, evt.PrettyName(), m.latencies[0].operation)
	assert.NotZero(t, m.latencies[0].latency)
}

func TestPluginValidateContainerAdjustmentMetrics(t *testing.T) {
	m, p := setupTestPlugin()

	evt := api.Event_VALIDATE_CONTAINER_ADJUSTMENT
	req := &api.ValidateContainerAdjustmentRequest{}
	err := p.ValidateContainerAdjustment(context.Background(), req)
	assert.NoError(t, err)

	assert.Len(t, m.invocations, 1)
	assert.Equal(t, p.name(), m.invocations[0].pluginName)
	assert.Equal(t, evt.PrettyName(), m.invocations[0].operation)
	assert.Nil(t, m.invocations[0].err)

	assert.Len(t, m.latencies, 1)
	assert.Equal(t, p.name(), m.latencies[0].pluginName)
	assert.Equal(t, evt.PrettyName(), m.latencies[0].operation)
	assert.NotZero(t, m.latencies[0].latency)

	assert.Len(t, m.adjustments, 0)
}
