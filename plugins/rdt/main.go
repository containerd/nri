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
	"strconv"
	"strings"

	"github.com/containerd/log"
	"github.com/sirupsen/logrus"

	"github.com/containerd/nri/pkg/api"
	"github.com/containerd/nri/pkg/stub"
)

type plugin struct {
	stub stub.Stub
	l    *logrus.Logger
}

func main() {
	l := logrus.StandardLogger()
	l.SetFormatter(&logrus.TextFormatter{
		PadLevelText: true,
	})

	pluginIdx := flag.String("idx", "", "plugin index to register to NRI")
	verbose := flag.Bool("verbose", false, "enable verbose logging")

	flag.Parse()
	l.WithField("verbose", *verbose).Info("Starting plugin")

	if *verbose {
		l.SetLevel(logrus.DebugLevel)
	}

	opts := []stub.Option{}
	if *pluginIdx != "" {
		opts = append(opts, stub.WithPluginIdx(*pluginIdx))
	}

	p := &plugin{l: l}
	var err error
	if p.stub, err = stub.New(p, opts...); err != nil {
		l.Fatalf("Failed to create plugin stub: %v", err)
	}

	if err := p.stub.Run(context.Background()); err != nil {
		l.Errorf("Plugin exited with error %v", err)
		os.Exit(1)
	}
}

func (p *plugin) CreateContainer(ctx context.Context, pod *api.PodSandbox, container *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
	l := logrus.NewEntry(p.l)
	if pod != nil {
		l = l.WithFields(logrus.Fields{"namespace": pod.Namespace, "pod": pod.Name})
	}
	if container != nil {
		l = l.WithField("container", container.Name)
	}
	ctx = log.WithLogger(ctx, l)
	log.G(ctx).Debug("Create container")

	adjustment := &api.ContainerAdjustment{}
	err := adjustRdt(ctx, adjustment, container.Name, pod.Annotations)
	if err != nil {
		return nil, nil, err
	}
	return adjustment, nil, nil
}

func adjustRdt(ctx context.Context, adjustment *api.ContainerAdjustment, container string, annotations map[string]string) error {
	rdtAnnotationKeySuffix := ".rdt.noderesource.dev/container." + container

	for k, v := range annotations {
		if strings.HasSuffix(k, rdtAnnotationKeySuffix) {
			fieldName := strings.TrimSuffix(k, rdtAnnotationKeySuffix)
			switch fieldName {
			case "closid":
				adjustClosID(ctx, adjustment, v)
			case "schemata":
				adjustSchemata(ctx, adjustment, v)
			case "enablemonitoring":
				if err := adjustEnableMonitoring(ctx, adjustment, v); err != nil {
					return err
				}
			default:
				log.G(ctx).WithField("field_name", fieldName).Info("Unknown rdt field")
			}
		}
	}
	return nil
}

func adjustClosID(ctx context.Context, adjustment *api.ContainerAdjustment, value string) {
	log.G(ctx).WithField("closid", value).Info("Adjust closid")
	adjustment.SetLinuxRDTClosID(value)
}

func adjustSchemata(ctx context.Context, adjustment *api.ContainerAdjustment, value string) {
	schemata := strings.Split(value, ",")
	log.G(ctx).WithField("schemata", schemata).Info("Adjust schemata")
	adjustment.SetLinuxRDTSchemata(schemata)
}

func adjustEnableMonitoring(ctx context.Context, adjustment *api.ContainerAdjustment, value string) error {
	enable, err := strconv.ParseBool(value)
	if err != nil {
		return err
	}
	log.G(ctx).WithField("enablemonitoring", enable).Info("Adjust enablemonitoring")
	adjustment.SetLinuxRDTEnableMonitoring(enable)
	return nil
}
