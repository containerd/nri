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

package adaptation

import (
	"fmt"

	"github.com/containerd/nri/pkg/auth"
	"github.com/containerd/nri/pkg/log"
)

// identityKey is a loaded runtime identity: a named key pair a plugin can
// request the runtime to prove it possesses.
type identityKey struct {
	name    string
	algo    string
	private auth.PrivateKey
	public  auth.PublicKey
}

// identityIndex returns the key an identity is indexed by. A public key
// is only meaningful for the algorithm it belongs to, and the algorithm
// of a connection is named by the plugin, so both are part of it.
func identityIndex(algo string, pub []byte) string {
	return algo + "\x00" + string(pub)
}

// setupIdentities loads the configured runtime identity keys and indexes
// them by algorithm and public key, which is how a plugin names the one
// it requests.
//
// This happens once, when the runtime starts, so that a key file which
// cannot be read or cannot be used fails immediately instead of at the
// first plugin connection.
func (r *Adaptation) setupIdentities() error {
	if r.auth == nil || len(r.auth.Identities) == 0 {
		return nil
	}

	identities := map[string]*identityKey{}

	for name, cfg := range r.auth.Identities {
		algo := cfg.Algorithm
		if algo == "" {
			algo = DefaultAuthentication
		}

		impl, err := auth.Get(algo)
		if err != nil {
			return fmt.Errorf("identity %q: %w", name, err)
		}

		private, err := auth.NewFileKeyFetcher(cfg.PrivateKeyPath, "").PrivateKey()
		if err != nil {
			return fmt.Errorf("identity %q: failed to read private key: %w", name, err)
		}

		public, err := impl.PublicKey(private)
		if err != nil {
			return fmt.Errorf("identity %q: failed to derive public key: %w", name, err)
		}

		index := identityIndex(algo, public)
		if other, ok := identities[index]; ok {
			return fmt.Errorf("identities %q and %q have the same %s key",
				name, other.name, algo)
		}

		identities[index] = &identityKey{
			name:    name,
			algo:    algo,
			private: private,
			public:  public,
		}

		log.Infof(noCtx, "runtime identity %q (%s) available for authentication", name, algo)
	}

	r.identities = identities

	return nil
}

// lookupIdentity returns the identity with the given algorithm and
// public key, or nil if the runtime does not have it.
func (r *Adaptation) lookupIdentity(algo string, pub []byte) *identityKey {
	if len(r.identities) == 0 || algo == "" || len(pub) == 0 {
		return nil
	}

	return r.identities[identityIndex(algo, pub)]
}
