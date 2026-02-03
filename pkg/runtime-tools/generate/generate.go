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

package generate

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	rspec "github.com/opencontainers/runtime-spec/specs-go"

	nri "github.com/containerd/nri/pkg/api"
)

const (
	// UnlimitedPidsLimit indicates unlimited Linux PIDs limit.
	UnlimitedPidsLimit = -1
)

// UnderlyingGenerator is the interface for
// [github.com/opencontainers/runtime-tools/generate.Generator].
type UnderlyingGenerator interface {
	AddAnnotation(key, value string)
	AddDevice(device rspec.LinuxDevice)
	AddOrReplaceLinuxNamespace(ns string, path string) error
	AddPostStartHook(postStartHook rspec.Hook)
	AddPostStopHook(postStopHook rspec.Hook)
	AddPreStartHook(preStartHook rspec.Hook)
	AddProcessEnv(name, value string)
	AddProcessRlimits(typ string, hard, soft uint64)
	AddLinuxResourcesDevice(allow bool, devType string, major, minor *int64, access string)
	AddLinuxResourcesHugepageLimit(pageSize string, limit uint64)
	AddLinuxResourcesUnified(key, val string)
	AddMount(mnt rspec.Mount)
	AddLinuxSysctl(key, value string)
	ClearMounts()
	ClearProcessEnv()
	Mounts() []rspec.Mount
	RemoveAnnotation(key string)
	RemoveDevice(path string)
	RemoveLinuxNamespace(ns string) error
	RemoveMount(dest string)
	RemoveLinuxSysctl(key string)
	SetProcessArgs(args []string)
	SetLinuxCgroupsPath(path string)
	SetLinuxResourcesCPUCpus(cpus string)
	SetLinuxResourcesCPUMems(mems string)
	SetLinuxResourcesCPUPeriod(period uint64)
	SetLinuxResourcesCPUQuota(quota int64)
	SetLinuxResourcesCPURealtimePeriod(period uint64)
	SetLinuxResourcesCPURealtimeRuntime(time int64)
	SetLinuxResourcesCPUShares(shares uint64)
	SetLinuxResourcesMemoryLimit(limit int64)
	SetLinuxResourcesMemorySwap(swap int64)
	SetLinuxRootPropagation(rp string) error
	SetProcessOOMScoreAdj(adj int)
	Spec() *rspec.Spec
}

// GeneratorOption is an option for Generator().
type GeneratorOption func(*Generator)

// Generator extends a stock runtime-tools Generator and extends it with
// a few functions for handling NRI container adjustment.
type Generator struct {
	UnderlyingGenerator
	Config            *rspec.Spec
	filterLabels      func(map[string]string) (map[string]string, error)
	filterAnnotations func(map[string]string) (map[string]string, error)
	filterSysctl      func(map[string]string) (map[string]string, error)
	resolveBlockIO    func(string) (*rspec.LinuxBlockIO, error)
	resolveRdt        func(string) (*rspec.LinuxIntelRdt, error)
	injectCDIDevices  func(*rspec.Spec, []string) error
	checkResources    func(*rspec.LinuxResources) error
	logger            Logger
	owners            *nri.FieldOwners
}

// SpecGenerator returns a wrapped OCI Spec Generator.
func SpecGenerator(gg UnderlyingGenerator, opts ...GeneratorOption) *Generator {
	g := &Generator{
		UnderlyingGenerator: gg,
		Config:              gg.Spec(),
	}
	g.filterLabels = nopFilter
	g.filterAnnotations = nopFilter
	g.filterSysctl = nopFilter
	for _, o := range opts {
		o(g)
	}
	return g
}

// WithLabelFilter provides an option for filtering or rejecting labels.
func WithLabelFilter(fn func(map[string]string) (map[string]string, error)) GeneratorOption {
	return func(g *Generator) {
		g.filterLabels = fn
	}
}

// WithAnnotationFilter provides an option for filtering or rejecting annotations.
func WithAnnotationFilter(fn func(map[string]string) (map[string]string, error)) GeneratorOption {
	return func(g *Generator) {
		g.filterAnnotations = fn
	}
}

// WithBlockIOResolver specifies a function for resolving Block I/O classes by name.
func WithBlockIOResolver(fn func(string) (*rspec.LinuxBlockIO, error)) GeneratorOption {
	return func(g *Generator) {
		g.resolveBlockIO = fn
	}
}

// WithRdtResolver specifies a function for resolving RDT classes by name.
func WithRdtResolver(fn func(string) (*rspec.LinuxIntelRdt, error)) GeneratorOption {
	return func(g *Generator) {
		g.resolveRdt = fn
	}
}

// WithResourceChecker specifies a function to perform final resource adjustment.
func WithResourceChecker(fn func(*rspec.LinuxResources) error) GeneratorOption {
	return func(g *Generator) {
		g.checkResources = fn
	}
}

// WithCDIDeviceInjector specifies a runtime-specific function to use for CDI
// device resolution and injection into an OCI Spec.
func WithCDIDeviceInjector(fn func(*rspec.Spec, []string) error) GeneratorOption {
	return func(g *Generator) {
		g.injectCDIDevices = fn
	}
}

// WithLogger specifies a function for logging (audit) messages.
func WithLogger(logger Logger, owners *nri.FieldOwners) GeneratorOption {
	return func(g *Generator) {
		g.logger = logger
		g.owners = owners
	}
}

// Fields can be used to pass extra information for logged messages.
type Fields = map[string]any

// Logger is a function for logging (audit) messages.
type Logger = func(event string, fields Fields)

