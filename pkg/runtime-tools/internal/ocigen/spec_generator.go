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

package ocigen

import (
	"fmt"
	"strings"

	rspec "github.com/opencontainers/runtime-spec/specs-go"
)

// Generator implements [generate.UnderlyingGenerator] directly on an *rspec.Spec,
// removing the need to import github.com/opencontainers/runtime-tools/generate.
type Generator struct {
	spec *rspec.Spec
}

// New creates a new Generator for modifying spec.
func New(spec *rspec.Spec) *Generator {
	return &Generator{spec: spec}
}

// Spec returns the generated OCI runtime specification.
func (g *Generator) Spec() *rspec.Spec {
	return g.spec
}

func (g *Generator) initConfig() {
	if g.spec == nil {
		g.spec = &rspec.Spec{}
	}
}

func (g *Generator) initProcess() {
	g.initConfig()
	if g.spec.Process == nil {
		g.spec.Process = &rspec.Process{}
	}
}

func (g *Generator) initLinux() {
	g.initConfig()
	if g.spec.Linux == nil {
		g.spec.Linux = &rspec.Linux{}
	}
}

func (g *Generator) initLinuxResources() {
	g.initLinux()
	if g.spec.Linux.Resources == nil {
		g.spec.Linux.Resources = &rspec.LinuxResources{}
	}
}

func (g *Generator) initLinuxCPU() {
	g.initLinuxResources()
	if g.spec.Linux.Resources.CPU == nil {
		g.spec.Linux.Resources.CPU = &rspec.LinuxCPU{}
	}
}

func (g *Generator) initLinuxMemory() {
	g.initLinuxResources()
	if g.spec.Linux.Resources.Memory == nil {
		g.spec.Linux.Resources.Memory = &rspec.LinuxMemory{}
	}
}

func (g *Generator) initHooks() {
	g.initConfig()
	if g.spec.Hooks == nil {
		g.spec.Hooks = &rspec.Hooks{}
	}
}

func (g *Generator) initAnnotations() {
	g.initConfig()
	if g.spec.Annotations == nil {
		g.spec.Annotations = map[string]string{}
	}
}

func (g *Generator) initSysctl() {
	g.initLinux()
	if g.spec.Linux.Sysctl == nil {
		g.spec.Linux.Sysctl = map[string]string{}
	}
}

// AddAnnotation adds or replaces an annotation in the specification.
func (g *Generator) AddAnnotation(key, value string) {
	g.initAnnotations()
	g.spec.Annotations[key] = value
}

// RemoveAnnotation removes an annotation from the specification.
func (g *Generator) RemoveAnnotation(key string) {
	if g.spec != nil && g.spec.Annotations != nil {
		delete(g.spec.Annotations, key)
	}
}

// AddDevice adds a Linux device to the specification.
func (g *Generator) AddDevice(device rspec.LinuxDevice) {
	g.initLinux()
	for i, d := range g.spec.Linux.Devices {
		if d.Path == device.Path {
			g.spec.Linux.Devices[i] = device
			return
		}
	}
	g.spec.Linux.Devices = append(g.spec.Linux.Devices, device)
}

// RemoveDevice removes the Linux device with the given path from the specification.
func (g *Generator) RemoveDevice(path string) {
	if g.spec == nil || g.spec.Linux == nil {
		return
	}
	for i, d := range g.spec.Linux.Devices {
		if d.Path == path {
			g.spec.Linux.Devices = append(g.spec.Linux.Devices[:i], g.spec.Linux.Devices[i+1:]...)
			return
		}
	}
}

// AddOrReplaceLinuxNamespace adds or replaces a Linux namespace in the specification.
func (g *Generator) AddOrReplaceLinuxNamespace(ns string, path string) error {
	nsType, err := namespaceType(ns)
	if err != nil {
		return err
	}
	g.initLinux()
	for i, n := range g.spec.Linux.Namespaces {
		if n.Type == nsType {
			g.spec.Linux.Namespaces[i].Path = path
			return nil
		}
	}
	g.spec.Linux.Namespaces = append(g.spec.Linux.Namespaces, rspec.LinuxNamespace{Type: nsType, Path: path})
	return nil
}

// RemoveLinuxNamespace removes a Linux namespace from the specification.
func (g *Generator) RemoveLinuxNamespace(ns string) error {
	nsType, err := namespaceType(ns)
	if err != nil {
		return err
	}
	if g.spec == nil || g.spec.Linux == nil {
		return nil
	}
	for i, n := range g.spec.Linux.Namespaces {
		if n.Type == nsType {
			g.spec.Linux.Namespaces = append(g.spec.Linux.Namespaces[:i], g.spec.Linux.Namespaces[i+1:]...)
			return nil
		}
	}
	return nil
}

