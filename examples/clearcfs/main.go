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

	"github.com/containerd/cgroups/v3"
	"github.com/containerd/cgroups/v3/cgroup1"
	"github.com/containerd/cgroups/v3/cgroup2"
	"github.com/containerd/log"
	"github.com/containerd/nri/pkg/api"
	"github.com/containerd/nri/pkg/stub"
	"github.com/opencontainers/runtime-spec/specs-go"
)

// clearCFS clears any CFS quotas for containers.
type clearCFS struct{}

func (c *clearCFS) CreateContainer(ctx context.Context, _ *api.PodSandbox, container *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
	if container.Annotations["qos.class"] != "ls" {
		return nil, nil, nil
	}
	if container.Linux == nil {
		return nil, nil, nil
	}

	log.G(ctx).Debugf("clearing CFS quota for %s", container.Id)

	if err := clearCFSQuota(container.Linux.CgroupsPath); err != nil {
		return nil, nil, err
	}

	return nil, nil, nil
}

func clearCFSQuota(path string) error {
	switch cgroups.Mode() {
	case cgroups.Unified:
		control, err := cgroup2.Load(path)
		if err != nil {
			return err
		}
		return control.Update(&cgroup2.Resources{
			CPU: &cgroup2.CPU{
				Max: cgroup2.NewCPUMax(nil, nil),
			},
		})

	case cgroups.Legacy, cgroups.Hybrid:
		control, err := cgroup1.Load(cgroup1.StaticPath(path))
		if err != nil {
			return err
		}

		quota := int64(-1)
		return control.Update(&specs.LinuxResources{
			CPU: &specs.LinuxCPU{
				Quota: &quota,
			},
		})

	default:
		return fmt.Errorf("cgroups are not available")
	}
}

func main() {
	s, err := stub.New(&clearCFS{}, stub.WithPluginName("clearcfs"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err := s.Run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