// Adjust adjusts all aspects of the OCI Spec that NRI knows/cares about.
func (g *Generator) Adjust(adjust *nri.ContainerAdjustment) error {
	if adjust == nil {
		return nil
	}

	if err := g.AdjustAnnotations(adjust.GetAnnotations()); err != nil {
		return fmt.Errorf("failed to adjust annotations in OCI Spec: %w", err)
	}
	g.AdjustEnv(adjust.GetEnv())
	g.AdjustArgs(adjust.GetArgs())
	g.AdjustHooks(adjust.GetHooks())
	if err := g.InjectCDIDevices(adjust.GetCDIDevices()); err != nil {
		return err
	}
	g.AdjustDevices(adjust.GetLinux().GetDevices())
	g.AdjustCgroupsPath(adjust.GetLinux().GetCgroupsPath())
	g.AdjustOomScoreAdj(adjust.GetLinux().GetOomScoreAdj())
	g.AdjustIOPriority(adjust.GetLinux().GetIoPriority())
	g.AdjustLinuxScheduler(adjust.GetLinux().GetScheduler())

	if err := g.AdjustSeccompPolicy(adjust.GetLinux().GetSeccompPolicy()); err != nil {
		return err
	}
	if err := g.AdjustNamespaces(adjust.GetLinux().GetNamespaces()); err != nil {
		return err
	}
	if err := g.AdjustSysctl(adjust.GetLinux().GetSysctl()); err != nil {
		return err
	}
	g.AdjustLinuxNetDevices(adjust.GetLinux().GetNetDevices())

	resources := adjust.GetLinux().GetResources()
	if err := g.AdjustResources(resources); err != nil {
		return err
	}
	if err := g.AdjustBlockIOClass(resources.GetBlockioClass().Get()); err != nil {
		return err
	}
	if err := g.AdjustRdtClass(resources.GetRdtClass().Get()); err != nil {
		return err
	}
	g.AdjustRdt(adjust.GetLinux().GetRdt())

	if err := g.AdjustMounts(adjust.GetMounts()); err != nil {
		return err
	}
	if err := g.AdjustRlimits(adjust.GetRlimits()); err != nil {
		return err
	}

	return nil
}

// AdjustEnv adjusts the environment of the OCI Spec.
func (g *Generator) AdjustEnv(env []*nri.KeyValue) {
	mod := map[string]*nri.KeyValue{}

	for _, e := range env {
		key, _ := nri.IsMarkedForRemoval(e.Key)
		mod[key] = e
	}

	// first modify existing environment
	if len(mod) > 0 && g.Config != nil && g.Config.Process != nil {
		old := g.Config.Process.Env
		g.ClearProcessEnv()
		for _, e := range old {
			keyval := strings.SplitN(e, "=", 2)
			if len(keyval) < 2 {
				continue
			}
			if m, ok := mod[keyval[0]]; ok {
				delete(mod, keyval[0])
				if key, marked := m.IsMarkedForRemoval(); !marked {
					g.log(AuditAddProcessEnv,
						Fields{
							"value.name":  m.Key,
							"value.value": "<value omitted>",
							"nri.plugin":  g.compoundOwner(nri.Field_Env, m.Key),
						},
					)
					g.AddProcessEnv(m.Key, m.Value)
				} else {
					g.log(AuditRemoveProcessEnv,
						Fields{
							"value.name": key,
							"nri.plugin": g.compoundOwner(nri.Field_Env, key),
						},
					)
				}
				continue
			}
			g.log(AuditAddProcessEnv, Fields{
				"value.name":  keyval[0],
				"value.value": keyval[1],
				"nri.plugin":  g.compoundOwner(nri.Field_Env, keyval[0]),
			})
			g.AddProcessEnv(keyval[0], keyval[1])
		}
	}

	// then append remaining unprocessed adjustments (new variables)
	for _, e := range env {
		if _, marked := e.IsMarkedForRemoval(); marked {
			continue
		}
		if _, ok := mod[e.Key]; ok {
			g.log(AuditAddProcessEnv, Fields{
				"value.name":  e.Key,
				"value.value": e.Value,
				"nri.plugin":  g.compoundOwner(nri.Field_Env, e.Key),
			})
			g.AddProcessEnv(e.Key, e.Value)
		}
	}
}

// AdjustArgs adjusts the process arguments in the OCI Spec.
func (g *Generator) AdjustArgs(args []string) {
	if len(args) != 0 {
		g.log(AuditSetProcessArgs,
			Fields{
				"args":       strings.Join(args, " "),
				"nri.plugin": g.simpleOwner(nri.Field_Args),
			})
		g.SetProcessArgs(args)
	}
}

// AdjustAnnotations adjusts the annotations in the OCI Spec.
func (g *Generator) AdjustAnnotations(annotations map[string]string) error {
	var err error

	if annotations, err = g.filterAnnotations(annotations); err != nil {
		return err
	}
	for k, v := range annotations {
		if key, marked := nri.IsMarkedForRemoval(k); marked {
			g.log(AuditRemoveAnnotation,
				Fields{
					"value.key":  key,
					"nri.plugin": g.compoundOwner(nri.Field_Annotations, key),
				})
		} else {
			g.log(AuditAddAnnotation,
				Fields{
					"value.key":   k,
					"value.value": v,
					"nri.plugin":  g.compoundOwner(nri.Field_Annotations, k),
				})
			g.AddAnnotation(k, v)
		}
	}

	return nil
}

