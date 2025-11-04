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

package convert

import (
	v1alpha1 "github.com/containerd/nri/pkg/api/v1alpha1"
	v1beta1 "github.com/containerd/nri/pkg/api/v1beta1"
)

// This package provides conversion functions for v1alpha1 and v1beta1
// revisions of API data types and messages. We use them on the runtime
// side in NRI to provide protocol translation for plugins which talk
// the older revision of the API.
//
// Notes:
// Some of the conversion functions are unused in protocol translation
// itself. These are the conversion for plugin initiated requests from
// v1beta1 to v1alpha1 and responses to them from v1alpha1 to v1beta1,
// and runtime initiated requests from v1alpha1 to v1beta1 and responses
// to them from v1beta1 to v1alpha1. These functions are only used in
// tests to verify the idempotency of message conversion, from v1alpha1
// to v1beta1 and back.
//
// This is visible in the chosen naming convention as well. Request and
// response conversion functions used in translation omit the revision
// suffix for brevity. The rest have a revision suffix.

// RegisterPluginRequest converts the request between v1alpha1 and v1beta1.
func RegisterPluginRequest(v1a1 *v1alpha1.RegisterPluginRequest) *v1beta1.RegisterPluginRequest {
	if v1a1 == nil {
		return nil
	}

	return &v1beta1.RegisterPluginRequest{
		PluginName: v1a1.PluginName,
		PluginIdx:  v1a1.PluginIdx,
	}
}

// RegisterPluginRequestToV1alpha1 converts the request between v1alpha1 and v1beta1.
func RegisterPluginRequestToV1alpha1(v1b1 *v1beta1.RegisterPluginRequest) *v1alpha1.RegisterPluginRequest {
	if v1b1 == nil {
		return nil
	}

	return &v1alpha1.RegisterPluginRequest{
		PluginName: v1b1.PluginName,
		PluginIdx:  v1b1.PluginIdx,
	}
}

// RegisterPluginResponse converts the reply between v1alpha1 and v1beta1.
func RegisterPluginResponse(v1b1 *v1beta1.RegisterPluginResponse) *v1alpha1.Empty {
	if v1b1 == nil {
		return nil
	}

	return &v1alpha1.Empty{}
}

// RegisterPluginResponseToV1beta1 converts the reply between v1alpha1 and v1beta1.
func RegisterPluginResponseToV1beta1(v1a1 *v1alpha1.Empty) *v1beta1.RegisterPluginResponse {
	if v1a1 == nil {
		return nil
	}

	return &v1beta1.RegisterPluginResponse{}
}

// UpdateContainersRequest converts the request between v1alpha1 and v1beta1.
func UpdateContainersRequest(v1a1 *v1alpha1.UpdateContainersRequest) *v1beta1.UpdateContainersRequest {
	if v1a1 == nil {
		return nil
	}

	return &v1beta1.UpdateContainersRequest{
		Update: ContainerUpdateSliceToV1beta1(v1a1.Update),
		Evict:  ContainerEvictionSliceToV1beta1(v1a1.Evict),
	}
}

// UpdateContainersRequestToV1alpha1 converts the request between v1alpha1 and v1beta1.
func UpdateContainersRequestToV1alpha1(v1b1 *v1beta1.UpdateContainersRequest) *v1alpha1.UpdateContainersRequest {
	if v1b1 == nil {
		return nil
	}

	return &v1alpha1.UpdateContainersRequest{
		Update: ContainerUpdateSliceToV1alpha1(v1b1.Update),
		Evict:  ContainerEvictionSliceToV1alpha1(v1b1.Evict),
	}
}

// UpdateContainersResponse converts the reply between v1alpha1 and v1beta1.
func UpdateContainersResponse(v1b1 *v1beta1.UpdateContainersResponse) *v1alpha1.UpdateContainersResponse {
	if v1b1 == nil {
		return nil
	}

	return &v1alpha1.UpdateContainersResponse{
		Failed: ContainerUpdateSliceToV1alpha1(v1b1.Failed),
	}
}

// UpdateContainersResponseToV1beta1 converts the reply between v1alpha1 and v1beta1.
func UpdateContainersResponseToV1beta1(v1a1 *v1alpha1.UpdateContainersResponse) *v1beta1.UpdateContainersResponse {
	if v1a1 == nil {
		return nil
	}

	return &v1beta1.UpdateContainersResponse{
		Failed: ContainerUpdateSliceToV1beta1(v1a1.Failed),
	}
}

