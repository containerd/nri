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
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"errors"
	fmt "fmt"

	chacha "golang.org/x/crypto/chacha20poly1305"
)

var (
	// ErrInvalidKey indicates failures to encode, decode or use a key.
	ErrInvalidKey = errors.New("invalid key")
	// ErrChallenge indicate failure to generate or verify challenge.
	ErrChallenge = errors.New("challenge failed")
	// ErrAuthFailed indicates failed authentication.
	ErrAuthFailed = errors.New("authentication failed")
)

// GenerateKeyPair generates a private/public key pair for ECDH.
func GenerateKeyPair() (*PrivateKey, *PublicKey, error) {
	privK, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	priv := &PrivateKey{
		PrivateKey: privK,
	}
	pub := &PublicKey{
		PublicKey: privK.Public().(*ecdh.PublicKey),
	}

	return priv, pub, nil
}

// PrivateKey is an ECDH private key.
type PrivateKey struct {
	*ecdh.PrivateKey
	bytes []byte
}

// DecodePrivateKey returns a new private key for the encoded data.
func DecodePrivateKey(bytes []byte) (*PrivateKey, error) {
	k := &PrivateKey{}
	if err := k.Decode(bytes); err != nil {
		return nil, err
	}
	return k, nil
}

// Clear clears all data from the key.
func (k *PrivateKey) Clear() {
	if k == nil {
		return
	}

	k.PrivateKey = nil

	for i := range k.bytes {
		k.bytes[i] = 0
	}
}

// Encode encodes the private key.
func (k *PrivateKey) Encode() []byte {
	bytes := make([]byte, base64.StdEncoding.EncodedLen(len(k.Bytes())))
	base64.StdEncoding.Encode(bytes, k.Bytes())
	k.bytes = bytes

	return k.bytes
}

// Decode decodes the private key.
func (k *PrivateKey) Decode(bytes []byte) error {
	data := make([]byte, base64.StdEncoding.DecodedLen(len(bytes)))
	n, err := base64.StdEncoding.Decode(data, bytes)
	if err != nil {
		return fmt.Errorf("%w: failed to decode private key: %w", ErrInvalidKey, err)
	}

	key, err := ecdh.X25519().NewPrivateKey(data[:n])
	if err != nil {
		return fmt.Errorf("%w: failed to decode private key: %w", ErrInvalidKey, err)
	}

	k.PrivateKey = key

	return nil
}

// SharedSecret performs an ECDH exchange and returns the shared secret.
func (k *PrivateKey) SharedSecret(peer *PublicKey) ([]byte, error) {
	secret, err := k.ECDH(peer.PublicKey)
	if err != nil {
		return nil, err
	}

	if len(secret) > chacha.KeySize {
		return secret[:chacha.KeySize], nil
	}

	return secret, nil
}

// Seal encrypts the given cleartext for the given peer.
func (k *PrivateKey) Seal(peer *PublicKey, cleartext []byte) ([]byte, error) {
	key, err := k.SharedSecret(peer)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrChallenge, err)
	}

	aead, err := chacha.New(key[:])
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrChallenge, err)
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrChallenge, err)
	}

	return aead.Seal(nonce, nonce, cleartext, nil), nil
}

// Open decrypts the ciphertext from the given peer.
func (k *PrivateKey) Open(peer *PublicKey, ciphertext []byte) ([]byte, error) {
	key, err := k.SharedSecret(peer)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrChallenge, err)
	}

	aead, err := chacha.New(key[:])
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrChallenge, err)
	}

	nonceSize := aead.NonceSize()
	if l := len(ciphertext); l < nonceSize {
		return nil, fmt.Errorf("%w: challenge too short (%d < %d)", ErrChallenge, l, nonceSize)
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	cleartext, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrChallenge, err)
	}

	return cleartext, nil
}

// PublicKey is an ECDH public key.
type PublicKey struct {
	*ecdh.PublicKey
	bytes []byte
}

// DecodePublicKey returns a new public key for the encoded data.
func DecodePublicKey(bytes []byte) (*PublicKey, error) {
	k := &PublicKey{}
	if err := k.Decode(bytes); err != nil {
		return nil, err
	}
	return k, nil
}

// Clear clears all data from the key.
func (k *PublicKey) Clear() {
	if k == nil {
		return
	}

	k.PublicKey = nil

	for i := range k.bytes {
		k.bytes[i] = 0
	}
}

// Encode encodes the public key.
func (k *PublicKey) Encode() []byte {
	bytes := make([]byte, base64.StdEncoding.EncodedLen(len(k.Bytes())))
	base64.StdEncoding.Encode(bytes, k.Bytes())
	k.bytes = bytes

	return k.bytes
}

// Decode decodes the public key.
func (k *PublicKey) Decode(bytes []byte) error {
	data := make([]byte, base64.StdEncoding.DecodedLen(len(bytes)))
	n, err := base64.StdEncoding.Decode(data, bytes)
	if err != nil {
		return fmt.Errorf("%w: failed to decode public key: %w", ErrInvalidKey, err)
	}

	key, err := ecdh.X25519().NewPublicKey(data[:n])
	if err != nil {
		return fmt.Errorf("%w: failed to decode public key: %w", ErrInvalidKey, err)
	}

	k.PublicKey = key

	return nil
}