// AdjustHooks adjusts the OCI hooks in the OCI Spec.
func (g *Generator) AdjustHooks(hooks *nri.Hooks) {
	if hooks == nil {
		return
	}
	for _, h := range hooks.Prestart {
		g.log(AuditAddOCIHook,
			Fields{
				"value.type": "PreStart",
				"value.path": h.Path,
				"nri.plugin": g.simpleOwner(nri.Field_OciHooks),
			})
		g.AddPreStartHook(h.ToOCI())
	}
	for _, h := range hooks.Poststart {
		g.log(AuditAddOCIHook,
			Fields{
				"value.type": "PostStart",
				"value.path": h.Path,
				"nri.plugin": g.simpleOwner(nri.Field_OciHooks),
			})
		g.AddPostStartHook(h.ToOCI())
	}
	for _, h := range hooks.Poststop {
		g.log(AuditAddOCIHook,
			Fields{
				"value.type": "PostStop",
				"value.path": h.Path,
				"nri.plugin": g.simpleOwner(nri.Field_OciHooks),
			})
		g.AddPostStopHook(h.ToOCI())
	}
	for _, h := range hooks.CreateRuntime {
		g.log(AuditAddOCIHook,
			Fields{
				"value.type": "CreateRuntime",
				"value.path": h.Path,
				"nri.plugin": g.simpleOwner(nri.Field_OciHooks),
			})
		g.AddCreateRuntimeHook(h.ToOCI())
	}
	for _, h := range hooks.CreateContainer {
		g.log(AuditAddOCIHook,
			Fields{
				"value.type": "CreateContainer",
				"value.path": h.Path,
				"nri.plugin": g.simpleOwner(nri.Field_OciHooks),
			})
		g.AddCreateContainerHook(h.ToOCI())
	}
	for _, h := range hooks.StartContainer {
		g.log(AuditAddOCIHook,
			Fields{
				"value.type": "StartContainer",
				"value.path": h.Path,
				"nri.plugin": g.simpleOwner(nri.Field_OciHooks),
			})
		g.AddStartContainerHook(h.ToOCI())
	}
}

// AdjustResources adjusts the (Linux) resources in the OCI Spec.
func (g *Generator) AdjustResources(r *nri.LinuxResources) error {
	if r == nil {
		return nil
	}

	g.initConfigLinux()

	if r.Cpu != nil {
		if r.Cpu.Period != nil {
			v := r.Cpu.GetPeriod().GetValue()
			g.log(AuditSetLinuxCPUPeriod,
				Fields{
					"value":      v,
					"nri.plugin": g.simpleOwner(nri.Field_CPUPeriod),
				})
			g.SetLinuxResourcesCPUPeriod(v)
		}
		if r.Cpu.Quota != nil {
			v := r.Cpu.GetQuota().GetValue()
			g.log(AuditSetLinuxCPUQuota,
				Fields{
					"value":      v,
					"nri.plugin": g.simpleOwner(nri.Field_CPUQuota),
				})
			g.SetLinuxResourcesCPUQuota(v)
		}
		if r.Cpu.Shares != nil {
			v := r.Cpu.GetShares().GetValue()
			g.log(AuditSetLinuxCPUShares,
				Fields{
					"value":      v,
					"nri.plugin": g.simpleOwner(nri.Field_CPUShares),
				})
			g.SetLinuxResourcesCPUShares(v)
		}
		if r.Cpu.Cpus != "" {
			v := r.Cpu.GetCpus()
			g.log(AuditSetLinuxCPUSetCPUs,
				Fields{
					"value":      v,
					"nri.plugin": g.simpleOwner(nri.Field_CPUSetCPUs),
				})
			g.SetLinuxResourcesCPUCpus(v)
		}
		if r.Cpu.Mems != "" {
			v := r.Cpu.GetMems()
			g.log(AuditSetLinuxCPUSetMems,
				Fields{
					"value":      v,
					"nri.plugin": g.simpleOwner(nri.Field_CPUSetMems),
				})
			g.SetLinuxResourcesCPUMems(v)
		}
		if r.Cpu.RealtimeRuntime != nil {
			v := r.Cpu.GetRealtimeRuntime().GetValue()
			g.log(AuditSetLinuxCPURealtimeRuntime,
				Fields{
					"value":      v,
					"nri.plugin": g.simpleOwner(nri.Field_CPURealtimeRuntime),
				})
			g.SetLinuxResourcesCPURealtimeRuntime(v)
		}
		if r.Cpu.RealtimePeriod != nil {
			v := r.Cpu.GetRealtimePeriod().GetValue()
			g.log(AuditSetLinuxCPURealtimePeriod,
				Fields{
					"value":      v,
					"nri.plugin": g.simpleOwner(nri.Field_CPURealtimePeriod),
				})
			g.SetLinuxResourcesCPURealtimePeriod(v)
		}
	}
	if r.Memory != nil {
		if l := r.Memory.GetLimit().GetValue(); l != 0 {
			g.log(AuditSetLinuxMemLimit,
				Fields{
					"value":      l,
					"nri.plugin": g.simpleOwner(nri.Field_MemLimit),
				})
			g.SetLinuxResourcesMemoryLimit(l)
			g.log(AuditSetLinuxMemSwapLimit,
				Fields{
					"value":      l,
					"nri.plugin": g.simpleOwner(nri.Field_MemSwapLimit),
				})
			g.SetLinuxResourcesMemorySwap(l)
		}
	}
	for _, l := range r.HugepageLimits {
		g.log(AuditSetLinuxHugepageLimit,
			Fields{
				"value.pagesize": l.PageSize,
				"value.limit":    l.Limit,
				"nri.plugin":     g.compoundOwner(nri.Field_HugepageLimits, l.PageSize),
			})
		g.AddLinuxResourcesHugepageLimit(l.PageSize, l.Limit)
	}
	for k, v := range r.Unified {
		g.log(AuditSetLinuxResourceUnified,
			Fields{
				"value.name":  k,
				"value.value": v,
				"nri.plugin":  g.compoundOwner(nri.Field_CgroupsUnified, k),
			})
		g.AddLinuxResourcesUnified(k, v)
	}
	if v := r.GetPids(); v != nil {
		g.log(AuditSetLinuxPidsLimit,
			Fields{
				"value":      v.GetLimit(),
				"nri.plugin": g.simpleOwner(nri.Field_PidsLimit),
			})
		g.SetLinuxResourcesPidsLimit(v.GetLimit())
	}
	// TODO(klihub): check this, I think it's input-only and therefore should be
	// always empty. We don't provide an adjustment setter for it and we don't
	// collect it during plugin response processing. If it is so, we should also
	// check for any plugin trying to manually set it in the response and error
	// out on the stub side if one does.
	for _, d := range r.Devices {
		g.log(AuditAddLinuxDeviceRule, Fields{
			"value.allow":  d.Allow,
			"value.type":   d.Type,
			"value.major":  d.Major.Get(),
			"value.minor":  d.Minor.Get(),
			"value.access": d.Access,
		})
		g.AddLinuxResourcesDevice(d.Allow, d.Type, d.Major.Get(), d.Minor.Get(), d.Access)
	}
	if g.checkResources != nil {
		if err := g.checkResources(g.Config.Linux.Resources); err != nil {
			return fmt.Errorf("failed to adjust resources in OCI Spec: %w", err)
		}
	}

	return nil
}