// ConfigureRequest converts the request between v1alpha1 and v1beta1.
func ConfigureRequest(v1b1 *v1beta1.ConfigureRequest) *v1alpha1.ConfigureRequest {
	if v1b1 == nil {
		return nil
	}

	return &v1alpha1.ConfigureRequest{
		Config:              v1b1.Config,
		RuntimeName:         v1b1.RuntimeName,
		RuntimeVersion:      v1b1.RuntimeVersion,
		RegistrationTimeout: v1b1.RegistrationTimeout,
		RequestTimeout:      v1b1.RequestTimeout,
	}
}

// ConfigureRequestToV1beta1 converts the request between v1alpha1 and v1beta1.
func ConfigureRequestToV1beta1(v1a1 *v1alpha1.ConfigureRequest) *v1beta1.ConfigureRequest {
	if v1a1 == nil {
		return nil
	}

	return &v1beta1.ConfigureRequest{
		Config:              v1a1.Config,
		RuntimeName:         v1a1.RuntimeName,
		RuntimeVersion:      v1a1.RuntimeVersion,
		RegistrationTimeout: v1a1.RegistrationTimeout,
		RequestTimeout:      v1a1.RequestTimeout,
	}
}

// ConfigureResponse converts the reply between v1alpha1 and v1beta1.
func ConfigureResponse(v1a1 *v1alpha1.ConfigureResponse) *v1beta1.ConfigureResponse {
	if v1a1 == nil {
		return nil
	}

	return &v1beta1.ConfigureResponse{
		Events: v1a1.Events,
	}
}

// ConfigureResponseToV1alpha1 converts the reply between v1alpha1 and v1beta1.
func ConfigureResponseToV1alpha1(v1b1 *v1beta1.ConfigureResponse) *v1alpha1.ConfigureResponse {
	if v1b1 == nil {
		return nil
	}

	return &v1alpha1.ConfigureResponse{
		Events: v1b1.Events,
	}
}

// SynchronizeRequest converts the request between v1alpha1 and v1beta1.
func SynchronizeRequest(v1b1 *v1beta1.SynchronizeRequest) *v1alpha1.SynchronizeRequest {
	if v1b1 == nil {
		return nil
	}

	return &v1alpha1.SynchronizeRequest{
		Pods:       PodSandboxSliceToV1alpha1(v1b1.Pods),
		Containers: ContainerSliceToV1alpha1(v1b1.Containers),
		More:       v1b1.More,
	}
}

// SynchronizeRequestToV1beta1 converts the request between v1alpha1 and v1beta1.
func SynchronizeRequestToV1beta1(v1a1 *v1alpha1.SynchronizeRequest) *v1beta1.SynchronizeRequest {
	if v1a1 == nil {
		return nil
	}

	return &v1beta1.SynchronizeRequest{
		Pods:       PodSandboxSliceToV1beta1(v1a1.Pods),
		Containers: ContainerSliceToV1beta1(v1a1.Containers),
		More:       v1a1.More,
	}
}

// SynchronizeResponse converts the reply between v1alpha1 and v1beta1.
func SynchronizeResponse(v1a1 *v1alpha1.SynchronizeResponse) *v1beta1.SynchronizeResponse {
	if v1a1 == nil {
		return nil
	}

	return &v1beta1.SynchronizeResponse{
		Update: ContainerUpdateSliceToV1beta1(v1a1.Update),
		More:   v1a1.More,
	}
}

// SynchronizeResponseToV1alpha1 converts the reply between v1alpha1 and v1beta1.
func SynchronizeResponseToV1alpha1(v1b1 *v1beta1.SynchronizeResponse) *v1alpha1.SynchronizeResponse {
	if v1b1 == nil {
		return nil
	}

	return &v1alpha1.SynchronizeResponse{
		Update: ContainerUpdateSliceToV1alpha1(v1b1.Update),
		More:   v1b1.More,
	}
}

