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

// PodSandboxToV1alpha1 converts the type between v1alpha1 and v1beta1.
func PodSandboxToV1alpha1(v1b1 *v1beta1.PodSandbox) *v1alpha1.PodSandbox {
	if v1b1 == nil {
		return nil
	}

	v1a1 := &v1alpha1.PodSandbox{
		Id:             v1b1.Id,
		Name:           v1b1.Name,
		Uid:            v1b1.Uid,
		Namespace:      v1b1.Namespace,
		Labels:         v1b1.Labels,
		Annotations:    v1b1.Annotations,
		RuntimeHandler: v1b1.RuntimeHandler,
		Linux:          LinuxPodSandboxToV1alpha1(v1b1.Linux),
		Pid:            v1b1.Pid,
		Ips:            v1b1.Ips,
	}

	return v1a1
}

// PodSandboxToV1beta1 converts the type between v1alpha1 and v1beta1.
func PodSandboxToV1beta1(v1a1 *v1alpha1.PodSandbox) *v1beta1.PodSandbox {
	if v1a1 == nil {
		return nil
	}

	v1b1 := &v1beta1.PodSandbox{
		Id:             v1a1.Id,
		Name:           v1a1.Name,
		Uid:            v1a1.Uid,
		Namespace:      v1a1.Namespace,
		Labels:         v1a1.Labels,
		Annotations:    v1a1.Annotations,
		RuntimeHandler: v1a1.RuntimeHandler,
		Linux:          LinuxPodSandboxToV1beta1(v1a1.Linux),
		Pid:            v1a1.Pid,
		Ips:            v1a1.Ips,
	}

	return v1b1
}

// PodSandboxSliceToV1alpha1 converts the types between v1alpha1 and v1beta1.
func PodSandboxSliceToV1alpha1(v1b1 []*v1beta1.PodSandbox) []*v1alpha1.PodSandbox {
	if v1b1 == nil {
		return nil
	}

	v1a1 := make([]*v1alpha1.PodSandbox, 0, len(v1b1))
	for _, p := range v1b1 {
		v1a1 = append(v1a1, PodSandboxToV1alpha1(p))
	}

	return v1a1
}

// PodSandboxSliceToV1beta1 converts the types between v1alpha1 and v1beta1.
func PodSandboxSliceToV1beta1(v1a1 []*v1alpha1.PodSandbox) []*v1beta1.PodSandbox {
	if v1a1 == nil {
		return nil
	}

	v1b1 := make([]*v1beta1.PodSandbox, 0, len(v1a1))
	for _, p := range v1a1 {
		v1b1 = append(v1b1, PodSandboxToV1beta1(p))
	}

	return v1b1
}

// LinuxPodSandboxToV1alpha1 converts the type between v1alpha1 and v1beta1.
func LinuxPodSandboxToV1alpha1(v1b1 *v1beta1.LinuxPodSandbox) *v1alpha1.LinuxPodSandbox {
	if v1b1 == nil {
		return nil
	}

	v1a1 := &v1alpha1.LinuxPodSandbox{
		PodOverhead:  LinuxResourcesToV1alpha1(v1b1.PodOverhead),
		PodResources: LinuxResourcesToV1alpha1(v1b1.PodResources),
		CgroupParent: v1b1.CgroupParent,
		CgroupsPath:  v1b1.CgroupsPath,
		Namespaces:   LinuxNamespaceSliceToV1alpha1(v1b1.Namespaces),
		Resources:    LinuxResourcesToV1alpha1(v1b1.Resources),
	}

	return v1a1
}

// LinuxPodSandboxToV1beta1 converts the type between v1alpha1 and v1beta1.
func LinuxPodSandboxToV1beta1(v1a1 *v1alpha1.LinuxPodSandbox) *v1beta1.LinuxPodSandbox {
	if v1a1 == nil {
		return nil
	}

	v1b1 := &v1beta1.LinuxPodSandbox{
		PodOverhead:  LinuxResourcesToV1beta1(v1a1.PodOverhead),
		PodResources: LinuxResourcesToV1beta1(v1a1.PodResources),
		CgroupParent: v1a1.CgroupParent,
		CgroupsPath:  v1a1.CgroupsPath,
		Namespaces:   LinuxNamespaceSliceToV1beta1(v1a1.Namespaces),
		Resources:    LinuxResourcesToV1beta1(v1a1.Resources),
	}

	return v1b1
}

// ContainerToV1alpha1 converts the type between v1alpha1 and v1beta1.
func ContainerToV1alpha1(v1b1 *v1beta1.Container) *v1alpha1.Container {
	if v1b1 == nil {
		return nil
	}

	v1a1 := &v1alpha1.Container{
		Id:            v1b1.Id,
		PodSandboxId:  v1b1.PodSandboxId,
		Name:          v1b1.Name,
		State:         v1alpha1.ContainerState(v1b1.State),
		Labels:        v1b1.Labels,
		Annotations:   v1b1.Annotations,
		Args:          v1b1.Args,
		Env:           v1b1.Env,
		Mounts:        MountSliceToV1alpha1(v1b1.Mounts),
		Hooks:         HooksToV1alpha1(v1b1.Hooks),
		Linux:         LinuxContainerToV1alpha1(v1b1.Linux),
		Pid:           v1b1.Pid,
		Rlimits:       POSIXRlimitsSliceToV1alpha1(v1b1.Rlimits),
		CreatedAt:     v1b1.CreatedAt,
		StartedAt:     v1b1.StartedAt,
		FinishedAt:    v1b1.FinishedAt,
		ExitCode:      v1b1.ExitCode,
		StatusReason:  v1b1.StatusReason,
		StatusMessage: v1b1.StatusMessage,
		CDIDevices:    CDIDeviceSliceToV1alpha1(v1b1.CDIDevices),
		User:          UserToV1alpha1(v1b1.User),
	}

	return v1a1
}

// ContainerToV1beta1 converts the type between v1alpha1 and v1beta1.
func ContainerToV1beta1(v1a1 *v1alpha1.Container) *v1beta1.Container {
	if v1a1 == nil {
		return nil
	}

	v1b1 := &v1beta1.Container{
		Id:            v1a1.Id,
		PodSandboxId:  v1a1.PodSandboxId,
		Name:          v1a1.Name,
		State:         v1beta1.ContainerState(v1a1.State),
		Labels:        v1a1.Labels,
		Annotations:   v1a1.Annotations,
		Args:          v1a1.Args,
		Env:           v1a1.Env,
		Mounts:        MountSliceToV1beta1(v1a1.Mounts),
		Hooks:         HooksToV1beta1(v1a1.Hooks),
		Linux:         LinuxContainerToV1beta1(v1a1.Linux),
		Pid:           v1a1.Pid,
		Rlimits:       POSIXRlimitsSliceToV1beta1(v1a1.Rlimits),
		CreatedAt:     v1a1.CreatedAt,
		StartedAt:     v1a1.StartedAt,
		FinishedAt:    v1a1.FinishedAt,
		ExitCode:      v1a1.ExitCode,
		StatusReason:  v1a1.StatusReason,
		StatusMessage: v1a1.StatusMessage,
		CDIDevices:    CDIDeviceSliceToV1beta1(v1a1.CDIDevices),
		User:          UserToV1beta1(v1a1.User),
	}

	return v1b1
}

