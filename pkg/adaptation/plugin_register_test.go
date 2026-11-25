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
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// testRegisteringPlugin returns an external plugin which has a connection
// but has not registered yet, so it has no name of its own.
func testRegisteringPlugin() *plugin {
	return &plugin{
		regC: make(chan error, 1),
		impl: &pluginType{},
		r:    &Adaptation{registry: newRegistry(maxRetiredPlugins)},
	}
}

// registrationStatus returns whether a status was delivered to start(),
// and the status itself.
func registrationStatus(p *plugin) (bool, error) {
	select {
	case err := <-p.regC:
		return true, err
	default:
		return false, nil
	}
}

func TestRegisterPluginRefusesASecondRegistration(t *testing.T) {
	for _, tc := range []struct {
		name string
		idx  string
		base string
	}{
		{name: "another name", idx: "20", base: "other"},
		{name: "the same name", idx: "10", base: "test"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var (
				ctx = context.Background()
				p   = testRegisteringPlugin()
			)

			_, err := p.RegisterPlugin(ctx, &RegisterPluginRequest{
				PluginName: "test",
				PluginIdx:  "10",
			})
			require.NoError(t, err, "first registration")
			require.Equal(t, "10-test", p.name(), "name after registration")

			delivered, status := registrationStatus(p)
			require.True(t, delivered, "first registration delivers a status")
			require.NoError(t, status, "status of the first registration")

			_, err = p.RegisterPlugin(ctx, &RegisterPluginRequest{
				PluginName: tc.base,
				PluginIdx:  tc.idx,
			})
			require.Error(t, err, "second registration")
			require.Equal(t, "10-test", p.name(),
				"a refused registration must not rename the plugin")

			delivered, _ = registrationStatus(p)
			require.False(t, delivered,
				"a refused second registration must not deliver another status")
		})
	}
}

func TestRegisterPluginClaimsItsName(t *testing.T) {
	var (
		ctx = context.Background()
		p   = testRegisteringPlugin()
	)

	_, err := p.RegisterPlugin(ctx, &RegisterPluginRequest{
		PluginName: "test",
		PluginIdx:  "10",
	})
	require.NoError(t, err)

	err = p.r.registry.Claim(&plugin{idx: "10", base: "test"})
	require.Error(t, err, "a second instance claiming a name being registered")
}

func TestRegisterPluginRefusesAnInvalidName(t *testing.T) {
	for _, tc := range []struct {
		name string
		idx  string
		base string
	}{
		{name: "invalid name", idx: "10", base: "no/slashes"},
		{name: "invalid index", idx: "foo", base: "test"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var (
				ctx = context.Background()
				p   = testRegisteringPlugin()
			)

			_, err := p.RegisterPlugin(ctx, &RegisterPluginRequest{
				PluginName: tc.base,
				PluginIdx:  tc.idx,
			})
			require.Error(t, err, "registration with an invalid name")

			delivered, status := registrationStatus(p)
			require.True(t, delivered, "a refused registration delivers a status")
			require.Error(t, status, "status of a refused registration")

			// A refused registration leaves the plugin unregistered, so a
			// corrected one is still possible.
			_, err = p.RegisterPlugin(ctx, &RegisterPluginRequest{
				PluginName: "test",
				PluginIdx:  "10",
			})
			require.NoError(t, err, "registration after a refused one")
		})
	}
}

// A connection can go down while a plugin registers. Its name must not
// stay claimed, or no instance of that name could register again.
func TestRegisterPluginRefusesAClosedConnection(t *testing.T) {
	var (
		ctx = context.Background()
		p   = testRegisteringPlugin()
	)

	p.closed = true

	_, err := p.RegisterPlugin(ctx, &RegisterPluginRequest{
		PluginName: "test",
		PluginIdx:  "10",
	})
	require.Error(t, err, "registration on a closed connection")

	require.NoError(t, p.r.registry.Claim(&plugin{idx: "10", base: "test"}),
		"the name of a refused registration must be free")
}
