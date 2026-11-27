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

package api_test

import (
	"testing"

	"github.com/containerd/nri/pkg/api"

	"github.com/stretchr/testify/require"
)

// TestGetPluginMap verifies how plugins are looked up for validation.
//
// Plugins are recorded by their qualified name, the index and the name
// joined by a dash, which is unique. They are also recorded by their
// bare name, which is not: two plugins can share a name if they have
// different indices. A lookup by bare name therefore cannot answer
// which plugin is meant, so the entry it finds carries no role. Doing
// it the other way around would let a plugin pick up the role of
// another plugin which happens to share its name.
func TestGetPluginMap(t *testing.T) {
	req := &api.ValidateContainerAdjustmentRequest{}

	req.AddPlugin("foo", "00", &api.Role{Role: "privileged"})
	req.AddPlugin("foo", "10", &api.Role{Role: "restricted"})
	req.AddPlugin("bar", "20", nil)

	plugins := req.GetPluginMap()

	t.Run("qualified names carry the role of the plugin", func(t *testing.T) {
		require.Equal(t, "privileged", plugins["00-foo"].GetRole())
		require.Equal(t, "restricted", plugins["10-foo"].GetRole())
		require.Equal(t, "", plugins["20-bar"].GetRole())
	})

	t.Run("a shared bare name carries no role", func(t *testing.T) {
		require.Contains(t, plugins, "foo")
		require.Equal(t, "", plugins["foo"].GetRole(),
			"bare name of plugins sharing it was given a role")
		require.Empty(t, plugins["foo"].GetTags(),
			"bare name of plugins sharing it was given role tags")
	})

	t.Run("an unshared bare name carries no role either", func(t *testing.T) {
		require.Contains(t, plugins, "bar")
		require.Equal(t, "", plugins["bar"].GetRole())
	})

	t.Run("plugins which did not take part are not found", func(t *testing.T) {
		require.NotContains(t, plugins, "30-baz")
		require.NotContains(t, plugins, "baz")
	})
}