// ContainerSliceToV1alpha1 converts the types between v1alpha1 and v1beta1.
func ContainerSliceToV1alpha1(v1b1 []*v1beta1.Container) []*v1alpha1.Container {
	if v1b1 == nil {
		return nil
	}

	v1a1 := make([]*v1alpha1.Container, 0, len(v1b1))
	for _, c := range v1b1 {
		v1a1 = append(v1a1, ContainerToV1alpha1(c))
	}

	return v1a1
}

// ContainerSliceToV1beta1 converts the types between v1alpha1 and v1beta1.
func ContainerSliceToV1beta1(v1a1 []*v1alpha1.Container) []*v1beta1.Container {
	if v1a1 == nil {
		return nil
	}

	v1b1 := make([]*v1beta1.Container, 0, len(v1a1))
	for _, c := range v1a1 {
		v1b1 = append(v1b1, ContainerToV1beta1(c))
	}

	return v1b1
}

// MountSliceToV1alpha1 converts the request between v1alpha1 and v1beta1.
func MountSliceToV1alpha1(v1b1 []*v1beta1.Mount) []*v1alpha1.Mount {
	if v1b1 == nil {
		return nil
	}

	v1a1 := make([]*v1alpha1.Mount, 0, len(v1b1))
	for _, m := range v1b1 {
		if m != nil {
			v1a1 = append(v1a1, &v1alpha1.Mount{
				Source:      m.Source,
				Destination: m.Destination,
				Type:        m.Type,
				Options:     m.Options,
			})
		}
	}

	return v1a1
}

// MountSliceToV1beta1 converts the request between v1alpha1 and v1beta1.
func MountSliceToV1beta1(v1a1 []*v1alpha1.Mount) []*v1beta1.Mount {
	if v1a1 == nil {
		return nil
	}

	v1b1 := make([]*v1beta1.Mount, 0, len(v1a1))
	for _, m := range v1a1 {
		if m != nil {
			v1b1 = append(v1b1, &v1beta1.Mount{
				Source:      m.Source,
				Destination: m.Destination,
				Type:        m.Type,
				Options:     m.Options,
			})
		}
	}

	return v1b1
}

// HooksToV1alpha1 converts the request between v1alpha1 and v1beta1.
func HooksToV1alpha1(v1b1 *v1beta1.Hooks) *v1alpha1.Hooks {
	if v1b1 == nil {
		return nil
	}

	v1a1 := &v1alpha1.Hooks{
		Prestart:        HookSliceToV1alpha1(v1b1.Prestart),
		CreateRuntime:   HookSliceToV1alpha1(v1b1.CreateRuntime),
		CreateContainer: HookSliceToV1alpha1(v1b1.CreateContainer),
		StartContainer:  HookSliceToV1alpha1(v1b1.StartContainer),
		Poststart:       HookSliceToV1alpha1(v1b1.Poststart),
		Poststop:        HookSliceToV1alpha1(v1b1.Poststop),
	}

	return v1a1
}

// HooksToV1beta1 converts the request between v1alpha1 and v1beta1.
func HooksToV1beta1(v1a1 *v1alpha1.Hooks) *v1beta1.Hooks {
	if v1a1 == nil {
		return nil
	}

	v1b1 := &v1beta1.Hooks{
		Prestart:        HookSliceToV1beta1(v1a1.Prestart),
		CreateRuntime:   HookSliceToV1beta1(v1a1.CreateRuntime),
		CreateContainer: HookSliceToV1beta1(v1a1.CreateContainer),
		StartContainer:  HookSliceToV1beta1(v1a1.StartContainer),
		Poststart:       HookSliceToV1beta1(v1a1.Poststart),
		Poststop:        HookSliceToV1beta1(v1a1.Poststop),
	}

	return v1b1
}

// HookSliceToV1alpha1 converts the request between v1alpha1 and v1beta1.
func HookSliceToV1alpha1(v1b1 []*v1beta1.Hook) []*v1alpha1.Hook {
	if v1b1 == nil {
		return nil
	}

	v1a1 := make([]*v1alpha1.Hook, 0, len(v1b1))
	for _, h := range v1b1 {
		if h != nil {
			v1a1 = append(v1a1, &v1alpha1.Hook{
				Path:    h.Path,
				Args:    h.Args,
				Env:     h.Env,
				Timeout: OptionalIntToV1alpha1(h.Timeout),
			})
		}
	}

	return v1a1
}

// HookSliceToV1beta1 converts the request between v1alpha1 and v1beta1.
func HookSliceToV1beta1(v1a1 []*v1alpha1.Hook) []*v1beta1.Hook {
	if v1a1 == nil {
		return nil
	}

	v1b1 := make([]*v1beta1.Hook, 0, len(v1a1))
	for _, h := range v1a1 {
		if h != nil {
			v1b1 = append(v1b1, &v1beta1.Hook{
				Path:    h.Path,
				Args:    h.Args,
				Env:     h.Env,
				Timeout: OptionalIntToV1beta1(h.Timeout),
			})
		}
	}

	return v1b1
}

// LinuxContainerToV1alpha1 converts the request between v1alpha1 and v1beta1.
func LinuxContainerToV1alpha1(v1b1 *v1beta1.LinuxContainer) *v1alpha1.LinuxContainer {
	if v1b1 == nil {
		return nil
	}

	v1a1 := &v1alpha1.LinuxContainer{
		Namespaces:     LinuxNamespaceSliceToV1alpha1(v1b1.Namespaces),
		Devices:        LinuxDeviceSliceToV1alpha1(v1b1.Devices),
		Resources:      LinuxResourcesToV1alpha1(v1b1.Resources),
		OomScoreAdj:    OptionalIntToV1alpha1(v1b1.OomScoreAdj),
		CgroupsPath:    v1b1.CgroupsPath,
		IoPriority:     LinuxIOPriorityToV1alpha1(v1b1.IoPriority),
		SeccompProfile: SecurityProfileToV1alpha1(v1b1.SeccompProfile),
		SeccompPolicy:  LinuxSeccompToV1alpha1(v1b1.SeccompPolicy),
	}

	return v1a1
}

// LinuxContainerToV1beta1 converts the request between v1alpha1 and v1beta1.
func LinuxContainerToV1beta1(v1a1 *v1alpha1.LinuxContainer) *v1beta1.LinuxContainer {
	if v1a1 == nil {
		return nil
	}

	v1b1 := &v1beta1.LinuxContainer{
		Namespaces:     LinuxNamespaceSliceToV1beta1(v1a1.Namespaces),
		Devices:        LinuxDeviceSliceToV1beta1(v1a1.Devices),
		Resources:      LinuxResourcesToV1beta1(v1a1.Resources),
		OomScoreAdj:    OptionalIntToV1beta1(v1a1.OomScoreAdj),
		CgroupsPath:    v1a1.CgroupsPath,
		IoPriority:     LinuxIOPriorityToV1beta1(v1a1.IoPriority),
		SeccompProfile: SecurityProfileToV1beta1(v1a1.SeccompProfile),
		SeccompPolicy:  LinuxSeccompToV1beta1(v1a1.SeccompPolicy),
	}

	return v1b1
}

