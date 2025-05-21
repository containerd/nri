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

package ecdh

import (
	"crypto/sha256"
	"fmt"
	"slices"

	"github.com/containerd/nri/pkg/auth"
)

const (
	// Name of this implementation.
	Name = "ecdh+chacha+sha256"
)

// Implementation is our authentication implementation.
type Implementation struct{}

var _ auth.Implementation = &Implementation{}

// Authentication encapsulates the state of an authentication in progress.
type Authentication struct {
	priv *PrivateKey
	pub  *PublicKey
	peer *PublicKey
	seed []byte
}

var _ auth.Authentication = &Authentication{}

// Name returns the name of this implementation.
func (*Implementation) Name() string {
	return Name
}

// NewWithKeys instantiates authentication with the given key pair.
func (*Implementation) NewWithKeys(priv auth.PrivateKey, pub auth.PublicKey) (auth.Authentication, error) {
	privKey, err := DecodePrivateKey(priv)
	if err != nil {
		return nil, err
	}

	pubKey, err := DecodePublicKey(pub)
	if err != nil {
		return nil, err
	}

	return &Authentication{
		priv: privKey,
		pub:  pubKey,
	}, nil
}

// NewWithEphemeralKeys instantiates authentication with a new temporary key pair.
func (*Implementation) NewWithEphemeralKeys() (auth.Authentication, error) {
	privKey, pubKey, err := GenerateKeyPair()
	if err != nil {
		return nil, err
	}
	return &Authentication{
		priv: privKey,
		pub:  pubKey,
	}, nil
}

// Challenge creates a challenge for the given seed and peer.
func (a *Authentication) Challenge(seed []byte, peer auth.PublicKey) ([]byte, auth.PublicKey, error) {
	peerKey, err := DecodePublicKey(peer)
	if err != nil {
		return nil, nil, err
	}

	a.peer = peerKey
	a.seed = seed

	challenge, err := a.priv.Seal(a.peer, seed)
	if err != nil {
		return nil, nil, err
	}
	return challenge, a.pub.Encode(), nil
}

// Response creates a response to the given challenge from peer.
func (a *Authentication) Response(challenge []byte, peer auth.PublicKey) ([]byte, error) {
	peerKey, err := DecodePublicKey(peer)
	if err != nil {
		return nil, err
	}

	a.peer = peerKey

	seed, err := a.priv.Open(a.peer, challenge)
	if err != nil {
		return nil, err
	}

	response := sha256.Sum256(seed)

	return a.priv.Seal(a.peer, response[:])
}

// Verify verifies the given response for a previously created challenge.
func (a *Authentication) Verify(cipher []byte) error {
	response, err := a.priv.Open(a.peer, cipher)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrChallenge, err)
	}

	sum := sha256.Sum256(a.seed)
	if !slices.Equal(response, sum[:]) {
		return fmt.Errorf("%w: incorrect response", ErrAuthFailed)
	}

	return nil
}

func init() {
	if err := auth.Register(&Implementation{}); err != nil {
		panic(err)
	}
}
