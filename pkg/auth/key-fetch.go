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
	"bytes"
	"os"
)

// KeyFetcher is the interface used by the stub to fetch authentication keys.
type KeyFetcher interface {
	PrivateKey() (PrivateKey, error)
	PublicKey() (PublicKey, error)
	ClearKeys()
}

// FileKeyFetcher fetches authentication keys two files, one for the private
// and another for the public key.
type FileKeyFetcher struct {
	privatePath string
	publicPath  string
	private     PrivateKey
	public      PublicKey
}

// NewFileKeyFetcher creates a new key fetcher for the given files.
func NewFileKeyFetcher(privatePath, publicPath string) *FileKeyFetcher {
	return &FileKeyFetcher{
		privatePath: privatePath,
		publicPath:  publicPath,
	}
}

// PrivateKey return the private key from the file.
func (f *FileKeyFetcher) PrivateKey() (PrivateKey, error) {
	if f.private != nil {
		return f.private, nil
	}

	data, err := readKey(f.privatePath)
	if err != nil {
		return nil, err
	}

	f.private = PrivateKey(data)
	return f.private, nil
}

// PublicKey return the public key from the file.
func (f *FileKeyFetcher) PublicKey() (PublicKey, error) {
	if f.public != nil {
		return f.public, nil
	}

	data, err := readKey(f.publicPath)
	if err != nil {
		return nil, err
	}

	f.public = PublicKey(data)
	return f.public, nil
}

// ClearKeys clears any cached key data.
func (f *FileKeyFetcher) ClearKeys() {
	for i := range f.private {
		f.private[i] = 0
	}
	f.private = nil
	for i := range f.public {
		f.public[i] = 0
	}
	f.public = nil
}

func readKey(file string) ([]byte, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	return bytes.TrimRight(data, "\r\n"), nil
}

// MemKeyFetcher returns keys from memory.
type MemKeyFetcher struct {
	private PrivateKey
	public  PublicKey
}

// NewMemKeyFetcher returns a KeyFetcher for the given keys.
// The keys are not copied. In particular this means that if
// ClearKeys is ever called, it will clear these keys.
func NewMemKeyFetcher(private, public []byte) *MemKeyFetcher {
	return &MemKeyFetcher{
		private: private,
		public:  public,
	}
}

// PrivateKey returns the private key from memory.
func (f *MemKeyFetcher) PrivateKey() (PrivateKey, error) {
	return f.private, nil
}

// PublicKey returns the public key from memory.
func (f *MemKeyFetcher) PublicKey() (PublicKey, error) {
	return f.public, nil
}

// ClearKeys clears the keys from memory. This clears the keys
// passed to the constructor. If this is not desired the keys
// should be copied before passing them to the constructor.
func (f *MemKeyFetcher) ClearKeys() {
	for i := range f.private {
		f.private[i] = 0
	}
	f.private = nil
	for i := range f.public {
		f.public[i] = 0
	}
	f.public = nil
}
