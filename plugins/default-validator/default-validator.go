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

package validator

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	yaml "gopkg.in/yaml.v3"

	"github.com/containerd/nri/pkg/api"
	"github.com/containerd/nri/pkg/log"
	"github.com/containerd/nri/pkg/plugin"
)

// DefaultValidatorConfig is the configuration for the default validator plugin.
type DefaultValidatorConfig struct {
	// Enable the default validator plugin.
	Enable bool `yaml:"enable" toml:"enable"`
	*Config
	// Roles provide per-role overrides to the default configuration above.
	Roles map[string]*Config `yaml:"roles" toml:"roles"`
	// RequiredPlugins list globally required plugins. These must be present
	// or otherwise validation will fail.
	// WARNING: This is a global setting and will affect all containers. In
	// particular, if you configure any globally required plugins, you should
	// annotate your static pods to tolerate missing plugins. Failing to do
	// so will prevent static pods from starting.
	// Notes:
	//   Containers can be annotated to tolerate missing plugins using the
	//   toleration annotation, if one is set.
	RequiredPlugins []string `yaml:"requiredPlugins" toml:"required_plugins"`
	// TolerateMissingPlugins is an optional annotation key. If set, it can
	// be used to annotate containers to tolerate missing required plugins.
	TolerateMissingAnnotation string `yaml:"tolerateMissingPluginsAnnotation" toml:"tolerate_missing_plugins_annotation"`
}

// Config provides validation defaults or per role configuration.
type Config struct {
	// RejectOCIHookAdjustment fails validation if OCI hooks are adjusted.
	RejectOCIHookAdjustment *bool `yaml:"rejectOCIHookAdjustment" toml:"reject_oci_hook_adjustment"`
	// RejectRuntimeDefaultSeccompAdjustment fails validation if a runtime default seccomp
	// policy is adjusted.
	RejectRuntimeDefaultSeccompAdjustment *bool `yaml:"rejectRuntimeDefaultSeccompAdjustment" toml:"reject_runtime_default_seccomp_adjustment"`
	// RejectUnconfinedSeccompAdjustment fails validation if an unconfined seccomp policy is
	// adjusted.
	RejectUnconfinedSeccompAdjustment *bool `yaml:"rejectUnconfinedSeccompAdjustment" toml:"reject_unconfined_seccomp_adjustment"`
	// RejectCustomSeccompAdjustment fails validation if a custom seccomp policy (aka LOCALHOST)
	// is adjusted.
	RejectCustomSeccompAdjustment *bool `yaml:"rejectCustomSeccompAdjustment" toml:"reject_custom_seccomp_adjustment"`
	// RejectNamespaceAdjustment fails validation if any plugin adjusts Linux namespaces.
	RejectNamespaceAdjustment *bool `yaml:"rejectNamespaceAdjustment" toml:"reject_namespace_adjustment"`
	// RejectSysctlAdjustment fails validation if any plugin adjusts sysctls
	RejectSysctlAdjustment *bool `yaml:"rejectSysctlAdjustment" toml:"reject_sysctl_adjustment"`
}

// DefaultValidator implements default validation.
type DefaultValidator struct {
	cfg DefaultValidatorConfig
}

const (
	// RequiredPlugins is the annotation key for extra required plugins.
	RequiredPlugins = plugin.RequiredPluginsAnnotation
)

var (
	// ErrValidation is returned if validation rejects an adjustment.
	ErrValidation = errors.New("validation error")
)

// NewDefaultValidator creates a new instance of the validator.
func NewDefaultValidator(cfg *DefaultValidatorConfig) *DefaultValidator {
	return &DefaultValidator{cfg: *cfg}
}

// SetConfig sets new configuration for the validator.
func (v *DefaultValidator) SetConfig(cfg *DefaultValidatorConfig) {
	if cfg == nil {
		return
	}
	v.cfg = *cfg
}

// ValidateContainerAdjustment validates a container adjustment.
func (v *DefaultValidator) ValidateContainerAdjustment(ctx context.Context, req *api.ValidateContainerAdjustmentRequest) error {
	log.Debugf(ctx, "Validating adjustment of container %s/%s/%s",
		req.GetPod().GetNamespace(), req.GetPod().GetName(), req.GetContainer().GetName())

	plugins := req.GetPluginMap()

	if err := v.validateOCIHooks(req, plugins); err != nil {
		log.Errorf(ctx, "rejecting adjustment: %v", err)
		return err
	}

	if err := v.validateSeccompPolicy(req, plugins); err != nil {
		log.Errorf(ctx, "rejecting adjustment: %v", err)
		return err
	}

	if err := v.validateNamespaces(req, plugins); err != nil {
		log.Errorf(ctx, "rejecting adjustment: %v", err)
		return err
	}

	if err := v.validateRequiredPlugins(req, plugins); err != nil {
		log.Errorf(ctx, "rejecting adjustment: %v", err)
		return err
	}

	if err := v.validateSysctl(req, plugins); err != nil {
		log.Errorf(ctx, "rejecting adjustment: %v", err)
		return err
	}

	return nil
}