// RunPodSandboxRequest converts the request between v1alpha1 and v1beta1.
func RunPodSandboxRequest(v1b1 *v1beta1.RunPodSandboxRequest) *v1alpha1.StateChangeEvent {
	if v1b1 == nil {
		return nil
	}

	v1a1 := &v1alpha1.StateChangeEvent{
		Event: v1alpha1.Event_RUN_POD_SANDBOX,
		Pod:   PodSandboxToV1alpha1(v1b1.Pod),
	}

	return v1a1
}

// RunPodSandboxRequestToV1beta1 converts the request between v1alpha1 and v1beta1.
func RunPodSandboxRequestToV1beta1(v1a1 *v1alpha1.StateChangeEvent) *v1beta1.RunPodSandboxRequest {
	if v1a1 == nil {
		return nil
	}

	v1b1 := &v1beta1.RunPodSandboxRequest{
		Pod: PodSandboxToV1beta1(v1a1.Pod),
	}

	return v1b1
}

// RunPodSandboxResponse converts the reply between v1alpha1 and v1beta1.
func RunPodSandboxResponse(v1a1 *v1alpha1.Empty) *v1beta1.RunPodSandboxResponse {
	if v1a1 == nil {
		return nil
	}

	return &v1beta1.RunPodSandboxResponse{}
}

// RunPodSandboxResponseToV1alpha1 converts the reply between v1alpha1 and v1beta1.
func RunPodSandboxResponseToV1alpha1(v1b1 *v1beta1.RunPodSandboxResponse) *v1alpha1.Empty {
	if v1b1 == nil {
		return nil
	}

	return &v1alpha1.Empty{}
}

// UpdatePodSandboxRequest converts the request between v1alpha1 and v1beta1.
func UpdatePodSandboxRequest(v1b1 *v1beta1.UpdatePodSandboxRequest) *v1alpha1.UpdatePodSandboxRequest {
	if v1b1 == nil {
		return nil
	}

	v1a1 := &v1alpha1.UpdatePodSandboxRequest{
		Pod:                    PodSandboxToV1alpha1(v1b1.Pod),
		OverheadLinuxResources: LinuxResourcesToV1alpha1(v1b1.OverheadLinuxResources),
		LinuxResources:         LinuxResourcesToV1alpha1(v1b1.LinuxResources),
	}

	return v1a1
}

// UpdatePodSandboxRequestToV1beta1 converts the request between v1alpha1 and v1beta1.
func UpdatePodSandboxRequestToV1beta1(v1a1 *v1alpha1.UpdatePodSandboxRequest) *v1beta1.UpdatePodSandboxRequest {
	if v1a1 == nil {
		return nil
	}

	v1b1 := &v1beta1.UpdatePodSandboxRequest{
		Pod:                    PodSandboxToV1beta1(v1a1.Pod),
		OverheadLinuxResources: LinuxResourcesToV1beta1(v1a1.OverheadLinuxResources),
		LinuxResources:         LinuxResourcesToV1beta1(v1a1.LinuxResources),
	}

	return v1b1
}

// UpdatePodSandboxResponse converts the reply between v1alpha1 and v1beta1.
func UpdatePodSandboxResponse(v1a1 *v1alpha1.UpdatePodSandboxResponse) *v1beta1.UpdatePodSandboxResponse {
	if v1a1 == nil {
		return nil
	}

	return &v1beta1.UpdatePodSandboxResponse{}
}

// UpdatePodSandboxResponseToV1alpha1 converts the reply between v1alpha1 and v1beta1.
func UpdatePodSandboxResponseToV1alpha1(v1b1 *v1beta1.UpdatePodSandboxResponse) *v1alpha1.UpdatePodSandboxResponse {
	if v1b1 == nil {
		return nil
	}

	return &v1alpha1.UpdatePodSandboxResponse{}
}

// PostUpdatePodSandboxRequest converts the request between v1alpha1 and v1beta1.
func PostUpdatePodSandboxRequest(v1b1 *v1beta1.PostUpdatePodSandboxRequest) *v1alpha1.StateChangeEvent {
	if v1b1 == nil {
		return nil
	}

	return &v1alpha1.StateChangeEvent{
		Event: v1alpha1.Event_POST_UPDATE_POD_SANDBOX,
		Pod:   PodSandboxToV1alpha1(v1b1.Pod),
	}
}

