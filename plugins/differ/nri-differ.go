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
	"container/list"
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/r3labs/diff/v3"
	"github.com/sirupsen/logrus"
	"github.com/sters/yaml-diff/yamldiff"
	"sigs.k8s.io/yaml"

	"github.com/containerd/nri/pkg/api"
	"github.com/containerd/nri/pkg/stub"
)

type config struct {
	Indices      string `json:"indices"`
	LogFile      string `json:"logFile"`
	VerboseLevel int    `json:"verboseLevel"`
	Yaml         bool   `json:"yaml"`
}

type pluginIndex struct {
	prevIndex  int
	nextIndex  int
	prevValues *list.List // Contains changedValue list from previous index
}

type changedValue struct {
	podSet       bool
	pod          api.PodSandbox
	containerSet bool
	container    api.Container
}

type plugin struct {
	stub stub.Stub
	mask stub.EventMask
	name string
	idx  int
}

var (
	cfg     config
	log     *logrus.Logger
	indices map[int]pluginIndex
)

func (p *plugin) Configure(_ context.Context, nriCfg string) (stub.EventMask, error) {
	log.Infof("got configuration data: %q", nriCfg)
	if nriCfg == "" {
		return p.mask, nil
	}

	oldCfg := cfg
	err := yaml.Unmarshal([]byte(nriCfg), &cfg)
	if err != nil {
		return 0, fmt.Errorf("failed to parse provided configuration: %w", err)
	}

	p.mask, err = api.ParseEventMask("all")
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

func setValue(newValue *changedValue, pod *api.PodSandbox, container *api.Container) {
	if pod != nil {
		newValue.podSet = true
		newValue.pod = *pod
	}

	if container != nil {
		newValue.containerSet = true
		newValue.container = *container
	}
}

func (p *plugin) saveValue(pod *api.PodSandbox, container *api.Container) {
	newValue := &changedValue{}
	setValue(newValue, pod, container)

	indices[p.idx].prevValues.PushBack(newValue)
}

func (p *plugin) differ(apifunc string, pod *api.PodSandbox, container *api.Container) {
	// If we are the first plugin, then no need to diff
	if indices[p.idx].prevIndex < 0 {
		if cfg.VerboseLevel > 0 {
			if container != nil {
				p.dump(apifunc, "pod", pod, "container", container)
			} else {
				p.dump(apifunc, "pod", pod)
			}
		}

		p.saveValue(pod, container)
	} else {
		element := indices[indices[p.idx].prevIndex].prevValues.Front()
		if element == nil {
			// This might happen during startup when all indices of the plugin
			// are not yet started (they are startedasynchronously in
			// parallel).
			log.Debugf("[%d] %s: skipping nil element, plugin startup not yet finished", p.idx, apifunc)
			return
		}
		initialValue := element.Value.(*changedValue)

		indices[indices[p.idx].prevIndex].prevValues.Remove(element)

		if pod != nil {
			if initialValue.podSet {
				podChanged := *pod

				if cfg.Yaml {
					p.printYamlDiff(apifunc, "pod", initialValue.pod, podChanged)
				} else {
					changelog, err := diff.Diff(initialValue.pod, podChanged)
					if err != nil {
						log.Errorf("%v", err)
						return
					}

					p.printDiff(apifunc, &changelog, "pod", initialValue.pod, podChanged)
				}
			}
		}

		if container != nil {
			if initialValue.containerSet {
				containerChanged := *container

				if cfg.Yaml {
					p.printYamlDiff(apifunc, "container", initialValue.container, containerChanged)
				} else {
					changelog, err := diff.Diff(initialValue.container, containerChanged)
					if err != nil {
						log.Errorf("%v", err)
						return
					}

					p.printDiff(apifunc, &changelog, "container", initialValue.container, containerChanged)
				}
			}
		}

		// Push to next index so it can diff things too
		if indices[p.idx].nextIndex > 0 {
			p.saveValue(pod, container)
		}
	}
}

func (p *plugin) Synchronize(_ context.Context, pods []*api.PodSandbox, containers []*api.Container) ([]*api.ContainerUpdate, error) {
	if cfg.VerboseLevel > 2 {
		p.dump("Synchronize", "pods", pods, "containers", containers)
	}

	return nil, nil
}

func (p *plugin) Shutdown(_ context.Context) {
	p.dump("Shutdown")
}

func (p *plugin) RunPodSandbox(_ context.Context, pod *api.PodSandbox) error {
	p.differ("RunPodSandbox", pod, nil)
	return nil
}

func (p *plugin) StopPodSandbox(_ context.Context, pod *api.PodSandbox) error {
	p.differ("StopPodSandbox", pod, nil)
	return nil
}

func (p *plugin) RemovePodSandbox(_ context.Context, pod *api.PodSandbox) error {
	p.differ("RemovePodSandbox", pod, nil)
	return nil
}

func (p *plugin) CreateContainer(_ context.Context, pod *api.PodSandbox, container *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
	p.differ("CreateContainer", pod, container)

	adjust := &api.ContainerAdjustment{}

	return adjust, nil, nil
}

func (p *plugin) PostCreateContainer(_ context.Context, pod *api.PodSandbox, container *api.Container) error {
	p.differ("PostCreateContainer", pod, container)
	return nil
}

func (p *plugin) StartContainer(_ context.Context, pod *api.PodSandbox, container *api.Container) error {
	p.differ("StartContainer", pod, container)
	return nil
}

func (p *plugin) PostStartContainer(_ context.Context, pod *api.PodSandbox, container *api.Container) error {
	p.differ("PostStartContainer", pod, container)
	return nil
}

func (p *plugin) UpdateContainer(_ context.Context, pod *api.PodSandbox, container *api.Container, r *api.LinuxResources) ([]*api.ContainerUpdate, error) {
	p.differ("UpdateContainer", pod, container)

	return nil, nil
}

func (p *plugin) PostUpdateContainer(_ context.Context, pod *api.PodSandbox, container *api.Container) error {
	p.differ("PostUpdateContainer", pod, container)
	return nil
}

func (p *plugin) StopContainer(_ context.Context, pod *api.PodSandbox, container *api.Container) ([]*api.ContainerUpdate, error) {
	p.differ("StopContainer", pod, container)

	return nil, nil
}

func (p *plugin) RemoveContainer(_ context.Context, pod *api.PodSandbox, container *api.Container) error {
	p.differ("RemoveContainer", pod, container)
	return nil
}

func (p *plugin) onClose() {
	log.Infof("Connection to the runtime lost, exiting...")
	os.Exit(1)
}

// Dump one or more objects, with an optional global prefix and per-object tags.
func (p *plugin) dump(args ...interface{}) {
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
			log.Infof("%s %s: %s:", p.name, prefix, tag)
			for _, line := range strings.Split(strings.TrimSpace(string(msg)), "\n") {
				log.Infof("%s %s:    %s", p.name, prefix, line)
			}
		} else {
			log.Infof("%s %s:", p.name, tag)
			for _, line := range strings.Split(strings.TrimSpace(string(msg)), "\n") {
				log.Infof("%s  %s", p.name, line)
			}
		}
	}
}

