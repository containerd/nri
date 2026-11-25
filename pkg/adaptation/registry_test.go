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
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRegistryCommitReplacesTheLivePlugin(t *testing.T) {
	var (
		reg       = newRegistry()
		incumbent = &plugin{idx: "10", base: "test"}
		newcomer  = &plugin{idx: "10", base: "test"}
	)

	require.NoError(t, reg.Claim(incumbent))
	replaced, err := reg.Commit(incumbent)
	require.NoError(t, err)
	require.Nil(t, replaced, "nothing to replace for an unclaimed name")
	require.Equal(t, incumbent, reg.Get("10", "test"))

	require.NoError(t, reg.Claim(newcomer))
	replaced, err = reg.Commit(newcomer)
	require.NoError(t, err)
	require.Equal(t, incumbent, replaced, "the replaced instance")
	require.Equal(t, newcomer, reg.Get("10", "test"))
}

func TestRegistryReleaseOfRetiredPluginKeepsLiveOne(t *testing.T) {
	var (
		reg       = newRegistry()
		incumbent = &plugin{idx: "10", base: "test"}
		newcomer  = &plugin{idx: "10", base: "test"}
	)

	require.NoError(t, reg.Claim(incumbent))
	reg.Commit(incumbent)
	require.NoError(t, reg.Claim(newcomer))
	reg.Commit(newcomer)

	// The retired instance goes away after it was replaced.
	reg.Release(incumbent)

	require.Equal(t, newcomer, reg.Get("10", "test"),
		"the live plugin after its retired predecessor was released")
}

func TestRegistryClaimRefusesConcurrentRegistration(t *testing.T) {
	var (
		reg   = newRegistry()
		first = &plugin{idx: "10", base: "test"}
		other = &plugin{idx: "10", base: "test"}
	)

	require.NoError(t, reg.Claim(first))
	require.Error(t, reg.Claim(other), "a second claim of a name being registered")

	reg.Release(first)
	require.NoError(t, reg.Claim(other), "a claim after the first one was released")
}

// A plugin which lost its claim must not become the live instance. It
// goes away while it registers, and the claim goes with it.
func TestRegistryCommitNeedsTheClaim(t *testing.T) {
	var (
		reg       = newRegistry()
		incumbent = &plugin{idx: "10", base: "test"}
		newcomer  = &plugin{idx: "10", base: "test"}
	)

	require.NoError(t, reg.Claim(incumbent))
	_, err := reg.Commit(incumbent)
	require.NoError(t, err)

	require.NoError(t, reg.Claim(newcomer))
	reg.Release(newcomer)

	_, err = reg.Commit(newcomer)
	require.Error(t, err, "commit of a plugin which lost its claim")
	require.Equal(t, incumbent, reg.Get("10", "test"),
		"the live plugin after a failed commit")
}
