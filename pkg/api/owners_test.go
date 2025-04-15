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

func TestSimpleClaims(t *testing.T) {
	o := api.NewOwningPlugins()

	// claim hooks
	err := o.ClaimHooks("ctr0", "test0")
	require.NoError(t, err, "hooks")

	// claim memory limit
	err = o.ClaimMemLimit("ctr0", "test0")
	require.NoError(t, err, "memory limit")

	// claim memory limit of another container
	err = o.ClaimMemLimit("ctr1", "test0")
	require.NoError(t, err, "another memory limit")

	// claim memory limit of same container by another plugin
	err = o.ClaimMemLimit("ctr0", "test1")
	require.Error(t, err, "same memory limit by another plugin, should conflict")

	// claim CPU shares
	err = o.ClaimCPUShares("ctr0", "test0")
	require.NoError(t, err, "CPU shares")

	// claim CPU shares of another container
	err = o.ClaimCPUShares("ctr1", "test0")
	require.NoError(t, err, "other CPU shares")

	// claim CPU shares of same container by another plugin
	err = o.ClaimCPUShares("ctr0", "test1")
	require.Error(t, err, "same CPU shares by another plugin, should conflict")

	// claim args
	err = o.ClaimArgs("ctr0", "test0")
	require.NoError(t, err, "args")

	// claim same args by another plugin
	err = o.ClaimArgs("ctr0", "test1")
	require.Error(t, err, "same args by another plugin, should conflict")

	// clear args
	o.ClearArgs("ctr0", "test1")

	// claim args after clearing
	err = o.ClaimArgs("ctr0", "test1")
	require.NoError(t, err, "try again same args, should not conflict")

	// clear args
	o.ClearArgs("ctr0", "test1")

	// claim args by another plugin
	err = o.ClaimArgs("ctr0", "test0")
	require.Error(t, err, "try again same args by non-clearing plugin, should conflict")
}

func TestCompoundClaims(t *testing.T) {
	o := api.NewOwningPlugins()

	// claim environment variable
	err := o.ClaimEnv("ctr0", "VAR0", "test0")
	require.NoError(t, err, "env VAR0")

	// claim another environment variable
	err = o.ClaimEnv("ctr0", "VAR1", "test0")
	require.NoError(t, err, "env VAR1")

	// claim environment variable of another container
	err = o.ClaimEnv("ctr1", "VAR0", "test1")
	require.NoError(t, err, "env VAR0 of another container")

	// claim already claimed environment variable by another plugin
	err = o.ClaimEnv("ctr0", "VAR1", "test1")
	require.Error(t, err, "env VAR1 of same container by another plugin, should conflict")

	// clear environment variable
	o.ClearEnv("ctr0", "VAR1", "test1")

	// claim same environment variable
	err = o.ClaimEnv("ctr0", "VAR1", "test1")
	require.NoError(t, err, "try again env VAR1, should not conflict")

	// clear again environment variable
	o.ClearEnv("ctr0", "VAR1", "test0")

	// claim same environment variable by another plugin
	err = o.ClaimEnv("ctr0", "VAR1", "test1")
	require.Error(t, err, "try again env VAR1 by non-clearing plugin, should conflict")

	require.Equal(t, api.Field_Annotations.String(), "Annotations", "annotation field name")
}