// PostUpdatePodSandboxRequestToV1beta1 converts the request between v1alpha1 and v1beta1.
func PostUpdatePodSandboxRequestToV1beta1(v1a1 *v1alpha1.StateChangeEvent) *v1beta1.PostUpdatePodSandboxRequest {
	if v1a1 == nil {
		return nil
	}

	return &v1beta1.PostUpdatePodSandboxRequest{
		Pod: PodSandboxToV1beta1(v1a1.Pod),
	}
}

// PostUpdatePodSandboxResponse converts the reply between v1alpha1 and v1beta1.
func PostUpdatePodSandboxResponse(v1a1 *v1alpha1.Empty) *v1beta1.PostUpdatePodSandboxResponse {
	if v1a1 == nil {
		return nil
	}

	return &v1beta1.PostUpdatePodSandboxResponse{}
}

// PostUpdatePodSandboxResponseToV1alpha1 converts the reply between v1alpha1 and v1beta1.
func PostUpdatePodSandboxResponseToV1alpha1(v1b1 *v1beta1.PostUpdatePodSandboxResponse) *v1alpha1.Empty {
	if v1b1 == nil {
		return nil
	}

	return &v1alpha1.Empty{}
}

// StopPodSandboxRequest converts the request between v1alpha1 and v1beta1.
func StopPodSandboxRequest(v1b1 *v1beta1.StopPodSandboxRequest) *v1alpha1.StateChangeEvent {
	if v1b1 == nil {
		return nil
	}

	return &v1alpha1.StateChangeEvent{
		Event: v1alpha1.Event_STOP_POD_SANDBOX,
		Pod:   PodSandboxToV1alpha1(v1b1.Pod),
	}
}

// StopPodSandboxRequestToV1beta1 converts the request between v1alpha1 and v1beta1.
func StopPodSandboxRequestToV1beta1(v1a1 *v1alpha1.StateChangeEvent) *v1beta1.StopPodSandboxRequest {
	if v1a1 == nil {
		return nil
	}

	return &v1beta1.StopPodSandboxRequest{
		Pod: PodSandboxToV1beta1(v1a1.Pod),
	}
}

// StopPodSandboxResponse converts the reply between v1alpha1 and v1beta1.
func StopPodSandboxResponse(v1a1 *v1alpha1.Empty) *v1beta1.StopPodSandboxResponse {
	if v1a1 == nil {
		return nil
	}

	return &v1beta1.StopPodSandboxResponse{}
}

// StopPodSandboxResponseToV1alpha1 converts the reply between v1alpha1 and v1beta1.
func StopPodSandboxResponseToV1alpha1(v1b1 *v1beta1.StopPodSandboxResponse) *v1alpha1.Empty {
	if v1b1 == nil {
		return nil
	}

	return &v1alpha1.Empty{}
}

// RemovePodSandboxRequest converts the request between v1alpha1 and v1beta1.
func RemovePodSandboxRequest(v1b1 *v1beta1.RemovePodSandboxRequest) *v1alpha1.StateChangeEvent {
	if v1b1 == nil {
		return nil
	}

	return &v1alpha1.StateChangeEvent{
		Event: v1alpha1.Event_REMOVE_POD_SANDBOX,
		Pod:   PodSandboxToV1alpha1(v1b1.Pod),
	}
}

// RemovePodSandboxRequestToV1beta1 converts the request between v1alpha1 and v1beta1.
func RemovePodSandboxRequestToV1beta1(v1a1 *v1alpha1.StateChangeEvent) *v1beta1.RemovePodSandboxRequest {
	if v1a1 == nil {
		return nil
	}

	return &v1beta1.RemovePodSandboxRequest{
		Pod: PodSandboxToV1beta1(v1a1.Pod),
	}
}

// RemovePodSandboxResponse converts the reply between v1alpha1 and v1beta1.
func RemovePodSandboxResponse(v1a1 *v1alpha1.Empty) *v1beta1.RemovePodSandboxResponse {
	if v1a1 == nil {
		return nil
	}

	return &v1beta1.RemovePodSandboxResponse{}
}

// RemovePodSandboxResponseToV1alpha1 converts the reply between v1alpha1 and v1beta1.
func RemovePodSandboxResponseToV1alpha1(v1b1 *v1beta1.RemovePodSandboxResponse) *v1alpha1.Empty {
	if v1b1 == nil {
		return nil
	}

	return &v1alpha1.Empty{}
}

