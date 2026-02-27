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
	"testing"

	"github.com/containerd/nri/pkg/auth/ecdh"
	"github.com/stretchr/testify/require"
)

type (
	PrivateKey = ecdh.PrivateKey
	PublicKey  = ecdh.PublicKey
)

var (
	GenerateKeyPair = ecdh.GenerateKeyPair
)

func TestECDH(t *testing.T) {
	var (
		spriv, cpriv *PrivateKey
		spub, cpub   *PublicKey
		err          error
	)

	t.Run("generate key pairs", func(t *testing.T) {
		spriv, spub, err = GenerateKeyPair()
		require.NoError(t, err, "failed to generate key pair")
		cpriv, cpub, err = GenerateKeyPair()
		require.NoError(t, err, "failed to generate key pair")
	})

	t.Run("encode and decode keys", func(t *testing.T) {
		priv := &PrivateKey{}
		err = priv.Decode(spriv.Encode())
		require.NoError(t, err, "failed to decode private key")
		require.Equal(t, spriv.Encode(), priv.Encode(), "private key encoding/decoding mismatch")

		pub := &PublicKey{}
		err = pub.Decode(spub.Encode())
		require.NoError(t, err, "failed to decode public key")
		require.Equal(t, spub.Encode(), pub.Encode(), "public key encoding/decoding mismatch")

		err = priv.Decode(cpriv.Encode())
		require.NoError(t, err, "failed to decode private key")
		require.Equal(t, cpriv.Encode(), priv.Encode(), "private key encoding/decoding mismatch")

		err = pub.Decode(cpub.Encode())
		require.NoError(t, err, "failed to decode public key")
		require.Equal(t, cpub.Encode(), pub.Encode(), "public key encoding/decoding mismatch")
	})

	t.Run("generate shared secrets", func(t *testing.T) {
		ssecret, err := spriv.SharedSecret(cpub)
		require.NoError(t, err, "failed to generate shared secret")

		csecret, err := cpriv.SharedSecret(spub)
		require.NoError(t, err, "failed to generate shared secret")

		require.Equal(t, ssecret, csecret, "shared secrets do not match")
	})

	t.Run("encrypt and decrypt messages", func(t *testing.T) {
		message := []byte("this is a secret message")
		cipher, err := spriv.Seal(cpub, message)
		require.NoError(t, err, "failed to encrypt message")

		plain, err := cpriv.Open(spub, cipher)
		require.NoError(t, err, "failed to decrypt message")
		require.Equal(t, message, plain, "decrypted message does not match original")
	})
}
