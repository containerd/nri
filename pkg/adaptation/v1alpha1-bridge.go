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

	old "github.com/containerd/nri/pkg/api"
	"github.com/containerd/nri/pkg/api/convert"
	api "github.com/containerd/nri/pkg/api/v1beta1"
)

type v1alpha1Bridge struct {
	new *plugin
	old *v1alpha1Plugin
}

type v1alpha1Plugin struct {
	p old.PluginService
}

func (p *plugin) RegisterV1Alpha1Bridge() {
	b := &v1alpha1Bridge{new: p}
	old.RegisterRuntimeService(p.rpcs, b)
}

func (b *v1alpha1Bridge) RegisterPlugin(ctx context.Context, req *old.RegisterPluginRequest) (*old.Empty, error) {
	b.old = &v1alpha1Plugin{
		p: old.NewPluginClient(b.new.rpcc),
	}
	b.new.impl.ttrpcImpl = b.old

	nreq := convert.RegisterPluginRequest(req)
	nrpl, err := b.new.RegisterPlugin(ctx, nreq)
	if err != nil {
		return nil, err
	}

	return convert.RegisterPluginResponse(nrpl), nil
}

func (b *v1alpha1Bridge) UpdateContainers(ctx context.Context, req *old.UpdateContainersRequest) (*old.UpdateContainersResponse, error) {
	nreq := convert.UpdateContainersRequest(req)
	nrpl, err := b.new.UpdateContainers(ctx, nreq)
	if err != nil {
		return nil, err
	}

	return convert.UpdateContainersResponse(nrpl), nil
}

func (p *v1alpha1Plugin) Configure(ctx context.Context, req *api.ConfigureRequest) (*api.ConfigureResponse, error) {
	oreq := convert.ConfigureRequest(req)
	orpl, err := p.p.Configure(ctx, oreq)
	if err != nil {
		return nil, err
	}

	return convert.ConfigureResponse(orpl), nil
}

func (p *v1alpha1Plugin) Synchronize(ctx context.Context, req *api.SynchronizeRequest) (*api.SynchronizeResponse, error) {
	oreq := convert.SynchronizeRequest(req)
	orpl, err := p.p.Synchronize(ctx, oreq)
	if err != nil {
		return nil, err
	}

	return convert.SynchronizeResponse(orpl), nil
}

func (p *v1alpha1Plugin) Shutdown(_ context.Context, _ *api.ShutdownRequest) (*api.ShutdownResponse, error) {
	return &api.ShutdownResponse{}, nil
}

func (p *v1alpha1Plugin) RunPodSandbox(ctx context.Context, req *api.RunPodSandboxRequest) (*api.RunPodSandboxResponse, error) {
	oreq := convert.RunPodSandboxRequest(req)
	orpl, err := p.p.StateChange(ctx, oreq)
	if err != nil {
		return nil, err
	}

	return convert.RunPodSandboxResponse(orpl), nil
}

func (p *v1alpha1Plugin) UpdatePodSandbox(ctx context.Context, req *api.UpdatePodSandboxRequest) (*api.UpdatePodSandboxResponse, error) {
	oreq := convert.UpdatePodSandboxRequest(req)
	orpl, err := p.p.UpdatePodSandbox(ctx, oreq)
	if err != nil {
		return nil, err
	}

	return convert.UpdatePodSandboxResponse(orpl), nil
}

func (p *v1alpha1Plugin) PostUpdatePodSandbox(ctx context.Context, req *api.PostUpdatePodSandboxRequest) (*api.PostUpdatePodSandboxResponse, error) {
	oreq := convert.PostUpdatePodSandboxRequest(req)
	orpl, err := p.p.StateChange(ctx, oreq)
	if err != nil {
		return nil, err
	}

	return convert.PostUpdatePodSandboxResponse(orpl), nil
}

func (p *v1alpha1Plugin) StopPodSandbox(ctx context.Context, req *api.StopPodSandboxRequest) (*api.StopPodSandboxResponse, error) {
	oreq := convert.StopPodSandboxRequest(req)
	orpl, err := p.p.StateChange(ctx, oreq)
	if err != nil {
		return nil, err
	}

	return convert.StopPodSandboxResponse(orpl), nil
}