// AdjustBlockIOClass adjusts the block I/O class in the OCI Spec.
func (g *Generator) AdjustBlockIOClass(blockIOClass *string) error {
	if blockIOClass == nil || g.resolveBlockIO == nil {
		return nil
	}

	if *blockIOClass == "" {
		g.log(AuditClearLinuxBlkioClass,
			Fields{
				"nri.plugin": g.simpleOwner(nri.Field_BlockioClass),
			})
		g.ClearLinuxResourcesBlockIO()
		return nil
	}

	blockIO, err := g.resolveBlockIO(*blockIOClass)
	if err != nil {
		return fmt.Errorf("failed to adjust BlockIO class in OCI Spec: %w", err)
	}

	g.log(AuditSetLinuxBlkioClass,
		Fields{
			"value":      *blockIOClass,
			"nri.plugin": g.simpleOwner(nri.Field_BlockioClass),
		})
	g.SetLinuxResourcesBlockIO(blockIO)
	return nil
}

// AdjustRdtClass adjusts the RDT class in the OCI Spec.
func (g *Generator) AdjustRdtClass(rdtClass *string) error {
	if rdtClass == nil || g.resolveRdt == nil {
		return nil
	}

	if *rdtClass == "" {
		g.log(AuditClearLinuxRdtClass,
			Fields{
				"nri.plugin": g.simpleOwner(nri.Field_RdtClass),
			})
		g.ClearLinuxIntelRdt()
		return nil
	}

	rdt, err := g.resolveRdt(*rdtClass)
	if err != nil {
		return fmt.Errorf("failed to adjust RDT class in OCI Spec: %w", err)
	}

	g.log(AuditSetLinuxRdtClass,
		Fields{
			"value":      *rdtClass,
			"nri.plugin": g.simpleOwner(nri.Field_RdtClass),
		})
	g.SetLinuxIntelRdt(rdt)
	return nil
}

// AdjustRdt adjusts the intelRdt object in the OCI Spec.
func (g *Generator) AdjustRdt(r *nri.LinuxRdt) {
	if r == nil {
		return
	}

	if r.Remove {
		g.log(AuditClearLinuxRdt, nil)
		g.ClearLinuxIntelRdt()
	}

	g.AdjustRdtClosID(r.ClosId.Get())
	g.AdjustRdtSchemata(r.Schemata.Get())
	g.AdjustRdtEnableMonitoring(r.EnableMonitoring.Get())
}

// AdjustRdtClosID adjusts the RDT CLOS id in the OCI Spec.
func (g *Generator) AdjustRdtClosID(value *string) {
	if value != nil {
		g.log(AuditSetLinuxRdtClosID,
			Fields{
				"value":      *value,
				"nri.plugin": g.simpleOwner(nri.Field_RdtClosID),
			})
		g.SetLinuxIntelRdtClosID(*value)
	}
}

// AdjustRdtSchemata adjusts the RDT schemata in the OCI Spec.
func (g *Generator) AdjustRdtSchemata(value *[]string) {
	if value != nil {
		g.log(AuditSetLinuxRdtSchemata,
			Fields{
				"value":      *value,
				"nri.plugin": g.simpleOwner(nri.Field_RdtSchemata),
			})
		g.SetLinuxIntelRdtSchemata(*value)
	}
}

// AdjustRdtEnableMonitoring adjusts the RDT monitoring in the OCI Spec.
func (g *Generator) AdjustRdtEnableMonitoring(value *bool) {
	if value != nil {
		g.log(AuditSetLinuxRdtMonitoring,
			Fields{
				"value":      *value,
				"nri.plugin": g.simpleOwner(nri.Field_RdtEnableMonitoring),
			})
		g.SetLinuxIntelRdtEnableMonitoring(*value)
	}
}

// AdjustCgroupsPath adjusts the cgroup pseudofs path in the OCI Spec.
func (g *Generator) AdjustCgroupsPath(path string) {
	if path != "" {
		g.log(AuditSetLinuxCgroupsPath,
			Fields{
				"value":      path,
				"nri.plugin": g.simpleOwner(nri.Field_CgroupsPath),
			})
		g.SetLinuxCgroupsPath(path)
	}
}