// LinuxResourcesToV1alpha1 converts the request between v1alpha1 and v1beta1.
func LinuxResourcesToV1alpha1(v1b1 *v1beta1.LinuxResources) *v1alpha1.LinuxResources {
	if v1b1 == nil {
		return nil
	}

	v1a1 := &v1alpha1.LinuxResources{
		Memory:         LinuxMemoryToV1alpha1(v1b1.Memory),
		Cpu:            LinuxCPUToV1alpha1(v1b1.Cpu),
		HugepageLimits: HugepageLimitSliceToV1alpha1(v1b1.HugepageLimits),
		BlockioClass:   OptionalStringToV1alpha1(v1b1.BlockioClass),
		RdtClass:       OptionalStringToV1alpha1(v1b1.RdtClass),
		Unified:        v1b1.Unified,
		Devices:        LinuxDeviceCgroupSliceToV1alpha1(v1b1.Devices),
		Pids:           LinuxPidsToV1alpha1(v1b1.Pids),
	}

	return v1a1
}

// LinuxResourcesToV1beta1 converts the request between v1alpha1 and v1beta1.
func LinuxResourcesToV1beta1(v1a1 *v1alpha1.LinuxResources) *v1beta1.LinuxResources {
	if v1a1 == nil {
		return nil
	}

	v1b1 := &v1beta1.LinuxResources{
		Memory:         LinuxMemoryToV1beta1(v1a1.Memory),
		Cpu:            LinuxCPUToV1beta1(v1a1.Cpu),
		HugepageLimits: HugepageLimitSliceToV1beta1(v1a1.HugepageLimits),
		BlockioClass:   OptionalStringToV1beta1(v1a1.BlockioClass),
		RdtClass:       OptionalStringToV1beta1(v1a1.RdtClass),
		Unified:        v1a1.Unified,
		Devices:        LinuxDeviceCgroupSliceToV1beta1(v1a1.Devices),
		Pids:           LinuxPidsToV1beta1(v1a1.Pids),
	}

	return v1b1
}

// LinuxNamespaceSliceToV1alpha1 converts the request between v1alpha1 and v1beta1.
func LinuxNamespaceSliceToV1alpha1(v1b1 []*v1beta1.LinuxNamespace) []*v1alpha1.LinuxNamespace {
	if v1b1 == nil {
		return nil
	}

	v1a1 := make([]*v1alpha1.LinuxNamespace, 0, len(v1b1))
	for _, ns := range v1b1 {
		if ns != nil {
			v1a1 = append(v1a1, &v1alpha1.LinuxNamespace{
				Type: ns.Type,
				Path: ns.Path,
			})
		}
	}

	return v1a1
}

// LinuxNamespaceSliceToV1beta1 converts the request between v1alpha1 and v1beta1.
func LinuxNamespaceSliceToV1beta1(v1a1 []*v1alpha1.LinuxNamespace) []*v1beta1.LinuxNamespace {
	if v1a1 == nil {
		return nil
	}

	v1b1 := make([]*v1beta1.LinuxNamespace, 0, len(v1a1))
	for _, ns := range v1a1 {
		if ns != nil {
			v1b1 = append(v1b1, &v1beta1.LinuxNamespace{
				Type: ns.Type,
				Path: ns.Path,
			})
		}
	}

	return v1b1
}

// LinuxDeviceSliceToV1alpha1 converts the request between v1alpha1 and v1beta1.
func LinuxDeviceSliceToV1alpha1(v1b1 []*v1beta1.LinuxDevice) []*v1alpha1.LinuxDevice {
	if v1b1 == nil {
		return nil
	}

	v1a1 := make([]*v1alpha1.LinuxDevice, 0, len(v1b1))
	for _, d := range v1b1 {
		if d != nil {
			v1a1 = append(v1a1, &v1alpha1.LinuxDevice{
				Path:     d.Path,
				Type:     d.Type,
				Major:    d.Major,
				Minor:    d.Minor,
				FileMode: OptionalFileModeToV1alpha1(d.FileMode),
				Uid:      OptionalUInt32ToV1alpha1(d.Uid),
				Gid:      OptionalUInt32ToV1alpha1(d.Gid),
			})
		}
	}

	return v1a1
}

// LinuxDeviceSliceToV1beta1 converts the request between v1alpha1 and v1beta1.
func LinuxDeviceSliceToV1beta1(v1a1 []*v1alpha1.LinuxDevice) []*v1beta1.LinuxDevice {
	if v1a1 == nil {
		return nil
	}

	v1b1 := make([]*v1beta1.LinuxDevice, 0, len(v1a1))
	for _, d := range v1a1 {
		if d != nil {
			v1b1 = append(v1b1, &v1beta1.LinuxDevice{
				Path:     d.Path,
				Type:     d.Type,
				Major:    d.Major,
				Minor:    d.Minor,
				FileMode: OptionalFileModeToV1beta1(d.FileMode),
				Uid:      OptionalUInt32ToV1beta1(d.Uid),
				Gid:      OptionalUInt32ToV1beta1(d.Gid),
			})
		}
	}

	return v1b1
}

// LinuxMemoryToV1alpha1 converts the request between v1alpha1 and v1beta1.
func LinuxMemoryToV1alpha1(v1b1 *v1beta1.LinuxMemory) *v1alpha1.LinuxMemory {
	if v1b1 == nil {
		return nil
	}

	v1a1 := &v1alpha1.LinuxMemory{
		Limit:            OptionalInt64ToV1alpha1(v1b1.Limit),
		Reservation:      OptionalInt64ToV1alpha1(v1b1.Reservation),
		Swap:             OptionalInt64ToV1alpha1(v1b1.Swap),
		Kernel:           OptionalInt64ToV1alpha1(v1b1.Kernel),
		KernelTcp:        OptionalInt64ToV1alpha1(v1b1.KernelTcp),
		Swappiness:       OptionalUInt64ToV1alpha1(v1b1.Swappiness),
		DisableOomKiller: OptionalBoolToV1alpha1(v1b1.DisableOomKiller),
		UseHierarchy:     OptionalBoolToV1alpha1(v1b1.UseHierarchy),
	}

	return v1a1
}

