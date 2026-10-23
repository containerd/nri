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

// WithTestNRIVersion is an option to override the local NRI version
// for testing purposes.
func WithTestNRIVersion(v string) Option {
	return func(r *Adaptation) error {
		r.nriVersion = v
		return nil
	}
}

// PluginClosed returns a function that reports whether the connection of
// the named plugin is closed. The lookup takes the adaptation lock, so it
// must be called while the plugin is registered and no request is in
// progress. The returned function does not take the lock, so a test can
// poll it while a request is in progress. Tests use it to wait until the
// adaptation has noticed a closed connection.
func (r *Adaptation) PluginClosed(name string) func() bool {
	r.Lock()
	defer r.Unlock()
	for _, p := range r.plugins {
		if p.name() == name {
			return p.isClosed
		}
	}
	return func() bool { return true }
}
