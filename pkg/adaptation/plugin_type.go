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

	api "github.com/containerd/nri/pkg/api/v1beta1"
)

type pluginType struct {
	wasmImpl    api.Plugin
	ttrpcImpl   api.PluginService
	builtinImpl api.PluginService
}

var (
	errUnknownImpl = errors.New("unknown plugin implementation type")
)

func (p *pluginType) isWasm() bool {
	return p.wasmImpl != nil
}

func (p *pluginType) isTtrpc() bool {
	return p.ttrpcImpl != nil
}

func (p *pluginType) isBuiltin() bool {
	return p.builtinImpl != nil
}

func (p *pluginType) Synchronize(ctx context.Context, req *SynchronizeRequest) (*SynchronizeResponse, error) {
	switch {
	case p.ttrpcImpl != nil:
		return p.ttrpcImpl.Synchronize(ctx, req)
	case p.builtinImpl != nil:
		return p.builtinImpl.Synchronize(ctx, req)
	case p.wasmImpl != nil:
		return p.wasmImpl.Synchronize(ctx, req)
	}

	return nil, errUnknownImpl
}

func (p *pluginType) Configure(ctx context.Context, req *ConfigureRequest) (*ConfigureResponse, error) {
	switch {
	case p.ttrpcImpl != nil:
		return p.ttrpcImpl.Configure(ctx, req)
	case p.builtinImpl != nil:
		return p.builtinImpl.Configure(ctx, req)
	case p.wasmImpl != nil:
		return p.wasmImpl.Configure(ctx, req)
	}

	return nil, errUnknownImpl
}

func (p *pluginType) RunPodSandbox(ctx context.Context, req *RunPodSandboxRequest) (*RunPodSandboxResponse, error) {
	switch {
	case p.ttrpcImpl != nil:
		return p.ttrpcImpl.RunPodSandbox(ctx, req)
	case p.builtinImpl != nil:
		return p.builtinImpl.RunPodSandbox(ctx, req)
	case p.wasmImpl != nil:
		return p.wasmImpl.RunPodSandbox(ctx, req)
	}

	return nil, errUnknownImpl
}

func (p *pluginType) UpdatePodSandbox(ctx context.Context, req *UpdatePodSandboxRequest) (*UpdatePodSandboxResponse, error) {
	switch {
	case p.ttrpcImpl != nil:
		return p.ttrpcImpl.UpdatePodSandbox(ctx, req)
	case p.builtinImpl != nil:
		return p.builtinImpl.UpdatePodSandbox(ctx, req)
	case p.wasmImpl != nil:
		return p.wasmImpl.UpdatePodSandbox(ctx, req)
	}

	return nil, errUnknownImpl
}

func (p *pluginType) PostUpdatePodSandbox(ctx context.Context, req *PostUpdatePodSandboxRequest) (*PostUpdatePodSandboxResponse, error) {
	switch {
	case p.ttrpcImpl != nil:
		return p.ttrpcImpl.PostUpdatePodSandbox(ctx, req)
	case p.builtinImpl != nil:
		return p.builtinImpl.PostUpdatePodSandbox(ctx, req)
	case p.wasmImpl != nil:
		return p.wasmImpl.PostUpdatePodSandbox(ctx, req)
	}

	return nil, errUnknownImpl
}

func (p *pluginType) StopPodSandbox(ctx context.Context, req *StopPodSandboxRequest) (*StopPodSandboxResponse, error) {
	switch {
	case p.ttrpcImpl != nil:
		return p.ttrpcImpl.StopPodSandbox(ctx, req)
	case p.builtinImpl != nil:
		return p.builtinImpl.StopPodSandbox(ctx, req)
	case p.wasmImpl != nil:
		return p.wasmImpl.StopPodSandbox(ctx, req)
	}

	return nil, errUnknownImpl
}

func (p *pluginType) RemovePodSandbox(ctx context.Context, req *RemovePodSandboxRequest) (*RemovePodSandboxResponse, error) {
	switch {
	case p.ttrpcImpl != nil:
		return p.ttrpcImpl.RemovePodSandbox(ctx, req)
	case p.builtinImpl != nil:
		return p.builtinImpl.RemovePodSandbox(ctx, req)
	case p.wasmImpl != nil:
		return p.wasmImpl.RemovePodSandbox(ctx, req)
	}

	return nil, errUnknownImpl
}

