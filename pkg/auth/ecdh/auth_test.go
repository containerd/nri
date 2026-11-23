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

package ecdh_test

import (
	"crypto/rand"
	"testing"

	"github.com/containerd/nri/pkg/auth"
	"github.com/containerd/nri/pkg/auth/ecdh"
	"github.com/stretchr/testify/require"
)

func TestImplementation(t *testing.T) {
	var (
		impl      auth.Implementation
		runtime   auth.Authentication
		plugin    auth.Authentication
		pluginKey auth.PublicKey
		peer      auth.PublicKey
		challenge []byte
		response  []byte
		err       error
	)

	t.Run("Lookup authentication implementation", func(t *testing.T) {
		impl, err = auth.Get(ecdh.Name)
		require.NoError(t, err)
	})

	t.Run("Set up runtime side with ephemeral key pair", func(t *testing.T) {
		runtime, err = impl.NewWithEphemeralKeys(auth.RuntimeSide)
		require.NoError(t, err)
	})

	t.Run("Set up plugin with pre-generated key pair", func(t *testing.T) {
		priv, pub, err := ecdh.GenerateKeyPair()
		require.NoError(t, err)

		plugin, err = impl.NewWithKeys(auth.PluginSide, priv.Encode(), pub.Encode())
		require.NoError(t, err)

		pluginKey = pub.Encode()
	})

	t.Run("Generate a challenge in the runtime", func(t *testing.T) {
		seed := make([]byte, 32)
		_, err = rand.Read(seed)
		require.NoError(t, err)

		challenge, peer, err = runtime.Challenge(seed, pluginKey)
		require.NoError(t, err)
	})

	t.Run("Generate a response to the challenge in the plugin", func(t *testing.T) {
		response, err = plugin.Response(challenge, peer)
		require.NoError(t, err)
	})

	t.Run("Verify the plugin provided response in the runtime", func(t *testing.T) {
		require.NoError(t, runtime.Verify(response))
	})
}

// TestKeyDerivationPurposesAreProtocol pins the key derivation strings.
//
// These values go into the HKDF info of every derived key, so two peers
// only ever agree if they use the exact same strings. They are part of
// the protocol, not an implementation detail: changing any one of them
// is a wire-incompatible change which requires a new algorithm name.
//
// They are qualified by which peer proves its identity, so that a
// challenge or a response for one direction can never be opened as the
// other one's, even though both directions derive their keys from the
// same shared secret.
func TestKeyDerivationPurposesAreProtocol(t *testing.T) {
	require.Equal(t, "nri-auth/v1 "+ecdh.Name+" ", ecdh.KeyInfoPrefix)
	require.Equal(t, "plugin-auth/challenge", ecdh.PluginChallengeKey)
	require.Equal(t, "plugin-auth/response", ecdh.PluginResponseKey)
	require.Equal(t, "runtime-auth/challenge", ecdh.RuntimeChallengeKey)
	require.Equal(t, "runtime-auth/response", ecdh.RuntimeResponseKey)
}

func randomSeed(t *testing.T) []byte {
	t.Helper()
	seed := make([]byte, 32)
	_, err := rand.Read(seed)
	require.NoError(t, err)
	return seed
}

// TestMutualHandshake runs both directions of authentication over one
// pair of instances, in the order the wire uses them: the plugin
// challenges the runtime first and only answers the runtime's own
// challenge once it has verified the answer to its own.
func TestMutualHandshake(t *testing.T) {
	impl, err := auth.Get(ecdh.Name)
	require.NoError(t, err)

	runtimePriv, runtimePub, err := ecdh.GenerateKeyPair()
	require.NoError(t, err)
	pluginPriv, pluginPub, err := ecdh.GenerateKeyPair()
	require.NoError(t, err)

	runtime, err := impl.NewWithKeys(auth.RuntimeSide, runtimePriv.Encode(), runtimePub.Encode())
	require.NoError(t, err)
	plugin, err := impl.NewWithKeys(auth.PluginSide, pluginPriv.Encode(), pluginPub.Encode())
	require.NoError(t, err)

	// the plugin challenges the runtime with a seed of its own
	seedP := randomSeed(t)
	challengeP, pluginKey, err := plugin.Challenge(seedP, runtimePub.Encode())
	require.NoError(t, err)
	require.Equal(t, auth.PublicKey(pluginPub.Encode()), pluginKey)

	// the runtime answers it, proving it holds the private key
	responseR, err := runtime.Response(challengeP, pluginKey)
	require.NoError(t, err)
	require.NoError(t, plugin.Verify(responseR))

	// only then does the plugin answer the runtime's challenge
	seedR := randomSeed(t)
	challengeR, runtimeKey, err := runtime.Challenge(seedR, pluginKey)
	require.NoError(t, err)
	require.Equal(t, auth.PublicKey(runtimePub.Encode()), runtimeKey)

	responseP, err := plugin.Response(challengeR, runtimeKey)
	require.NoError(t, err)
	require.NoError(t, runtime.Verify(responseP))
}

