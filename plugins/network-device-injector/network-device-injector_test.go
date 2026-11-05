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
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"

	"github.com/containerd/nri/pkg/api"
)

// Path of a network namespace that does not exist. Opening it fails at the
// first step of inject()/release(), deterministically and without any
// privileges, so both can be exercised in plain CI environments.
const bogusNetns = "/nonexistent-network-device-injector-test"

func TestRunPodSandboxPropagatesInjectError(t *testing.T) {
	log = logrus.New()
	log.SetOutput(io.Discard)

	p := &plugin{}
	pod := annotatedPod(bogusNetns)
	if err := p.RunPodSandbox(context.Background(), pod); err == nil {
		t.Fatal("RunPodSandbox: failure to inject netdevice must be reported, got nil")
	}
}

// A release failure must be logged but must not fail the pod deletion:
// the kernel removes the devices together with the network namespace.
func TestStopPodSandboxLogsReleaseError(t *testing.T) {
	var buf bytes.Buffer
	log = logrus.New()
	log.SetOutput(&buf)

	p := &plugin{}
	pod := annotatedPod(bogusNetns)
	if err := p.StopPodSandbox(context.Background(), pod); err != nil {
		t.Fatalf("StopPodSandbox: release failure must not block pod deletion: %v", err)
	}
	if out := buf.String(); !strings.Contains(out, "failed to release netdevice") {
		t.Fatalf("StopPodSandbox: release failure must be logged, log output: %q", out)
	}
}

func annotatedPod(nsPath string) *api.PodSandbox {
	return &api.PodSandbox{
		Annotations: map[string]string{
			"netdevice.noderesource.dev/pod": "- name: dummy0\n  new_name: eth1\n",
		},
		Linux: &api.LinuxPodSandbox{
			Namespaces: []*api.LinuxNamespace{
				{Type: "network", Path: nsPath},
			},
		},
	}
}
