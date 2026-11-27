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
	"testing"

	"github.com/stretchr/testify/require"
	yaml "go.yaml.in/yaml/v3"

	"github.com/containerd/nri/pkg/api"
)

// legacyYAMLConfig is a configuration written against the validator
// configuration as it was before the restrictions moved into Config.
// All of its keys must keep working.
const legacyYAMLConfig = `
enable: true
rejectOCIHookAdjustment: true
rejectRuntimeDefaultSeccompAdjustment: true
rejectUnconfinedSeccompAdjustment: true
rejectCustomSeccompAdjustment: true
rejectNamespaceAdjustment: true
rejectSysctlAdjustment: true
requiredPlugins:
  - 00-foo
tolerateMissingPluginsAnnotation: tolerate-missing-plugins
`

// TestLegacyYAMLConfigDecode verifies that a configuration written for
// the pre-Config layout still decodes into the same restrictions.
//
// The restrictions now live in an embedded *Config. An embedded pointer
// is not inlined by the YAML decoder unless it is tagged as inline. If
// it is not, the decoder silently ignores the restriction keys, leaves
// Config nil, and reports no error. Restrictions which used to be in
// effect then turn off unnoticed.
func TestLegacyYAMLConfigDecode(t *testing.T) {
	cfg := &DefaultValidatorConfig{}
	require.NoError(t, yaml.Unmarshal([]byte(legacyYAMLConfig), cfg))

	require.True(t, cfg.Enable, "enable")
	require.Equal(t, []string{"00-foo"}, cfg.RequiredPlugins, "requiredPlugins")
	require.Equal(t, "tolerate-missing-plugins", cfg.TolerateMissingAnnotation,
		"tolerateMissingPluginsAnnotation")

	require.NotNil(t, cfg.Config, "restrictions were dropped while decoding")

	for _, tc := range []struct {
		key   string
		value *bool
	}{
		{"rejectOCIHookAdjustment", cfg.RejectOCIHookAdjustment},
		{"rejectRuntimeDefaultSeccompAdjustment", cfg.RejectRuntimeDefaultSeccompAdjustment},
		{"rejectUnconfinedSeccompAdjustment", cfg.RejectUnconfinedSeccompAdjustment},
		{"rejectCustomSeccompAdjustment", cfg.RejectCustomSeccompAdjustment},
		{"rejectNamespaceAdjustment", cfg.RejectNamespaceAdjustment},
		{"rejectSysctlAdjustment", cfg.RejectSysctlAdjustment},
	} {
		require.NotNil(t, tc.value, "%s was dropped while decoding", tc.key)
		require.True(t, *tc.value, "%s", tc.key)
	}
}

// TestLegacyYAMLConfigIsEnforced verifies that the restrictions of a
// decoded legacy configuration are actually enforced. A configuration
// which decodes into no restrictions fails open: the validator accepts
// adjustments it was configured to reject.
func TestLegacyYAMLConfigIsEnforced(t *testing.T) {
	cfg := &DefaultValidatorConfig{}
	require.NoError(t, yaml.Unmarshal([]byte(legacyYAMLConfig), cfg))

	var (
		v   = NewDefaultValidator(cfg)
		ctx = context.Background()

		pod = &api.PodSandbox{
			Id:        "pod-id",
			Name:      "pod-name",
			Namespace: "pod-namespace",
		}
		container = &api.Container{
			Id:   "container-id",
			Name: "container-name",
		}
		plugins = []*api.PluginInstance{
			{
				Name:  "foo",
				Index: "00",
			},
		}
	)

	for _, tc := range []struct {
		name   string
		adjust *api.ContainerAdjustment
		claim  func(*api.FieldOwners) error
	}{
		{
			name: "OCI hook injection",
			adjust: &api.ContainerAdjustment{
				Hooks: &api.Hooks{
					Prestart: []*api.Hook{
						{
							Path: "/bin/sh",
							Args: []string{"/bin/sh", "-c", "true"},
						},
					},
				},
			},
			claim: func(f *api.FieldOwners) error {
				return f.ClaimHooks("00-foo")
			},
		},
		{
			name: "sysctl adjustment",
			adjust: &api.ContainerAdjustment{
				Linux: &api.LinuxContainerAdjustment{
					Sysctl: map[string]string{
						"foo": "bar",
					},
				},
			},
			claim: func(f *api.FieldOwners) error {
				return f.ClaimSysctl("foo", "00-foo")
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			owners := &api.OwningPlugins{
				Owners: map[string]*api.FieldOwners{
					container.Id: {
						Simple:   make(map[int32]string),
						Compound: make(map[int32]*api.CompoundFieldOwners),
					},
				},
			}
			require.NoError(t, tc.claim(owners.Owners[container.Id]))

			req := &api.ValidateContainerAdjustmentRequest{
				Pod:       pod,
				Container: container,
				Plugins:   plugins,
				Adjust:    tc.adjust,
				Owners:    owners,
			}

			require.Error(t, v.ValidateContainerAdjustment(ctx, req),
				"restricted %s was accepted", tc.name)
		})
	}
}
