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

	"github.com/sirupsen/logrus"
	"sigs.k8s.io/yaml"

	"github.com/containerd/nri/pkg/api"
	"github.com/containerd/nri/pkg/stub"
)

type config struct {
	LogFile       string   `json:"logFile"`
	Events        []string `json:"events"`
	AddAnnotation string   `json:"addAnnotation"`
	SetAnnotation string   `json:"setAnnotation"`
	AddEnv        string   `json:"addEnv"`
	SetEnv        string   `json:"setEnv"`
}

type plugin struct {
	stub stub.Stub
	mask stub.EventMask
}

var (
	cfg config
	log *logrus.Logger
	_   = stub.ConfigureInterface(&plugin{})
)

func (p *plugin) Configure(_ context.Context, config, runtime, version string) (stub.EventMask, error) {
	log.Infof("got configuration data: %q from runtime %s %s", config, runtime, version)
	if config == "" {
		return p.mask, nil
	}

	oldCfg := cfg
	err := yaml.Unmarshal([]byte(config), &cfg)
	if err != nil {
		return 0, fmt.Errorf("failed to parse provided configuration: %w", err)
	}

	p.mask, err = api.ParseEventMask(cfg.Events...)
	if err != nil {
		return 0, fmt.Errorf("failed to parse events in configuration: %w", err)
	}

	if cfg.LogFile != oldCfg.LogFile {
		f, err := os.OpenFile(cfg.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Errorf("failed to open log file %q: %v", cfg.LogFile, err)
			return 0, fmt.Errorf("failed to open log file %q: %w", cfg.LogFile, err)
		}
		log.SetOutput(f)
	}

	return p.mask, nil
}

func (p *plugin) Synchronize(_ context.Context, pods []*api.PodSandbox, containers []*api.Container) ([]*api.ContainerUpdate, error) {
	dump("Synchronize", "pods", pods, "containers", containers)
	return nil, nil
}

func (p *plugin) Shutdown() {
	dump("Shutdown")
}

func (p *plugin) RunPodSandbox(_ context.Context, pod *api.PodSandbox) error {
	dump("RunPodSandbox", "pod", pod)
	return nil
}

func (p *plugin) UpdatePodSandbox(_ context.Context, pod *api.PodSandbox, overHeadResources, resources *api.LinuxResources) error {
	dump("UpdatePodSandbox", "pod", pod, "overHeadResources", overHeadResources, "resources", resources)
	return nil
}

func (p *plugin) PostUpdatePodSandbox(_ context.Context, pod *api.PodSandbox) error {
	dump("PostUpdatePodSandbox", "pod", pod)
	return nil
}

func (p *plugin) StopPodSandbox(_ context.Context, pod *api.PodSandbox) error {
	dump("StopPodSandbox", "pod", pod)
	return nil
}

func (p *plugin) RemovePodSandbox(_ context.Context, pod *api.PodSandbox) error {
	dump("RemovePodSandbox", "pod", pod)
	return nil
}

func (p *plugin) CreateContainer(_ context.Context, pod *api.PodSandbox, container *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
	dump("CreateContainer", "pod", pod, "container", container)

	adjust := &api.ContainerAdjustment{}

	if cfg.AddAnnotation != "" {
		adjust.AddAnnotation(cfg.AddAnnotation, fmt.Sprintf("logger-pid-%d", os.Getpid()))
	}
	if cfg.SetAnnotation != "" {
		adjust.RemoveAnnotation(cfg.SetAnnotation)
		adjust.AddAnnotation(cfg.SetAnnotation, fmt.Sprintf("logger-pid-%d", os.Getpid()))
	}
	if cfg.AddEnv != "" {
		adjust.AddEnv(cfg.AddEnv, fmt.Sprintf("logger-pid-%d", os.Getpid()))
	}
	if cfg.SetEnv != "" {
		adjust.RemoveEnv(cfg.SetEnv)
		adjust.AddEnv(cfg.SetEnv, fmt.Sprintf("logger-pid-%d", os.Getpid()))
	}

	return adjust, nil, nil
}

func (p *plugin) PostCreateContainer(_ context.Context, pod *api.PodSandbox, container *api.Container) error {
	dump("PostCreateContainer", "pod", pod, "container", container)
	return nil
}

func (p *plugin) StartContainer(_ context.Context, pod *api.PodSandbox, container *api.Container) error {
	dump("StartContainer", "pod", pod, "container", container)
	return nil
}

func (p *plugin) PostStartContainer(_ context.Context, pod *api.PodSandbox, container *api.Container) error {
	dump("PostStartContainer", "pod", pod, "container", container)
	return nil
}

func (p *plugin) UpdateContainer(_ context.Context, pod *api.PodSandbox, container *api.Container, r *api.LinuxResources) ([]*api.ContainerUpdate, error) {
	dump("UpdateContainer", "pod", pod, "container", container, "resources", r)
	return nil, nil
}

func (p *plugin) PostUpdateContainer(_ context.Context, pod *api.PodSandbox, container *api.Container) error {
	dump("PostUpdateContainer", "pod", pod, "container", container)
	return nil
}

func (p *plugin) StopContainer(_ context.Context, pod *api.PodSandbox, container *api.Container) ([]*api.ContainerUpdate, error) {
	dump("StopContainer", "pod", pod, "container", container)
	return nil, nil
}

func (p *plugin) RemoveContainer(_ context.Context, pod *api.PodSandbox, container *api.Container) error {
	dump("RemoveContainer", "pod", pod, "container", container)
	return nil
}

func (p *plugin) ValidateContainerAdjustment(_ context.Context, req *api.ValidateContainerAdjustmentRequest) error {
	dump("ValidateContainerAdjustment", "request", req)
	return nil
}

func (p *plugin) onClose() {
	log.Infof("Connection to the runtime lost, exiting...")
	os.Exit(1)
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
		pluginIdx string
		events    string
		opts      []stub.Option
		err       error
	)

	log = logrus.StandardLogger()
	log.SetFormatter(&logrus.TextFormatter{
		PadLevelText: true,
	})

	flag.StringVar(&pluginIdx, "idx", "", "plugin index to register to NRI")
	flag.StringVar(&events, "events", "all", "comma-separated list of events to subscribe for")
	flag.StringVar(&cfg.LogFile, "log-file", "", "logfile name, if logging to a file")
	flag.StringVar(&cfg.AddAnnotation, "add-annotation", "", "add this annotation to containers")
	flag.StringVar(&cfg.SetAnnotation, "set-annotation", "", "set this annotation on containers")
	flag.StringVar(&cfg.AddEnv, "add-env", "", "add this environment variable for containers")
	flag.StringVar(&cfg.SetEnv, "set-env", "", "set this environment variable for containers")
	flag.Parse()

	if cfg.LogFile != "" {
		f, err := os.OpenFile(cfg.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatalf("failed to open log file %q: %v", cfg.LogFile, err)
		}
		log.SetOutput(f)
	}

	if pluginIdx != "" {
		opts = append(opts, stub.WithPluginIdx(pluginIdx))
	}

	p := &plugin{}
	if p.mask, err = api.ParseEventMask(events); err != nil {
		log.Fatalf("failed to parse events: %v", err)
	}
	cfg.Events = strings.Split(events, ",")

	if p.stub, err = stub.New(p, append(opts, stub.WithOnClose(p.onClose))...); err != nil {
		log.Fatalf("failed to create plugin stub: %v", err)
	}

	err = p.stub.Run(context.Background())
	if err != nil {
		log.Errorf("plugin exited with error %v", err)
		os.Exit(1)
	}
}
