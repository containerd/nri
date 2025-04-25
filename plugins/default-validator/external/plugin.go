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

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"

	"github.com/containerd/nri/pkg/stub"
	validator "github.com/containerd/nri/plugins/default-validator"
)

type plugin struct {
	*validator.DefaultValidator
	stub stub.Stub
}

var (
	log *logrus.Logger
)

func (p *plugin) Configure(_ context.Context, config, _, _ string) (stub.EventMask, error) {
	if config == "" {
		log.Infof("No configuration provided, using defaults...")
		return 0, nil
	}

	cfg := &validator.DefaultValidatorConfig{}
	err := yaml.Unmarshal([]byte(config), cfg)
	if err != nil {
		return 0, fmt.Errorf("failed to parse configuration: %w", err)
	}

	if !cfg.Enable {
		log.Infof("Validation disabled, exiting....")
		os.Exit(0)
	}

	log.Infof("Using configuration %+v...", cfg)
	p.SetConfig(cfg)

	return 0, nil
}

func (p *plugin) onClose() {
	log.Infof("Connection to the runtime lost, exiting...")
	os.Exit(0)
}

func main() {
	var (
		pluginName string
		pluginIdx  string
		verbose    bool
		err        error
	)

	log = logrus.StandardLogger()
	log.SetFormatter(&logrus.TextFormatter{
		PadLevelText: true,
	})

	flag.StringVar(&pluginName, "name", "", "plugin name to register to NRI")
	flag.StringVar(&pluginIdx, "idx", "", "plugin index to register to NRI")
	flag.BoolVar(&verbose, "verbose", false, "use verbose (debug) logging")
	flag.Parse()

	if verbose {
		log.SetLevel(logrus.DebugLevel)
	}

	p := &plugin{
		DefaultValidator: validator.NewDefaultValidator(
			&validator.DefaultValidatorConfig{
				Enable: true,
			},
		),
	}

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
