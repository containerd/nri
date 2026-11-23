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
	"slices"
	sync "sync"
)

// IsAnonymous returns true if the plugin does not prove its own identity.
// An empty response is how a plugin says it claims nothing: it has finished
// the handshake, so it may register, but it is registered with no role,
// exactly as a plugin which never authenticated at all.
//
// This is the single definition of that rule. Nothing else should test the
// response for emptiness to decide what it means.
func (r *RespondChallengeRequest) IsAnonymous() bool {
	return len(r.GetResponse()) == 0
}

type (
	// PrivateKey in encoded format as stored in configuration.
	PrivateKey []byte
	// PublicKey in encoded format as stored in configuration
	// or transmitted over the wire.
	PublicKey []byte
)

// Side identifies which end of a connection an authentication instance
// runs on. It selects which key derivation purposes are used to prove
// this end's identity and which to verify the peer's. The two ends are
// mirror images of each other: what one proves with, the other verifies
// with.
//
// Both ends can authenticate, independently of each other. Either end
// can use a long-term key pair, to be authenticated by the peer, or an
// ephemeral one, when it only wants to authenticate the peer.
type Side int

const (
	// PluginSide is the plugin end of a connection.
	PluginSide Side = iota
	// RuntimeSide is the runtime end of a connection.
	RuntimeSide
)

// String returns the name of the side.
func (s Side) String() string {
	switch s {
	case PluginSide:
		return "plugin"
	case RuntimeSide:
		return "runtime"
	}
	return fmt.Sprintf("invalid side %d", int(s))
}

// Implementation of an authentication algorithm.
type Implementation interface {
	// Name returns the name of the implementation.
	Name() string
	// PublicKey returns the public key matching the given private key.
	// The runtime needs this to index its identity keys, which are
	// configured as private keys alone. Deriving one from the other
	// is specific to the algorithm, so it has to live here.
	PublicKey(PrivateKey) (PublicKey, error)
	// NewWithKeys instantiates authentication with the given key pair.
	// This is used by an end which wants to be authenticated by its
	// peer: a plugin proving its identity, or a runtime proving one of
	// its configured identities.
	NewWithKeys(Side, PrivateKey, PublicKey) (Authentication, error)
	// NewWithEphemeralKeys instantiates authentication with a new
	// temporary key pair. We use this for an end which does not prove
	// its own identity: a runtime with no identity key to present, or
	// a plugin which only wants to authenticate the runtime.
	NewWithEphemeralKeys(Side) (Authentication, error)
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

	if _, ok := implementations[impl.Name()]; ok {
		return fmt.Errorf("auth: implementation %q already registered", impl.Name())
	}

	implementations[impl.Name()] = impl

	return nil
}

// Get the implementation of the named authentication.
func Get(name string) (Implementation, error) {
	lock.Lock()
	defer lock.Unlock()

	impl, ok := implementations[name]
	if !ok {
		return nil, fmt.Errorf("auth: unknown implementation %q", name)
	}

	return impl, nil
}

var (
	// ErrInvalidConfig is returned if no/nil authentication configuration is provided.
	ErrInvalidConfig = errors.New("invalid authentication configuration provided")
	// ErrInvalidRole is returned if an invalid role entry is found in the configuration.
	ErrInvalidRole = errors.New("invalid role in configuration")
	// ErrInvalidIdentity is returned if an invalid identity entry is found
	// in the configuration.
	ErrInvalidIdentity = errors.New("invalid identity in configuration")
	// ErrKeyConflict is returned if a single key is configured for multiple roles.
	ErrKeyConflict = errors.New("key already exists")
	// ErrUnknownKey is returned if no role could be found for a key.
	ErrUnknownKey = errors.New("unknown key")
)

// Config represents authentication configuration. It maps authentication keys
// to roles, and optionally gives the runtime identities of its own.
type Config struct {
	Roles      map[string]*Role     `json:"roles"      toml:"roles"`
	Identities map[string]*Identity `json:"identities" toml:"identities"`
}