func (p *v1alpha1Plugin) RemovePodSandbox(ctx context.Context, req *api.RemovePodSandboxRequest) (*api.RemovePodSandboxResponse, error) {
	oreq := convert.RemovePodSandboxRequest(req)
	orpl, err := p.p.StateChange(ctx, oreq)
	if err != nil {
		return nil, err
	}

	return convert.RemovePodSandboxResponse(orpl), nil
}

func (p *v1alpha1Plugin) CreateContainer(ctx context.Context, req *api.CreateContainerRequest) (*api.CreateContainerResponse, error) {
	oreq := convert.CreateContainerRequest(req)
	orpl, err := p.p.CreateContainer(ctx, oreq)
	if err != nil {
		return nil, err
	}

	return convert.CreateContainerResponse(orpl), nil
}

func (p *v1alpha1Plugin) PostCreateContainer(ctx context.Context, req *api.PostCreateContainerRequest) (*api.PostCreateContainerResponse, error) {
	oreq := convert.PostCreateContainerRequest(req)
	orpl, err := p.p.StateChange(ctx, oreq)
	if err != nil {
		return nil, err
	}

	return convert.PostCreateContainerResponse(orpl), nil
}

func (p *v1alpha1Plugin) StartContainer(ctx context.Context, req *api.StartContainerRequest) (*api.StartContainerResponse, error) {
	oreq := convert.StartContainerRequest(req)
	orpl, err := p.p.StateChange(ctx, oreq)
	if err != nil {
		return nil, err
	}

	return convert.StartContainerResponse(orpl), nil
}

func (p *v1alpha1Plugin) PostStartContainer(ctx context.Context, req *api.PostStartContainerRequest) (*api.PostStartContainerResponse, error) {
	oreq := convert.PostStartContainerRequest(req)
	orpl, err := p.p.StateChange(ctx, oreq)
	if err != nil {
		return nil, err
	}

	return convert.PostStartContainerResponse(orpl), nil
}

func (p *v1alpha1Plugin) UpdateContainer(ctx context.Context, req *api.UpdateContainerRequest) (*api.UpdateContainerResponse, error) {
	oreq := convert.UpdateContainerRequest(req)
	orpl, err := p.p.UpdateContainer(ctx, oreq)
	if err != nil {
		return nil, err
	}

	return convert.UpdateContainerResponse(orpl), nil
}

func (p *v1alpha1Plugin) PostUpdateContainer(ctx context.Context, req *api.PostUpdateContainerRequest) (*api.PostUpdateContainerResponse, error) {
	oreq := convert.PostUpdateContainerRequest(req)
	orpl, err := p.p.StateChange(ctx, oreq)
	if err != nil {
		return nil, err
	}

	return convert.PostUpdateContainerResponse(orpl), nil
}

func (p *v1alpha1Plugin) StopContainer(ctx context.Context, req *api.StopContainerRequest) (*api.StopContainerResponse, error) {
	oreq := convert.StopContainerRequest(req)
	orpl, err := p.p.StopContainer(ctx, oreq)
	if err != nil {
		return nil, err
	}

	return convert.StopContainerResponse(orpl), nil
}

func (p *v1alpha1Plugin) RemoveContainer(ctx context.Context, req *api.RemoveContainerRequest) (*api.RemoveContainerResponse, error) {
	oreq := convert.RemoveContainerRequest(req)
	orpl, err := p.p.StateChange(ctx, oreq)
	if err != nil {
		return nil, err
	}

	return convert.RemoveContainerResponse(orpl), nil
}

func (p *v1alpha1Plugin) ValidateContainerAdjustment(ctx context.Context, req *api.ValidateContainerAdjustmentRequest) (*api.ValidateContainerAdjustmentResponse, error) {
	oreq := convert.ValidateContainerAdjustmentRequest(req)
	orpl, err := p.p.ValidateContainerAdjustment(ctx, oreq)
	if err != nil {
		return nil, err
	}

	return convert.ValidateContainerAdjustmentResponse(orpl), nil
}