// AdjustOomScoreAdj adjusts the kernel's Out-Of-Memory (OOM) killer score for the container.
// This may override kubelet's settings for OOM score.
func (g *Generator) AdjustOomScoreAdj(score *nri.OptionalInt) {
	if score != nil {
		v := int(score.Value)
		g.log(AuditSetProcessOOMScoreAdj,
			Fields{
				"value":      v,
				"nri.plugin": g.simpleOwner(nri.Field_OomScoreAdj),
			})
		g.SetProcessOOMScoreAdj(v)
	}
}

// AdjustIOPriority adjusts the IO priority of the container.
func (g *Generator) AdjustIOPriority(ioprio *nri.LinuxIOPriority) {
	if ioprio != nil {
		g.log(AuditSetLinuxIOPriority, Fields{
			"value.class":    ioprio.Class.String(),
			"value.priority": ioprio.Priority,
			"nri.plugin":     g.simpleOwner(nri.Field_IoPriority),
		})
		g.SetProcessIOPriority(ioprio.ToOCI())
	}
}

// AdjustSeccompPolicy adjusts the seccomp policy for the container, which may
// override kubelet's settings for the seccomp policy.
func (g *Generator) AdjustSeccompPolicy(policy *nri.LinuxSeccomp) error {
	if policy == nil {
		return nil
	}

	// Note: we explicitly do not use the SetDefaultSeccompAction() and
	// SetSeccompArchitecture() helpers from generate here, because they
	// expect a "humanized" version of the action (e.g. "allow" or "x86").
	// since these helpers do not exist for the below, we would be
	// inconsistent: here we would want the humanized strings, in favor of
	// the rspec definitions like SCMP_ACT_ALLOW. let's just use the rspec
	// versions everywhere since helpers don't exist in runtime-tools for
	// setting actual syscall policies, only default actions.
	archs := make([]rspec.Arch, len(policy.Architectures))
	for i, arch := range policy.Architectures {
		archs[i] = rspec.Arch(arch)
	}

	flags := make([]rspec.LinuxSeccompFlag, len(policy.Flags))
	for i, f := range policy.Flags {
		flags[i] = rspec.LinuxSeccompFlag(f)
	}

	g.log(AuditSetLinuxSeccompPolicy,
		Fields{
			"nri.plugin": g.simpleOwner(nri.Field_SeccompPolicy),
		})
	g.Config.Linux.Seccomp = &rspec.LinuxSeccomp{
		DefaultAction:    rspec.LinuxSeccompAction(policy.DefaultAction),
		Architectures:    archs,
		ListenerPath:     policy.ListenerPath,
		ListenerMetadata: policy.ListenerMetadata,
		Flags:            flags,
		Syscalls:         nri.ToOCILinuxSyscalls(policy.Syscalls),
	}

	return nil
}

// AdjustNamespaces adds or replaces namespaces in the OCI Spec.
func (g *Generator) AdjustNamespaces(namespaces []*nri.LinuxNamespace) error {
	for _, n := range namespaces {
		if n == nil {
			continue
		}
		if key, marked := n.IsMarkedForRemoval(); marked {
			g.log(AuditRemoveLinuxNamespace,
				Fields{
					"value.type": key,
					"nri.plugin": g.compoundOwner(nri.Field_Namespace, key),
				})
			if err := g.RemoveLinuxNamespace(key); err != nil {
				return err
			}
		} else {
			g.log(AuditSetLinuxNamespace, Fields{
				"value.type": n.Type,
				"value.path": n.Path,
				"nri.plugin": g.compoundOwner(nri.Field_Namespace, n.Type),
			})
			if err := g.AddOrReplaceLinuxNamespace(n.Type, n.Path); err != nil {
				return err
			}
		}
	}
	return nil
}

// AdjustSysctl adds, replaces, or removes the sysctl settings in the OCI Spec.
func (g *Generator) AdjustSysctl(sysctl map[string]string) error {
	var err error

	if sysctl, err = g.filterSysctl(sysctl); err != nil {
		return err
	}
	for k, v := range sysctl {
		if key, marked := nri.IsMarkedForRemoval(k); marked {
			g.log(AuditRemoveLinuxSysctl,
				Fields{
					"value.key":  key,
					"nri.plugin": g.compoundOwner(nri.Field_Sysctl, key),
				})
			g.RemoveLinuxSysctl(key)
		} else {
			g.log(AuditSetLinuxSysctl,
				Fields{
					"value.key":   k,
					"value.value": v,
					"nri.plugin":  g.compoundOwner(nri.Field_Sysctl, k),
				})
			g.AddLinuxSysctl(k, v)
		}
	}

	return nil
}

// AdjustLinuxScheduler adjusts linux scheduling policy parameters.
func (g *Generator) AdjustLinuxScheduler(sch *nri.LinuxScheduler) {
	if sch == nil {
		return
	}
	g.initConfigProcess()
	g.log(AuditSetLinuxScheduler,
		Fields{
			"value.policy":   sch.Policy.String(),
			"value.nice":     sch.Nice,
			"value.priority": sch.Priority,
			"value.runtime":  sch.Runtime,
			"value.deadline": sch.Deadline,
			"value.period":   sch.Period,
			"nri.plugin":     g.simpleOwner(nri.Field_LinuxSched),
		})
	g.Config.Process.Scheduler = sch.ToOCI()
}