// LinuxMemoryToV1beta1 converts the request between v1alpha1 and v1beta1.
func LinuxMemoryToV1beta1(v1a1 *v1alpha1.LinuxMemory) *v1beta1.LinuxMemory {
	if v1a1 == nil {
		return nil
	}

	v1b1 := &v1beta1.LinuxMemory{
		Limit:            OptionalInt64ToV1beta1(v1a1.Limit),
		Reservation:      OptionalInt64ToV1beta1(v1a1.Reservation),
		Swap:             OptionalInt64ToV1beta1(v1a1.Swap),
		Kernel:           OptionalInt64ToV1beta1(v1a1.Kernel),
		KernelTcp:        OptionalInt64ToV1beta1(v1a1.KernelTcp),
		Swappiness:       OptionalUInt64ToV1beta1(v1a1.Swappiness),
		DisableOomKiller: OptionalBoolToV1beta1(v1a1.DisableOomKiller),
		UseHierarchy:     OptionalBoolToV1beta1(v1a1.UseHierarchy),
	}

	return v1b1
}

// LinuxCPUToV1alpha1 converts the request between v1alpha1 and v1beta1.
func LinuxCPUToV1alpha1(v1b1 *v1beta1.LinuxCPU) *v1alpha1.LinuxCPU {
	if v1b1 == nil {
		return nil
	}

	v1a1 := &v1alpha1.LinuxCPU{
		Shares:          OptionalUInt64ToV1alpha1(v1b1.Shares),
		Quota:           OptionalInt64ToV1alpha1(v1b1.Quota),
		Period:          OptionalUInt64ToV1alpha1(v1b1.Period),
		RealtimeRuntime: OptionalInt64ToV1alpha1(v1b1.RealtimeRuntime),
		RealtimePeriod:  OptionalUInt64ToV1alpha1(v1b1.RealtimePeriod),
		Cpus:            v1b1.Cpus,
		Mems:            v1b1.Mems,
	}

	return v1a1
}

// LinuxCPUToV1beta1 converts the request between v1alpha1 and v1beta1.
func LinuxCPUToV1beta1(v1a1 *v1alpha1.LinuxCPU) *v1beta1.LinuxCPU {
	if v1a1 == nil {
		return nil
	}

	v1b1 := &v1beta1.LinuxCPU{
		Shares:          OptionalUInt64ToV1beta1(v1a1.Shares),
		Quota:           OptionalInt64ToV1beta1(v1a1.Quota),
		Period:          OptionalUInt64ToV1beta1(v1a1.Period),
		RealtimeRuntime: OptionalInt64ToV1beta1(v1a1.RealtimeRuntime),
		RealtimePeriod:  OptionalUInt64ToV1beta1(v1a1.RealtimePeriod),
		Cpus:            v1a1.Cpus,
		Mems:            v1a1.Mems,
	}

	return v1b1
}

// HugepageLimitSliceToV1alpha1 converts the request between v1alpha1 and v1beta1.
func HugepageLimitSliceToV1alpha1(v1b1 []*v1beta1.HugepageLimit) []*v1alpha1.HugepageLimit {
	if v1b1 == nil {
		return nil
	}

	v1a1 := make([]*v1alpha1.HugepageLimit, 0, len(v1b1))
	for _, h := range v1b1 {
		if h != nil {
			v1a1 = append(v1a1, &v1alpha1.HugepageLimit{
				PageSize: h.PageSize,
				Limit:    h.Limit,
			})
		}
	}

	return v1a1
}

// HugepageLimitSliceToV1beta1 converts the request between v1alpha1 and v1beta1.
func HugepageLimitSliceToV1beta1(v1a1 []*v1alpha1.HugepageLimit) []*v1beta1.HugepageLimit {
	if v1a1 == nil {
		return nil
	}

	v1b1 := make([]*v1beta1.HugepageLimit, 0, len(v1a1))
	for _, h := range v1a1 {
		if h != nil {
			v1b1 = append(v1b1, &v1beta1.HugepageLimit{
				PageSize: h.PageSize,
				Limit:    h.Limit,
			})
		}
	}

	return v1b1
}

// LinuxDeviceCgroupSliceToV1alpha1 converts the request between v1alpha1 and v1beta1.
func LinuxDeviceCgroupSliceToV1alpha1(v1b1 []*v1beta1.LinuxDeviceCgroup) []*v1alpha1.LinuxDeviceCgroup {
	if v1b1 == nil {
		return nil
	}

	v1a1 := make([]*v1alpha1.LinuxDeviceCgroup, 0, len(v1b1))
	for _, d := range v1b1 {
		if d != nil {
			v1a1 = append(v1a1, &v1alpha1.LinuxDeviceCgroup{
				Allow:  d.Allow,
				Type:   d.Type,
				Major:  OptionalInt64ToV1alpha1(d.Major),
				Minor:  OptionalInt64ToV1alpha1(d.Minor),
				Access: d.Access,
			})
		}
	}

	return v1a1
}

// LinuxDeviceCgroupSliceToV1beta1 converts the request between v1alpha1 and v1beta1.
func LinuxDeviceCgroupSliceToV1beta1(v1a1 []*v1alpha1.LinuxDeviceCgroup) []*v1beta1.LinuxDeviceCgroup {
	if v1a1 == nil {
		return nil
	}

	v1b1 := make([]*v1beta1.LinuxDeviceCgroup, 0, len(v1a1))
	for _, d := range v1a1 {
		if d != nil {
			v1b1 = append(v1b1, &v1beta1.LinuxDeviceCgroup{
				Allow:  d.Allow,
				Type:   d.Type,
				Major:  OptionalInt64ToV1beta1(d.Major),
				Minor:  OptionalInt64ToV1beta1(d.Minor),
				Access: d.Access,
			})
		}
	}

	return v1b1
}

// LinuxPidsToV1alpha1 converts the request between v1alpha1 and v1beta1.
func LinuxPidsToV1alpha1(v1b1 *v1beta1.LinuxPids) *v1alpha1.LinuxPids {
	if v1b1 == nil {
		return nil
	}

	v1a1 := &v1alpha1.LinuxPids{
		Limit: v1b1.Limit,
	}

	return v1a1
}

// LinuxPidsToV1beta1 converts the request between v1alpha1 and v1beta1.
func LinuxPidsToV1beta1(v1a1 *v1alpha1.LinuxPids) *v1beta1.LinuxPids {
	if v1a1 == nil {
		return nil
	}

	v1b1 := &v1beta1.LinuxPids{
		Limit: v1a1.Limit,
	}

	return v1b1
}

// LinuxIOPriorityToV1alpha1 converts the request between v1alpha1 and v1beta1.
func LinuxIOPriorityToV1alpha1(v1b1 *v1beta1.LinuxIOPriority) *v1alpha1.LinuxIOPriority {
	if v1b1 == nil {
		return nil
	}

	v1a1 := &v1alpha1.LinuxIOPriority{
		Class:    v1alpha1.IOPrioClass(v1b1.Class),
		Priority: v1b1.Priority,
	}

	return v1a1
}

// LinuxIOPriorityToV1beta1 converts the request between v1alpha1 and v1beta1.
func LinuxIOPriorityToV1beta1(v1a1 *v1alpha1.LinuxIOPriority) *v1beta1.LinuxIOPriority {
	if v1a1 == nil {
		return nil
	}

	v1b1 := &v1beta1.LinuxIOPriority{
		Class:    v1beta1.IOPrioClass(v1a1.Class),
		Priority: v1a1.Priority,
	}

	return v1b1
}

