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
	"fmt"
	"strings"
	"testing"

	"github.com/containerd/nri/pkg/api"
	"github.com/stretchr/testify/require"
)

func TestPluginSynchronizeOversizedSnapshot(t *testing.T) {
	for _, tc := range []struct {
		name       string
		containers int
		envVars    int
	}{
		{name: "uneven snapshot", containers: 9, envVars: 8},
		{name: "fewer than eight objects", containers: 4, envVars: 16},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Each snapshot exceeds the ttRPC limit, while each container fits on its own.
			value := strings.Repeat("x", 72<<10)
			env := make([]string, tc.envVars)
			for i := range env {
				env[i] = fmt.Sprintf("VAR_%d=%s", i, value)
			}

			pod := &api.PodSandbox{Id: "pod0"}
			ctrs := make(map[string]*api.Container, tc.containers)
			for i := range tc.containers {
				id := fmt.Sprintf("ctr%d", i)
				ctrs[id] = &api.Container{Id: id, PodSandboxId: pod.Id, Env: env}
			}

			runtime := &mockRuntime{pods: map[string]*api.PodSandbox{pod.Id: pod}, ctrs: ctrs}
			plugin := &mockPlugin{idx: "00", name: "test"}
			s := &Suite{}
			s.Prepare(t, runtime, plugin)
			s.Startup()

			plugin.mu.Lock()
			defer plugin.mu.Unlock()
			require.Len(t, plugin.pods, 1)
			require.True(t, protoEqual(pod, plugin.pods[pod.Id]), "pod %q was not synchronized", pod.Id)
			require.Len(t, plugin.ctrs, len(ctrs))
			for id, ctr := range ctrs {
				require.True(t, protoEqual(ctr, plugin.ctrs[id]), "container %q was not synchronized", id)
			}
		})
	}
}