// AdjustDevices adjusts the (Linux) devices in the OCI Spec.
func (g *Generator) AdjustDevices(devices []*nri.LinuxDevice) {
	for _, d := range devices {
		path, marked := d.IsMarkedForRemoval()
		g.log(AuditRemoveLinuxDevice,
			Fields{
				"value.path": path,
				"nri.plugin": g.compoundOwner(nri.Field_Devices, path),
			})
		g.RemoveDevice(path)
		if marked {
			continue
		}
		g.log(AuditAddLinuxDevice, Fields{
			"value.path":  d.Path,
			"value.type":  d.Type,
			"value.major": d.Major,
			"value.minor": d.Minor,
			"nri.plugin":  g.compoundOwner(nri.Field_Devices, d.Path),
		})
		g.AddDevice(d.ToOCI())
		major, minor, access := &d.Major, &d.Minor, d.AccessString()
		g.log(AuditAddLinuxDeviceRule, Fields{
			"value.allow":  true,
			"value.type":   d.Type,
			"value.major":  d.Major,
			"value.minor":  d.Minor,
			"value.access": access,
			"nri.plugin":   g.compoundOwner(nri.Field_Devices, d.Path),
		})
		g.AddLinuxResourcesDevice(true, d.Type, major, minor, access)
	}
}

// AdjustLinuxNetDevices adjusts the linux net devices in the OCI Spec.
func (g *Generator) AdjustLinuxNetDevices(devices map[string]*nri.LinuxNetDevice) error {
	for k, v := range devices {
		if key, marked := nri.IsMarkedForRemoval(k); marked {
			g.log(AuditRemoveLinuxNetDevice,
				Fields{
					"value.hostif": key,
					"nri.plugin":   g.compoundOwner(nri.Field_LinuxNetDevices, key),
				})
			g.RemoveLinuxNetDevice(key)
		} else {
			g.log(AuditAddLinuxNetDevice,
				Fields{
					"value.hostif":      k,
					"value.containerif": v,
					"nri.plugin":        g.compoundOwner(nri.Field_LinuxNetDevices, k),
				})
			g.AddLinuxNetDevice(k, v)
		}
	}

	return nil
}

// InjectCDIDevices injects the requested CDI devices into the OCI Spec.
// Devices are given by their fully qualified CDI device names. The
// actual device injection is done using a runtime-specific CDI
// injection function, set using the WithCDIDeviceInjector option.
func (g *Generator) InjectCDIDevices(devices []*nri.CDIDevice) error {
	if len(devices) == 0 || g.injectCDIDevices == nil {
		return nil
	}

	names := []string{}
	plugins := []string{}
	for _, d := range devices {
		names = append(names, d.Name)
		plugins = append(plugins, g.compoundOwner(nri.Field_CdiDevices, d.Name))
	}

	g.log(AuditInjectCDIDevices,
		Fields{
			"value":      strings.Join(names, ","),
			"nri.plugin": strings.Join(plugins, ","),
		})
	return g.injectCDIDevices(g.Config, names)
}

// AdjustRlimits adjusts the (Linux) POSIX resource limits in the OCI Spec.
func (g *Generator) AdjustRlimits(rlimits []*nri.POSIXRlimit) error {
	for _, l := range rlimits {
		if l == nil {
			continue
		}
		g.log(AuditAddProcessRlimits, Fields{
			"value.type": l.Type,
			"value.soft": l.Soft,
			"value.hard": l.Hard,
			"nri.plugin": g.compoundOwner(nri.Field_Rlimits, l.Type),
		})
		g.AddProcessRlimits(l.Type, l.Hard, l.Soft)
	}
	return nil
}

// AdjustMounts adjusts the mounts in the OCI Spec.
func (g *Generator) AdjustMounts(mounts []*nri.Mount) error {
	if len(mounts) == 0 {
		return nil
	}

	propagation := ""
	for _, m := range mounts {
		if destination, marked := m.IsMarkedForRemoval(); marked {
			g.log(AuditRemoveMount,
				Fields{
					"value.destination": destination,
					"nri.plugin":        g.compoundOwner(nri.Field_Mounts, destination),
				},
			)
			g.RemoveMount(destination)
			continue
		}

		plugin := g.compoundOwner(nri.Field_Mounts, m.Destination)
		g.log(AuditRemoveMount,
			Fields{
				"value.destination": m.Destination,
				"nri.plugin":        plugin,
			},
		)
		g.RemoveMount(m.Destination)

		mnt := m.ToOCI(&propagation)
		switch propagation {
		case "rprivate":
		case "rshared":
			if err := ensurePropagation(mnt.Source, "rshared"); err != nil {
				return fmt.Errorf("failed to adjust mounts in OCI Spec: %w", err)
			}
			g.log(AuditSetLinuxRootPropagation,
				Fields{
					"value":      "rshared",
					"nri.plugin": plugin,
				},
			)
			if err := g.SetLinuxRootPropagation("rshared"); err != nil {
				return fmt.Errorf("failed to adjust rootfs propagation in OCI Spec: %w", err)
			}
		case "rslave":
			if err := ensurePropagation(mnt.Source, "rshared", "rslave"); err != nil {
				return fmt.Errorf("failed to adjust mounts in OCI Spec: %w", err)
			}
			rootProp := g.Config.Linux.RootfsPropagation
			if rootProp != "rshared" && rootProp != "rslave" {
				g.log(AuditSetLinuxRootPropagation,
					Fields{
						"value":      "rslave",
						"nri.plugin": plugin,
					})
				if err := g.SetLinuxRootPropagation("rslave"); err != nil {
					return fmt.Errorf("failed to adjust rootfs propagation in OCI Spec: %w", err)
				}
			}
		}
		g.log(AuditAddMount, Fields{
			"value.destination": mnt.Destination,
			"value.source":      mnt.Source,
			"value.type":        mnt.Type,
			"value.options":     strings.Join(mnt.Options, ","),
			"nri.plugin":        plugin,
		})
		g.AddMount(mnt)
	}
	g.sortMounts()

	return nil
}