// TestReflectedChallengeDoesNotVerify feeds a plugin its own challenge
// back, as if the runtime had sent it. Both directions derive their
// keys from the same shared secret, so only the domain separation of
// the derivation purposes stops this from being answerable.
func TestReflectedChallengeDoesNotVerify(t *testing.T) {
	impl, err := auth.Get(ecdh.Name)
	require.NoError(t, err)

	_, runtimePub, err := ecdh.GenerateKeyPair()
	require.NoError(t, err)
	pluginPriv, pluginPub, err := ecdh.GenerateKeyPair()
	require.NoError(t, err)

	plugin, err := impl.NewWithKeys(auth.PluginSide, pluginPriv.Encode(), pluginPub.Encode())
	require.NoError(t, err)

	challengeP, _, err := plugin.Challenge(randomSeed(t), runtimePub.Encode())
	require.NoError(t, err)

	_, err = plugin.Response(challengeP, runtimePub.Encode())
	require.Error(t, err, "a reflected challenge must not be answerable")
}

// TestPeerKeyCannotChange verifies that an exchange is bound to a
// single peer. Both directions run against one peer key, so a second
// call naming a different one is a programming error, not a swap.
func TestPeerKeyCannotChange(t *testing.T) {
	impl, err := auth.Get(ecdh.Name)
	require.NoError(t, err)

	_, runtimePub, err := ecdh.GenerateKeyPair()
	require.NoError(t, err)
	_, otherPub, err := ecdh.GenerateKeyPair()
	require.NoError(t, err)
	pluginPriv, pluginPub, err := ecdh.GenerateKeyPair()
	require.NoError(t, err)

	plugin, err := impl.NewWithKeys(auth.PluginSide, pluginPriv.Encode(), pluginPub.Encode())
	require.NoError(t, err)

	challengeP, _, err := plugin.Challenge(randomSeed(t), runtimePub.Encode())
	require.NoError(t, err)

	_, err = plugin.Response(challengeP, otherPub.Encode())
	require.Error(t, err, "the peer key of an exchange must not change")
}

// TestDirectionPurposesAreSeparated verifies the domain separation
// between the two directions of authentication, the way the existing
// challenge/response separation test does for the two steps of one.
func TestDirectionPurposesAreSeparated(t *testing.T) {
	spriv, spub, err := ecdh.GenerateKeyPair()
	require.NoError(t, err)
	cpriv, cpub, err := ecdh.GenerateKeyPair()
	require.NoError(t, err)

	for _, tc := range []struct{ sealed, opened string }{
		{ecdh.PluginChallengeKey, ecdh.RuntimeChallengeKey},
		{ecdh.PluginResponseKey, ecdh.RuntimeResponseKey},
		{ecdh.RuntimeChallengeKey, ecdh.PluginChallengeKey},
		{ecdh.RuntimeResponseKey, ecdh.PluginResponseKey},
	} {
		t.Run(tc.sealed+" as "+tc.opened, func(t *testing.T) {
			cipher, err := spriv.Seal(cpub, tc.sealed, []byte("message"))
			require.NoError(t, err)
			_, err = cpriv.Open(spub, tc.opened, cipher)
			require.Error(t, err)
		})
	}
}

// TestPublicKeyDerivation verifies that the implementation can derive
// a public key from a private one. The runtime needs this to index its
// identity keys, which are configured as private keys only.
func TestPublicKeyDerivation(t *testing.T) {
	priv, pub, err := ecdh.GenerateKeyPair()
	require.NoError(t, err)

	impl, err := auth.Get(ecdh.Name)
	require.NoError(t, err)

	derived, err := impl.PublicKey(priv.Encode())
	require.NoError(t, err)
	require.Equal(t, auth.PublicKey(pub.Encode()), derived)
}
