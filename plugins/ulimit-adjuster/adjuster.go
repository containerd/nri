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

	"github.com/containerd/log"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/yaml"

	"github.com/containerd/nri/pkg/api"
	"github.com/containerd/nri/pkg/stub"
)

const (
	ulimitKey    = "ulimits.nri.containerd.io"
	rlimitPrefix = "RLIMIT_"
)

var (
	valid = map[string]struct{}{
		"AS":         {},
		"CORE":       {},
		"CPU":        {},
		"DATA":       {},
		"FSIZE":      {},
		"LOCKS":      {},
		"MEMLOCK":    {},
		"MSGQUEUE":   {},
		"NICE":       {},
		"NOFILE":     {},
		"NPROC":      {},
		"RSS":        {},
		"RTPRIO":     {},
		"RTTIME":     {},
		"SIGPENDING": {},
		"STACK":      {},
	}
)

func main() {
	var (
		pluginName string
		pluginIdx  string
		verbose    bool
		opts       []stub.Option
	)

	l := logrus.StandardLogger()
	l.SetFormatter(&logrus.TextFormatter{
		PadLevelText: true,
	})

	flag.StringVar(&pluginName, "name", "", "plugin name to register to NRI")
	flag.StringVar(&pluginIdx, "idx", "", "plugin index to register to NRI")
	flag.BoolVar(&verbose, "verbose", false, "enable (more) verbose logging")
	flag.Parse()
	ctx := log.WithLogger(context.Background(), l.WithField("name", pluginName).WithField("idx", pluginIdx))
	log.G(ctx).WithField("verbose", verbose).Info("starting plugin")

	if verbose {
		l.SetLevel(logrus.DebugLevel)
	}

	if pluginName != "" {
		opts = append(opts, stub.WithPluginName(pluginName))
	}
	if pluginIdx != "" {
		opts = append(opts, stub.WithPluginIdx(pluginIdx))
	}

	p := &plugin{l: log.G(ctx)}
	var err error
	if p.stub, err = stub.New(p, opts...); err != nil {
		log.G(ctx).Fatalf("failed to create plugin stub: %v", err)
	}

	if err := p.stub.Run(context.Background()); err != nil {
		log.G(ctx).Errorf("plugin exited with error %v", err)
		os.Exit(1)
	}
}

type plugin struct {
	stub stub.Stub
	l    *logrus.Entry
}

func (p *plugin) CreateContainer(
	ctx context.Context,
	pod *api.PodSandbox,
	container *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
	if pod != nil {
		p.l = p.l.WithField("pod", pod.Name)
	}
	if container != nil {
		p.l = p.l.WithField("container", container.Name)
	}
	ctx = log.WithLogger(ctx, p.l)
	log.G(ctx).Debug("create container")

	ulimits, err := parseUlimits(ctx, container.Name, pod.Annotations)
	if err != nil {
		log.G(ctx).WithError(err).Debug("failed to parse annotations")
		return nil, nil, err
	}

	adjust, err := adjustUlimits(ctx, ulimits)
	if err != nil {
		return nil, nil, err
	}
	return adjust, nil, nil
}

type ulimit struct {
	Type string `json:"type"`
	Hard uint64 `json:"hard"`
	Soft uint64 `json:"soft"`
}

func parseUlimits(ctx context.Context, container string, annotations map[string]string) ([]ulimit, error) {
	key := ulimitKey + "/container." + container
	val, ok := annotations[key]
	if !ok {
		log.G(ctx).Debugf("no annotations found with key %q", key)
		return nil, nil
	}
	ulimits := make([]ulimit, 0)
	if err := yaml.Unmarshal([]byte(val), &ulimits); err != nil {
		return nil, err
	}
	for i := range ulimits {
		u := ulimits[i]
		typ := strings.TrimPrefix(strings.ToUpper(u.Type), rlimitPrefix)
		if _, ok := valid[typ]; !ok {
			log.G(ctx).WithField("raw", u.Type).WithField("trimmed", typ).Debug("failed to parse type")
			return nil, fmt.Errorf("failed to parse type: %q", u.Type)
		}
		ulimits[i].Type = rlimitPrefix + typ
	}
	return ulimits, nil
}

func adjustUlimits(ctx context.Context, ulimits []ulimit) (*api.ContainerAdjustment, error) {
	adjust := &api.ContainerAdjustment{}
	for _, u := range ulimits {
		l := log.G(ctx).WithField("type", u.Type).WithField("hard", u.Hard).WithField("soft", u.Soft)
		if u.Hard < u.Soft {
			l.Debug("failed to apply ulimit with hard < soft")
			return nil, fmt.Errorf("ulimit %q must have hard limit >= soft limit", u.Type)
		}
		log.G(ctx).WithField("type", u.Type).WithField("hard", u.Hard).WithField("soft", u.Soft).Debug("adjust rlimit")
		adjust.AddRlimit(u.Type, u.Hard, u.Soft)
	}
	return adjust, nil
}
