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
	"strings"

	"github.com/containerd/nri/pkg/api"
	"github.com/containerd/nri/pkg/log"
	yaml "gopkg.in/yaml.v3"
)

type DefaultValidatorConfig struct {
	// Enable the default validator plugin.
	Enable bool `yaml:"enable" toml:"enable"`
	// RejectOCIHooks fails validation if any NRI plugin injects OCI hooks.
	RejectOCIHooks bool `yaml:"rejectOCIHooks" toml:"reject_oci_hooks"`
	// RequiredPlugins list globally required plugins. These must be present
	// or otherwise validation will fail.
	// WARNING: This is a global setting and will affect all containers. In
	// particular, if you configure any globally required plugins, you should
	// annotate your static pods to tolerate missing plugins. Failing to do
	// so will prevent static pods from starting.
	// Notes:
	//   Containers can be annotated to tolerate missing plugins using the
	//   "tolerate-missing-plugins.nri.io" boolean annotation.
	RequiredPlugins []string `yaml:"requiredPlugins" toml:"required_plugins"`
}

type DefaultValidator struct {
	cfg DefaultValidatorConfig
}

var (
	ErrValidation = errors.New("validation error")
)

const (
	RequiredPlugins        = api.RequiredPluginsAnnotation
	TolerateMissingPlugins = api.TolerateMissingPluginsAnnotation
)

// NewDefaultValidator creates a new instance of the default validator plugin.
func NewDefaultValidator(cfg *DefaultValidatorConfig) *DefaultValidator {
	return &DefaultValidator{cfg: *cfg}
}

func (v *DefaultValidator) SetConfig(cfg *DefaultValidatorConfig) {
	if cfg == nil {
		return
	}
	v.cfg = *cfg
}

func (v *DefaultValidator) ValidateContainerAdjustment(ctx context.Context, req *api.ValidateContainerAdjustmentRequest) error {
	log.Debugf(ctx, "Validating container adjustment of %s/%s/%s",
		req.GetPod().GetNamespace(), req.GetPod().GetName(), req.GetContainer().GetName())

	if err := v.validateOCIHooks(req); err != nil {
		log.Errorf(ctx, "rejecting adjusted container: %v", err)
		return err
	}

	if err := v.validateRequiredPlugins(req); err != nil {
		log.Errorf(ctx, "rejecting adjusted container: %v", err)
		return err
	}

	return nil
}

func (v *DefaultValidator) validateOCIHooks(req *api.ValidateContainerAdjustmentRequest) error {
	if !v.cfg.RejectOCIHooks || req.Adjust == nil {
		return nil
	}

	if plugins, claimed := req.Owners.HooksOwner(req.Container.Id); claimed {
		what := "plugin"
		if strings.Contains(plugins, ",") {
			what = "plugins"
		}

		return fmt.Errorf("%w: %s %q attempted restricted hook injection",
			ErrValidation, what, plugins)
	}

	return nil
}

func (v *DefaultValidator) validateRequiredPlugins(req *api.ValidateContainerAdjustmentRequest) error {
	var (
		container = req.GetContainer().GetName()
		required  = slices.Clone(v.cfg.RequiredPlugins)
	)

	value, ok := req.GetPod().GetEffectiveAnnotation(TolerateMissingPlugins, container)
	if ok {
		tolerateMissing := false
		if err := yaml.Unmarshal([]byte(value), &tolerateMissing); err != nil {
			return fmt.Errorf("invalid %s annotation %q: %w", TolerateMissingPlugins, value, err)
		}
		if tolerateMissing {
			return nil
		}
	}

	value, ok = req.GetPod().GetEffectiveAnnotation(RequiredPlugins, container)
	if ok {
		var annotated []string
		if err := yaml.Unmarshal([]byte(value), &annotated); err != nil {
			return fmt.Errorf("invalid %s annotation %q: %w", RequiredPlugins, value, err)
		}
		required = append(required, annotated...)
	}

	if len(required) == 0 {
		return nil
	}

	present := map[string]struct{}{}
	for _, p := range req.Plugins {
		if p != nil {
			present[p.Name] = struct{}{}
		}
	}

	for _, r := range required {
		if _, ok := present[r]; !ok {
			return fmt.Errorf("%w: required plugin %q not present", ErrValidation, r)
		}
	}

	return nil
}
