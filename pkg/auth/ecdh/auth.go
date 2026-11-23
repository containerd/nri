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
	Name = "ecdh+hkdf+chacha+sha256"

	// KeyInfoPrefix is the domain separation prefix for keys derived
	// by this implementation. The purpose of a key is appended to it.
	KeyInfoPrefix = "nri-auth/v1 " + Name + " "

	// PluginChallengeKey is the key derivation purpose for a challenge
	// which authenticates the plugin. The challenge and the response
	// are encrypted with keys derived for different purposes, so that
	// neither one can be replayed as the other.
	//
	// The purposes also name the peer whose identity is being proven.
	// Both directions of authentication derive their keys from the
	// same shared secret, so without this a challenge for one could
	// be reflected back as the challenge of the other.
	PluginChallengeKey = "plugin-auth/challenge"
	// PluginResponseKey is the key derivation purpose for a response
	// which authenticates the plugin.
	PluginResponseKey = "plugin-auth/response"
	// RuntimeChallengeKey is the key derivation purpose for a challenge
	// which authenticates the runtime.
	RuntimeChallengeKey = "runtime-auth/challenge"
	// RuntimeResponseKey is the key derivation purpose for a response
	// which authenticates the runtime.
	RuntimeResponseKey = "runtime-auth/response"
)

// purposes returns the key derivation purposes for proving the identity
// of the given side. A side proves with its own purposes and verifies
// with its peer's, so the two ends of a connection use these in mirror
// image of each other.
func purposes(s auth.Side) (challenge, response string) {
	if s == auth.RuntimeSide {
		return RuntimeChallengeKey, RuntimeResponseKey
	}
	return PluginChallengeKey, PluginResponseKey
}

// peerOf returns the side of our peer.
func peerOf(s auth.Side) auth.Side {
	if s == auth.RuntimeSide {
		return auth.PluginSide
	}
	return auth.RuntimeSide
}

// Implementation is our authentication implementation.
type Implementation struct{}

var _ auth.Implementation = &Implementation{}

// Authentication encapsulates the state of an authentication in progress.
//
// One instance serves both directions of authentication for one
// connection. The side it was created for decides which derivation
// purposes it proves its own identity with and which it verifies the
// peer's with.
type Authentication struct {
	side auth.Side
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

// PublicKey returns the public key matching the given private key.
func (*Implementation) PublicKey(priv auth.PrivateKey) (auth.PublicKey, error) {
	privKey, err := DecodePrivateKey(priv)
	if err != nil {
		return nil, err
	}
	defer privKey.Clear()

	return privKey.Public().Encode(), nil
}

// NewWithKeys instantiates authentication with the given key pair.
func (*Implementation) NewWithKeys(side auth.Side, priv auth.PrivateKey, pub auth.PublicKey) (auth.Authentication, error) {
	privKey, err := DecodePrivateKey(priv)
	if err != nil {
		return nil, err
	}

	pubKey, err := DecodePublicKey(pub)
	if err != nil {
		return nil, err
	}

	return &Authentication{
		side: side,
		priv: privKey,
		pub:  pubKey,
	}, nil
}

// NewWithEphemeralKeys instantiates authentication with a new temporary key pair.
func (*Implementation) NewWithEphemeralKeys(side auth.Side) (auth.Authentication, error) {
	privKey, pubKey, err := GenerateKeyPair()
	if err != nil {
		return nil, err
	}
	return &Authentication{
		side: side,
		priv: privKey,
		pub:  pubKey,
	}, nil
}

// setPeer records the peer of this exchange. Both directions of
// authentication run against a single peer, so a call which names a
// different one is refused instead of silently swapping it.
func (a *Authentication) setPeer(peer auth.PublicKey) error {
	peerKey, err := DecodePublicKey(peer)
	if err != nil {
		return err
	}

	if a.peer != nil && !slices.Equal(a.peer.Bytes(), peerKey.Bytes()) {
		return fmt.Errorf("%w: peer key of the exchange changed", ErrInvalidKey)
	}

	a.peer = peerKey

	return nil
}

// Challenge creates a challenge for the given seed and peer. It is
// answerable only by the peer, so it is sealed for the purposes the
// peer proves its identity with.
func (a *Authentication) Challenge(seed []byte, peer auth.PublicKey) ([]byte, auth.PublicKey, error) {
	if err := a.setPeer(peer); err != nil {
		return nil, nil, err
	}

	a.seed = seed

	challengeKey, _ := purposes(peerOf(a.side))

	challenge, err := a.priv.Seal(a.peer, challengeKey, seed)
	if err != nil {
		return nil, nil, err
	}
	return challenge, a.pub.Encode(), nil
}

// Response creates a response to the given challenge from peer. It
// proves our own identity, so it uses the purposes of our side.
func (a *Authentication) Response(challenge []byte, peer auth.PublicKey) ([]byte, error) {
	if err := a.setPeer(peer); err != nil {
		return nil, err
	}

	challengeKey, responseKey := purposes(a.side)

	seed, err := a.priv.Open(a.peer, challengeKey, challenge)
	if err != nil {
		return nil, err
	}

	response := sha256.Sum256(seed)

	return a.priv.Seal(a.peer, responseKey, response[:])
}

// Verify verifies the given response for a previously created challenge.
func (a *Authentication) Verify(cipher []byte) error {
	_, responseKey := purposes(peerOf(a.side))

	response, err := a.priv.Open(a.peer, responseKey, cipher)
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