func namespaceType(ns string) (rspec.LinuxNamespaceType, error) {
	switch ns {
	case "network":
		return rspec.NetworkNamespace, nil
	case "pid":
		return rspec.PIDNamespace, nil
	case "mount":
		return rspec.MountNamespace, nil
	case "ipc":
		return rspec.IPCNamespace, nil
	case "uts":
		return rspec.UTSNamespace, nil
	case "user":
		return rspec.UserNamespace, nil
	case "cgroup":
		return rspec.CgroupNamespace, nil
	case "time":
		return rspec.TimeNamespace, nil
	default:
		return "", fmt.Errorf("unrecognized namespace %q", ns)
	}
}

// AddPreStartHook adds a pre-start hook to the specification.
func (g *Generator) AddPreStartHook(hook rspec.Hook) {
	g.initHooks()
	g.spec.Hooks.Prestart = append(g.spec.Hooks.Prestart, hook) //nolint:staticcheck
}

// AddPostStartHook adds a post-start hook to the specification.
func (g *Generator) AddPostStartHook(hook rspec.Hook) {
	g.initHooks()
	g.spec.Hooks.Poststart = append(g.spec.Hooks.Poststart, hook)
}

// AddPostStopHook adds a post-stop hook to the specification.
func (g *Generator) AddPostStopHook(hook rspec.Hook) {
	g.initHooks()
	g.spec.Hooks.Poststop = append(g.spec.Hooks.Poststop, hook)
}

// AddProcessEnv adds or replaces an environment variable for the process.
func (g *Generator) AddProcessEnv(name, value string) {
	if name == "" {
		return
	}
	g.initProcess()
	prefix := name + "="
	for i, e := range g.spec.Process.Env {
		if strings.HasPrefix(e, prefix) {
			g.spec.Process.Env[i] = prefix + value
			return
		}
	}
	g.spec.Process.Env = append(g.spec.Process.Env, prefix+value)
}

// ClearProcessEnv removes all environment variables from the process.
func (g *Generator) ClearProcessEnv() {
	if g.spec == nil || g.spec.Process == nil {
		return
	}
	g.spec.Process.Env = []string{}
}

// SetProcessArgs sets the process arguments.
func (g *Generator) SetProcessArgs(args []string) {
	g.initProcess()
	g.spec.Process.Args = args
}

// SetProcessOOMScoreAdj sets the process OOM score adjustment.
func (g *Generator) SetProcessOOMScoreAdj(adj int) {
	g.initProcess()
	g.spec.Process.OOMScoreAdj = &adj
}

// AddMount adds a mount to the specification.
func (g *Generator) AddMount(mnt rspec.Mount) {
	g.initConfig()
	g.spec.Mounts = append(g.spec.Mounts, mnt)
}

// RemoveMount removes the mount with the given destination from the specification.
func (g *Generator) RemoveMount(dest string) {
	if g.spec == nil {
		return
	}
	for i, m := range g.spec.Mounts {
		if m.Destination == dest {
			g.spec.Mounts = append(g.spec.Mounts[:i], g.spec.Mounts[i+1:]...)
			return
		}
	}
}

// ClearMounts removes all mounts from the specification.
func (g *Generator) ClearMounts() {
	if g.spec == nil {
		return
	}
	g.spec.Mounts = []rspec.Mount{}
}

// Mounts returns the mounts in the specification.
func (g *Generator) Mounts() []rspec.Mount {
	if g.spec == nil {
		return nil
	}
	return g.spec.Mounts
}

// AddLinuxSysctl adds or replaces a Linux sysctl setting.
func (g *Generator) AddLinuxSysctl(key, value string) {
	g.initSysctl()
	g.spec.Linux.Sysctl[key] = value
}

// RemoveLinuxSysctl removes a Linux sysctl setting.
func (g *Generator) RemoveLinuxSysctl(key string) {
	if g.spec == nil || g.spec.Linux == nil || g.spec.Linux.Sysctl == nil {
		return
	}
	delete(g.spec.Linux.Sysctl, key)
}

// SetLinuxCgroupsPath sets the Linux cgroups path.
func (g *Generator) SetLinuxCgroupsPath(path string) {
	g.initLinux()
	g.spec.Linux.CgroupsPath = path
}

