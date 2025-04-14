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
	// Prefix of the key used for CDI device annotations.
	cdiDeviceKey = "cdi-devices.nri.io"
	// Prefix of the key user for scheduler attribute adjustment.
	schedulerKey = "scheduling-policy.nri.io"
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

// scheduler attribute adjustment
type scheduler struct {
	Policy   string   `json:"policy"`
	Nice     int32    `json:"nice"`
	Priority int32    `json:"priority"`
	Flags    []string `json:"flags"`
	Runtime  uint64   `json:"runtime"`
	Deadline uint64   `json:"deadline"`
	Period   uint64   `json:"period"`
}

// our injector plugin
type plugin struct {
	stub stub.Stub
}

// CreateContainer handles container creation requests.
func (p *plugin) CreateContainer(_ context.Context, pod *api.PodSandbox, ctr *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
	if verbose {
		dump("CreateContainer", "pod", pod, "container", ctr)
	}

	adjust := &api.ContainerAdjustment{}

	if err := injectDevices(pod, ctr, adjust); err != nil {
		return nil, nil, err
	}

	if err := injectCDIDevices(pod, ctr, adjust); err != nil {
		return nil, nil, err
	}

	if err := injectMounts(pod, ctr, adjust); err != nil {
		return nil, nil, err
	}

	if err := adjustScheduler(pod, ctr, adjust); err != nil {
		return nil, nil, err
	}

	if verbose {
		dump(containerName(pod, ctr), "ContainerAdjustment", adjust)
	}

	return adjust, nil, nil
}

func injectDevices(pod *api.PodSandbox, ctr *api.Container, a *api.ContainerAdjustment) error {
	devices, err := parseDevices(ctr.Name, pod.Annotations)
	if err != nil {
		return err
	}

	if len(devices) == 0 {
		log.Debugf("%s: no devices annotated...", containerName(pod, ctr))
		return nil
	}

	if verbose {
		dump(containerName(pod, ctr), "annotated devices", devices)
	}

	for _, d := range devices {
		a.AddDevice(d.toNRI())
		if !verbose {
			log.Infof("%s: injected device %q...", containerName(pod, ctr), d.Path)
		}
	}

	return nil
}

func parseDevices(ctr string, annotations map[string]string) ([]device, error) {
	var (
		devices []device
	)

	annotation := getAnnotation(annotations, deviceKey, ctr)
	if annotation == nil {
		return nil, nil
	}

	if err := yaml.Unmarshal(annotation, &devices); err != nil {
		return nil, fmt.Errorf("invalid device annotation %q: %w", string(annotation), err)
	}

	return devices, nil
}

func injectCDIDevices(pod *api.PodSandbox, ctr *api.Container, a *api.ContainerAdjustment) error {
	devices, err := parseCDIDevices(ctr.Name, pod.Annotations)
	if err != nil {
		return err
	}

	if len(devices) == 0 {
		log.Debugf("%s: no CDI devices annotated...", containerName(pod, ctr))
		return nil
	}

	if verbose {
		dump(containerName(pod, ctr), "annotated CDI devices", devices)
	}

	for _, name := range devices {
		a.AddCDIDevice(
			&api.CDIDevice{
				Name: name,
			},
		)
		if !verbose {
			log.Infof("%s: injected CDI device %q...", containerName(pod, ctr), name)
		}
	}

	return nil
}

func parseCDIDevices(ctr string, annotations map[string]string) ([]string, error) {
	var (
		cdiDevices []string
	)

	annotation := getAnnotation(annotations, cdiDeviceKey, ctr)
	if annotation == nil {
		return nil, nil
	}

	if err := yaml.Unmarshal(annotation, &cdiDevices); err != nil {
		return nil, fmt.Errorf("invalid CDI device annotation %q: %w", string(annotation), err)
	}

	return cdiDevices, nil
}

