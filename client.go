package nri

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/oci"
	types "github.com/containerd/nri/types/v1"
	"github.com/pkg/errors"
)

const (
	// DefaultBinaryPath for nri plugins
	DefaultBinaryPath = "/opt/nri/bin"
	// DefaultConfPath for the global nri configuration
	DefaultConfPath = "/etc/nri/conf.json"
)

// New nri client
func New() (*Client, error) {
	conf, err := loadConfig(DefaultConfPath)
	if err != nil {
		return nil, err
	}
	if err := os.Setenv("PATH", fmt.Sprintf("%s:%s", os.Getenv("PATH"), DefaultBinaryPath)); err != nil {
		return nil, err
	}
	return &Client{
		conf: conf,
	}, nil
}

// Client for calling nri plugins
type Client struct {
	conf *types.ConfigList
}

// Invoke the ConfList of nri plugins
func (c *Client) Invoke(ctx context.Context, task containerd.Task, state types.State) ([]*types.Result, error) {
	spec, err := task.Spec(ctx)
	if err != nil {
		return nil, err
	}
	rs, err := createSpec(spec)
	if err != nil {
		return nil, err
	}
	var results []*types.Result
	r := &types.Request{
		Version: c.conf.Version,
		ID:      task.ID(),
		Pid:     int(task.Pid()),
		State:   state,
		Spec:    rs,
	}
	for _, p := range c.conf.Plugins {
		r.Conf = p.Conf
		result, err := c.invokePlugin(ctx, p.Type, r)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, nil
}

func createSpec(spec *oci.Spec) (*types.Spec, error) {
	s := types.Spec{
		Namespaces:  make(map[string]string),
		Annotations: spec.Annotations,
	}
	switch {
	case spec.Linux != nil:
		s.CgroupsPath = spec.Linux.CgroupsPath
		data, err := json.Marshal(spec.Linux.Resources)
		if err != nil {
			return nil, err
		}
		s.Resources = json.RawMessage(data)
		for _, ns := range spec.Linux.Namespaces {
			s.Namespaces[string(ns.Type)] = ns.Path
		}
	case spec.Windows != nil:
		data, err := json.Marshal(spec.Windows.Resources)
		if err != nil {
			return nil, err
		}
		s.Resources = json.RawMessage(data)
	}
	return &s, nil
}

func (c *Client) invokePlugin(ctx context.Context, name string, r *types.Request) (*types.Result, error) {
	payload, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, name, "invoke")
	cmd.Stdin = bytes.NewBuffer(payload)
	cmd.Stderr = os.Stderr

	out, err := cmd.Output()
	if err != nil {
		return nil, errors.Wrapf(err, "%s: %s", name, out)
	}
	var result types.Result
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func loadConfig(path string) (*types.ConfigList, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &types.ConfigList{
				Version: "0.1",
			}, nil
		}
		return nil, err
	}
	var c types.ConfigList
	err = json.NewDecoder(f).Decode(&c)
	f.Close()
	if err != nil {
		return nil, err
	}
	return &c, nil
}
