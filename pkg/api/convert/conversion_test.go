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

package convert_test

import (
	"fmt"

	"github.com/containerd/nri/pkg/api/convert"
	v1alpha1 "github.com/containerd/nri/pkg/api/v1alpha1"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

	"testing"

	faker "github.com/brianvoe/gofakeit/v7"
	"github.com/stretchr/testify/require"

	yaml "gopkg.in/yaml.v3"
)

//
// Notes:
// In lack of a better idea, we use gofakeit to generate pseudo-random
// messages for conversion testing. The idea is to generate a message
// in one version, convert it to the other version and back, and then
// compare the result with the original message. If they are identical,
// the conversion is likely correct. Obviously this is not a 100% proof
// but should be good enough at least for a start.
//

func TestRegisterPluginConversion(t *testing.T) {
	t.Run("request", func(t *testing.T) {
		msg := &v1alpha1.RegisterPluginRequest{}
		faker.Struct(msg)
		chk := convert.RegisterPluginRequestToV1alpha1(
			convert.RegisterPluginRequest(msg),
		)

		require.True(t,
			protoEqual(chk, msg),
			protoUnexpectedDiff("converted", "original", chk, msg),
		)
	})

	t.Run("response", func(t *testing.T) {
		msg := &v1alpha1.Empty{}
		faker.Struct(msg)
		chk := convert.RegisterPluginResponse(
			convert.RegisterPluginResponseToV1beta1(msg),
		)

		require.True(t,
			protoEqual(chk, msg),
			protoUnexpectedDiff("converted", "original", chk, msg),
		)
	})
}

func TestUpdateContainersConversion(t *testing.T) {
	t.Run("request", func(t *testing.T) {
		msg := &v1alpha1.UpdateContainersRequest{}
		faker.Struct(msg)
		chk := convert.UpdateContainersRequestToV1alpha1(
			convert.UpdateContainersRequest(msg),
		)

		require.True(t,
			protoEqual(chk, msg),
			protoUnexpectedDiff("converted", "original", chk, msg),
		)
	})

	t.Run("response", func(t *testing.T) {
		msg := &v1alpha1.UpdateContainersResponse{}
		faker.Struct(msg)
		chk := convert.UpdateContainersResponse(
			convert.UpdateContainersResponseToV1beta1(msg),
		)

		require.True(t,
			protoEqual(chk, msg),
			protoUnexpectedDiff("converted", "original", chk, msg),
		)
	})
}

func TestConfigureConversion(t *testing.T) {
	t.Run("request", func(t *testing.T) {
		msg := &v1alpha1.ConfigureRequest{}
		faker.Struct(msg)
		chk := convert.ConfigureRequest(
			convert.ConfigureRequestToV1beta1(msg),
		)

		require.True(t,
			protoEqual(chk, msg),
			protoUnexpectedDiff("converted", "original", chk, msg),
		)
	})

	t.Run("response", func(t *testing.T) {
		msg := &v1alpha1.ConfigureResponse{}
		faker.Struct(msg)
		chk := convert.ConfigureResponseToV1alpha1(
			convert.ConfigureResponse(msg),
		)

		require.True(t,
			protoEqual(chk, msg),
			protoUnexpectedDiff("converted", "original", chk, msg),
		)
	})

}

func TestSynchronizeConversion(t *testing.T) {
	t.Run("request", func(t *testing.T) {
		msg := &v1alpha1.SynchronizeRequest{}
		faker.Struct(msg)
		chk := convert.SynchronizeRequest(
			convert.SynchronizeRequestToV1beta1(msg),
		)

		require.True(t,
			protoEqual(chk, msg),
			protoUnexpectedDiff("converted", "original", chk, msg),
		)
	})

	t.Run("response", func(t *testing.T) {
		msg := &v1alpha1.SynchronizeResponse{}
		faker.Struct(msg)
		chk := convert.SynchronizeResponseToV1alpha1(
			convert.SynchronizeResponse(msg),
		)

		require.True(t,
			protoEqual(chk, msg),
			protoUnexpectedDiff("converted", "original", chk, msg),
		)
	})
}