// sortMounts sorts the mounts in the generated OCI Spec.
func (g *Generator) sortMounts() {
	mounts := g.Mounts()
	g.ClearMounts()
	sort.Sort(orderedMounts(mounts))

	// TODO(klihub): This is now a bit ugly maybe we should introduce a
	// SetMounts([]rspec.Mount) to runtime-tools/generate.Generator. That
	// could also take care of properly sorting the mount slice.

	g.Config.Mounts = mounts
}

// orderedMounts defines how to sort an OCI Spec Mount slice.
// This is the almost the same implementation sa used by CRI-O and Docker,
// with a minor tweak for stable sorting order (easier to test):
//
//	https://github.com/moby/moby/blob/17.05.x/daemon/volumes.go#L26
type orderedMounts []rspec.Mount

// Len returns the number of mounts. Used in sorting.
func (m orderedMounts) Len() int {
	return len(m)
}

// Less returns true if the number of parts (a/b/c would be 3 parts) in the
// mount indexed by parameter 1 is less than that of the mount indexed by
// parameter 2. Used in sorting.
func (m orderedMounts) Less(i, j int) bool {
	ip, jp := m.parts(i), m.parts(j)
	if ip < jp {
		return true
	}
	if jp < ip {
		return false
	}
	return m[i].Destination < m[j].Destination
}

// Swap swaps two items in an array of mounts. Used in sorting
func (m orderedMounts) Swap(i, j int) {
	m[i], m[j] = m[j], m[i]
}

// parts returns the number of parts in the destination of a mount. Used in sorting.
func (m orderedMounts) parts(i int) int {
	return strings.Count(filepath.Clean(m[i].Destination), string(os.PathSeparator))
}

func nopFilter(m map[string]string) (map[string]string, error) {
	return m, nil
}

//
// TODO: these could be added to the stock Spec generator...
//

// AddCreateRuntimeHook adds a hooks new CreateRuntime hooks.
func (g *Generator) AddCreateRuntimeHook(hook rspec.Hook) {
	g.initConfigHooks()
	g.Config.Hooks.CreateRuntime = append(g.Config.Hooks.CreateRuntime, hook)
}

// AddCreateContainerHook adds a hooks new CreateContainer hooks.
func (g *Generator) AddCreateContainerHook(hook rspec.Hook) {
	g.initConfigHooks()
	g.Config.Hooks.CreateContainer = append(g.Config.Hooks.CreateContainer, hook)
}

// AddStartContainerHook adds a hooks new StartContainer hooks.
func (g *Generator) AddStartContainerHook(hook rspec.Hook) {
	g.initConfigHooks()
	g.Config.Hooks.StartContainer = append(g.Config.Hooks.StartContainer, hook)
}

// ClearLinuxIntelRdt clears RDT CLOS.
func (g *Generator) ClearLinuxIntelRdt() {
	g.initConfigLinux()
	g.Config.Linux.IntelRdt = nil
}

// SetLinuxIntelRdt sets RDT CLOS.
func (g *Generator) SetLinuxIntelRdt(rdt *rspec.LinuxIntelRdt) {
	g.initConfigLinux()
	g.Config.Linux.IntelRdt = rdt
}

// SetLinuxIntelRdtClosID sets g.Config.Linux.IntelRdt.ClosID
func (g *Generator) SetLinuxIntelRdtClosID(closID string) {
	g.initConfigLinuxIntelRdt()
	g.Config.Linux.IntelRdt.ClosID = closID
}

// SetLinuxIntelRdtEnableMonitoring sets g.Config.Linux.IntelRdt.EnableMonitoring
func (g *Generator) SetLinuxIntelRdtEnableMonitoring(value bool) {
	g.initConfigLinuxIntelRdt()
	g.Config.Linux.IntelRdt.EnableMonitoring = value
}

// SetLinuxIntelRdtSchemata sets g.Config.Linux.IntelRdt.Schemata
func (g *Generator) SetLinuxIntelRdtSchemata(schemata []string) {
	g.initConfigLinuxIntelRdt()
	g.Config.Linux.IntelRdt.Schemata = slices.Clone(schemata)
}

// ClearLinuxResourcesBlockIO clears Block I/O settings.
func (g *Generator) ClearLinuxResourcesBlockIO() {
	g.initConfigLinuxResources()
	g.Config.Linux.Resources.BlockIO = nil
}

// SetLinuxResourcesBlockIO sets Block I/O settings.
func (g *Generator) SetLinuxResourcesBlockIO(blockIO *rspec.LinuxBlockIO) {
	g.initConfigLinuxResources()
	g.Config.Linux.Resources.BlockIO = blockIO
}

// SetProcessIOPriority sets the (Linux) IO priority of the container.
func (g *Generator) SetProcessIOPriority(ioprio *rspec.LinuxIOPriority) {
	g.initConfigProcess()
	if ioprio != nil && ioprio.Class == "" {
		ioprio = nil
	}
	g.Config.Process.IOPriority = ioprio
}

// SetLinuxResourcesPidsLimit sets Linux PID limit. Starting with
// v1.3.0 opencontainers/runtime-spec switched the PID limit to
// *int64 from int64 with nil meaning "unlimited". We don't want
// to change our API types though, so instead we use a dedicated
// value for unlimited.
func (g *Generator) SetLinuxResourcesPidsLimit(limit int64) {
	g.initConfigLinuxResources()
	if g.Config.Linux.Resources.Pids == nil {
		g.Config.Linux.Resources.Pids = &rspec.LinuxPids{}
	}
	if limit > UnlimitedPidsLimit {
		g.Config.Linux.Resources.Pids.Limit = &limit
	}
}

