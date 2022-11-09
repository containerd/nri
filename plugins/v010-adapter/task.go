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
	"syscall"

	"github.com/containerd/containerd"
	ctrdapitypes "github.com/containerd/containerd/api/types"
	"github.com/containerd/containerd/cio"
	oci "github.com/opencontainers/runtime-spec/specs-go"
)

type fakeTask struct {
	id   string
	pid  uint32
	spec *oci.Spec
}

func newFakeTask(id string, pid uint32, spec *oci.Spec) containerd.Task {
	return &fakeTask{
		id:   id,
		pid:  pid,
		spec: spec,
	}
}

func (t *fakeTask) ID() string {
	return t.id
}

func (t *fakeTask) Pid() uint32 {
	return t.pid
}

func (t *fakeTask) Spec(context.Context) (*oci.Spec, error) {
	return t.spec, nil
}

func (t *fakeTask) Start(context.Context) error {
	return nil
}

func (t *fakeTask) Delete(context.Context, ...containerd.ProcessDeleteOpts) (*containerd.ExitStatus, error) {
	return nil, fmt.Errorf("unimplemented")
}

func (t *fakeTask) Kill(context.Context, syscall.Signal, ...containerd.KillOpts) error {
	return fmt.Errorf("unimplemented")
}

func (t *fakeTask) Wait(context.Context) (<-chan containerd.ExitStatus, error) {
	return nil, fmt.Errorf("unimplemented")
}

func (t *fakeTask) CloseIO(context.Context, ...containerd.IOCloserOpts) error {
	return fmt.Errorf("unimplemented")
}

func (t *fakeTask) Resize(context.Context, uint32, uint32) error {
	return fmt.Errorf("unimplemented")
}

func (t *fakeTask) IO() cio.IO {
	return nil
}

func (t *fakeTask) Status(context.Context) (containerd.Status, error) {
	return containerd.Status{}, fmt.Errorf("unimplemented")
}

func (t *fakeTask) Pause(context.Context) error {
	return fmt.Errorf("unimplemented")
}

func (t *fakeTask) Resume(context.Context) error {
	return fmt.Errorf("unimplemented")
}

func (t *fakeTask) Exec(context.Context, string, *oci.Process, cio.Creator) (containerd.Process, error) {
	return nil, fmt.Errorf("unimplemented")
}

func (t *fakeTask) Pids(context.Context) ([]containerd.ProcessInfo, error) {
	return nil, fmt.Errorf("unimplemented")
}

func (t *fakeTask) Checkpoint(context.Context, ...containerd.CheckpointTaskOpts) (containerd.Image, error) {
	return nil, fmt.Errorf("unimplemented")
}

func (t *fakeTask) Update(context.Context, ...containerd.UpdateTaskOpts) error {
	return fmt.Errorf("unimplemented")
}

func (t *fakeTask) LoadProcess(context.Context, string, cio.Attach) (containerd.Process, error) {
	return nil, fmt.Errorf("unimplemented")
}

func (t *fakeTask) Metrics(context.Context) (*ctrdapitypes.Metric, error) {
	return nil, fmt.Errorf("unimplemented")
}