func TestRunPodSandboxConversion(t *testing.T) {
	t.Run("request", func(t *testing.T) {
		msg := &v1alpha1.RunPodSandboxRequest{}
		faker.Struct(msg)
		msg.Event = v1alpha1.Event_RUN_POD_SANDBOX
		msg.Container = nil
		chk := convert.RunPodSandboxRequest(
			convert.RunPodSandboxRequestToV1beta1(msg),
		)

		require.True(t,
			protoEqual(chk, msg),
			protoUnexpectedDiff("converted", "original", chk, msg),
		)
	})

	t.Run("response", func(t *testing.T) {
		msg := &v1alpha1.Empty{}
		faker.Struct(msg)
		chk := convert.RunPodSandboxResponseToV1alpha1(
			convert.RunPodSandboxResponse(msg),
		)

		require.True(t,
			protoEqual(chk, msg),
			protoUnexpectedDiff("converted", "original", chk, msg),
		)
	})
}

func TestUpdatePodSandboxRequestConversion(t *testing.T) {
	t.Run("request", func(t *testing.T) {
		msg := &v1alpha1.UpdatePodSandboxRequest{}
		chk := convert.UpdatePodSandboxRequest(
			convert.UpdatePodSandboxRequestToV1beta1(msg),
		)

		require.True(t,
			protoEqual(chk, msg),
			protoUnexpectedDiff("converted", "original", chk, msg),
		)
	})

	t.Run("response", func(t *testing.T) {
		msg := &v1alpha1.UpdatePodSandboxResponse{}
		faker.Struct(msg)
		chk := convert.UpdatePodSandboxResponseToV1alpha1(
			convert.UpdatePodSandboxResponse(msg),
		)

		require.True(t,
			protoEqual(chk, msg),
			protoUnexpectedDiff("converted", "original", chk, msg),
		)
	})
}

func TestPostUpdatePodSandboxConversion(t *testing.T) {
	t.Run("request", func(t *testing.T) {
		msg := &v1alpha1.PostUpdatePodSandboxRequest{}
		faker.Struct(msg)
		msg.Event = v1alpha1.Event_POST_UPDATE_POD_SANDBOX
		msg.Container = nil
		chk := convert.PostUpdatePodSandboxRequest(
			convert.PostUpdatePodSandboxRequestToV1beta1(msg),
		)

		require.True(t,
			protoEqual(chk, msg),
			protoUnexpectedDiff("converted", "original", chk, msg),
		)
	})

	t.Run("response", func(t *testing.T) {
		msg := &v1alpha1.Empty{}
		faker.Struct(msg)
		chk := convert.PostUpdatePodSandboxResponseToV1alpha1(
			convert.PostUpdatePodSandboxResponse(msg),
		)

		require.True(t,
			protoEqual(chk, msg),
			protoUnexpectedDiff("converted", "original", chk, msg),
		)
	})
}

func TestStopPodSandboxConversion(t *testing.T) {
	t.Run("request", func(t *testing.T) {
		msg := &v1alpha1.StopPodSandboxRequest{}
		faker.Struct(msg)
		msg.Event = v1alpha1.Event_STOP_POD_SANDBOX
		msg.Container = nil
		chk := convert.StopPodSandboxRequest(
			convert.StopPodSandboxRequestToV1beta1(msg),
		)

		require.True(t,
			protoEqual(chk, msg),
			protoUnexpectedDiff("converted", "original", chk, msg),
		)
	})

	t.Run("response", func(t *testing.T) {
		msg := &v1alpha1.Empty{}
		faker.Struct(msg)
		chk := convert.StopPodSandboxResponseToV1alpha1(
			convert.StopPodSandboxResponse(msg),
		)

		require.True(t,
			protoEqual(chk, msg),
			protoUnexpectedDiff("converted", "original", chk, msg),
		)
	})
}