// Identity is a long-term key pair the runtime can prove possession of,
// to a plugin which demands it. A runtime holds any number of them, one
// per plugin which asks: a plugin names the public key it expects and
// the runtime answers with the matching identity, so two plugins never
// have to expect the same key.
//
// Only the private key is configured. The public key is derived from
// it, and is what a plugin names on the wire.
type Identity struct {
	// Identity is the name of this identity. It is filled in from the
	// name the identity is configured under, and is what logs and
	// errors refer to it by.
	Identity string `json:"identity" toml:"identity"`
	// Algorithm this key is for. Defaults to the default algorithm.
	Algorithm string `json:"algorithm" toml:"algorithm"`
	// PrivateKeyPath is the path to the private key of this identity.
	// It is a path rather than an inline value, because unlike the
	// public keys of a role this is secret and does not belong in the
	// configuration file of a runtime.
	PrivateKeyPath string `json:"privateKeyPath" toml:"private_key_path"`
}

// GetIdentity returns the name of the identity.
func (i *Identity) GetIdentity() string {
	if i == nil {
		return ""
	}
	return i.Identity
}

// Validate that all keys are unique in the configuration.
func (c *Config) Validate() error {
	if c == nil {
		return fmt.Errorf("%w: nil configuration", ErrInvalidConfig)
	}

	keyMap := map[string]*Role{}

	for name, role := range c.Roles {
		if role == nil {
			return fmt.Errorf("%w: nil role entry", ErrInvalidRole)
		}
		if role.Role == "" {
			role.Role = name
		}
		switch {
		case name == "":
			return fmt.Errorf("%w: role with empty name", ErrInvalidRole)
		case role.Role != name:
			return fmt.Errorf("%w: role name set (%q) but does not match map key (%q)",
				ErrInvalidRole, role.Role, name)
		case len(role.Keys) == 0:
			return fmt.Errorf("%w: role %q has no keys", ErrInvalidRole, role.Role)
		}

		for _, key := range role.Keys {
			if key == "" {
				return fmt.Errorf("%w: role %q has empty key", ErrInvalidRole, role.Role)
			}
			if other, ok := keyMap[key]; ok {
				return fmt.Errorf("%w: role conflict (%q, %q) for key %q",
					ErrKeyConflict, role.Role, other.Role, key)
			}
			keyMap[key] = role
		}
	}

	pathMap := map[string]*Identity{}

	for name, identity := range c.Identities {
		if identity == nil {
			return fmt.Errorf("%w: nil identity entry", ErrInvalidIdentity)
		}
		if identity.Identity == "" {
			identity.Identity = name
		}
		switch {
		case name == "":
			return fmt.Errorf("%w: identity with empty name", ErrInvalidIdentity)
		case identity.Identity != name:
			return fmt.Errorf("%w: identity name set (%q) but does not match map key (%q)",
				ErrInvalidIdentity, identity.Identity, name)
		case identity.PrivateKeyPath == "":
			return fmt.Errorf("%w: identity %q has no private key path",
				ErrInvalidIdentity, identity.Identity)
		}

		if other, ok := pathMap[identity.PrivateKeyPath]; ok {
			return fmt.Errorf("%w: identity conflict (%q, %q) for private key %q",
				ErrInvalidIdentity, identity.Identity, other.Identity, identity.PrivateKeyPath)
		}
		pathMap[identity.PrivateKeyPath] = identity
	}

	return nil
}

// GetRoleForKey returns the role for the given key.
func (c *Config) GetRoleForKey(keyBytes []byte) (*Role, error) {
	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("auth: unvalidated config with errors: %w", err)
	}

	for _, role := range c.Roles {
		if slices.Contains(role.Keys, string(keyBytes)) {
			return role, nil
		}
	}

	return nil, ErrUnknownKey
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
// tags can be used to establish an explicit association between roles and
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
