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
	"fmt"
	"os"
	"strings"

	"github.com/containers/common/pkg/hooks"
	rspec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/yaml"

	"github.com/containerd/nri/pkg/api"
	"github.com/containerd/nri/pkg/stub"
)

var (
	log     *logrus.Logger
	verbose bool
)

type plugin struct {
	stub stub.Stub
	mgr  *hooks.Manager
}

func (p *plugin) CreateContainer(_ context.Context, pod *api.PodSandbox, container *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
	ctrName := containerName(pod, container)

	if verbose {
		dump("CreateContainer", "pod", pod, "container", container)
	}

	annotations := map[string]string{}
	for k, v := range container.Annotations {
		annotations[k] = v
	}
	for k, v := range pod.Annotations {
		annotations[k] = v
	}
	hasBindMounts := len(container.Mounts) > 0

	spec := &rspec.Spec{
		Process: &rspec.Process{
			Args: container.Args,
		},
	}

	if verbose {
		dump(ctrName, "OCI Spec", spec)
		dump(ctrName, "Annotations", annotations)
	}

	if _, err := p.mgr.Hooks(spec, annotations, hasBindMounts); err != nil {
		log.Errorf("%s: failed to generate hooks: %v", ctrName, err)
		return nil, nil, fmt.Errorf("hook generation failed: %w", err)
	}

	if spec.Hooks == nil {
		log.Infof("%s: no hooks to inject, ignoring", ctrName)
		return nil, nil, nil
	}

	adjust := &api.ContainerAdjustment{}
	adjust.AddHooks(api.FromOCIHooks(spec.Hooks))

	if verbose {
		dump(ctrName, "ContainerAdjustment", adjust)
	} else {
		log.Infof("%s: OCI hooks injected", ctrName)
	}

	return adjust, nil, nil
}

// Construct a container name for log messages.
func containerName(pod *api.PodSandbox, container *api.Container) string {
	if pod != nil {
		return pod.Name + "/" + container.Name
	}
	return container.Name
}

// Dump one or more objects, with an optional global prefix and per-object tags.
func dump(args ...interface{}) {
	var (
		prefix string
		idx    int
	)

	if len(args)&0x1 == 1 {
		prefix = args[0].(string)
		idx++
	}

	for ; idx < len(args)-1; idx += 2 {
		tag, obj := args[idx], args[idx+1]
		msg, err := yaml.Marshal(obj)
		if err != nil {
			log.Infof("%s: %s: failed to dump object: %v", prefix, tag, err)
			continue
		}

		if prefix != "" {
			log.Infof("%s: %s:", prefix, tag)
			for _, line := range strings.Split(strings.TrimSpace(string(msg)), "\n") {
				log.Infof("%s:    %s", prefix, line)
			}
		} else {
			log.Infof("%s:", tag)
			for _, line := range strings.Split(strings.TrimSpace(string(msg)), "\n") {
				log.Infof("  %s", line)
			}
		}
	}
}

func main() {
	var (
		pluginIdx    string
		disableWatch bool
		opts         []stub.Option
		mgr          *hooks.Manager
		err          error
	)

	log = logrus.StandardLogger()
	log.SetFormatter(&logrus.TextFormatter{
		PadLevelText: true,
	})

	flag.StringVar(&pluginIdx, "idx", "", "plugin index to register to NRI")
	flag.BoolVar(&verbose, "verbose", false, "enable (more) verbose logging")
	flag.BoolVar(&disableWatch, "disableWatch", false, "disable watching hook directories for new hooks")
	flag.Parse()

	if pluginIdx != "" {
		opts = append(opts, stub.WithPluginIdx(pluginIdx))
	}

	if verbose {
		logrus.SetLevel(logrus.DebugLevel)
	}

	p := &plugin{}
	if p.stub, err = stub.New(p, opts...); err != nil {
		log.Errorf("failed to create plugin stub: %v", err)
		os.Exit(1)
	}

	ctx := context.Background()
	dirs := []string{hooks.DefaultDir, hooks.OverrideDir}
	mgr, err = hooks.New(ctx, dirs, []string{})
	if err != nil {
		log.Errorf("failed to set up hook manager: %v", err)
		os.Exit(1)
	}
	p.mgr = mgr

	if !disableWatch {
		for _, dir := range dirs {
			if err = os.MkdirAll(dir, 0755); err != nil {
				log.Errorf("failed to create directory %q: %v", dir, err)
				os.Exit(1)
			}
		}

		sync := make(chan error, 2)
		go mgr.Monitor(ctx, sync)

		err = <-sync
		if err != nil {
			log.Errorf("failed to monitor hook directories: %v", err)
			os.Exit(1)
		}
		log.Infof("watching directories %q for new changes", strings.Join(dirs, " "))
	}

	err = p.stub.Run(ctx)
	if err != nil {
		log.Errorf("plugin exited with error %v", err)
		os.Exit(1)
	}
}
