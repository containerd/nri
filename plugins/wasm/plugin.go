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

package main

import (
	"context"

	api "github.com/containerd/nri/pkg/api/v1beta1"
)

type plugin struct{}

func init() {
	api.RegisterPlugin(&plugin{})
}

func main() {}

func log(ctx context.Context, msg string) {
	api.NewHostFunctions().Log(ctx, &api.LogRequest{
		Msg:   "WASM: " + msg,
		Level: api.LogRequest_LEVEL_INFO,
	})
}

func (p *plugin) Configure(ctx context.Context, req *api.ConfigureRequest) (*api.ConfigureResponse, error) {
	log(ctx, "Got configure request")
	return nil, nil
}

func (p *plugin) Synchronize(ctx context.Context, req *api.SynchronizeRequest) (*api.SynchronizeResponse, error) {
	log(ctx, "Got synchronize request")
	return nil, nil
}

func (p *plugin) Shutdown(ctx context.Context, req *api.ShutdownRequest) (*api.ShutdownResponse, error) {
	log(ctx, "Got shutdown request")
	return nil, nil
}

func (p *plugin) RunPodSandbox(ctx context.Context, req *api.RunPodSandboxRequest) (*api.RunPodSandboxResponse, error) {
	log(ctx, "Got run pod sandbox request")
	return nil, nil
}

func (p *plugin) UpdatePodSandbox(ctx context.Context, req *api.UpdatePodSandboxRequest) (*api.UpdatePodSandboxResponse, error) {
	log(ctx, "Got update pod sandbox request")
	return nil, nil
}

func (p *plugin) PostUpdatePodSandbox(ctx context.Context, req *api.PostUpdatePodSandboxRequest) (*api.PostUpdatePodSandboxResponse, error) {
	log(ctx, "Got post update pod sandbox request")
	return nil, nil
}

func (p *plugin) StopPodSandbox(ctx context.Context, req *api.StopPodSandboxRequest) (*api.StopPodSandboxResponse, error) {
	log(ctx, "Got stop pod sandbox request")
	return nil, nil
}

func (p *plugin) RemovePodSandbox(ctx context.Context, req *api.RemovePodSandboxRequest) (*api.RemovePodSandboxResponse, error) {
	log(ctx, "Got remove pod sandbox request")
	return nil, nil
}

func (p *plugin) CreateContainer(ctx context.Context, req *api.CreateContainerRequest) (*api.CreateContainerResponse, error) {
	log(ctx, "Got create container request")
	return nil, nil
}

func (p *plugin) PostCreateContainer(ctx context.Context, req *api.PostCreateContainerRequest) (*api.PostCreateContainerResponse, error) {
	log(ctx, "Got post create container request")
	return nil, nil
}

func (p *plugin) StartContainer(ctx context.Context, req *api.StartContainerRequest) (*api.StartContainerResponse, error) {
	log(ctx, "Got start container request")
	return nil, nil
}

func (p *plugin) PostStartContainer(ctx context.Context, req *api.PostStartContainerRequest) (*api.PostStartContainerResponse, error) {
	log(ctx, "Got post start container request")
	return nil, nil
}

func (p *plugin) UpdateContainer(ctx context.Context, req *api.UpdateContainerRequest) (*api.UpdateContainerResponse, error) {
	log(ctx, "Got update container request")
	return nil, nil
}

func (p *plugin) PostUpdateContainer(ctx context.Context, req *api.PostUpdateContainerRequest) (*api.PostUpdateContainerResponse, error) {
	log(ctx, "Got post update container request")
	return nil, nil
}

func (p *plugin) StopContainer(ctx context.Context, req *api.StopContainerRequest) (*api.StopContainerResponse, error) {
	log(ctx, "Got stop container request")
	return nil, nil
}

func (p *plugin) RemoveContainer(ctx context.Context, req *api.RemoveContainerRequest) (*api.RemoveContainerResponse, error) {
	log(ctx, "Got remove container request")
	return nil, nil
}

func (p *plugin) ValidateContainerAdjustment(ctx context.Context, req *api.ValidateContainerAdjustmentRequest) (*api.ValidateContainerAdjustmentResponse, error) {
	return &api.ValidateContainerAdjustmentResponse{}, nil
}