// SetLinuxRootPropagation sets the Linux root filesystem propagation mode.
func (g *Generator) SetLinuxRootPropagation(rp string) error {
	switch rp {
	case "", "private", "rprivate", "slave", "rslave", "shared", "rshared", "unbindable", "runbindable":
	default:
		return fmt.Errorf("rootfs-propagation %q must be empty or one of (r)private|(r)slave|(r)shared|(r)unbindable", rp)
	}
	g.initLinux()
	g.spec.Linux.RootfsPropagation = rp
	return nil
}

// AddLinuxResourcesDevice adds a Linux device cgroup rule.
func (g *Generator) AddLinuxResourcesDevice(allow bool, devType string, major, minor *int64, access string) {
	g.initLinuxResources()
	g.spec.Linux.Resources.Devices = append(g.spec.Linux.Resources.Devices, rspec.LinuxDeviceCgroup{
		Allow:  allow,
		Type:   devType,
		Major:  major,
		Minor:  minor,
		Access: access,
	})
}

// AddLinuxResourcesHugepageLimit adds or replaces a Linux hugepage limit.
func (g *Generator) AddLinuxResourcesHugepageLimit(pageSize string, limit uint64) {
	g.initLinuxResources()
	for i, h := range g.spec.Linux.Resources.HugepageLimits {
		if h.Pagesize == pageSize {
			g.spec.Linux.Resources.HugepageLimits[i].Limit = limit
			return
		}
	}
	g.spec.Linux.Resources.HugepageLimits = append(g.spec.Linux.Resources.HugepageLimits,
		rspec.LinuxHugepageLimit{Pagesize: pageSize, Limit: limit})
}

// AddLinuxResourcesUnified adds or replaces a unified cgroup resource setting.
func (g *Generator) AddLinuxResourcesUnified(key, val string) {
	g.initLinuxResources()
	if g.spec.Linux.Resources.Unified == nil {
		g.spec.Linux.Resources.Unified = map[string]string{}
	}
	g.spec.Linux.Resources.Unified[key] = val
}

// SetLinuxResourcesCPUShares sets the Linux CPU shares.
func (g *Generator) SetLinuxResourcesCPUShares(shares uint64) {
	g.initLinuxCPU()
	g.spec.Linux.Resources.CPU.Shares = &shares
}

// SetLinuxResourcesCPUQuota sets the Linux CPU quota.
func (g *Generator) SetLinuxResourcesCPUQuota(quota int64) {
	g.initLinuxCPU()
	g.spec.Linux.Resources.CPU.Quota = &quota
}

// SetLinuxResourcesCPUPeriod sets the Linux CPU period.
func (g *Generator) SetLinuxResourcesCPUPeriod(period uint64) {
	g.initLinuxCPU()
	g.spec.Linux.Resources.CPU.Period = &period
}

// SetLinuxResourcesCPURealtimeRuntime sets the Linux CPU realtime runtime.
func (g *Generator) SetLinuxResourcesCPURealtimeRuntime(time int64) {
	g.initLinuxCPU()
	g.spec.Linux.Resources.CPU.RealtimeRuntime = &time
}

// SetLinuxResourcesCPURealtimePeriod sets the Linux CPU realtime period.
func (g *Generator) SetLinuxResourcesCPURealtimePeriod(period uint64) {
	g.initLinuxCPU()
	g.spec.Linux.Resources.CPU.RealtimePeriod = &period
}

// SetLinuxResourcesCPUCpus sets the Linux CPUs available to the process.
func (g *Generator) SetLinuxResourcesCPUCpus(cpus string) {
	g.initLinuxCPU()
	g.spec.Linux.Resources.CPU.Cpus = cpus
}

// SetLinuxResourcesCPUMems sets the Linux memory nodes available to the process.
func (g *Generator) SetLinuxResourcesCPUMems(mems string) {
	g.initLinuxCPU()
	g.spec.Linux.Resources.CPU.Mems = mems
}

// SetLinuxResourcesMemoryLimit sets the Linux memory limit.
func (g *Generator) SetLinuxResourcesMemoryLimit(limit int64) {
	g.initLinuxMemory()
	g.spec.Linux.Resources.Memory.Limit = &limit
}

// SetLinuxResourcesMemorySwap sets the Linux memory swap limit.
func (g *Generator) SetLinuxResourcesMemorySwap(swap int64) {
	g.initLinuxMemory()
	g.spec.Linux.Resources.Memory.Swap = &swap
}