func (p *pluginType) CreateContainer(ctx context.Context, req *CreateContainerRequest) (*CreateContainerResponse, error) {
	switch {
	case p.ttrpcImpl != nil:
		return p.ttrpcImpl.CreateContainer(ctx, req)
	case p.builtinImpl != nil:
		return p.builtinImpl.CreateContainer(ctx, req)
	case p.wasmImpl != nil:
		return p.wasmImpl.CreateContainer(ctx, req)
	}

	return nil, errUnknownImpl
}

func (p *pluginType) PostCreateContainer(ctx context.Context, req *PostCreateContainerRequest) (*PostCreateContainerResponse, error) {
	switch {
	case p.ttrpcImpl != nil:
		return p.ttrpcImpl.PostCreateContainer(ctx, req)
	case p.builtinImpl != nil:
		return p.builtinImpl.PostCreateContainer(ctx, req)
	case p.wasmImpl != nil:
		return p.wasmImpl.PostCreateContainer(ctx, req)
	}

	return nil, errUnknownImpl
}

func (p *pluginType) StartContainer(ctx context.Context, req *StartContainerRequest) (*StartContainerResponse, error) {
	switch {
	case p.ttrpcImpl != nil:
		return p.ttrpcImpl.StartContainer(ctx, req)
	case p.builtinImpl != nil:
		return p.builtinImpl.StartContainer(ctx, req)
	case p.wasmImpl != nil:
		return p.wasmImpl.StartContainer(ctx, req)
	}

	return nil, errUnknownImpl
}

func (p *pluginType) PostStartContainer(ctx context.Context, req *PostStartContainerRequest) (*PostStartContainerResponse, error) {
	switch {
	case p.ttrpcImpl != nil:
		return p.ttrpcImpl.PostStartContainer(ctx, req)
	case p.builtinImpl != nil:
		return p.builtinImpl.PostStartContainer(ctx, req)
	case p.wasmImpl != nil:
		return p.wasmImpl.PostStartContainer(ctx, req)
	}

	return nil, errUnknownImpl
}

func (p *pluginType) UpdateContainer(ctx context.Context, req *UpdateContainerRequest) (*UpdateContainerResponse, error) {
	switch {
	case p.ttrpcImpl != nil:
		return p.ttrpcImpl.UpdateContainer(ctx, req)
	case p.builtinImpl != nil:
		return p.builtinImpl.UpdateContainer(ctx, req)
	case p.wasmImpl != nil:
		return p.wasmImpl.UpdateContainer(ctx, req)
	}

	return nil, errUnknownImpl
}

func (p *pluginType) PostUpdateContainer(ctx context.Context, req *PostUpdateContainerRequest) (*PostUpdateContainerResponse, error) {
	switch {
	case p.ttrpcImpl != nil:
		return p.ttrpcImpl.PostUpdateContainer(ctx, req)
	case p.builtinImpl != nil:
		return p.builtinImpl.PostUpdateContainer(ctx, req)
	case p.wasmImpl != nil:
		return p.wasmImpl.PostUpdateContainer(ctx, req)
	}

	return nil, errUnknownImpl
}

func (p *pluginType) StopContainer(ctx context.Context, req *StopContainerRequest) (*StopContainerResponse, error) {
	switch {
	case p.ttrpcImpl != nil:
		return p.ttrpcImpl.StopContainer(ctx, req)
	case p.builtinImpl != nil:
		return p.builtinImpl.StopContainer(ctx, req)
	case p.wasmImpl != nil:
		return p.wasmImpl.StopContainer(ctx, req)
	}

	return nil, errUnknownImpl
}

func (p *pluginType) RemoveContainer(ctx context.Context, req *RemoveContainerRequest) (*RemoveContainerResponse, error) {
	switch {
	case p.ttrpcImpl != nil:
		return p.ttrpcImpl.RemoveContainer(ctx, req)
	case p.builtinImpl != nil:
		return p.builtinImpl.RemoveContainer(ctx, req)
	case p.wasmImpl != nil:
		return p.wasmImpl.RemoveContainer(ctx, req)
	}

	return nil, errUnknownImpl
}

func (p *pluginType) ValidateContainerAdjustment(ctx context.Context, req *ValidateContainerAdjustmentRequest) (*ValidateContainerAdjustmentResponse, error) {
	switch {
	case p.ttrpcImpl != nil:
		return p.ttrpcImpl.ValidateContainerAdjustment(ctx, req)
	case p.builtinImpl != nil:
		return p.builtinImpl.ValidateContainerAdjustment(ctx, req)
	case p.wasmImpl != nil:
		return p.wasmImpl.ValidateContainerAdjustment(ctx, req)
	}

	return nil, errUnknownImpl
}
