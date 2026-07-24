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
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/containerd/nri/pkg/api"
)

func TestValidateRequiredPlugins(t *testing.T) {
	type testCase struct {
		name      string
		cfg       *DefaultValidatorConfig
		pod       *api.PodSandbox
		container *api.Container
		plugins   []*api.PluginInstance
		fail      bool
	}

	for _, tc := range []*testCase{
		{
			name: "no required plugins",
			cfg: &DefaultValidatorConfig{
				Enable: true,
			},
			pod: &api.PodSandbox{
				Id:        "pod-id",
				Name:      "pod-name",
				Namespace: "pod-namespace",
			},
			container: &api.Container{
				Id:   "container-id",
				Name: "container-name",
			},
		},
		{
			name: "missing annotated required plugin",
			cfg: &DefaultValidatorConfig{
				Enable: true,
			},
			pod: &api.PodSandbox{
				Id:        "pod-id",
				Name:      "pod-name",
				Namespace: "pod-namespace",
				Annotations: map[string]string{
					"required-plugins.noderesource.dev/container.container-name": "[ plugin ]",
				},
			},
			container: &api.Container{
				Id:   "container-id",
				Name: "container-name",
			},
			fail: true,
		},
		{
			name: "present annotated required plugin",
			cfg: &DefaultValidatorConfig{
				Enable: true,
			},
			pod: &api.PodSandbox{
				Id:        "pod-id",
				Name:      "pod-name",
				Namespace: "pod-namespace",
				Annotations: map[string]string{
					"required-plugins.noderesource.dev/container.container-name": "[ plugin ]",
				},
			},
			container: &api.Container{
				Id:   "container-id",
				Name: "container-name",
			},
			plugins: []*api.PluginInstance{
				{
					Name:  "plugin",
					Index: "00",
				},
			},
		},

		{
			name: "missing global required plugin",
			cfg: &DefaultValidatorConfig{
				Enable:          true,
				RequiredPlugins: []string{"plugin"},
			},
			pod: &api.PodSandbox{
				Id:        "pod-id",
				Name:      "pod-name",
				Namespace: "pod-namespace",
			},
			container: &api.Container{
				Id:   "container-id",
				Name: "container-name",
			},
			fail: true,
		},
		{
			name: "present global required plugin",
			cfg: &DefaultValidatorConfig{
				Enable:          true,
				RequiredPlugins: []string{"plugin"},
			},
			pod: &api.PodSandbox{
				Id:        "pod-id",
				Name:      "pod-name",
				Namespace: "pod-namespace",
			},
			container: &api.Container{
				Id:   "container-id",
				Name: "container-name",
			},
			plugins: []*api.PluginInstance{
				{
					Name:  "plugin",
					Index: "00",
				},
			},
		},
		{
			name: "tolerated missing (global required) plugin",
			cfg: &DefaultValidatorConfig{
				Enable:                    true,
				RequiredPlugins:           []string{"plugin"},
				TolerateMissingAnnotation: "tolerate-missing-plugins",
			},
			pod: &api.PodSandbox{
				Id:        "pod-id",
				Name:      "pod-name",
				Namespace: "pod-namespace",
				Annotations: map[string]string{
					"tolerate-missing-plugins": "true",
				},
			},
			container: &api.Container{
				Id:   "container-id",
				Name: "container-name",
			},
		},
		{
			name: "present annotated and global required plugin",
			cfg: &DefaultValidatorConfig{
				Enable:          true,
				RequiredPlugins: []string{"plugin1"},
			},
			pod: &api.PodSandbox{
				Id:        "pod-id",
				Name:      "pod-name",
				Namespace: "pod-namespace",
				Annotations: map[string]string{
					"required-plugins.noderesource.dev/container.container-name": "[ plugin2 ]",
				},
			},
			container: &api.Container{
				Id:   "container-id",
				Name: "container-name",
			},
			plugins: []*api.PluginInstance{
				{
					Name:  "plugin1",
					Index: "00",
				},
				{
					Name:  "plugin2",
					Index: "01",
				},
			},
		},
		{
			name: "missing annotated with present global required plugin",
			cfg: &DefaultValidatorConfig{
				Enable:          true,
				RequiredPlugins: []string{"plugin1"},
			},
			pod: &api.PodSandbox{
				Id:        "pod-id",
				Name:      "pod-name",
				Namespace: "pod-namespace",
				Annotations: map[string]string{
					"required-plugins.noderesource.dev/container.container-name": "[ plugin2 ]",
				},
			},
			container: &api.Container{
				Id:   "container-id",
				Name: "container-name",
			},
			plugins: []*api.PluginInstance{
				{
					Name:  "plugin1",
					Index: "00",
				},
			},
			fail: true,
		},
		{
			name: "present annotated with missing global required plugin",
			cfg: &DefaultValidatorConfig{
				Enable:          true,
				RequiredPlugins: []string{"plugin1"},
			},
			pod: &api.PodSandbox{
				Id:        "pod-id",
				Name:      "pod-name",
				Namespace: "pod-namespace",
				Annotations: map[string]string{
					"required-plugins.noderesource.dev/container.container-name": "[ plugin2 ]",
				},
			},
			container: &api.Container{
				Id:   "container-id",
				Name: "container-name",
			},
			plugins: []*api.PluginInstance{
				{
					Name:  "plugin2",
					Index: "00",
				},
			},
			fail: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var (
				v   = NewDefaultValidator(tc.cfg)
				req = &api.ValidateContainerAdjustmentRequest{
					Pod:       tc.pod,
					Container: tc.container,
					Plugins:   tc.plugins,
				}
			)

			err := v.validateRequiredPlugins(req, req.GetPluginMap())
			if tc.fail {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateSysctl(t *testing.T) {
	type testCase struct {
		name      string
		cfg       *DefaultValidatorConfig
		pod       *api.PodSandbox
		container *api.Container
		plugins   []*api.PluginInstance
		adjust    *api.ContainerAdjustment
		claim     func(f *api.FieldOwners) error
		fail      bool
	}

	for _, tc := range []*testCase{
		{
			name: "disallowed sysctl adjustment",
			cfg: &DefaultValidatorConfig{
				Enable: true,
				Config: &Config{
					RejectSysctlAdjustment: func() *bool { v := true; return &v }(),
				},
			},
			pod: &api.PodSandbox{
				Id:        "pod-id",
				Name:      "pod-name",
				Namespace: "pod-namespace",
			},
			container: &api.Container{
				Id:   "container-id",
				Name: "container-name",
			},
			plugins: []*api.PluginInstance{
				{
					Name:  "plugin2",
					Index: "00",
				},
			},
			adjust: &api.ContainerAdjustment{
				Linux: &api.LinuxContainerAdjustment{
					Sysctl: map[string]string{
						"foo": "bar",
					},
				},
			},
			claim: func(f *api.FieldOwners) error {
				return f.ClaimSysctl("foo", "plugin2")
			},
			fail: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := NewDefaultValidator(tc.cfg)
			owners := &api.OwningPlugins{
				Owners: make(map[string]*api.FieldOwners),
			}
			owners.Owners[tc.container.Id] = &api.FieldOwners{
				Simple:   make(map[int32]string),
				Compound: make(map[int32]*api.CompoundFieldOwners),
			}
			if tc.claim != nil {
				require.NoError(t, tc.claim(owners.Owners[tc.container.Id]))
			}

			req := &api.ValidateContainerAdjustmentRequest{
				Pod:       tc.pod,
				Container: tc.container,
				Plugins:   tc.plugins,
				Adjust:    tc.adjust,
				Owners:    owners,
			}

			err := v.validateSysctl(req, nil)
			if tc.fail {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
