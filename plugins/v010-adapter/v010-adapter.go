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
	"flag"
	"os"

	"github.com/sirupsen/logrus"

	"github.com/containerd/nri"
	"github.com/containerd/nri/pkg/api"
	"github.com/containerd/nri/pkg/stub"
	nriv1 "github.com/containerd/nri/types/v1"
	oci "github.com/opencontainers/runtime-spec/specs-go"
)

type plugin struct {
	stub stub.Stub
}

var (
	log *logrus.Logger
)

func (p *plugin) Configure(_ context.Context, config, runtime, version string) (stub.EventMask, error) {
	log.Infof("Connected to %s/%s...", runtime, version)
	return 0, nil
}

func (p *plugin) Shutdown(_ context.Context) {
	log.Info("Runtime shutting down...")
}

func (p *plugin) RunPodSandbox(_ context.Context, pod *api.PodSandbox) error {
	log.Infof("Started pod %s/%s...", pod.GetNamespace(), pod.GetName())

	nric, err := nri.New()
	if err != nil {
		log.WithError(err).Errorf("unable to create nri client")
		return err
	}

	if nric == nil {
		return nil
	}

	ctx := context.Background()
	nriSB := sandboxFromPod(pod)
	task := newFakeTask(pod.GetId(), pod.GetPid(), ociSpecFromPod(pod))

	if _, err := nric.InvokeWithSandbox(ctx, task, nriv1.Create, nriSB); err != nil {
		log.WithError(err).Errorf("nri invoke failed")
		return err
	}

	return nil
}

func (p *plugin) StopPodSandbox(_ context.Context, pod *api.PodSandbox) error {
	log.Infof("Stopped pod %s/%s...", pod.GetNamespace(), pod.GetName())

	nric, err := nri.New()
	if err != nil {
		log.WithError(err).Errorf("unable to create nri client")
		return err
	}

	if nric == nil {
		return nil
	}

	ctx := context.Background()
	nriSB := sandboxFromPod(pod)
	task := newFakeTask(pod.GetId(), pod.GetPid(), ociSpecFromPod(pod))

	if _, err := nric.InvokeWithSandbox(ctx, task, nriv1.Delete, nriSB); err != nil {
		log.WithError(err).Errorf("Failed to delete nri for %q", task.ID())
		return err
	}

	return nil
}

func (p *plugin) StartContainer(_ context.Context, pod *api.PodSandbox, ctr *api.Container) error {
	log.Infof("Starting container %s/%s/%s...", pod.GetNamespace(), pod.GetName(), ctr.GetName())

	nric, err := nri.New()
	if err != nil {
		log.WithError(err).Errorf("unable to create nri client")
		return err
	}

	if nric == nil {
		return nil
	}

	ctx := context.Background()
	nriSB := sandboxFromPod(pod)
	task := newFakeTask(ctr.GetId(), ctr.GetPid(), ociSpecFromContainer(ctr))

	if _, err := nric.InvokeWithSandbox(ctx, task, nriv1.Create, nriSB); err != nil {
		log.WithError(err).Errorf("nri invoke failed")
		return err
	}

	return nil
}

func (p *plugin) StopContainer(_ context.Context, pod *api.PodSandbox, ctr *api.Container) ([]*api.ContainerUpdate, error) {
	log.Infof("Stopped container %s/%s/%s...", pod.GetNamespace(), pod.GetName(), ctr.GetName())

	nric, err := nri.New()
	if err != nil {
		log.WithError(err).Errorf("unable to create nri client")
		return []*api.ContainerUpdate{}, nil
	}

	if nric == nil {
		return []*api.ContainerUpdate{}, nil
	}

	ctx := context.Background()
	nriSB := sandboxFromPod(pod)
	task := newFakeTask(ctr.GetId(), ctr.GetPid(), ociSpecFromContainer(ctr))

	if _, err := nric.InvokeWithSandbox(ctx, task, nriv1.Delete, nriSB); err != nil {
		log.WithError(err).Errorf("Failed to delete nri for %q", task.ID())
	}

	return []*api.ContainerUpdate{}, nil
}

func (p *plugin) onClose() {
	log.Infof("Connection to the runtime lost, exiting...")
	os.Exit(1)
}

func sandboxFromPod(pod *api.PodSandbox) *nri.Sandbox {
	return &nri.Sandbox{
		ID:     pod.GetId(),
		Labels: pod.GetLabels(),
	}
}

func ociSpecFromPod(pod *api.PodSandbox) *oci.Spec {
	return &oci.Spec{
		Annotations: pod.GetAnnotations(),
		Linux: &oci.Linux{
			Namespaces:  namespacesToSlice(pod.GetLinux().GetNamespaces()),
			Resources:   pod.GetLinux().GetResources().ToOCI(),
			CgroupsPath: pod.GetLinux().GetCgroupsPath(),
		},
	}
}

func ociSpecFromContainer(ctr *api.Container) *oci.Spec {
	return &oci.Spec{
		Annotations: ctr.GetAnnotations(),
		Linux: &oci.Linux{
			Namespaces:  namespacesToSlice(ctr.GetLinux().GetNamespaces()),
			Resources:   ctr.GetLinux().GetResources().ToOCI(),
			CgroupsPath: ctr.GetLinux().GetCgroupsPath(),
		},
	}
}

func namespacesToSlice(namespaces []*api.LinuxNamespace) []oci.LinuxNamespace {
	var slice []oci.LinuxNamespace

	for _, ns := range namespaces {
		slice = append(slice, oci.LinuxNamespace{
			Type: oci.LinuxNamespaceType(ns.Type),
			Path: ns.Path,
		})
	}

	return slice
}

func main() {
	var (
		pluginIdx string
		err       error
	)

	log = logrus.StandardLogger()
	log.SetFormatter(&logrus.TextFormatter{
		PadLevelText: true,
	})

	flag.StringVar(&pluginIdx, "idx", "", "plugin index to register to NRI")
	flag.Parse()

	p := &plugin{}
	opts := []stub.Option{
		stub.WithOnClose(p.onClose),
	}
	if pluginIdx != "" {
		opts = append(opts, stub.WithPluginIdx(pluginIdx))
	}

	if p.stub, err = stub.New(p, opts...); err != nil {
		log.Fatalf("failed to create plugin stub: %v", err)
	}

	if err = p.stub.Run(context.Background()); err != nil {
		log.Errorf("plugin exited (%v)", err)
		os.Exit(1)
	}
}
