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
	fmt "fmt"
	sync "sync"
)

type (
	// PrivateKey in encoded format as stored in configuration.
	PrivateKey []byte
	// PublicKey in encoded format as stored in configuration
	// or transmitted over the wire.
	PublicKey []byte
)

// Implementation of an authentication algorithm.
type Implementation interface {
	// Name returns the name of the implementation.
	Name() string
	// NewWithKeys instantiates authentication with the given key pair.
	// This is always used on the plugin side to set up athentication
	// with a pre-generated key pair.
	NewWithKeys(PrivateKey, PublicKey) (Authentication, error)
	// NewWithEphemeralKeys instantiates authentication with a new temporary
	// key pair. We use this on the runtime side to generate a new key pair
	// for each authentication request.
	NewWithEphemeralKeys() (Authentication, error)
}

// Authentication is an instance of authentication in progress.
type Authentication interface {
	//  Challenge creates a challenge for the seed and peer public key.
	Challenge(seed []byte, peer PublicKey) ([]byte, PublicKey, error)
	// Response creates a response to the given challenge and peer public key.
	Response(challenge []byte, peer PublicKey) ([]byte, error)
	// Verify verifies the given response for a previously created challenge.
	Verify(response []byte) error
}

var (
	implementations = map[string]Implementation{}
	lock            sync.Mutex
)

// Register an authentication implementation.
func Register(impl Implementation) error {
	lock.Lock()
	defer lock.Unlock()

	if _, exists := implementations[impl.Name()]; exists {
		return fmt.Errorf("auth: implementation %q already registered", impl.Name())
	}

	implementations[impl.Name()] = impl

	return nil
}

// Get the implementation of the named authentication.
func Get(name string) (Implementation, error) {
	lock.Lock()
	defer lock.Unlock()

	impl, exists := implementations[name]
	if !exists {
		return nil, fmt.Errorf("auth: unknown implementation %q", name)
	}

	return impl, nil
}

var (
	// ErrKeyConflict is returned if a single key is configured for multiple roles.
	ErrKeyConflict = errors.New("key already exists")
	// ErrUnknownKey is returned if no role could be found for a key.
	ErrUnknownKey = errors.New("unknown key")
)

// Config represents authentication configuration. It maps authentication keys
// to roles.
type Config struct {
	Roles  []*Role `json:"roles" toml:"roles"`
	keyMap map[string]*Role
}

// Validate that all keys are unique in the configuration.
func (c *Config) Validate() error {
	if c.keyMap != nil {
		return nil
	}

	var (
		keyMap = make(map[string]*Role)
		roles  = make(map[string]*Role)
	)

	for _, role := range c.Roles {
		if _, ok := roles[role.Role]; ok {
			return fmt.Errorf("%w: duplicate role name %q", ErrKeyConflict, role.Role)
		}
		roles[role.Role] = role

		for _, key := range role.Keys {
			if other, ok := keyMap[key]; ok {
				return fmt.Errorf("%w: role conflict (%q, %q) for key %q",
					ErrKeyConflict, role.Role, other.Role, key)
			}
			keyMap[key] = role
		}
	}

	c.keyMap = keyMap

	return nil
}

// GetRoleForKey returns the role for the given key.
func (c *Config) GetRoleForKey(keyBytes []byte) (*Role, error) {
	if c.keyMap == nil {
		if err := c.Validate(); err != nil {
			return nil, fmt.Errorf("auth: unvalidated config with errors: %w", err)
		}
	}

	role, ok := c.keyMap[string(keyBytes)]
	if !ok {
		return nil, ErrUnknownKey
	}

	return role, nil
}

// Role represents one or more authenticated plugins. A role has a name and
// and a set of associated plugin public keys. The name and the keys must be
// unique within the set of roles. After successful authentication a plugin
// is assigned the role its key is associated with.
//
// A role can also have a set of opaque tags associated with it. These carry
// no semantic meaning for the authentication process or NRI itself. Tags
// can be used in validation, to attach authorization semantics to explicit
// tags instead of rather implicitly to role names.
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
