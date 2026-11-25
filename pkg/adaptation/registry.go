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
	stopped bool               // no more claims or commits
}

// newRegistry returns an empty plugin registry.
func newRegistry() *registry {
	return &registry{
		live:    make(map[string]*plugin),
		pending: make(map[string]*plugin),
	}
}

// Claim records a plugin as the pending registration of its name. It
// fails if another instance of that name is already being registered, or
// if the plugin is not allowed to displace the live instance.
func (reg *registry) Claim(p *plugin) error {
	reg.Lock()
	defer reg.Unlock()

	name := p.name()
	if reg.stopped {
		return fmt.Errorf("cannot register plugin %q, NRI is stopping", name)
	}

	if other, ok := reg.pending[name]; ok && other != p {
		return fmt.Errorf("another registration of plugin %q (%s) is in progress",
			name, other.origin)
	}

	if live, ok := reg.live[name]; ok && live != p {
		if err := mayDisplace(live, p); err != nil {
			return err
		}
	}

	reg.pending[name] = p
	return nil
}

// mayDisplace returns an error if a plugin is not allowed to take over
// the name of the live instance.
func mayDisplace(live, p *plugin) error {
	if p.origin == originExternal && live.origin != originExternal {
		return fmt.Errorf("plugin %q is %s, an external plugin cannot displace it",
			live.name(), live.origin)
	}

	return nil
}

// Commit makes a claimed plugin the live one of its name and returns the
// instance it replaced, if there was one. It fails if the plugin lost its
// claim, which happens if the plugin goes away while it registers.
func (reg *registry) Commit(p *plugin) (*plugin, error) {
	reg.Lock()
	defer reg.Unlock()

	name := p.name()
	if reg.stopped {
		return nil, fmt.Errorf("cannot activate plugin %q, NRI is stopping", name)
	}
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

// Stop empties the registry and refuses later claims and commits. It
// returns every plugin it dropped, including the ones which were still
// registering.
func (reg *registry) Stop() []*plugin {
	reg.Lock()
	defer reg.Unlock()

	var plugins []*plugin
	for _, p := range reg.pending {
		plugins = append(plugins, p)
	}
	for _, p := range reg.live {
		plugins = append(plugins, p)
	}

	reg.pending = make(map[string]*plugin)
	reg.live = make(map[string]*plugin)
	reg.stopped = true

	return plugins
}

// Start lets the registry take claims again.
func (reg *registry) Start() {
	reg.Lock()
	defer reg.Unlock()

	reg.stopped = false
}

// Get returns the live plugin of an indexed name, or nil if it has none.
func (reg *registry) Get(idx, base string) *plugin {
	reg.Lock()
	defer reg.Unlock()

	return reg.live[idx+"-"+base]
}
