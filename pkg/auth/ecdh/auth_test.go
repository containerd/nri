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
		runtime, err = impl.NewWithEphemeralKeys()
		require.NoError(t, err)
	})

	t.Run("Set up plugin with pre-generated key pair", func(t *testing.T) {
		priv, pub, err := ecdh.GenerateKeyPair()
		require.NoError(t, err)

		plugin, err = impl.NewWithKeys(priv.Encode(), pub.Encode())
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