func (v *DefaultValidator) validateOCIHooks(req *api.ValidateContainerAdjustmentRequest, plugins map[string]*api.PluginInstance) error {
	if req.Adjust == nil {
		return nil
	}

	owners, claimed := req.Owners.HooksOwner(req.Container.Id)
	if !claimed {
		return nil
	}

	defaults := v.cfg.Config
	rejected := []string{}

	for _, p := range strings.Split(owners, ",") {
		if instance, ok := plugins[p]; ok {
			cfg := v.cfg.GetConfig(instance.GetRole())
			if cfg.DenyOCIHookInjection(defaults) {
				rejected = append(rejected, p)
			}
		}
	}

	if len(rejected) == 0 {
		return nil
	}

	offender := fmt.Sprintf("plugin(s) %q", strings.Join(rejected, ","))

	return fmt.Errorf("%w: %s attempted restricted OCI hook injection", ErrValidation, offender)
}

func (v *DefaultValidator) validateSeccompPolicy(req *api.ValidateContainerAdjustmentRequest, plugins map[string]*api.PluginInstance) error {
	if req.Adjust == nil {
		return nil
	}

	owner, claimed := req.Owners.SeccompPolicyOwner(req.Container.Id)
	if !claimed {
		return nil
	}

	var (
		cfg      *Config
		defaults = v.cfg.Config
	)

	if instance, ok := plugins[owner]; ok {
		cfg = v.cfg.GetConfig(instance.GetRole())
	}

	profile := req.Container.GetLinux().GetSeccompProfile()
	switch {
	case profile == nil || profile.GetProfileType() == api.SecurityProfile_UNCONFINED:
		if cfg.DenyUnconfinedSeccompAdjustment(defaults) {
			return fmt.Errorf("%w: plugin %s attempted restricted "+
				" unconfined seccomp policy adjustment", ErrValidation, owner)
		}

	case profile.GetProfileType() == api.SecurityProfile_RUNTIME_DEFAULT:
		if cfg.DenyRuntimeDefaultSeccompAdjustment(defaults) {
			return fmt.Errorf("%w: plugin %s attempted restricted "+
				"runtime default seccomp policy adjustment", ErrValidation, owner)
		}

	case profile.GetProfileType() == api.SecurityProfile_LOCALHOST:
		if cfg.DenyCustomSeccompAdjustment(defaults) {
			return fmt.Errorf("%w: plugin %s attempted restricted "+
				" custom seccomp policy adjustment", ErrValidation, owner)
		}
	}

	return nil
}

func (v *DefaultValidator) validateNamespaces(req *api.ValidateContainerAdjustmentRequest, plugins map[string]*api.PluginInstance) error {
	if req.Adjust == nil {
		return nil
	}

	owners, claimed := req.Owners.NamespaceOwners(req.Container.Id)
	if !claimed {
		return nil
	}

	defaults := v.cfg.Config
	rejected := []string{}

	for ns, p := range owners {
		if instance, ok := plugins[p]; ok {
			cfg := v.cfg.GetConfig(instance.GetRole())
			if cfg.DenyNamespaceAdjustment(defaults) {
				rejected = append(rejected, ns)
			}
		}
	}

	if len(rejected) == 0 {
		return nil
	}

	offenders := ""
	sep := ""

	for _, ns := range rejected {
		plugin := owners[ns]
		offenders += sep + fmt.Sprintf("%q (namespace %q)", plugin, ns)
		sep = ", "
	}

	return fmt.Errorf("%w: attempted restricted namespace adjustment by %s",
		ErrValidation, offenders)
}

func (v *DefaultValidator) validateSysctl(req *api.ValidateContainerAdjustmentRequest, plugins map[string]*api.PluginInstance) error {
	if req.Adjust == nil || req.Adjust.Linux == nil {
		return nil
	}

	var (
		defaults = v.cfg.Config
		cfg      *Config
		owners   []string
		rejected []string
	)

	for key := range req.Adjust.Linux.Sysctl {
		owner, claimed := req.Owners.SysctlOwner(req.Container.Id, key)
		if !claimed {
			continue
		}

		if instance, ok := plugins[owner]; ok {
			cfg = v.cfg.GetConfig(instance.GetRole())
		}

		if cfg.DenySysctlAdjustment(defaults) {
			rejected = append(rejected, key)
			owners = append(owners, owner)
		}
	}

	if len(owners) == 0 {
		return nil
	}

	return fmt.Errorf("%w: attempted restricted sysctl adjustment of key(s) %s by plugin(s) %s", ErrValidation, strings.Join(rejected, ","), strings.Join(owners, ", "))
}