func injectMounts(pod *api.PodSandbox, ctr *api.Container, a *api.ContainerAdjustment) error {
	mounts, err := parseMounts(ctr.Name, pod.Annotations)
	if err != nil {
		return err
	}

	if len(mounts) == 0 {
		log.Debugf("%s: no mounts annotated...", containerName(pod, ctr))
		return nil
	}

	if verbose {
		dump(containerName(pod, ctr), "annotated mounts", mounts)
	}

	for _, m := range mounts {
		a.AddMount(m.toNRI())
		if !verbose {
			log.Infof("%s: injected mount %q -> %q...", containerName(pod, ctr),
				m.Source, m.Destination)
		}
	}

	return nil
}

func parseMounts(ctr string, annotations map[string]string) ([]mount, error) {
	var (
		mounts []mount
	)

	annotation := getAnnotation(annotations, mountKey, ctr)
	if annotation == nil {
		return nil, nil
	}

	if err := yaml.Unmarshal(annotation, &mounts); err != nil {
		return nil, fmt.Errorf("invalid mount annotation %q: %w", string(annotation), err)
	}

	return mounts, nil
}

func adjustScheduler(pod *api.PodSandbox, ctr *api.Container, a *api.ContainerAdjustment) error {
	sch, err := parseScheduler(ctr.Name, pod.Annotations)
	if err != nil {
		return err
	}

	if sch == nil {
		log.Debugf("%s: no scheduling attributes annotated...", containerName(pod, ctr))
		return nil
	}

	if verbose {
		dump(containerName(pod, ctr), "annotated scheduling attributes", sch)
	}

	a.SetLinuxScheduler(sch.ToNRI())
	if !verbose {
		log.Infof("%s: adjusted scheduling attributes to %s...", containerName(pod, ctr), sch)
	}

	return nil
}

func parseScheduler(ctr string, annotations map[string]string) (*scheduler, error) {
	var (
		sch = &scheduler{}
	)

	annotation := getAnnotation(annotations, schedulerKey, ctr)
	if annotation == nil {
		return nil, nil
	}

	if err := yaml.Unmarshal(annotation, sch); err != nil {
		return nil, fmt.Errorf("invalid scheduler annotation %q: %w", string(annotation), err)
	}

	return sch, nil
}

func getAnnotation(annotations map[string]string, mainKey, ctr string) []byte {
	for _, key := range []string{
		mainKey + "/container." + ctr,
		mainKey + "/pod",
		mainKey,
	} {
		if value, ok := annotations[key]; ok {
			return []byte(value)
		}
	}

	return nil
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

// Convert scheduling attributes to the NRI API representation.
func (sch *scheduler) ToNRI() *api.LinuxScheduler {
	apiSch := &api.LinuxScheduler{
		Policy:   api.LinuxSchedulerPolicy(api.LinuxSchedulerPolicy_value[sch.Policy]),
		Nice:     sch.Nice,
		Priority: sch.Priority,
		Runtime:  sch.Runtime,
		Deadline: sch.Deadline,
		Period:   sch.Period,
	}

	for _, f := range sch.Flags {
		apiSch.Flags = append(apiSch.Flags, api.LinuxSchedulerFlag(api.LinuxSchedulerFlag_value[f]))
	}

	return apiSch
}

func (sc *scheduler) String() string {
	if sc == nil {
		return "<no scheduling attributes>"
	}

	s := fmt.Sprintf("<scheduler policy=%s", sc.Policy)
	if sc.Nice != 0 {
		s += fmt.Sprintf(", nice=%d", sc.Nice)
	}
	if sc.Priority != 0 {
		s += fmt.Sprintf(", priority=%d", sc.Priority)
	}
	if sc.Runtime != 0 {
		s += fmt.Sprintf(", runtime=%d, deadline=%d, period=%d", sc.Runtime, sc.Deadline, sc.Period)
	}
	if len(sc.Flags) > 0 {
		s += fmt.Sprintf(", flags=%v", sc.Flags)
	}
	s += ">"

	return s
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