// CreateContainerRequest converts the request between v1alpha1 and v1beta1.
func CreateContainerRequest(v1b1 *v1beta1.CreateContainerRequest) *v1alpha1.CreateContainerRequest {
	if v1b1 == nil {
		return nil
	}

	return &v1alpha1.CreateContainerRequest{
		Pod:       PodSandboxToV1alpha1(v1b1.Pod),
		Container: ContainerToV1alpha1(v1b1.Container),
	}
}

// CreateContainerRequestToV1beta1 converts the request between v1alpha1 and v1beta1.
func CreateContainerRequestToV1beta1(v1a1 *v1alpha1.CreateContainerRequest) *v1beta1.CreateContainerRequest {
	if v1a1 == nil {
		return nil
	}

	return &v1beta1.CreateContainerRequest{
		Pod:       PodSandboxToV1beta1(v1a1.Pod),
		Container: ContainerToV1beta1(v1a1.Container),
	}
}

// CreateContainerResponse converts the reply between v1alpha1 and v1beta1.
func CreateContainerResponse(v1a1 *v1alpha1.CreateContainerResponse) *v1beta1.CreateContainerResponse {
	if v1a1 == nil {
		return nil
	}

	return &v1beta1.CreateContainerResponse{
		Adjust: ContainerAdjustmentToV1beta1(v1a1.Adjust),
		Update: ContainerUpdateSliceToV1beta1(v1a1.Update),
		Evict:  ContainerEvictionSliceToV1beta1(v1a1.Evict),
	}
}

// CreateContainerResponseToV1alpha1 converts the reply between v1alpha1 and v1beta1.
func CreateContainerResponseToV1alpha1(v1b1 *v1beta1.CreateContainerResponse) *v1alpha1.CreateContainerResponse {
	if v1b1 == nil {
		return nil
	}

	return &v1alpha1.CreateContainerResponse{
		Adjust: ContainerAdjustmentToV1alpha1(v1b1.Adjust),
		Update: ContainerUpdateSliceToV1alpha1(v1b1.Update),
		Evict:  ContainerEvictionSliceToV1alpha1(v1b1.Evict),
	}
}

// PostCreateContainerRequest converts the request between v1alpha1 and v1beta1.
func PostCreateContainerRequest(v1b1 *v1beta1.PostCreateContainerRequest) *v1alpha1.StateChangeEvent {
	if v1b1 == nil {
		return nil
	}

	return &v1alpha1.StateChangeEvent{
		Event:     v1alpha1.Event_POST_CREATE_CONTAINER,
		Pod:       PodSandboxToV1alpha1(v1b1.Pod),
		Container: ContainerToV1alpha1(v1b1.Container),
	}
}

// PostCreateContainerRequestToV1beta1 converts the request between v1alpha1 and v1beta1.
func PostCreateContainerRequestToV1beta1(v1a1 *v1alpha1.StateChangeEvent) *v1beta1.PostCreateContainerRequest {
	if v1a1 == nil {
		return nil
	}

	return &v1beta1.PostCreateContainerRequest{
		Pod:       PodSandboxToV1beta1(v1a1.Pod),
		Container: ContainerToV1beta1(v1a1.Container),
	}
}

// PostCreateContainerResponse converts the reply between v1alpha1 and v1beta1.
func PostCreateContainerResponse(v1a1 *v1alpha1.Empty) *v1beta1.PostCreateContainerResponse {
	if v1a1 == nil {
		return nil
	}

	return &v1beta1.PostCreateContainerResponse{}
}

// PostCreateContainerResponseToV1alpha1 converts the reply between v1alpha1 and v1beta1.
func PostCreateContainerResponseToV1alpha1(v1b1 *v1beta1.PostCreateContainerResponse) *v1alpha1.Empty {
	if v1b1 == nil {
		return nil
	}

	return &v1alpha1.Empty{}
}

// StartContainerRequest converts the request between v1alpha1 and v1beta1.
func StartContainerRequest(v1b1 *v1beta1.StartContainerRequest) *v1alpha1.StateChangeEvent {
	if v1b1 == nil {
		return nil
	}

	return &v1alpha1.StateChangeEvent{
		Event:     v1alpha1.Event_START_CONTAINER,
		Pod:       PodSandboxToV1alpha1(v1b1.Pod),
		Container: ContainerToV1alpha1(v1b1.Container),
	}
}