func (p *plugin) printDiff(apifunc string, changelog *diff.Changelog, obj string, origValue interface{}, changedValue interface{}) {
	if cfg.VerboseLevel > 1 {
		log.Infof("[%d] Original values for %s", p.idx, obj)
		p.dump(apifunc, obj, origValue)
	}

	if len(*changelog) == 0 {
		log.Infof("[%d] %s: %s: %s", p.idx, apifunc, obj, "<no changes>")
		return
	}

	for _, item := range *changelog {
		log.Infof("[%d] %s: %s: %s: %v: From: %v -> To: %v", p.idx, apifunc, obj, item.Type, item.Path, item.From, item.To)
	}

	if cfg.VerboseLevel > 1 {
		log.Infof("[%d] Values after changes for %s", p.idx, obj)
		p.dump(apifunc, obj, changedValue)
	}
}

func (p *plugin) printYamlDiff(apifunc string, obj string, origValue interface{}, changedValue interface{}) {
	msg, err := yaml.Marshal(origValue)
	if err != nil {
		return
	}
	yamlOrig := yamldiff.Load(string(msg[:]))

	msg, err = yaml.Marshal(changedValue)
	if err != nil {
		return
	}
	yamlChanged := yamldiff.Load(string(msg[:]))

	for _, diff := range yamldiff.Do(yamlOrig, yamlChanged) {
		if diff.Status == yamldiff.DiffStatusSame {
			log.Infof("[%d] %s: %s: %s", p.idx, apifunc, obj, "<no changes>")
			break
		}

		log.Infof("[%d] %s: %s: %v", p.idx, apifunc, obj, diff.Diff)
	}
}

