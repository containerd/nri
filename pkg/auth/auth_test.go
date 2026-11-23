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

package auth_test

import (
	"testing"

	"github.com/containerd/nri/pkg/auth"
	"github.com/stretchr/testify/require"
)

// TestConfigValidateIdentities verifies that runtime identities are
// validated the same way roles are: configuration which cannot work is
// rejected when it is loaded, not when a plugin first connects.
func TestConfigValidateIdentities(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  *auth.Config
		err  error
	}{
		{
			name: "identities and no roles",
			cfg: &auth.Config{
				Identities: map[string]*auth.Identity{
					"vendor": {PrivateKeyPath: "/keys/vendor"},
				},
			},
		},
		{
			name: "roles and no identities",
			cfg: &auth.Config{
				Roles: map[string]*auth.Role{
					"role": {Keys: []string{"key"}},
				},
			},
		},
		{
			name: "both",
			cfg: &auth.Config{
				Roles: map[string]*auth.Role{
					"role": {Keys: []string{"key"}},
				},
				Identities: map[string]*auth.Identity{
					"vendor": {PrivateKeyPath: "/keys/vendor"},
				},
			},
		},
		{
			name: "nil identity entry",
			cfg: &auth.Config{
				Identities: map[string]*auth.Identity{"vendor": nil},
			},
			err: auth.ErrInvalidIdentity,
		},
		{
			name: "name disagreeing with the key",
			cfg: &auth.Config{
				Identities: map[string]*auth.Identity{
					"vendor": {Identity: "other", PrivateKeyPath: "/keys/vendor"},
				},
			},
			err: auth.ErrInvalidIdentity,
		},
		{
			name: "empty name",
			cfg: &auth.Config{
				Identities: map[string]*auth.Identity{
					"": {PrivateKeyPath: "/keys/vendor"},
				},
			},
			err: auth.ErrInvalidIdentity,
		},
		{
			name: "no private key path",
			cfg: &auth.Config{
				Identities: map[string]*auth.Identity{
					"vendor": {},
				},
			},
			err: auth.ErrInvalidIdentity,
		},
		{
			name: "two identities sharing a private key path",
			cfg: &auth.Config{
				Identities: map[string]*auth.Identity{
					"vendor": {PrivateKeyPath: "/keys/shared"},
					"other":  {PrivateKeyPath: "/keys/shared"},
				},
			},
			err: auth.ErrInvalidIdentity,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.err == nil {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, tc.err)
		})
	}
}

// TestConfigValidateFillsIdentityName verifies that an identity learns
// its name from its map key, so that the operator's handle on it does
// not have to be repeated inside the entry.
func TestConfigValidateFillsIdentityName(t *testing.T) {
	cfg := &auth.Config{
		Identities: map[string]*auth.Identity{
			"vendor-a": {PrivateKeyPath: "/keys/vendor-a"},
		},
	}

	require.NoError(t, cfg.Validate())
	require.Equal(t, "vendor-a", cfg.Identities["vendor-a"].Identity)
}