// StartContainerRequestToV1beta1 converts the request between v1alpha1 and v1beta1.
func StartContainerRequestToV1beta1(v1a1 *v1alpha1.StateChangeEvent) *v1beta1.StartContainerRequest {
	if v1a1 == nil {
		return nil
	}

	return &v1beta1.StartContainerRequest{
		Pod:       PodSandboxToV1beta1(v1a1.Pod),
		Container: ContainerToV1beta1(v1a1.Container),
	}
}

// StartContainerResponse converts the reply between v1alpha1 and v1beta1.
func StartContainerResponse(v1a1 *v1alpha1.Empty) *v1beta1.StartContainerResponse {
	if v1a1 == nil {
		return nil
	}

	return &v1beta1.StartContainerResponse{}
}

// StartContainerResponseToV1alpha1 converts the reply between v1alpha1 and v1beta1.
func StartContainerResponseToV1alpha1(v1b1 *v1beta1.StartContainerResponse) *v1alpha1.Empty {
	if v1b1 == nil {
		return nil
	}

	return &v1alpha1.Empty{}
}

// PostStartContainerRequest converts the request between v1alpha1 and v1beta1.
func PostStartContainerRequest(v1b1 *v1beta1.PostStartContainerRequest) *v1alpha1.StateChangeEvent {
	if v1b1 == nil {
		return nil
	}

	return &v1alpha1.StateChangeEvent{
		Event:     v1alpha1.Event_POST_START_CONTAINER,
		Pod:       PodSandboxToV1alpha1(v1b1.Pod),
		Container: ContainerToV1alpha1(v1b1.Container),
	}
}

// PostStartContainerRequestToV1beta1 converts the request between v1alpha1 and v1beta1.
func PostStartContainerRequestToV1beta1(v1a1 *v1alpha1.StateChangeEvent) *v1beta1.PostStartContainerRequest {
	if v1a1 == nil {
		return nil
	}

	return &v1beta1.PostStartContainerRequest{
		Pod:       PodSandboxToV1beta1(v1a1.Pod),
		Container: ContainerToV1beta1(v1a1.Container),
	}
}

// PostStartContainerResponse converts the reply between v1alpha1 and v1beta1.
func PostStartContainerResponse(v1a1 *v1alpha1.Empty) *v1beta1.PostStartContainerResponse {
	if v1a1 == nil {
		return nil
	}

	return &v1beta1.PostStartContainerResponse{}
}

// PostStartContainerResponseToV1alpha1 converts the reply between v1alpha1 and v1beta1.
func PostStartContainerResponseToV1alpha1(v1b1 *v1beta1.PostStartContainerResponse) *v1alpha1.Empty {
	if v1b1 == nil {
		return nil
	}

	return &v1alpha1.Empty{}
}

// UpdateContainerRequest converts the request between v1alpha1 and v1beta1.
func UpdateContainerRequest(v1b1 *v1beta1.UpdateContainerRequest) *v1alpha1.UpdateContainerRequest {
	if v1b1 == nil {
		return nil
	}

	return &v1alpha1.UpdateContainerRequest{
		Pod:            PodSandboxToV1alpha1(v1b1.Pod),
		Container:      ContainerToV1alpha1(v1b1.Container),
		LinuxResources: LinuxResourcesToV1alpha1(v1b1.LinuxResources),
	}
}

// UpdateContainerRequestToV1beta1 converts the request between v1alpha1 and v1beta1.
func UpdateContainerRequestToV1beta1(v1a1 *v1alpha1.UpdateContainerRequest) *v1beta1.UpdateContainerRequest {
	if v1a1 == nil {
		return nil
	}

	return &v1beta1.UpdateContainerRequest{
		Pod:            PodSandboxToV1beta1(v1a1.Pod),
		Container:      ContainerToV1beta1(v1a1.Container),
		LinuxResources: LinuxResourcesToV1beta1(v1a1.LinuxResources),
	}
}

// UpdateContainerResponse converts the reply between v1alpha1 and v1beta1.
func UpdateContainerResponse(v1a1 *v1alpha1.UpdateContainerResponse) *v1beta1.UpdateContainerResponse {
	if v1a1 == nil {
		return nil
	}

	return &v1beta1.UpdateContainerResponse{
		Update: ContainerUpdateSliceToV1beta1(v1a1.Update),
		Evict:  ContainerEvictionSliceToV1beta1(v1a1.Evict),
	}
}

