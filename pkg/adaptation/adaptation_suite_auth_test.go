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
	"slices"
	"testing"

	nri "github.com/containerd/nri/pkg/adaptation"
	"github.com/containerd/nri/pkg/auth"
	"github.com/containerd/nri/pkg/stub"
	"github.com/stretchr/testify/require"
)

func TestPluginAuthentication(t *testing.T) {
	type Keys struct {
		private []byte
		public  []byte
	}

	var (
		key0 = &Keys{
			private: []byte("TkCE8UGWdLKjXzrUzc6AI6pFuma39+wL4d1P/gyqI0A="),
			public:  []byte("OlqYOu+/5pP9S+6XpeiC3ryr/iKEkZiTdo9VmMohbFY="),
		}
		key1 = &Keys{
			private: []byte("26zy0XoZOAGW5y1HwzR3dxbCr/K2pZaZvQO+8VQUgWs="),
			public:  []byte("PG+8RDcmJ+bznMdR0RpDjJHju+DTFnmkiv9e71yndnk="),
		}

		runtimeOptions = []nri.Option{
			nri.WithAuthConfig(&auth.Config{
				Roles: map[string]*auth.Role{
					"role0": {
						Keys: []string{
							string(key0.public),
						},
					},
					"role1": {
						Keys: []string{
							string(key1.public),
						},
					},
				},
			}),
		}
	)

	s := &Suite{}

	t.Run("should succeed with the correct key", func(t *testing.T) {
		s.Prepare(
			t,
			&mockRuntime{options: runtimeOptions},
			&mockPlugin{
				idx:  "00",
				name: "foo",
				options: []stub.Option{
					stub.WithAuthentication(
						nri.DefaultAuthentication,
						auth.NewMemKeyFetcher(
							slices.Clone(key0.private),
							slices.Clone(key0.public),
						),
					),
				},
			},
			&mockPlugin{
				idx:  "10",
				name: "bar",
				options: []stub.Option{
					stub.WithAuthentication(
						nri.DefaultAuthentication,
						auth.NewMemKeyFetcher(
							slices.Clone(key1.private),
							slices.Clone(key1.public),
						),
					),
				},
			},
		)
		s.Startup()

		s.WaitForPluginsToSync(s.plugin("00-foo"), s.plugin("10-bar"))
		require.Equal(t, s.plugin("00-foo").stub.GetRole(), "role0")
		require.Equal(t, s.plugin("10-bar").stub.GetRole(), "role1")
	})

	t.Run("should fail with incorrect keys", func(t *testing.T) {
		s.Prepare(
			t,
			&mockRuntime{options: runtimeOptions},
			&mockPlugin{
				idx:  "00",
				name: "foo",
				options: []stub.Option{
					stub.WithAuthentication(
						nri.DefaultAuthentication,
						auth.NewMemKeyFetcher(
							slices.Clone(key0.private),
							slices.Clone(key1.public),
						),
					),
				},
			},
			&mockPlugin{
				idx:  "10",
				name: "bar",
				options: []stub.Option{
					stub.WithAuthentication(
						nri.DefaultAuthentication,
						auth.NewMemKeyFetcher(
							slices.Clone(key1.private),
							slices.Clone(key0.public),
						),
					),
				},
			},
		)
		s.StartRuntime()
		require.Error(t, s.StartPlugin(s.plugins[0]))
		require.Error(t, s.StartPlugin(s.plugins[1]))
	})
}
