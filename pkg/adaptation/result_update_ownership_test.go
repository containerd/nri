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

	"github.com/containerd/nri/pkg/api"
	"github.com/stretchr/testify/require"
)

func TestNoopResourceUpdatePreservesOwners(t *testing.T) {
	for name, owners := range map[string]*api.OwningPlugins{
		"nil map":     {},
		"absent ID":   api.NewOwningPlugins(),
		"nil entry":   {Owners: map[string]*api.FieldOwners{"ctr0": nil}},
		"empty entry": {Owners: map[string]*api.FieldOwners{"ctr0": api.NewFieldOwners()}},
		"nil fields":  {Owners: map[string]*api.FieldOwners{"ctr0": {}}},
	} {
		for _, resources := range []*api.LinuxResources{nil, {}} {
			t.Run(name+"/resources="+resources.String(), func(t *testing.T) {
				r := collectUpdateContainerResult(&api.UpdateContainerRequest{Container: &api.Container{Id: "ctr0"}})
				r.owners = owners
				before, hadOwners := owners.Owners["ctr0"]
				mapWasNil := owners.Owners == nil
				u := &api.ContainerUpdate{ContainerId: "ctr0", Linux: &api.LinuxContainerUpdate{Resources: resources}}
				reply, err := r.getContainerUpdate(u, "test")
				require.NoError(t, err)
				require.NoError(t, r.updateResources(reply, u, "test"))
				after, hasOwners := owners.Owners["ctr0"]
				require.Equal(t, hadOwners, hasOwners)
				require.Equal(t, mapWasNil, owners.Owners == nil)
				require.Equal(t, before, after)
			})
		}
	}
}

func TestFailedResourceUpdateRestoresOwnerEntry(t *testing.T) {
	for _, hadOwners := range []bool{false, true} {
		t.Run(map[bool]string{false: "absent ID", true: "empty entry"}[hadOwners], func(t *testing.T) {
			r := collectUpdateContainerResult(&api.UpdateContainerRequest{Container: &api.Container{Id: "ctr0"}})
			if hadOwners {
				r.owners.Owners["ctr0"] = api.NewFieldOwners()
			}
			require.NoError(t, r.owners.ClaimCPUShares("other", "prior"))
			before, other := r.owners.Owners["ctr0"], r.owners.Owners["other"]
			u := &api.ContainerUpdate{ContainerId: "ctr0", IgnoreFailure: true}
			u.AddLinuxHugepageLimit("2M", 256)
			u.AddLinuxHugepageLimit("2M", 512)
			reply, err := r.getContainerUpdate(u, "test")
			require.NoError(t, err)
			require.ErrorContains(t, r.updateResources(reply, u, "test"), "HugepageLimits 2M")
			after, hasOwners := r.owners.Owners["ctr0"]
			require.Equal(t, hadOwners, hasOwners)
			require.Same(t, before, after)
			_, claimed := r.owners.HugepageLimitOwner("ctr0", "2M")
			require.False(t, claimed, "failed update must not leave a new claim")
			require.Same(t, other, r.owners.Owners["other"])
		})
	}
}