// UpdateContainerResponseToV1alpha1 converts the reply between v1alpha1 and v1beta1.
func UpdateContainerResponseToV1alpha1(v1b1 *v1beta1.UpdateContainerResponse) *v1alpha1.UpdateContainerResponse {
	if v1b1 == nil {
		return nil
	}

	return &v1alpha1.UpdateContainerResponse{
		Update: ContainerUpdateSliceToV1alpha1(v1b1.Update),
		Evict:  ContainerEvictionSliceToV1alpha1(v1b1.Evict),
	}
}

// PostUpdateContainerRequest converts the request between v1alpha1 and v1beta1.
func PostUpdateContainerRequest(v1b1 *v1beta1.PostUpdateContainerRequest) *v1alpha1.StateChangeEvent {
	if v1b1 == nil {
		return nil
	}

	return &v1alpha1.StateChangeEvent{
		Event:     v1alpha1.Event_POST_UPDATE_CONTAINER,
		Pod:       PodSandboxToV1alpha1(v1b1.Pod),
		Container: ContainerToV1alpha1(v1b1.Container),
	}
}

// PostUpdateContainerRequestToV1beta1 converts the request between v1alpha1 and v1beta1.
func PostUpdateContainerRequestToV1beta1(v1a1 *v1alpha1.StateChangeEvent) *v1beta1.PostUpdateContainerRequest {
	if v1a1 == nil {
		return nil
	}

	return &v1beta1.PostUpdateContainerRequest{
		Pod:       PodSandboxToV1beta1(v1a1.Pod),
		Container: ContainerToV1beta1(v1a1.Container),
	}
}

// PostUpdateContainerResponse converts the reply between v1alpha1 and v1beta1.
func PostUpdateContainerResponse(v1a1 *v1alpha1.Empty) *v1beta1.PostUpdateContainerResponse {
	if v1a1 == nil {
		return nil
	}

	return &v1beta1.PostUpdateContainerResponse{}
}

// PostUpdateContainerResponseToV1alpha1 converts the reply between v1alpha1 and v1beta1.
func PostUpdateContainerResponseToV1alpha1(v1b1 *v1beta1.PostUpdateContainerResponse) *v1alpha1.Empty {
	if v1b1 == nil {
		return nil
	}

	return &v1alpha1.Empty{}
}

// StopContainerRequest converts the request between v1alpha1 and v1beta1.
func StopContainerRequest(v1b1 *v1beta1.StopContainerRequest) *v1alpha1.StopContainerRequest {
	if v1b1 == nil {
		return nil
	}

	return &v1alpha1.StopContainerRequest{
		Pod:       PodSandboxToV1alpha1(v1b1.Pod),
		Container: ContainerToV1alpha1(v1b1.Container),
	}
}

// StopContainerRequestToV1beta1 converts the request between v1alpha1 and v1beta1.
func StopContainerRequestToV1beta1(v1a1 *v1alpha1.StopContainerRequest) *v1beta1.StopContainerRequest {
	if v1a1 == nil {
		return nil
	}

	return &v1beta1.StopContainerRequest{
		Pod:       PodSandboxToV1beta1(v1a1.Pod),
		Container: ContainerToV1beta1(v1a1.Container),
	}
}

// StopContainerResponse converts the reply between v1alpha1 and v1beta1.
func StopContainerResponse(v1a1 *v1alpha1.StopContainerResponse) *v1beta1.StopContainerResponse {
	if v1a1 == nil {
		return nil
	}

	return &v1beta1.StopContainerResponse{
		Update: ContainerUpdateSliceToV1beta1(v1a1.Update),
	}
}

// StopContainerResponseToV1alpha1 converts the reply between v1alpha1 and v1beta1.
func StopContainerResponseToV1alpha1(v1b1 *v1beta1.StopContainerResponse) *v1alpha1.StopContainerResponse {
	if v1b1 == nil {
		return nil
	}

	return &v1alpha1.StopContainerResponse{
		Update: ContainerUpdateSliceToV1alpha1(v1b1.Update),
	}
}