func TestRemovePodSandboxConversion(t *testing.T) {
	t.Run("request", func(t *testing.T) {
		msg := &v1alpha1.RemovePodSandboxRequest{}
		faker.Struct(msg)
		msg.Event = v1alpha1.Event_REMOVE_POD_SANDBOX
		msg.Container = nil
		chk := convert.RemovePodSandboxRequest(
			convert.RemovePodSandboxRequestToV1beta1(msg),
		)

		require.True(t,
			protoEqual(chk, msg),
			protoUnexpectedDiff("converted", "original", chk, msg),
		)
	})

	t.Run("response", func(t *testing.T) {
		msg := &v1alpha1.Empty{}
		faker.Struct(msg)
		chk := convert.RemovePodSandboxResponseToV1alpha1(
			convert.RemovePodSandboxResponse(msg),
		)

		require.True(t,
			protoEqual(chk, msg),
			protoUnexpectedDiff("converted", "original", chk, msg),
		)
	})
}

func TestCreateContainerConversion(t *testing.T) {
	t.Run("request", func(t *testing.T) {
		msg := &v1alpha1.CreateContainerRequest{}
		chk := convert.CreateContainerRequest(
			convert.CreateContainerRequestToV1beta1(msg),
		)

		require.True(t,
			protoEqual(chk, msg),
			protoUnexpectedDiff("converted", "original", chk, msg),
		)
	})

	t.Run("response", func(t *testing.T) {
		msg := &v1alpha1.CreateContainerResponse{}
		faker.Struct(msg)
		chk := convert.CreateContainerResponseToV1alpha1(
			convert.CreateContainerResponse(msg),
		)

		require.True(t,
			protoEqual(chk, msg),
			protoUnexpectedDiff("converted", "original", chk, msg),
		)
	})
}

func TestPostCreateContainerConversion(t *testing.T) {
	t.Run("request", func(t *testing.T) {
		msg := &v1alpha1.PostCreateContainerRequest{}
		faker.Struct(msg)
		msg.Event = v1alpha1.Event_POST_CREATE_CONTAINER
		chk := convert.PostCreateContainerRequest(
			convert.PostCreateContainerRequestToV1beta1(msg),
		)

		require.True(t,
			protoEqual(chk, msg),
			protoUnexpectedDiff("converted", "original", chk, msg),
		)
	})

	t.Run("response", func(t *testing.T) {
		msg := &v1alpha1.Empty{}
		chk := convert.PostCreateContainerResponseToV1alpha1(
			convert.PostCreateContainerResponse(msg),
		)

		require.True(t,
			protoEqual(chk, msg),
			protoUnexpectedDiff("converted", "original", chk, msg),
		)
	})
}

func TestStartContainerConversion(t *testing.T) {
	t.Run("request", func(t *testing.T) {
		msg := &v1alpha1.StartContainerRequest{}
		faker.Struct(msg)
		msg.Event = v1alpha1.Event_START_CONTAINER
		chk := convert.StartContainerRequest(
			convert.StartContainerRequestToV1beta1(msg),
		)

		require.True(t,
			protoEqual(chk, msg),
			protoUnexpectedDiff("converted", "original", chk, msg),
		)
	})

	t.Run("response", func(t *testing.T) {
		msg := &v1alpha1.Empty{}
		chk := convert.StartContainerResponseToV1alpha1(
			convert.StartContainerResponse(msg),
		)

		require.True(t,
			protoEqual(chk, msg),
			protoUnexpectedDiff("converted", "original", chk, msg),
		)
	})
}

func TestPostStartContainerConversion(t *testing.T) {
	t.Run("request", func(t *testing.T) {
		msg := &v1alpha1.PostStartContainerRequest{}
		faker.Struct(msg)
		msg.Event = v1alpha1.Event_POST_START_CONTAINER
		chk := convert.PostStartContainerRequest(
			convert.PostStartContainerRequestToV1beta1(msg),
		)

		require.True(t,
			protoEqual(chk, msg),
			protoUnexpectedDiff("converted", "original", chk, msg),
		)
	})

	t.Run("response", func(t *testing.T) {
		msg := &v1alpha1.Empty{}
		chk := convert.PostStartContainerResponseToV1alpha1(
			convert.PostStartContainerResponse(msg),
		)

		require.True(t,
			protoEqual(chk, msg),
			protoUnexpectedDiff("converted", "original", chk, msg),
		)
	})
}