// SecurityProfileToV1alpha1 converts the request between v1alpha1 and v1beta1.
func SecurityProfileToV1alpha1(v1b1 *v1beta1.SecurityProfile) *v1alpha1.SecurityProfile {
	if v1b1 == nil {
		return nil
	}

	v1a1 := &v1alpha1.SecurityProfile{
		ProfileType:  v1alpha1.SecurityProfile_ProfileType(v1b1.ProfileType),
		LocalhostRef: v1b1.LocalhostRef,
	}

	return v1a1
}

// SecurityProfileToV1beta1 converts the request between v1alpha1 and v1beta1.
func SecurityProfileToV1beta1(v1a1 *v1alpha1.SecurityProfile) *v1beta1.SecurityProfile {
	if v1a1 == nil {
		return nil
	}

	v1b1 := &v1beta1.SecurityProfile{
		ProfileType:  v1beta1.SecurityProfile_ProfileType(v1a1.ProfileType),
		LocalhostRef: v1a1.LocalhostRef,
	}

	return v1b1
}

// LinuxSeccompToV1alpha1 converts the request between v1alpha1 and v1beta1.
func LinuxSeccompToV1alpha1(v1b1 *v1beta1.LinuxSeccomp) *v1alpha1.LinuxSeccomp {
	if v1b1 == nil {
		return nil
	}

	v1a1 := &v1alpha1.LinuxSeccomp{
		DefaultAction:    v1b1.DefaultAction,
		DefaultErrno:     OptionalUInt32ToV1alpha1(v1b1.DefaultErrno),
		Architectures:    v1b1.Architectures,
		Flags:            v1b1.Flags,
		ListenerPath:     v1b1.ListenerPath,
		ListenerMetadata: v1b1.ListenerMetadata,
		Syscalls:         LinuxSyscallSliceToV1alpha1(v1b1.Syscalls),
	}

	return v1a1
}

// LinuxSeccompToV1beta1 converts the request between v1alpha1 and v1beta1.
func LinuxSeccompToV1beta1(v1a1 *v1alpha1.LinuxSeccomp) *v1beta1.LinuxSeccomp {
	if v1a1 == nil {
		return nil
	}

	v1b1 := &v1beta1.LinuxSeccomp{
		DefaultAction:    v1a1.DefaultAction,
		DefaultErrno:     OptionalUInt32ToV1beta1(v1a1.DefaultErrno),
		Architectures:    v1a1.Architectures,
		Flags:            v1a1.Flags,
		ListenerPath:     v1a1.ListenerPath,
		ListenerMetadata: v1a1.ListenerMetadata,
		Syscalls:         LinuxSyscallSliceToV1beta1(v1a1.Syscalls),
	}

	return v1b1
}

// LinuxSyscallSliceToV1alpha1 converts the request between v1alpha1 and v1beta1.
func LinuxSyscallSliceToV1alpha1(v1b1 []*v1beta1.LinuxSyscall) []*v1alpha1.LinuxSyscall {
	if v1b1 == nil {
		return nil
	}

	v1a1 := make([]*v1alpha1.LinuxSyscall, 0, len(v1b1))
	for _, s := range v1b1 {
		if s != nil {
			v1a1 = append(v1a1, &v1alpha1.LinuxSyscall{
				Names:    s.Names,
				Action:   s.Action,
				ErrnoRet: OptionalUInt32ToV1alpha1(s.ErrnoRet),
				Args:     LinuxSeccompArgSliceToV1alpha1(s.Args),
			})
		}
	}

	return v1a1
}

// LinuxSyscallSliceToV1beta1 converts the request between v1alpha1 and v1beta1.
func LinuxSyscallSliceToV1beta1(v1a1 []*v1alpha1.LinuxSyscall) []*v1beta1.LinuxSyscall {
	if v1a1 == nil {
		return nil
	}

	v1b1 := make([]*v1beta1.LinuxSyscall, 0, len(v1a1))
	for _, s := range v1a1 {
		if s != nil {
			v1b1 = append(v1b1, &v1beta1.LinuxSyscall{
				Names:    s.Names,
				Action:   s.Action,
				ErrnoRet: OptionalUInt32ToV1beta1(s.ErrnoRet),
				Args:     LinuxSeccompArgSliceToV1beta1(s.Args),
			})
		}
	}

	return v1b1
}

// LinuxSeccompArgSliceToV1alpha1 converts the request between v1alpha1 and v1beta1.
func LinuxSeccompArgSliceToV1alpha1(v1b1 []*v1beta1.LinuxSeccompArg) []*v1alpha1.LinuxSeccompArg {
	if v1b1 == nil {
		return nil
	}

	v1a1 := make([]*v1alpha1.LinuxSeccompArg, 0, len(v1b1))
	for _, a := range v1b1 {
		if a != nil {
			v1a1 = append(v1a1, &v1alpha1.LinuxSeccompArg{
				Index:    a.Index,
				Value:    a.Value,
				ValueTwo: a.ValueTwo,
				Op:       a.Op,
			})
		}
	}

	return v1a1
}

// LinuxSeccompArgSliceToV1beta1 converts the request between v1alpha1 and v1beta1.
func LinuxSeccompArgSliceToV1beta1(v1a1 []*v1alpha1.LinuxSeccompArg) []*v1beta1.LinuxSeccompArg {
	if v1a1 == nil {
		return nil
	}

	v1b1 := make([]*v1beta1.LinuxSeccompArg, 0, len(v1a1))
	for _, a := range v1a1 {
		if a != nil {
			v1b1 = append(v1b1, &v1beta1.LinuxSeccompArg{
				Index:    a.Index,
				Value:    a.Value,
				ValueTwo: a.ValueTwo,
				Op:       a.Op,
			})
		}
	}

	return v1b1
}

// OptionalIntToV1alpha1 converts the request between v1alpha1 and v1beta1.
func OptionalIntToV1alpha1(v1b1 *v1beta1.OptionalInt) *v1alpha1.OptionalInt {
	if v1b1 == nil {
		return nil
	}

	v1a1 := &v1alpha1.OptionalInt{
		Value: v1b1.Value,
	}

	return v1a1
}

// OptionalIntToV1beta1 converts the request between v1alpha1 and v1beta1.
func OptionalIntToV1beta1(v1a1 *v1alpha1.OptionalInt) *v1beta1.OptionalInt {
	if v1a1 == nil {
		return nil
	}

	v1b1 := &v1beta1.OptionalInt{
		Value: v1a1.Value,
	}

	return v1b1
}

// OptionalStringToV1alpha1 converts the request between v1alpha1 and v1beta1.
func OptionalStringToV1alpha1(v1b1 *v1beta1.OptionalString) *v1alpha1.OptionalString {
	if v1b1 == nil {
		return nil
	}

	v1a1 := &v1alpha1.OptionalString{
		Value: v1b1.Value,
	}

	return v1a1
}

