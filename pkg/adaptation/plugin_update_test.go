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

package adaptation_test

import (
	"testing"
	"time"

	"github.com/containerd/nri/pkg/api"

	"github.com/stretchr/testify/require"
)

// A plugin which has registered, but which the runtime does not consider
// live, must not be able to request unsolicited container updates.
func TestUpdateContainersFromPluginWhichIsNotLive(t *testing.T) {
	var (
		s      = &Suite{}
		plugin = &mockPlugin{idx: "10", name: "test"}
	)

	s.Prepare(t, &mockRuntime{})
	s.StartRuntime()

	// Hold off plugin synchronization. The accept loop registers and
	// configures the plugin, then blocks before synchronizing it, so the
	// plugin is connected and registered but not among the runtime's
	// plugins.
	block := s.runtime.runtime.BlockPluginSync()
	defer block.Unblock()

	s.StartPlugins(plugin)
	require.NoError(t, plugin.Wait(PluginConfigured, time.After(startupTimeout)))

	_, err := plugin.stub.UpdateContainers([]*api.ContainerUpdate{
		{ContainerId: "ctr0"},
	})
	require.Error(t, err, "unsolicited update from a plugin which is not live")
}
