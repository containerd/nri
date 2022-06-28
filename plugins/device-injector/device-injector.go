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

const (
	// Prefix of the key used for device annotations.
	deviceKey = "devices.nri.io"
	// Prefix of the key used for mount annotations.
	mountKey = "mounts.nri.io"
)

var (
	log     *logrus.Logger
	verbose bool
)

// an annotated device
type device struct {
	Path     string `json:"path"`
	Type     string `json:"type"`
	Major    int64  `json:"major"`
	Minor    int64  `json:"minor"`
	FileMode uint32 `json:"file_mode"`
	UID      uint32 `json:"uid"`
	GID      uint32 `json:"gid"`
}

// an annotated mount
type mount struct {
	Source      string   `json:"source"`
	Destination string   `json:"destination"`
	Type        string   `json:"type"`
	Options     []string `json:"options"`
}

// our injector plugin
type plugin struct {
	stub stub.Stub
}

// CreateContainer handles container creation requests.
func (p *plugin) CreateContainer(pod *api.PodSandbox, container *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
	var (
		ctrName string
		devices []device
		mounts  []mount
		err     error
	)

	ctrName = containerName(pod, container)

	if verbose {
		dump("CreateContainer", "pod", pod, "container", container)
	}

	adjust := &api.ContainerAdjustment{}

	// inject devices to container
	devices, err = parseDevices(container.Name, pod.Annotations)
	if err != nil {
		return nil, nil, err
	}

	if len(devices) == 0 {
		log.Infof("%s: no devices annotated...", ctrName)
	} else {
		if verbose {
			dump(ctrName, "annotated devices", devices)
		}

		for _, d := range devices {
			adjust.AddDevice(d.toNRI())
			if !verbose {
				log.Infof("%s: injected device %q...", ctrName, d.Path)
			}
		}
	}

	// inject mounts to container
	mounts, err = parseMounts(container.Name, pod.Annotations)
	if err != nil {
		return nil, nil, err
	}

	if len(mounts) == 0 {
		log.Infof("%s: no mounts annotated...", ctrName)
	} else {
		if verbose {
			dump(ctrName, "annotated mounts", mounts)
		}

		for _, m := range mounts {
			adjust.AddMount(m.toNRI())
			if !verbose {
				log.Infof("%s: injected mount %q -> %q...", ctrName, m.Source, m.Destination)
			}
		}
	}

	if verbose {
		dump(ctrName, "ContainerAdjustment", adjust)
	}

	return adjust, nil, nil
}

func parseDevices(ctr string, annotations map[string]string) ([]device, error) {
	var (
		key        string
		annotation []byte
		devices    []device
	)

	// look up effective device annotation and unmarshal devices
	for _, key = range []string{
		deviceKey + "/container." + ctr,
		deviceKey + "/pod",
		deviceKey,
	} {
		if value, ok := annotations[key]; ok {
			annotation = []byte(value)
			break
		}
	}

	if annotation == nil {
		return nil, nil
	}

	if err := yaml.Unmarshal(annotation, &devices); err != nil {
		return nil, fmt.Errorf("invalid device annotation %q: %w", key, err)
	}

	return devices, nil
}

func parseMounts(ctr string, annotations map[string]string) ([]mount, error) {
	var (
		key        string
		annotation []byte
		mounts     []mount
	)

	// look up effective device annotation and unmarshal devices
	for _, key = range []string{
		mountKey + "/container." + ctr,
		mountKey + "/pod",
		mountKey,
	} {
		if value, ok := annotations[key]; ok {
			annotation = []byte(value)
			break
		}
	}

	if annotation == nil {
		return nil, nil
	}

	if err := yaml.Unmarshal(annotation, &mounts); err != nil {
		return nil, fmt.Errorf("invalid mount annotation %q: %w", key, err)
	}

	return mounts, nil
}

// Convert a device to the NRI API representation.
func (d *device) toNRI() *api.LinuxDevice {
	apiDev := &api.LinuxDevice{
		Path:  d.Path,
		Type:  d.Type,
		Major: d.Major,
		Minor: d.Minor,
	}
	if d.FileMode != 0 {
		apiDev.FileMode = api.FileMode(d.FileMode)
	}
	if d.UID != 0 {
		apiDev.Uid = api.UInt32(d.UID)
	}
	if d.GID != 0 {
		apiDev.Gid = api.UInt32(d.GID)
	}
	return apiDev
}

// Convert a device to the NRI API representation.
func (m *mount) toNRI() *api.Mount {
	apiMnt := &api.Mount{
		Source:      m.Source,
		Destination: m.Destination,
		Type:        m.Type,
		Options:     m.Options,
	}
	return apiMnt
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
		pluginName string
		pluginIdx  string
		opts       []stub.Option
		err        error
	)

	log = logrus.StandardLogger()
	log.SetFormatter(&logrus.TextFormatter{
		PadLevelText: true,
	})

	flag.StringVar(&pluginName, "name", "", "plugin name to register to NRI")
	flag.StringVar(&pluginIdx, "idx", "", "plugin index to register to NRI")
	flag.BoolVar(&verbose, "verbose", false, "enable (more) verbose logging")
	flag.Parse()

	if pluginName != "" {
		opts = append(opts, stub.WithPluginName(pluginName))
	}
	if pluginIdx != "" {
		opts = append(opts, stub.WithPluginIdx(pluginIdx))
	}

	p := &plugin{}
	if p.stub, err = stub.New(p, opts...); err != nil {
		log.Fatalf("failed to create plugin stub: %v", err)
	}

	err = p.stub.Run(context.Background())
	if err != nil {
		log.Errorf("plugin exited with error %v", err)
		os.Exit(1)
	}
}