// RemoveContainerRequest converts the request between v1alpha1 and v1beta1.
func RemoveContainerRequest(v1b1 *v1beta1.RemoveContainerRequest) *v1alpha1.StateChangeEvent {
	if v1b1 == nil {
		return nil
	}

	return &v1alpha1.StateChangeEvent{
		Event:     v1alpha1.Event_REMOVE_CONTAINER,
		Pod:       PodSandboxToV1alpha1(v1b1.Pod),
		Container: ContainerToV1alpha1(v1b1.Container),
	}
}

// RemoveContainerRequestToV1beta1 converts the request between v1alpha1 and v1beta1.
func RemoveContainerRequestToV1beta1(v1a1 *v1alpha1.StateChangeEvent) *v1beta1.RemoveContainerRequest {
	if v1a1 == nil {
		return nil
	}

	return &v1beta1.RemoveContainerRequest{
		Pod:       PodSandboxToV1beta1(v1a1.Pod),
		Container: ContainerToV1beta1(v1a1.Container),
	}
}

// RemoveContainerResponse converts the reply between v1alpha1 and v1beta1.
func RemoveContainerResponse(v1a1 *v1alpha1.Empty) *v1beta1.RemoveContainerResponse {
	if v1a1 == nil {
		return nil
	}

	return &v1beta1.RemoveContainerResponse{}
}

// RemoveContainerResponseToV1alpha1 converts the reply between v1alpha1 and v1beta1.
func RemoveContainerResponseToV1alpha1(v1b1 *v1beta1.RemoveContainerResponse) *v1alpha1.Empty {
	if v1b1 == nil {
		return nil
	}

	return &v1alpha1.Empty{}
}

// ValidateContainerAdjustmentRequest converts the request between v1alpha1 and v1beta1.
func ValidateContainerAdjustmentRequest(v1b1 *v1beta1.ValidateContainerAdjustmentRequest) *v1alpha1.ValidateContainerAdjustmentRequest {
	if v1b1 == nil {
		return nil
	}

	return &v1alpha1.ValidateContainerAdjustmentRequest{
		Pod:       PodSandboxToV1alpha1(v1b1.Pod),
		Container: ContainerToV1alpha1(v1b1.Container),
		Adjust:    ContainerAdjustmentToV1alpha1(v1b1.Adjust),
		Update:    ContainerUpdateSliceToV1alpha1(v1b1.Update),
		Owners:    OwningPluginsToV1alpha1(v1b1.Owners),
		Plugins:   PluginInstanceSliceToV1alpha1(v1b1.Plugins),
	}
}

// ValidateContainerAdjustmentRequestToV1beta1 converts the request between v1alpha1 and v1beta1.
func ValidateContainerAdjustmentRequestToV1beta1(v1a1 *v1alpha1.ValidateContainerAdjustmentRequest) *v1beta1.ValidateContainerAdjustmentRequest {
	if v1a1 == nil {
		return nil
	}

	return &v1beta1.ValidateContainerAdjustmentRequest{
		Pod:       PodSandboxToV1beta1(v1a1.Pod),
		Container: ContainerToV1beta1(v1a1.Container),
		Adjust:    ContainerAdjustmentToV1beta1(v1a1.Adjust),
		Update:    ContainerUpdateSliceToV1beta1(v1a1.Update),
		Owners:    OwningPluginsToV1beta1(v1a1.Owners),
		Plugins:   PluginInstanceSliceToV1beta1(v1a1.Plugins),
	}
}

// ValidateContainerAdjustmentResponse converts the reply between v1alpha1 and v1beta1.
func ValidateContainerAdjustmentResponse(v1a1 *v1alpha1.ValidateContainerAdjustmentResponse) *v1beta1.ValidateContainerAdjustmentResponse {
	if v1a1 == nil {
		return nil
	}

	return &v1beta1.ValidateContainerAdjustmentResponse{
		Reject: v1a1.Reject,
		Reason: v1a1.Reason,
	}
}

// ValidateContainerAdjustmentResponseToV1alpha1 converts the reply between v1alpha1 and v1beta1.
func ValidateContainerAdjustmentResponseToV1alpha1(v1b1 *v1beta1.ValidateContainerAdjustmentResponse) *v1alpha1.ValidateContainerAdjustmentResponse {
	if v1b1 == nil {
		return nil
	}

	return &v1alpha1.ValidateContainerAdjustmentResponse{
		Reject: v1b1.Reject,
		Reason: v1b1.Reason,
	}
}