// OptionalStringToV1beta1 converts the request between v1alpha1 and v1beta1.
func OptionalStringToV1beta1(v1a1 *v1alpha1.OptionalString) *v1beta1.OptionalString {
	if v1a1 == nil {
		return nil
	}

	v1b1 := &v1beta1.OptionalString{
		Value: v1a1.Value,
	}

	return v1b1
}

// OptionalUInt32ToV1alpha1 converts the request between v1alpha1 and v1beta1.
func OptionalUInt32ToV1alpha1(v1b1 *v1beta1.OptionalUInt32) *v1alpha1.OptionalUInt32 {
	if v1b1 == nil {
		return nil
	}

	v1a1 := &v1alpha1.OptionalUInt32{
		Value: v1b1.Value,
	}

	return v1a1
}

// OptionalUInt32ToV1beta1 converts the request between v1alpha1 and v1beta1.
func OptionalUInt32ToV1beta1(v1a1 *v1alpha1.OptionalUInt32) *v1beta1.OptionalUInt32 {
	if v1a1 == nil {
		return nil
	}

	v1b1 := &v1beta1.OptionalUInt32{
		Value: v1a1.Value,
	}

	return v1b1
}

// OptionalInt64ToV1alpha1 converts the request between v1alpha1 and v1beta1.
func OptionalInt64ToV1alpha1(v1b1 *v1beta1.OptionalInt64) *v1alpha1.OptionalInt64 {
	if v1b1 == nil {
		return nil
	}

	v1a1 := &v1alpha1.OptionalInt64{
		Value: v1b1.Value,
	}

	return v1a1
}

// OptionalInt64ToV1beta1 converts the request between v1alpha1 and v1beta1.
func OptionalInt64ToV1beta1(v1a1 *v1alpha1.OptionalInt64) *v1beta1.OptionalInt64 {
	if v1a1 == nil {
		return nil
	}

	v1b1 := &v1beta1.OptionalInt64{
		Value: v1a1.Value,
	}

	return v1b1
}

// OptionalUInt64ToV1alpha1 converts the request between v1alpha1 and v1beta1.
func OptionalUInt64ToV1alpha1(v1b1 *v1beta1.OptionalUInt64) *v1alpha1.OptionalUInt64 {
	if v1b1 == nil {
		return nil
	}

	v1a1 := &v1alpha1.OptionalUInt64{
		Value: v1b1.Value,
	}

	return v1a1
}

// OptionalUInt64ToV1beta1 converts the request between v1alpha1 and v1beta1.
func OptionalUInt64ToV1beta1(v1a1 *v1alpha1.OptionalUInt64) *v1beta1.OptionalUInt64 {
	if v1a1 == nil {
		return nil
	}

	v1b1 := &v1beta1.OptionalUInt64{
		Value: v1a1.Value,
	}

	return v1b1
}

// OptionalBoolToV1alpha1 converts the request between v1alpha1 and v1beta1.
func OptionalBoolToV1alpha1(v1b1 *v1beta1.OptionalBool) *v1alpha1.OptionalBool {
	if v1b1 == nil {
		return nil
	}

	v1a1 := &v1alpha1.OptionalBool{
		Value: v1b1.Value,
	}

	return v1a1
}

// OptionalBoolToV1beta1 converts the request between v1alpha1 and v1beta1.
func OptionalBoolToV1beta1(v1a1 *v1alpha1.OptionalBool) *v1beta1.OptionalBool {
	if v1a1 == nil {
		return nil
	}

	v1b1 := &v1beta1.OptionalBool{
		Value: v1a1.Value,
	}

	return v1b1
}

// OptionalFileModeToV1alpha1 converts the request between v1alpha1 and v1beta1.
func OptionalFileModeToV1alpha1(v1b1 *v1beta1.OptionalFileMode) *v1alpha1.OptionalFileMode {
	if v1b1 == nil {
		return nil
	}

	v1a1 := &v1alpha1.OptionalFileMode{
		Value: v1b1.Value,
	}

	return v1a1
}

// OptionalFileModeToV1beta1 converts the request between v1alpha1 and v1beta1.
func OptionalFileModeToV1beta1(v1a1 *v1alpha1.OptionalFileMode) *v1beta1.OptionalFileMode {
	if v1a1 == nil {
		return nil
	}

	v1b1 := &v1beta1.OptionalFileMode{
		Value: v1a1.Value,
	}

	return v1b1
}

// POSIXRlimitsSliceToV1alpha1 converts the request between v1alpha1 and v1beta1.
func POSIXRlimitsSliceToV1alpha1(v1b1 []*v1beta1.POSIXRlimit) []*v1alpha1.POSIXRlimit {
	if v1b1 == nil {
		return nil
	}

	v1a1 := make([]*v1alpha1.POSIXRlimit, 0, len(v1b1))
	for _, r := range v1b1 {
		if r != nil {
			v1a1 = append(v1a1, &v1alpha1.POSIXRlimit{
				Type: r.Type,
				Hard: r.Hard,
				Soft: r.Soft,
			})
		}
	}

	return v1a1
}

// POSIXRlimitsSliceToV1beta1 converts the request between v1alpha1 and v1beta1.
func POSIXRlimitsSliceToV1beta1(v1a1 []*v1alpha1.POSIXRlimit) []*v1beta1.POSIXRlimit {
	if v1a1 == nil {
		return nil
	}

	v1b1 := make([]*v1beta1.POSIXRlimit, 0, len(v1a1))
	for _, r := range v1a1 {
		if r != nil {
			v1b1 = append(v1b1, &v1beta1.POSIXRlimit{
				Type: r.Type,
				Hard: r.Hard,
				Soft: r.Soft,
			})
		}
	}

	return v1b1
}

// CDIDeviceSliceToV1alpha1 converts the request between v1alpha1 and v1beta1.
func CDIDeviceSliceToV1alpha1(v1b1 []*v1beta1.CDIDevice) []*v1alpha1.CDIDevice {
	if v1b1 == nil {
		return nil
	}

	v1a1 := make([]*v1alpha1.CDIDevice, 0, len(v1b1))
	for _, d := range v1b1 {
		if d != nil {
			v1a1 = append(v1a1, &v1alpha1.CDIDevice{
				Name: d.Name,
			})
		}
	}

	return v1a1
}

// CDIDeviceSliceToV1beta1 converts the request between v1alpha1 and v1beta1.
func CDIDeviceSliceToV1beta1(v1a1 []*v1alpha1.CDIDevice) []*v1beta1.CDIDevice {
	if v1a1 == nil {
		return nil
	}

	v1b1 := make([]*v1beta1.CDIDevice, 0, len(v1a1))
	for _, d := range v1a1 {
		if d != nil {
			v1b1 = append(v1b1, &v1beta1.CDIDevice{
				Name: d.Name,
			})
		}
	}

	return v1b1
}

// UserToV1alpha1 converts the request between v1alpha1 and v1beta1.
func UserToV1alpha1(v1b1 *v1beta1.User) *v1alpha1.User {
	if v1b1 == nil {
		return nil
	}

	v1a1 := &v1alpha1.User{
		Uid:            v1b1.Uid,
		Gid:            v1b1.Gid,
		AdditionalGids: v1b1.AdditionalGids,
	}

	return v1a1
}

