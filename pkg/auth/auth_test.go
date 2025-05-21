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
)

type (
	PrivateKey = auth.PrivateKey
	PublicKey  = auth.PublicKey
)

var (
	GenerateKeyPair = auth.GenerateKeyPair
)

func TestAuth(t *testing.T) {
	var (
		sprivate, spriv, cprivate, cpriv *PrivateKey
		spublic, spub, cpublic, cpub     *PublicKey
		challenge, response              []byte
		err                              error
	)

	t.Run("generate key pairs", func(t *testing.T) {
		sprivate, spublic, err = GenerateKeyPair()
		if err != nil {
			t.Fatalf("failed to generate key pair: %v", err)
		}
		cprivate, cpublic, err = GenerateKeyPair()
		if err != nil {
			t.Fatalf("failed to generate key pair: %v", err)
		}
	})

	t.Run("encode/decode keys", func(t *testing.T) {
		spriv = &PrivateKey{}
		err = spriv.Decode(sprivate.Encode())
		if err != nil {
			t.Fatalf("failed to decode private key: %v", err)
		}
		spub = &PublicKey{}
		err = spub.Decode(spublic.Encode())
		if err != nil {
			t.Fatalf("failed to decode public key: %v", err)
		}
		cpriv = &PrivateKey{}
		err = cpriv.Decode(cprivate.Encode())
		if err != nil {
			t.Fatalf("failed to decode private key: %v", err)
		}
		cpub = &PublicKey{}
		err = cpub.Decode(cpublic.Encode())
		if err != nil {
			t.Fatalf("failed to decode public key: %v", err)
		}
	})

	t.Run("generate shared secrets", func(t *testing.T) {
		ssecret, err := spriv.SharedSecret(cpub)
		if err != nil {
			t.Fatalf("failed to generate shared secret: %v", err)
		}
		csecret, err := cpriv.SharedSecret(spub)
		if err != nil {
			t.Fatalf("failed to generate shared secret: %v", err)
		}
		if string(ssecret) != string(csecret) {
			t.Fatalf("shared secrets do not match")
		}
	})

	t.Run("generate challenge", func(t *testing.T) {
		challenge, err = spriv.GenerateChallenge(cpub)
		if err != nil {
			t.Fatalf("failed to generate challenge: %v", err)
		}
	})

	t.Run("generate response", func(t *testing.T) {
		response, err = cpriv.GenerateResponse(spub, challenge)
		if err != nil {
			t.Fatalf("failed to generate response: %v", err)
		}
	})

	t.Run("verify response", func(t *testing.T) {
		if err = spriv.VerifyResponse(cpub, response); err != nil {
			t.Fatalf("failed to verify response: %v", err)
		}
	})
}
