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

	"github.com/containerd/nri/pkg/api"
)

// TestRoleOverrides verifies that the restrictions applied to an
// adjustment are the ones configured for the role of the plugin which
// made it.
//
// Every restriction is checked against every combination of default and
// per role configuration which decides between them: an override which
// loosens the default, one which tightens it, an override which says
// nothing about the restriction, and a role with no configuration of
// its own. A plugin without a role gets the defaults.
func TestRoleOverrides(t *testing.T) {
	const owner = "00-foo"

	var (
		yes = true
		no  = false
	)

	// A restriction, the adjustment tripping it, the claim recording
	// the plugin as the owner of that adjustment, and the setter for
	// the configuration field restricting it.
	type restriction struct {
		name   string
		adjust *api.ContainerAdjustment
		claim  func(*api.FieldOwners) error
		reject func(*Config, bool)
	}

	restrictions := []*restriction{
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
			claim:  func(f *api.FieldOwners) error { return f.ClaimHooks(owner) },
			reject: func(c *Config, v bool) { c.RejectOCIHookAdjustment = &v },
		},
		{
			name: "namespace adjustment",
			adjust: &api.ContainerAdjustment{
				Linux: &api.LinuxContainerAdjustment{
					Namespaces: []*api.LinuxNamespace{
						{
							Type: "network",
							Path: "/var/run/netns/test",
						},
					},
				},
			},
			claim:  func(f *api.FieldOwners) error { return f.ClaimNamespace("network", owner) },
			reject: func(c *Config, v bool) { c.RejectNamespaceAdjustment = &v },
		},
		{
			name: "sysctl adjustment",
			adjust: &api.ContainerAdjustment{
				Linux: &api.LinuxContainerAdjustment{
					Sysctl: map[string]string{
						"net.ipv4.ip_forward": "1",
					},
				},
			},
			claim:  func(f *api.FieldOwners) error { return f.ClaimSysctl("net.ipv4.ip_forward", owner) },
			reject: func(c *Config, v bool) { c.RejectSysctlAdjustment = &v },
		},
		{
			name: "unconfined seccomp policy adjustment",
			adjust: &api.ContainerAdjustment{
				Linux: &api.LinuxContainerAdjustment{},
			},
			claim:  func(f *api.FieldOwners) error { return f.ClaimSeccompPolicy(owner) },
			reject: func(c *Config, v bool) { c.RejectUnconfinedSeccompAdjustment = &v },
		},
	}

	// A configuration of the restriction in the defaults and per role,
	// the role of the plugin making the adjustment, and whether the
	// adjustment should be rejected. A nil override is a role which is
	// configured but says nothing about the restriction.
	type testCase struct {
		name      string
		defaults  *bool
		overrides map[string]*bool
		role      string
		reject    bool
	}

	for _, tc := range []*testCase{
		{
			name:      "role override loosens the default",
			defaults:  &yes,
			overrides: map[string]*bool{"privileged": &no},
			role:      "privileged",
			reject:    false,
		},
		{
			name:      "role override tightens the default",
			defaults:  &no,
			overrides: map[string]*bool{"restricted": &yes},
			role:      "restricted",
			reject:    true,
		},
		{
			name:      "role override says nothing about the restriction",
			defaults:  &yes,
			overrides: map[string]*bool{"partial": nil},
			role:      "partial",
			reject:    true,
		},
		{
			name:      "role of the plugin has no configuration",
			defaults:  &yes,
			overrides: map[string]*bool{"privileged": &no},
			role:      "other",
			reject:    true,
		},
		{
			name:      "plugin has no role",
			defaults:  &yes,
			overrides: map[string]*bool{"privileged": &no},
			role:      "",
			reject:    true,
		},
		{
			name:      "plugin has no role and the default is unset",
			overrides: map[string]*bool{"privileged": &yes},
			role:      "",
			reject:    false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, r := range restrictions {
				t.Run(r.name, func(t *testing.T) {
					cfg := &DefaultValidatorConfig{Enable: true}

					if tc.defaults != nil {
						cfg.Config = &Config{}
						r.reject(cfg.Config, *tc.defaults)
					}

					if tc.overrides != nil {
						cfg.Roles = map[string]*Config{}
						for role, override := range tc.overrides {
							cfg.Roles[role] = &Config{}
							if override != nil {
								r.reject(cfg.Roles[role], *override)
							}
						}
					}

					container := &api.Container{Id: "ctr0", Name: "ctr0"}

					owners := &api.OwningPlugins{
						Owners: map[string]*api.FieldOwners{
							container.Id: {
								Simple:   make(map[int32]string),
								Compound: make(map[int32]*api.CompoundFieldOwners),
							},
						},
					}
					require.NoError(t, r.claim(owners.Owners[container.Id]))

					req := &api.ValidateContainerAdjustmentRequest{
						Pod:       &api.PodSandbox{Id: "pod0", Name: "pod0"},
						Container: container,
						Plugins: []*api.PluginInstance{
							{
								Name:  "foo",
								Index: "00",
								Role:  tc.role,
							},
						},
						Adjust: r.adjust,
						Owners: owners,
					}

					err := NewDefaultValidator(cfg).ValidateContainerAdjustment(context.Background(), req)
					if tc.reject {
						require.Error(t, err, "restricted %s was accepted", r.name)
					} else {
						require.NoError(t, err, "allowed %s was rejected", r.name)
					}
				})
			}
		})
	}
}