// UserToV1beta1 converts the request between v1alpha1 and v1beta1.
func UserToV1beta1(v1a1 *v1alpha1.User) *v1beta1.User {
	if v1a1 == nil {
		return nil
	}

	v1b1 := &v1beta1.User{
		Uid:            v1a1.Uid,
		Gid:            v1a1.Gid,
		AdditionalGids: v1a1.AdditionalGids,
	}

	return v1b1
}

// ContainerAdjustmentToV1alpha1 converts the request between v1alpha1 and v1beta1.
func ContainerAdjustmentToV1alpha1(v1b1 *v1beta1.ContainerAdjustment) *v1alpha1.ContainerAdjustment {
	if v1b1 == nil {
		return nil
	}

	v1a1 := &v1alpha1.ContainerAdjustment{
		Annotations: v1b1.Annotations,
		Mounts:      MountSliceToV1alpha1(v1b1.Mounts),
		Env:         KeyValueSliceToV1alpha1(v1b1.Env),
		Hooks:       HooksToV1alpha1(v1b1.Hooks),
		Linux:       LinuxContainerAdjustmentToV1alpha1(v1b1.Linux),
		Rlimits:     POSIXRlimitsSliceToV1alpha1(v1b1.Rlimits),
		CDIDevices:  CDIDeviceSliceToV1alpha1(v1b1.CDIDevices),
		Args:        v1b1.Args,
	}

	return v1a1
}

// ContainerAdjustmentToV1beta1 converts the request between v1alpha1 and v1beta1.
func ContainerAdjustmentToV1beta1(v1a1 *v1alpha1.ContainerAdjustment) *v1beta1.ContainerAdjustment {
	if v1a1 == nil {
		return nil
	}

	v1b1 := &v1beta1.ContainerAdjustment{
		Annotations: v1a1.Annotations,
		Mounts:      MountSliceToV1beta1(v1a1.Mounts),
		Env:         KeyValueSliceToV1beta1(v1a1.Env),
		Hooks:       HooksToV1beta1(v1a1.Hooks),
		Linux:       LinuxContainerAdjustmentToV1beta1(v1a1.Linux),
		Rlimits:     POSIXRlimitsSliceToV1beta1(v1a1.Rlimits),
		CDIDevices:  CDIDeviceSliceToV1beta1(v1a1.CDIDevices),
		Args:        v1a1.Args,
	}

	return v1b1
}

// ContainerUpdateSliceToV1alpha1 converts the request between v1alpha1 and v1beta1.
func ContainerUpdateSliceToV1alpha1(v1b1 []*v1beta1.ContainerUpdate) []*v1alpha1.ContainerUpdate {
	if v1b1 == nil {
		return nil
	}

	v1a1 := make([]*v1alpha1.ContainerUpdate, 0, len(v1b1))
	for _, u := range v1b1 {
		if u != nil {
			v1a1 = append(v1a1, &v1alpha1.ContainerUpdate{
				ContainerId:   u.ContainerId,
				Linux:         LinuxContainerUpdateToV1alpha1(u.Linux),
				IgnoreFailure: u.IgnoreFailure,
			})
		}
	}

	return v1a1
}

// ContainerUpdateSliceToV1beta1 converts the request between v1alpha1 and v1beta1.
func ContainerUpdateSliceToV1beta1(v1a1 []*v1alpha1.ContainerUpdate) []*v1beta1.ContainerUpdate {
	if v1a1 == nil {
		return nil
	}

	v1b1 := make([]*v1beta1.ContainerUpdate, 0, len(v1a1))
	for _, u := range v1a1 {
		if u != nil {
			v1b1 = append(v1b1, &v1beta1.ContainerUpdate{
				ContainerId:   u.ContainerId,
				Linux:         LinuxContainerUpdateToV1beta1(u.Linux),
				IgnoreFailure: u.IgnoreFailure,
			})
		}
	}

	return v1b1
}

// KeyValueSliceToV1alpha1 converts the request between v1alpha1 and v1beta1.
func KeyValueSliceToV1alpha1(v1b1 []*v1beta1.KeyValue) []*v1alpha1.KeyValue {
	if v1b1 == nil {
		return nil
	}

	v1a1 := make([]*v1alpha1.KeyValue, 0, len(v1b1))
	for _, kv := range v1b1 {
		if kv != nil {
			v1a1 = append(v1a1, &v1alpha1.KeyValue{
				Key:   kv.Key,
				Value: kv.Value,
			})
		}
	}

	return v1a1
}

// KeyValueSliceToV1beta1 converts the request between v1alpha1 and v1beta1.
func KeyValueSliceToV1beta1(v1a1 []*v1alpha1.KeyValue) []*v1beta1.KeyValue {
	if v1a1 == nil {
		return nil
	}

	v1b1 := make([]*v1beta1.KeyValue, 0, len(v1a1))
	for _, kv := range v1a1 {
		if kv != nil {
			v1b1 = append(v1b1, &v1beta1.KeyValue{
				Key:   kv.Key,
				Value: kv.Value,
			})
		}
	}

	return v1b1
}

// LinuxContainerAdjustmentToV1alpha1 converts the request between v1alpha1 and v1beta1.
func LinuxContainerAdjustmentToV1alpha1(v1b1 *v1beta1.LinuxContainerAdjustment) *v1alpha1.LinuxContainerAdjustment {
	if v1b1 == nil {
		return nil
	}

	v1a1 := &v1alpha1.LinuxContainerAdjustment{
		Devices:       LinuxDeviceSliceToV1alpha1(v1b1.Devices),
		Resources:     LinuxResourcesToV1alpha1(v1b1.Resources),
		CgroupsPath:   v1b1.CgroupsPath,
		OomScoreAdj:   OptionalIntToV1alpha1(v1b1.OomScoreAdj),
		IoPriority:    LinuxIOPriorityToV1alpha1(v1b1.IoPriority),
		SeccompPolicy: LinuxSeccompToV1alpha1(v1b1.SeccompPolicy),
		Namespaces:    LinuxNamespaceSliceToV1alpha1(v1b1.Namespaces),
	}

	return v1a1
}

// LinuxContainerAdjustmentToV1beta1 converts the request between v1alpha1 and v1beta1.
func LinuxContainerAdjustmentToV1beta1(v1a1 *v1alpha1.LinuxContainerAdjustment) *v1beta1.LinuxContainerAdjustment {
	if v1a1 == nil {
		return nil
	}

	v1b1 := &v1beta1.LinuxContainerAdjustment{
		Devices:       LinuxDeviceSliceToV1beta1(v1a1.Devices),
		Resources:     LinuxResourcesToV1beta1(v1a1.Resources),
		CgroupsPath:   v1a1.CgroupsPath,
		OomScoreAdj:   OptionalIntToV1beta1(v1a1.OomScoreAdj),
		IoPriority:    LinuxIOPriorityToV1beta1(v1a1.IoPriority),
		SeccompPolicy: LinuxSeccompToV1beta1(v1a1.SeccompPolicy),
		Namespaces:    LinuxNamespaceSliceToV1beta1(v1a1.Namespaces),
	}

	return v1b1
}