func (v *DefaultValidator) validateRequiredPlugins(req *api.ValidateContainerAdjustmentRequest, plugins map[string]*api.PluginInstance) error {
	var (
		container = req.GetContainer().GetName()
		required  = slices.Clone(v.cfg.RequiredPlugins)
	)

	if tolerateMissing := v.cfg.TolerateMissingAnnotation; tolerateMissing != "" {
		value, ok := plugin.GetEffectiveAnnotation(req.GetPod(), tolerateMissing, container)
		if ok {
			tolerate, err := strconv.ParseBool(value)
			if err != nil {
				return fmt.Errorf("invalid %s annotation %q: %w", tolerateMissing, value, err)
			}
			if tolerate {
				return nil
			}
		}
	}

	if value, ok := plugin.GetEffectiveAnnotation(req.GetPod(), RequiredPlugins, container); ok {
		var annotated []string
		if err := yaml.Unmarshal([]byte(value), &annotated); err != nil {
			return fmt.Errorf("invalid %s annotation %q: %w", RequiredPlugins, value, err)
		}
		required = append(required, annotated...)
	}

	if len(required) == 0 {
		return nil
	}

	missing := []string{}

	for _, r := range required {
		if _, ok := plugins[r]; !ok {
			missing = append(missing, r)
		}
	}

	if len(missing) == 0 {
		return nil
	}

	offender := ""

	if len(missing) == 1 {
		offender = fmt.Sprintf("required plugin %q", missing[0])
	} else {
		offender = fmt.Sprintf("required plugins %q", strings.Join(missing, ","))
	}

	return fmt.Errorf("%w: %s not present", ErrValidation, offender)
}

// GetConfig returns overrides for the named role if it exists in the
// configuration.
func (cfg *DefaultValidatorConfig) GetConfig(role string) *Config {
	if cfg == nil || cfg.Roles == nil {
		return nil
	}
	return cfg.Roles[role]
}

// DenyOCIHookInjection checks whether OCI hook injection should be denied
// based on the configuration, using an optional fallbak configuration if
// this one is nil or omits configuration.
func (cfg *Config) DenyOCIHookInjection(fallback *Config) bool {
	if cfg != nil && cfg.RejectOCIHookAdjustment != nil {
		return *cfg.RejectOCIHookAdjustment
	}

	return fallback != nil && fallback.DenyOCIHookInjection(nil)
}

// DenyUnconfinedSeccompAdjustment checks whether adjustment of an unconfined
// seccomp policy should be denied based on the configuration, using an optional
// fallback configuration if this one is nil or omits configuration.
func (cfg *Config) DenyUnconfinedSeccompAdjustment(fallback *Config) bool {
	if cfg != nil && cfg.RejectUnconfinedSeccompAdjustment != nil {
		return *cfg.RejectUnconfinedSeccompAdjustment
	}

	return fallback != nil && fallback.DenyUnconfinedSeccompAdjustment(nil)
}

// DenyRuntimeDefaultSeccompAdjustment checks whether adjustment of a runtime
// default seccomp policy should be denied based on the configuration, using an
// optional fallback configuration if this one is nil or omits configuration.
func (cfg *Config) DenyRuntimeDefaultSeccompAdjustment(fallback *Config) bool {
	if cfg != nil && cfg.RejectRuntimeDefaultSeccompAdjustment != nil {
		return *cfg.RejectRuntimeDefaultSeccompAdjustment
	}

	return fallback != nil && fallback.DenyRuntimeDefaultSeccompAdjustment(nil)
}

// DenyCustomSeccompAdjustment checks whether adjustment of a custom (localhost)
// seccomp policy should be denied based on the configuration, using an optional
// fallback configuration if this one is nil or omits configuration.
func (cfg *Config) DenyCustomSeccompAdjustment(fallback *Config) bool {
	if cfg != nil && cfg.RejectCustomSeccompAdjustment != nil {
		return *cfg.RejectCustomSeccompAdjustment
	}

	return fallback != nil && fallback.DenyCustomSeccompAdjustment(nil)
}

// DenyNamespaceAdjustment checks whether adjustment of Linux namespace should
// be denied based on the configuration, using an optional fallback configuration
// if this one is nil or omits configuration.
func (cfg *Config) DenyNamespaceAdjustment(fallback *Config) bool {
	if cfg != nil && cfg.RejectNamespaceAdjustment != nil {
		return *cfg.RejectNamespaceAdjustment
	}

	return fallback != nil && fallback.DenyNamespaceAdjustment(nil)
}

// DenySysctlAdjustment checks whether adjustment of Linux sysctl entries should
// be denied based on the configuration, using an optional fallback configuration
// if this one is nil or omits configuration.
func (cfg *Config) DenySysctlAdjustment(fallback *Config) bool {
	if cfg != nil && cfg.RejectSysctlAdjustment != nil {
		return *cfg.RejectSysctlAdjustment
	}

	return fallback != nil && fallback.DenySysctlAdjustment(nil)
}
