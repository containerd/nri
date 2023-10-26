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

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/containerd/cgroups"
	"github.com/containerd/nri/skel"
	types "github.com/containerd/nri/types/v1"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
)

// clearCFS clears any cfs quotas for the containers
type clearCFS struct {
}

func (c *clearCFS) Type() string {
	return "clearcfs"
}

func (c *clearCFS) Invoke(ctx context.Context, r *types.Request) (*types.Result, error) {
	result := r.NewResult(c.Type())

	if r.State != types.Create {
		return result, nil
	}

	switch r.Spec.Annotations["qos.class"] {
	case "ls":
		logrus.Debugf("clearing cfs for %s", r.ID)
		control, err := cgroups.Load(cgroups.V1, cgroups.StaticPath(r.Spec.CgroupsPath))
		if err != nil {
			return nil, err
		}

		quota := int64(-1)
		return result, control.Update(&specs.LinuxResources{
			CPU: &specs.LinuxCPU{
				Quota: &quota,
			},
		})
	}
	return result, nil
}

func main() {
	ctx := context.Background()
	if err := skel.Run(ctx, &clearCFS{}); err != nil {
		fmt.Fprintf(os.Stderr, "%s", err)
		os.Exit(1)
	}
}