// AddLinuxNetDevice adds a new Linux net device.
func (g *Generator) AddLinuxNetDevice(hostDev string, device *nri.LinuxNetDevice) {
	if device == nil {
		return
	}
	g.initConfigLinuxNetDevices()
	g.Config.Linux.NetDevices[hostDev] = device.ToOCI()
}

// RemoveLinuxNetDevice removes a Linux net device.
func (g *Generator) RemoveLinuxNetDevice(hostDev string) {
	g.initConfigLinuxNetDevices()
	delete(g.Config.Linux.NetDevices, hostDev)
}

func (g *Generator) initConfig() {
	if g.Config == nil {
		g.Config = &rspec.Spec{}
	}
}

func (g *Generator) initConfigProcess() {
	g.initConfig()
	if g.Config.Process == nil {
		g.Config.Process = &rspec.Process{}
	}
}

func (g *Generator) initConfigHooks() {
	g.initConfig()
	if g.Config.Hooks == nil {
		g.Config.Hooks = &rspec.Hooks{}
	}
}

func (g *Generator) initConfigLinux() {
	g.initConfig()
	if g.Config.Linux == nil {
		g.Config.Linux = &rspec.Linux{}
	}
}

func (g *Generator) initConfigLinuxResources() {
	g.initConfigLinux()
	if g.Config.Linux.Resources == nil {
		g.Config.Linux.Resources = &rspec.LinuxResources{}
	}
}

func (g *Generator) initConfigLinuxNetDevices() {
	g.initConfigLinux()
	if g.Config.Linux.NetDevices == nil {
		g.Config.Linux.NetDevices = map[string]rspec.LinuxNetDevice{}
	}
}

func (g *Generator) initConfigLinuxIntelRdt() {
	g.initConfigLinux()
	if g.Config.Linux.IntelRdt == nil {
		g.Config.Linux.IntelRdt = &rspec.LinuxIntelRdt{}
	}
}

func (g *Generator) log(event string, fields Fields) {
	if g.logger != nil {
		g.logger(event, fields)
	}
}

func (g *Generator) simpleOwner(field nri.Field) string {
	owner := "unknown"

	if g.owners != nil {
		owner, _ = g.owners.SimpleOwner(field.Key())
	}

	return owner
}

func (g *Generator) compoundOwner(field nri.Field, subField string) string {
	owner := "unknown"

	if g.owners != nil {
		owner, _ = g.owners.CompoundOwner(field.Key(), subField)
	}

	return owner
}

// Audit 'events' we use in logged audit messages.
const ( //nolint:revive
	AuditRemoveProcessEnv           = "remove environment variable"
	AuditAddProcessEnv              = "add environment variable"
	AuditSetProcessArgs             = "set process arguments"
	AuditRemoveAnnotation           = "remove annotation"
	AuditAddAnnotation              = "add annotation"
	AuditAddOCIHook                 = "add OCI hook"
	AuditSetLinuxCPUPeriod          = "set linux CPU period"
	AuditSetLinuxCPUQuota           = "set linux CPU quota"
	AuditSetLinuxCPUShares          = "set linux CPU shares"
	AuditSetLinuxCPUSetCPUs         = "set linux cpuset CPUs"
	AuditSetLinuxCPUSetMems         = "set linux cpuset mems"
	AuditSetLinuxCPURealtimeRuntime = "set linux cpuset mems"
	AuditSetLinuxCPURealtimePeriod  = "set linux cpuset mems"
	AuditSetLinuxMemLimit           = "set linux memory limit"
	AuditSetLinuxMemSwapLimit       = "set linux swap limit"
	AuditSetLinuxHugepageLimit      = "set linux hugepage limit"
	AuditSetLinuxResourceUnified    = "set linux cgroups unified resource"
	AuditSetLinuxPidsLimit          = "set linux PIDs limit"
	AuditClearLinuxBlkioClass       = "clear linux blkio class"
	AuditSetLinuxBlkioClass         = "set linux blkio class"
	AuditClearLinuxRdtClass         = "clear linux RDT class"
	AuditSetLinuxRdtClass           = "set linux RDT class"
	AuditClearLinuxRdt              = "clear linux RDT"
	AuditSetLinuxRdtClosID          = "set linux RDT CLOS ID"
	AuditSetLinuxRdtSchemata        = "set linux RDT schemata"
	AuditSetLinuxRdtMonitoring      = "set linux RDT monitoring"
	AuditSetLinuxCgroupsPath        = "set linux cgroups path"
	AuditSetProcessOOMScoreAdj      = "set process OOM score adjustment"
	AuditSetLinuxIOPriority         = "set process IO priority"
	AuditSetLinuxSeccompPolicy      = "set linux seccomp policy"
	AuditRemoveLinuxNamespace       = "remove linux namespace"
	AuditSetLinuxNamespace          = "set linux namespace"
	AuditRemoveLinuxSysctl          = "remove linux sysctl"
	AuditSetLinuxSysctl             = "set linux sysctl"
	AuditSetLinuxScheduler          = "set linux scheduler"
	AuditRemoveLinuxDevice          = "remove linux device"
	AuditAddLinuxDevice             = "add linux device"
	AuditAddLinuxDeviceRule         = "add linux device rule"
	AuditRemoveLinuxNetDevice       = "remove linux net device"
	AuditAddLinuxNetDevice          = "add linux net device"
	AuditInjectCDIDevices           = "inject CDI devices"
	AuditAddProcessRlimits          = "add process rlimits"
	AuditRemoveMount                = "remove mount"
	AuditAddMount                   = "add mount"
	AuditSetLinuxRootPropagation    = "set linux root propagation"
)