func TestUpdateContainerConversion(t *testing.T) {
	t.Run("request", func(t *testing.T) {
		msg := &v1alpha1.UpdateContainerRequest{}
		faker.Struct(msg)
		chk := convert.UpdateContainerRequest(
			convert.UpdateContainerRequestToV1beta1(msg),
		)

		require.True(t,
			protoEqual(chk, msg),
			protoUnexpectedDiff("converted", "original", chk, msg),
		)
	})

	t.Run("response", func(t *testing.T) {
		msg := &v1alpha1.UpdateContainerResponse{}
		faker.Struct(msg)
		chk := convert.UpdateContainerResponseToV1alpha1(
			convert.UpdateContainerResponse(msg),
		)

		require.True(t,
			protoEqual(chk, msg),
			protoUnexpectedDiff("converted", "original", chk, msg),
		)
	})
}

func TestPostUpdateContainerConversion(t *testing.T) {
	t.Run("request", func(t *testing.T) {
		msg := &v1alpha1.PostUpdateContainerRequest{}
		faker.Struct(msg)
		msg.Event = v1alpha1.Event_POST_UPDATE_CONTAINER
		chk := convert.PostUpdateContainerRequest(
			convert.PostUpdateContainerRequestToV1beta1(msg),
		)

		require.True(t,
			protoEqual(chk, msg),
			protoUnexpectedDiff("converted", "original", chk, msg),
		)
	})

	t.Run("response", func(t *testing.T) {
		msg := &v1alpha1.Empty{}
		chk := convert.PostUpdateContainerResponseToV1alpha1(
			convert.PostUpdateContainerResponse(msg),
		)

		require.True(t,
			protoEqual(chk, msg),
			protoUnexpectedDiff("converted", "original", chk, msg),
		)
	})
}

func TestStopContainerConversion(t *testing.T) {
	t.Run("request", func(t *testing.T) {
		msg := &v1alpha1.StopContainerRequest{}
		faker.Struct(msg)
		chk := convert.StopContainerRequest(
			convert.StopContainerRequestToV1beta1(msg),
		)

		require.True(t,
			protoEqual(chk, msg),
			protoUnexpectedDiff("converted", "original", chk, msg),
		)
	})

	t.Run("response", func(t *testing.T) {
		msg := &v1alpha1.StopContainerResponse{}
		faker.Struct(msg)
		chk := convert.StopContainerResponseToV1alpha1(
			convert.StopContainerResponse(msg),
		)

		require.True(t,
			protoEqual(chk, msg),
			protoUnexpectedDiff("converted", "original", chk, msg),
		)
	})
}

func TestRemoveContainerConversion(t *testing.T) {
	t.Run("request", func(t *testing.T) {
		msg := &v1alpha1.RemoveContainerRequest{}
		faker.Struct(msg)
		msg.Event = v1alpha1.Event_REMOVE_CONTAINER
		chk := convert.RemoveContainerRequest(
			convert.RemoveContainerRequestToV1beta1(msg),
		)

		require.True(t,
			protoEqual(chk, msg),
			protoUnexpectedDiff("converted", "original", chk, msg),
		)
	})

	t.Run("response", func(t *testing.T) {
		msg := &v1alpha1.Empty{}
		chk := convert.RemoveContainerResponseToV1alpha1(
			convert.RemoveContainerResponse(msg),
		)

		require.True(t,
			protoEqual(chk, msg),
			protoUnexpectedDiff("converted", "original", chk, msg),
		)
	})
}

func protoDiff(a, b proto.Message) string {
	return cmp.Diff(a, b, protocmp.Transform())
}

func protoEqual(a, b proto.Message) bool {
	return cmp.Equal(a, b, cmpopts.EquateEmpty(), protocmp.Transform())
}

func protoUnexpectedDiff(aKind, bKind string, a, b proto.Message) string {
	aData, _ := yaml.Marshal(a)
	bData, _ := yaml.Marshal(b)
	return fmt.Sprintf("diff:\n%s\n", protoDiff(a, b)) +
		fmt.Sprintf("%s message:\n%s\n", aKind, aData) +
		fmt.Sprintf("%s message:\n%s\n", bKind, bData)
}

func init() {
	// Make sure gofakeit properly generates test conversion data for
	// our deeply nested structs, slices of pointers to structs, etc.
	// The default is 10 which is not enough for some of our data types.
	faker.RecursiveDepth = 25
}
