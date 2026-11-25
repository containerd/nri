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
	"sync"
)

// registry tracks registered plugins by their indexed name. A name has
// at most one live plugin, the instance which gets relayed requests and
// events. Another instance of the same name replaces it once that
// instance has synchronized. A replaced instance is retired: it is taken
// out of the request and validator paths, but stays connected.
type registry struct {
	sync.Mutex
	live    map[string]*plugin // live plugin of each name
	pending map[string]*plugin // claimed, not committed yet
}

// newRegistry returns an empty plugin registry.
func newRegistry() *registry {
	return &registry{
		live:    make(map[string]*plugin),
		pending: make(map[string]*plugin),
	}
}

// Claim records a plugin as the pending registration of its name. It
// fails if another instance of that name is already being registered.
func (reg *registry) Claim(p *plugin) error {
	reg.Lock()
	defer reg.Unlock()

	name := p.name()
	if other, ok := reg.pending[name]; ok && other != p {
		return fmt.Errorf("another registration of plugin %q is in progress", name)
	}

	reg.pending[name] = p
	return nil
}

// Commit makes a claimed plugin the live one of its name and returns the
// instance it replaced, if there was one. It fails if the plugin lost its
// claim, which happens if the plugin goes away while it registers.
func (reg *registry) Commit(p *plugin) (*plugin, error) {
	reg.Lock()
	defer reg.Unlock()

	name := p.name()
	if reg.pending[name] != p {
		return nil, fmt.Errorf("plugin %q lost its claim on the name", name)
	}

	delete(reg.pending, name)

	replaced := reg.live[name]
	reg.live[name] = p

	if replaced == p {
		return nil, nil
	}

	return replaced, nil
}

// Release drops a plugin's claim on its name, pending or committed.
func (reg *registry) Release(p *plugin) {
	reg.Lock()
	defer reg.Unlock()

	name := p.name()
	if reg.pending[name] == p {
		delete(reg.pending, name)
	}

	// Check identity before dropping the live entry: a retired instance
	// going away must not take its replacement out of service.
	if reg.live[name] == p {
		delete(reg.live, name)
	}
}

// Get returns the live plugin of an indexed name, or nil if it has none.
func (reg *registry) Get(idx, base string) *plugin {
	reg.Lock()
	defer reg.Unlock()

	return reg.live[idx+"-"+base]
}