// LinuxContainerUpdateToV1alpha1 converts the request between v1alpha1 and v1beta1.
func LinuxContainerUpdateToV1alpha1(v1b1 *v1beta1.LinuxContainerUpdate) *v1alpha1.LinuxContainerUpdate {
	if v1b1 == nil {
		return nil
	}

	v1a1 := &v1alpha1.LinuxContainerUpdate{
		Resources: LinuxResourcesToV1alpha1(v1b1.Resources),
	}

	return v1a1
}

// LinuxContainerUpdateToV1beta1 converts the request between v1alpha1 and v1beta1.
func LinuxContainerUpdateToV1beta1(v1a1 *v1alpha1.LinuxContainerUpdate) *v1beta1.LinuxContainerUpdate {
	if v1a1 == nil {
		return nil
	}

	v1b1 := &v1beta1.LinuxContainerUpdate{
		Resources: LinuxResourcesToV1beta1(v1a1.Resources),
	}

	return v1b1
}

// ContainerEvictionSliceToV1alpha1 converts the request between v1alpha1 and v1beta1.
func ContainerEvictionSliceToV1alpha1(v1b1 []*v1beta1.ContainerEviction) []*v1alpha1.ContainerEviction {
	if v1b1 == nil {
		return nil
	}

	v1a1 := make([]*v1alpha1.ContainerEviction, 0, len(v1b1))
	for _, e := range v1b1 {
		if e != nil {
			v1a1 = append(v1a1, &v1alpha1.ContainerEviction{
				ContainerId: e.ContainerId,
				Reason:      e.Reason,
			})
		}
	}

	return v1a1
}

// ContainerEvictionSliceToV1beta1 converts the request between v1alpha1 and v1beta1.
func ContainerEvictionSliceToV1beta1(v1a1 []*v1alpha1.ContainerEviction) []*v1beta1.ContainerEviction {
	if v1a1 == nil {
		return nil
	}

	v1b1 := make([]*v1beta1.ContainerEviction, 0, len(v1a1))
	for _, e := range v1a1 {
		if e != nil {
			v1b1 = append(v1b1, &v1beta1.ContainerEviction{
				ContainerId: e.ContainerId,
				Reason:      e.Reason,
			})
		}
	}

	return v1b1
}

// OwningPluginsToV1alpha1 converts the request between v1alpha1 and v1beta1.
func OwningPluginsToV1alpha1(v1b1 *v1beta1.OwningPlugins) *v1alpha1.OwningPlugins {
	if v1b1 == nil {
		return nil
	}

	return &v1alpha1.OwningPlugins{
		Owners: FieldOwnersMapToV1alpha1(v1b1.Owners),
	}
}

// OwningPluginsToV1beta1 converts the request between v1alpha1 and v1beta1.
func OwningPluginsToV1beta1(v1a1 *v1alpha1.OwningPlugins) *v1beta1.OwningPlugins {
	if v1a1 == nil {
		return nil
	}

	return &v1beta1.OwningPlugins{
		Owners: FieldOwnersMapToV1beta1(v1a1.Owners),
	}
}

// FieldOwnersMapToV1alpha1 converts the request between v1alpha1 and v1beta1.
func FieldOwnersMapToV1alpha1(v1b1 map[string]*v1beta1.FieldOwners) map[string]*v1alpha1.FieldOwners {
	if v1b1 == nil {
		return nil
	}

	v1a1 := make(map[string]*v1alpha1.FieldOwners, len(v1b1))
	for id, o := range v1b1 {
		if o != nil {
			v1a1[id] = &v1alpha1.FieldOwners{
				Simple:   o.Simple,
				Compound: CompoundFieldOwnersMapToV1alpha1(o.Compound),
			}
		}
	}

	return v1a1
}

// FieldOwnersMapToV1beta1 converts the request between v1alpha1 and v1beta1.
func FieldOwnersMapToV1beta1(v1a1 map[string]*v1alpha1.FieldOwners) map[string]*v1beta1.FieldOwners {
	if v1a1 == nil {
		return nil
	}

	v1b1 := make(map[string]*v1beta1.FieldOwners, len(v1a1))
	for id, o := range v1a1 {
		if o != nil {
			v1b1[id] = &v1beta1.FieldOwners{
				Simple:   o.Simple,
				Compound: CompoundFieldOwnersMapToV1beta1(o.Compound),
			}
		}
	}

	return v1b1
}

// CompoundFieldOwnersMapToV1alpha1 converts the request between v1alpha1 and v1beta1.
func CompoundFieldOwnersMapToV1alpha1(v1b1 map[int32]*v1beta1.CompoundFieldOwners) map[int32]*v1alpha1.CompoundFieldOwners {
	if v1b1 == nil {
		return nil
	}

	v1a1 := make(map[int32]*v1alpha1.CompoundFieldOwners, len(v1b1))
	for f, o := range v1b1 {
		if o != nil {
			v1a1[f] = &v1alpha1.CompoundFieldOwners{
				Owners: o.Owners,
			}
		}
	}

	return v1a1
}

// CompoundFieldOwnersMapToV1beta1 converts the request between v1alpha1 and v1beta1.
func CompoundFieldOwnersMapToV1beta1(v1a1 map[int32]*v1alpha1.CompoundFieldOwners) map[int32]*v1beta1.CompoundFieldOwners {
	if v1a1 == nil {
		return nil
	}

	v1b1 := make(map[int32]*v1beta1.CompoundFieldOwners, len(v1a1))
	for f, o := range v1a1 {
		if o != nil {
			v1b1[f] = &v1beta1.CompoundFieldOwners{
				Owners: o.Owners,
			}
		}
	}

	return v1b1
}

// PluginInstanceSliceToV1alpha1 converts the request between v1alpha1 and v1beta1.
func PluginInstanceSliceToV1alpha1(v1b1 []*v1beta1.PluginInstance) []*v1alpha1.PluginInstance {
	if v1b1 == nil {
		return nil
	}

	v1a1 := make([]*v1alpha1.PluginInstance, 0, len(v1b1))
	for _, p := range v1b1 {
		if p != nil {
			v1a1 = append(v1a1, &v1alpha1.PluginInstance{
				Name:  p.Name,
				Index: p.Index,
			})
		}
	}

	return v1a1
}

// PluginInstanceSliceToV1beta1 converts the request between v1alpha1 and v1beta1.
func PluginInstanceSliceToV1beta1(v1a1 []*v1alpha1.PluginInstance) []*v1beta1.PluginInstance {
	if v1a1 == nil {
		return nil
	}

	v1b1 := make([]*v1beta1.PluginInstance, 0, len(v1a1))
	for _, p := range v1a1 {
		if p != nil {
			v1b1 = append(v1b1, &v1beta1.PluginInstance{
				Name:  p.Name,
				Index: p.Index,
			})
		}
	}

	return v1b1
}
