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
	"net"
	"os"
	"runtime"
	"strings"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
	"sigs.k8s.io/yaml"

	"github.com/containerd/nri/pkg/api"
	"github.com/containerd/nri/pkg/stub"
)

const (
	// Prefix of the key used for network device annotations.
	netdeviceKey    = "netdevice.noderesource.dev"
	oldNetdeviceKey = "netdevices.nri.containerd.io" // Deprecated
)

var (
	log     *logrus.Logger
	verbose bool
)

// an annotated netdevice
// https://man7.org/linux/man-pages/man7/netdevice.7.html
type netdevice struct {
	Name    string `json:"name"`     // name in the runtime namespace
	NewName string `json:"new_name"` // name inside the pod namespace
	Address string `json:"address"`
	Prefix  int    `json:"prefix"`
	MTU     int    `json:"mtu"`
}

func (n *netdevice) inject(nsPath string) error {
	// Lock the OS Thread so we don't accidentally switch namespaces
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	containerNs, err := ns.GetNS(nsPath)
	if err != nil {
		return err
	}
	defer containerNs.Close()

	hostDev, err := netlink.LinkByName(n.Name)
	if err != nil {
		return err
	}

	_, err = moveLinkIn(hostDev, containerNs, n.NewName)
	if err != nil {
		return fmt.Errorf("failed to move link %v", err)
	}
	return nil
}

// remove the network device from the Pod namespace and recover its name
// Leaves the interface in down state to avoid issues with the root network.
func (n *netdevice) release(nsPath string) error {
	// Lock the OS Thread so we don't accidentally switch namespaces
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	containerNs, err := ns.GetNS(nsPath)
	if err != nil {
		return err
	}
	defer containerNs.Close()

	err = moveLinkOut(containerNs, n.NewName)
	if err != nil {
		return err
	}

	return nil
}

// our injector plugin
type plugin struct {
	stub stub.Stub
}

func (p *plugin) RunPodSandbox(_ context.Context, pod *api.PodSandbox) error {
	log.WithField("namespace", pod.GetNamespace()).WithField("name", pod.GetName).Debug("Started pod...")
	if verbose {
		dump("RunPodSandbox", "pod", pod)
	}

	// inject associated netdevices (based on received pod annotations) into the pod
	// network namespace that will be attached to the pod's containers
	netdevices, err := parseNetdevices(pod.Annotations)
	if err != nil {
		return err
	}

	if len(netdevices) == 0 {
		return nil
	}

	// get the pod network namespace
	var ns string
	for _, namespace := range pod.Linux.GetNamespaces() {
		if namespace.Type == "network" {
			ns = namespace.Path
			break
		}
	}

	// Pods running on the host network namespace has this value empty
	if ns == "" {
		log.WithField("namespace", pod.GetNamespace()).WithField("name", pod.GetName).Info("Pod using host namespace, skipping ...")
		return fmt.Errorf("trying to inject network device on host network Pod")
	}

	// attach the network devices to the pod namespace
	for _, n := range netdevices {
		err = n.inject(ns)
		if err != nil {
			return nil
		}
	}
	return nil
}

func (p *plugin) StopPodSandbox(_ context.Context, pod *api.PodSandbox) error {
	log.WithField("namespace", pod.GetNamespace()).WithField("name", pod.GetName).Debug("Stopped pod...")
	if verbose {
		dump("StopPodSandbox", "pod", pod)
	}
	// release associated devices of the netdevice to the Pod
	netdevices, err := parseNetdevices(pod.Annotations)
	if err != nil {
		return err
	}

	if len(netdevices) == 0 {
		return nil
	}

	// get the pod network namespace
	var ns string
	for _, namespace := range pod.Linux.GetNamespaces() {
		if namespace.Type == "network" {
			ns = namespace.Path
			break
		}
	}
	// TODO check host network namespace
	if ns == "" {
		return nil
	}

	// release the network devices from the pod namespace
	for _, n := range netdevices {
		err = n.release(ns)
		if err != nil {
			return nil
		}
	}

	return nil
}

func parseNetdevices(annotations map[string]string) ([]netdevice, error) {
	var (
		key        string
		annotation []byte
		netdevices []netdevice
	)

	// look up effective device annotation and unmarshal devices
	for _, key = range []string{
		netdeviceKey + "/pod",
		oldNetdeviceKey + "/pod",
		netdeviceKey,
		oldNetdeviceKey,
	} {
		if value, ok := annotations[key]; ok {
			annotation = []byte(value)
			break
		}
	}

	if annotation == nil {
		return nil, nil
	}

	if err := yaml.Unmarshal(annotation, &netdevices); err != nil {
		return nil, fmt.Errorf("invalid device annotation %q: %w", key, err)
	}

	// validate and default
	for _, n := range netdevices {
		if n.NewName == "" {
			n.NewName = n.Name
		}
		if n.Address != "" {
			ip := net.ParseIP(n.Address)
			if ip == nil {
				return nil, fmt.Errorf("error parsing address %s", n.Address)
			}

			if n.Prefix == 0 {
				if ip.To4() == nil {
					n.Prefix = 128
				} else {
					n.Prefix = 32
				}
			}
		}

	}
	return netdevices, nil
}

// Dump one or more objects, with an optional global prefix and per-object tags.
func dump(args ...interface{}) {
	var (
		prefix string
		idx    int
	)

	if len(args) == 1 {
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
		opts      []stub.Option
		err       error
	)

	log = logrus.StandardLogger()
	log.SetFormatter(&logrus.TextFormatter{
		PadLevelText: true,
	})

	flag.StringVar(&pluginIdx, "idx", "", "plugin index to register to NRI")
	flag.BoolVar(&verbose, "verbose", false, "enable (more) verbose logging")
	flag.Parse()

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
