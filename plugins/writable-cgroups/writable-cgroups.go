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
	"slices"
	"strings"

	"github.com/moby/sys/mountinfo"
	"github.com/sirupsen/logrus"

	"github.com/containerd/nri/pkg/api"
	"github.com/containerd/nri/pkg/stub"
)

const (
	// WritableCgroupsAnnotation is the annotation key that enables writable cgroups.
	WritableCgroupsAnnotation = "cgroups.noderesource.dev/writable"
	// CgroupsMount is the path to the cgroups mount.
	CgroupsMount = "/sys/fs/cgroup"
)

var (
	log     *logrus.Logger
	verbose bool
)

type plugin struct {
	stub           stub.Stub
	hostNsDelegate bool
}

func main() {
	var (
		pluginIdx     string
		socketPath    string
		hostMountFile string
		opts          []stub.Option
		err           error
	)

	log = logrus.StandardLogger()
	log.SetFormatter(&logrus.TextFormatter{
		PadLevelText: true,
	})

	flag.StringVar(&pluginIdx, "idx", "", "plugin index to register to NRI")
	flag.StringVar(&socketPath, "socket-path", "", "path of the NRI socket file")
	flag.StringVar(&hostMountFile, "host-mount-file", "/host/proc/1/mountinfo", "path to the host mountinfo file to check for nsdelegate")
	flag.BoolVar(&verbose, "verbose", false, "enable (more) verbose logging")
	flag.Parse()

	if pluginIdx != "" {
		opts = append(opts, stub.WithPluginIdx(pluginIdx))
	}

	if socketPath != "" {
		opts = append(opts, stub.WithSocketPath(socketPath))
	}

	if verbose {
		log.SetLevel(logrus.DebugLevel)
	}

	nsDelegate, err := checkHostNsDelegate(hostMountFile)
	if err != nil {
		log.WithError(err).Warnf("Failed to check for nsdelegate option on host cgroup mount, disabling writable cgroups")
		nsDelegate = false
	} else if !nsDelegate {
		log.Warn("Host cgroup mount does not have 'nsdelegate' option, disabling writable cgroups")
	} else {
		log.Info("Host cgroup mount has 'nsdelegate' option, enabling writable cgroups support")
	}

	p := &plugin{hostNsDelegate: nsDelegate}
	if p.stub, err = stub.New(p, opts...); err != nil {
		log.Fatalf("failed to create plugin stub: %v", err)
	}

	if err = p.stub.Run(context.Background()); err != nil {
		log.Errorf("plugin exited with error %v", err)
		os.Exit(1)
	}
}

func (p *plugin) CreateContainer(ctx context.Context, pod *api.PodSandbox, container *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
	if pod == nil || container == nil {
		return nil, nil, nil
	}

	l := log.WithFields(logrus.Fields{
		"container": container.Name,
		"pod":       pod.Name,
		"namespace": pod.Namespace,
	})

	l.Debug("Started CreateContainer")

	if !isWritableCgroupsEnabled(pod, container, l) {
		return nil, nil, nil
	}

	if !p.hostNsDelegate {
		l.Warn("cgroup mount on host does not have 'nsdelegate' option; ignoring request to make cgroups writable")
		return nil, nil, nil
	}

	oldMount := findCgroupsMount(container.Mounts)
	if oldMount == nil {
		err := fmt.Errorf("no %s mount found to modify, this is unexpected", CgroupsMount)
		l.WithError(err).Error("failed to find cgroup mount")
		return nil, nil, err
	}

	newMount, changed := createReadWriteMount(oldMount)
	if !changed {
		l.Debug("cgroup mount is already read-write")
		return nil, nil, nil
	}

	adjust := &api.ContainerAdjustment{}
	adjust.AddMount(newMount)
	adjust.RemoveMount(oldMount.Destination)

	l.Info("Successfully adjusted mounts for writable cgroups")
	return adjust, nil, nil
}

// checkHostNsDelegate checks if the cgroup2 mount on the host has the 'nsdelegate' option.
func checkHostNsDelegate(mountFilePath string) (bool, error) {
	f, err := os.Open(mountFilePath)
	if err != nil {
		return false, fmt.Errorf("failed to open mount file %q: %w", mountFilePath, err)
	}
	defer f.Close()

	filter := func(info *mountinfo.Info) (skip, stop bool) {
		if info.FSType != "cgroup2" {
			return true, false
		}
		if info.Mountpoint == "/sys/fs/cgroup" || info.Mountpoint == "/sys/fs/cgroup/unified" {
			return false, true
		}
		return true, false
	}

	mounts, err := mountinfo.GetMountsFromReader(f, filter)
	if err != nil {
		return false, fmt.Errorf("error scanning mount file: %w", err)
	}

	if len(mounts) == 0 {
		return false, nil
	}

	return slices.Contains(strings.Split(mounts[0].VFSOptions, ","), "nsdelegate"), nil
}

// isWritableCgroupsEnabled checks if the pod has the writable cgroups annotation.
func isWritableCgroupsEnabled(pod *api.PodSandbox, container *api.Container, l *logrus.Entry) bool {
	perContainerAnnotation := fmt.Sprintf("%s.container.%s", WritableCgroupsAnnotation, container.Name)
	if val, ok := pod.Annotations[perContainerAnnotation]; ok {
		l.WithFields(logrus.Fields{
			"annotation": perContainerAnnotation,
			"value":      val,
		}).Debug("Found per-container annotation")
		return val == "true"
	}

	if val, ok := pod.Annotations[WritableCgroupsAnnotation]; ok {
		l.WithFields(logrus.Fields{
			"annotation": WritableCgroupsAnnotation,
			"value":      val,
		}).Debug("Found pod-level annotation")
		return val == "true"
	}

	l.WithField("annotation", WritableCgroupsAnnotation).Debug("Pod-level annotation not found")
	return false
}

// findCgroupsMount finds the cgroups mount in the list of container mounts.
func findCgroupsMount(mounts []*api.Mount) *api.Mount {
	for _, m := range mounts {
		if m.Destination == CgroupsMount {
			return m
		}
	}
	return nil
}

// createReadWriteMount creates a new mount with read-write permissions.
// It returns the new mount and a boolean indicating if the mount was changed.
func createReadWriteMount(oldMount *api.Mount) (*api.Mount, bool) {
	newOptions := make([]string, 0, len(oldMount.Options))
	hasRW := false
	changed := false

	for _, opt := range oldMount.Options {
		switch opt {
		case "rw":
			hasRW = true
			newOptions = append(newOptions, opt)
		case "ro":
			changed = true
			// Skip "ro".
		default:
			newOptions = append(newOptions, opt)
		}
	}

	if !hasRW {
		newOptions = append(newOptions, "rw")
		changed = true
	}

	if !changed {
		return oldMount, false
	}

	return &api.Mount{
		Destination: oldMount.Destination,
		Type:        oldMount.Type,
		Source:      oldMount.Source,
		Options:     newOptions,
	}, true
}
