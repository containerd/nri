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

package stub_test

import (
	"context"
	"testing"
	"time"

	"github.com/containerd/nri/pkg/api"
	"github.com/containerd/nri/pkg/stub"
	"github.com/stretchr/testify/require"
)

type testPlugin struct{}

func (testPlugin) RunPodSandbox(context.Context, *api.PodSandbox) error {
	return nil
}

func TestWithPluginRegistrationTimeout(t *testing.T) {
	const want = 7 * time.Second

	s, err := stub.New(testPlugin{},
		stub.WithPluginName("test"),
		stub.WithPluginIdx("00"),
		stub.WithPluginRegistrationTimeout(want),
	)
	require.NoError(t, err)
	require.Equal(t, want, s.RegistrationTimeout())
}

func TestDefaultRegistrationTimeout(t *testing.T) {
	s, err := stub.New(testPlugin{},
		stub.WithPluginName("test"),
		stub.WithPluginIdx("00"),
	)
	require.NoError(t, err)
	require.Equal(t, stub.DefaultRegistrationTimeout, s.RegistrationTimeout())
}