func startPlugin(wg *sync.WaitGroup, pluginIdx int) {
	var (
		opts []stub.Option
		err  error
	)

	defer wg.Done()

	idxStr := fmt.Sprintf("%02d", pluginIdx)

	if idxStr != "" {
		opts = append(opts, stub.WithPluginIdx(idxStr))
	}

	p := &plugin{}
	if p.mask, err = api.ParseEventMask("all"); err != nil {
		log.Fatalf("Failed to parse events: %v", err)
	}

	p.name = fmt.Sprintf("[%s]", idxStr)
	p.idx = pluginIdx

	if p.stub, err = stub.New(p, append(opts, stub.WithOnClose(p.onClose))...); err != nil {
		log.Fatalf("Failed to create plugin stub: %v", err)
	}

	err = p.stub.Run(context.Background())
	if err != nil {
		log.Errorf("Plugin exited with error %v", err)
		os.Exit(1)
	}
}

func main() {
	log = logrus.StandardLogger()
	log.SetFormatter(&logrus.TextFormatter{
		PadLevelText: true,
	})

	flag.StringVar(&cfg.LogFile, "log-file", "", "logfile name, if logging to a file")
	flag.IntVar(&cfg.VerboseLevel, "verbose-level", 0,
		"Print extra information,\n"+
			"level 0 (default) prints only the changes done by plugins,\n"+
			"level 1 prints original data for the first invocation of this plugin,\n"+
			"level 2 prints original and changed data together with the difference,\n"+
			"level 3 prints all the data received (prints lot of data).")
	flag.StringVar(&cfg.Indices, "indices", "0,99",
		"Comma separated list of indices where to install the differ plugin to monitor the changes.\n"+
			"Example: \"-indices 45,50,80\" will print the changes generated by plugins in\n"+
			"indices 45, 50 and 80. Note that this plugin will install itself to index 0 and 99\n"+
			"if this parameter is not given.")
	flag.BoolVar(&cfg.Yaml, "yaml", false, "Print the diff in yaml")
	flag.Parse()

	if cfg.LogFile != "" {
		f, err := os.OpenFile(cfg.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatalf("failed to open log file %q: %v", cfg.LogFile, err)
		}
		log.SetOutput(f)
	}

	wg := new(sync.WaitGroup)

	indexCount := strings.Count(cfg.Indices, ",")
	if indexCount == 0 {
		log.Fatalf("There must be at least two index given.")
		return
	}

	indices = make(map[int]pluginIndex)
	prevIndex := -1

	for _, idxStr := range strings.Split(cfg.Indices, ",") {
		idx, _ := strconv.Atoi(idxStr)

		entry := indices[idx]
		entry.prevIndex = prevIndex
		entry.prevValues = list.New()
		indices[idx] = entry

		if prevIndex >= 0 {
			if prevEntry, ok := indices[prevIndex]; ok {
				prevEntry.nextIndex = idx
				indices[prevIndex] = prevEntry
			}
		}

		prevIndex = idx

		wg.Add(1)

		go startPlugin(wg, idx)
	}

	entry := indices[prevIndex]
	entry.nextIndex = -1
	indices[prevIndex] = entry

	wg.Wait()
}
