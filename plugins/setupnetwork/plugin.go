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
	"encoding/json"
	"flag"
	"fmt"
	"github.com/containerd/go-cni"
	"github.com/containerd/nri/pkg/api"
	"github.com/containerd/nri/pkg/stub"
	"github.com/sirupsen/logrus"
	"os"
	"sigs.k8s.io/yaml"
)

type config struct {
	CfgParam1 string `json:"cfgParam1"`
}

type plugin struct {
	stub stub.Stub
	mask stub.EventMask
}

var (
	cfg config
	log *logrus.Logger
)

func (p *plugin) Configure(_ context.Context, config, runtime, version string) (stub.EventMask, error) {
	log.Infof("Connected to %s/%s...", runtime, version)

	if config == "" {
		return 0, nil
	}

	err := yaml.Unmarshal([]byte(config), &cfg)
	if err != nil {
		return 0, fmt.Errorf("failed to parse configuration: %w", err)
	}

	return 0, nil
}

func (p *plugin) RunPodSandbox(_ context.Context, pod *api.PodSandbox) error {
	log.Infof("Started pod %s/%s...", pod.GetNamespace(), pod.GetName())
	return nil
}

func (p *plugin) StopPodSandbox(_ context.Context, pod *api.PodSandbox) error {
	log.Infof("Stopped pod %s/%s...", pod.GetNamespace(), pod.GetName())
	return nil
}

func (p *plugin) RemovePodSandbox(_ context.Context, pod *api.PodSandbox) error {
	log.Infof("Removed pod %s/%s...", pod.GetNamespace(), pod.GetName())
	return nil
}

func (p *plugin) NetworkConfigurationChanged(_ context.Context, cniconfigs []*api.CNIConfig) ([]*api.CNIConfig, error) {
	log.Infof("NetworkConfigurationChanged...")
	for _, config := range cniconfigs {
		log.Infof("CNI config for '%s': %v",
			config.Name, config.NetworkConf)
	}
	return cniconfigs, nil
}

func (p *plugin) PreSetupNetwork(_ context.Context, pod *api.PodSandbox, cniconfigs []*api.CNIConfig) ([]*api.CNICapabilities, error) {
	var err error
	caps := make(map[string][]byte)
	cnicaps := []*api.CNICapabilities{}

	log.Infof("PreSetupNetwork for '%s/%s'...", pod.GetNamespace(), pod.GetName())
	log.Infof("PreSetupNetwork for '%s/%s' received CNI configs '%v'...", pod.GetNamespace(), pod.GetName(), cniconfigs)

	bandwidth := cni.BandWidth{
		IngressRate:  450000,
		IngressBurst: 1000000,
		EgressRate:   600000,
		EgressBurst:  800000,
	}

	if caps["bandwidth"], err = json.Marshal(bandwidth); err != nil {
		log.Infof("Could not marshal struct %e", err)
		return nil, nil
	}

	for _, config := range cniconfigs {
		if config.Name != "cni-loopback" {
			cnicaps = append(cnicaps, &api.CNICapabilities{
				Name:         config.Name,
				Capabilities: caps,
			})
		}
	}

	log.Infof("Returning CNI capabilities '%v'", cnicaps)
	return cnicaps, nil
}

func (p *plugin) PostSetupNetwork(_ context.Context, pod *api.PodSandbox, result []*api.Result) ([]*api.Result, error) {
	var prevResult *api.Result

	log.Infof("PostSetupNetwork for '%s/%s'...", pod.GetNamespace(), pod.GetName())

	for _, prevResult = range result {
		log.Infof("CNI result for '%s' CNI version '%s': %v", prevResult.Name, prevResult.CniVersion, prevResult)
	}

	return result, nil
}

func (p *plugin) PreNetworkDeleted(_ context.Context, pod *api.PodSandbox) error {
	log.Infof("PreNetworkDeleted for %s/%s...", pod.GetNamespace(), pod.GetName())

	return nil
}

func (p *plugin) PostNetworkDeleted(_ context.Context, pod *api.PodSandbox) error {
	log.Infof("PostNetworkDeleted for %s/%s...", pod.GetNamespace(), pod.GetName())

	return nil
}

func (p *plugin) onClose() {
	log.Infof("Connection to the runtime lost, exiting...")
	os.Exit(0)
}

func main() {
	var (
		pluginName string
		pluginIdx  string
		err        error
	)

	log = logrus.StandardLogger()
	log.SetFormatter(&logrus.TextFormatter{
		PadLevelText: true,
	})

	flag.StringVar(&pluginName, "name", "", "plugin name to register to NRI")
	flag.StringVar(&pluginIdx, "idx", "", "plugin index to register to NRI")
	flag.Parse()

	p := &plugin{}
	opts := []stub.Option{
		stub.WithOnClose(p.onClose),
	}
	if pluginName != "" {
		opts = append(opts, stub.WithPluginName(pluginName))
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
