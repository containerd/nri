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

package auth

import (
	"errors"
	"fmt"
)

var (
	// ErrKeyConflict indicates multiple conflicting identities for a key.
	ErrKeyConflict = errors.New("conflicting roles for key")
	// ErrUnknownKey indicates an unknown key.
	ErrUnknownKey = errors.New("unknown key")
)

// Config contains authentication configuration. It has a set of roles and
// ensures (syntactic) validity of role keys and uniqueness of key to role
// mapping. It also provides fast role lookup by key.
type Config struct {
	Roles  []*Role `json:"roles" toml:"roles"`
	keyMap map[string]*Role
}

// Validate the configuration. Validation checks that all keys are
// syntactically valid and that all keys map to a single role.
func (c *Config) Validate() error {
	if c.keyMap != nil {
		return nil
	}

	keyMap := make(map[string]*Role)
	for _, id := range c.Roles {
		for _, key := range id.Keys {
			if o, ok := keyMap[key]; ok {
				return fmt.Errorf("%w: role conflict (%q, %q) for key %q",
					ErrKeyConflict, id.Role, o.Role, key)
			}
			if _, err := DecodePublicKey([]byte(key)); err != nil {
				return err
			}
			keyMap[key] = id
		}
	}
	c.keyMap = keyMap

	return nil
}

// GetRoleByKey returns the role and the decoded key for the given key.
func (c *Config) GetRoleByKey(keyBytes []byte) (*Role, *PublicKey, error) {
	id, ok := c.keyMap[string(keyBytes)]
	if !ok {
		return nil, nil, ErrUnknownKey
	}

	pub, err := DecodePublicKey(keyBytes)
	if err != nil {
		return nil, nil, err
	}

	return id, pub, nil
}

// Role describes one or more authenticated entities, or identities. A role
// has a name, one or more associated authentication keys and an opaque set
// of tags. A role is used to map identities to a set of permissions. It is
// possible to have a one-to-one or a one-to-many mapping between roles and
// identities by assigning one or more keys to a role. Roles are identified
// solely by a key during authentication.
type Role struct {
	Role string            `json:"role" toml:"role"`
	Keys []string          `json:"keys" toml:"keys"`
	Tags map[string]string `json:"tags" toml:"tags"`
}

// GetRole returns the name of the role.
func (r *Role) GetRole() string {
	if r == nil {
		return ""
	}
	return r.Role
}

// GetTags returns any tags associated with the role. Tags are opaque and
// carry no semantic meaning for authentication itself. However, they can
// be used for authorization by custom validating plugins. For instance,
// tags can be used to esablish an explicit association between roles and
// the NRI capabilities allowed for that role by using a tag representing
// each capability. Then a custom validator plugin can authorize roles by
// checking explicit tags instead of implicit semantics associated merely
// with the name of the role.
func (r *Role) GetTags() map[string]string {
	if r == nil {
		return nil
	}
	return r.Tags
}
