//go:build unix

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

package adaptation

import (
	"bufio"
	"context"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/containerd/nri/pkg/api"
)

// startShellPlugin launches a shell script as a plugin process and waits for
// the script to report that it is ready, ie. that its signal traps are set up.
func startShellPlugin(t *testing.T, script string) *plugin {
	t.Helper()

	cmd := exec.Command("sh", "-c", script)
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("failed to create stdout pipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start plugin process: %v", err)
	}
	if _, err := bufio.NewReader(out).ReadString('\n'); err != nil {
		cmd.Process.Kill()
		t.Fatalf("plugin process failed to get ready: %v", err)
	}

	return &plugin{
		idx:  "00",
		base: "test",
		cmd:  cmd,
		impl: &pluginType{},
	}
}

func TestStopPluginSignals(t *testing.T) {
	const loop = `while :; do sleep 0.05; done`

	for _, tc := range []struct {
		name       string
		script     string
		stopSignal syscall.Signal
		timeout    time.Duration
		killed     bool
	}{
		{
			name:    "exits on default SIGTERM",
			script:  `trap 'exit 0' TERM; echo ready; ` + loop,
			timeout: 10 * time.Second,
		},
		{
			name:    "ignores SIGTERM, is killed after the timeout",
			script:  `trap '' TERM; echo ready; ` + loop,
			timeout: 300 * time.Millisecond,
			killed:  true,
		},
		{
			name:       "exits on configured stop signal",
			script:     `trap '' TERM; trap 'exit 0' USR1; echo ready; ` + loop,
			stopSignal: syscall.SIGUSR1,
			timeout:    10 * time.Second,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			SetPluginStopTimeout(tc.timeout)
			t.Cleanup(func() { SetPluginStopTimeout(DefaultPluginStopTimeout) })

			p := startShellPlugin(t, tc.script)
			p.stopSignal = tc.stopSignal

			pid := p.cmd.Process.Pid
			start := time.Now()
			p.stop()
			elapsed := time.Since(start)

			if tc.killed {
				if elapsed < tc.timeout {
					t.Errorf("plugin was stopped after %v, before the %v stop timeout", elapsed, tc.timeout)
				}
			} else if elapsed >= tc.timeout {
				t.Errorf("plugin did not exit gracefully, stop took %v", elapsed)
			}

			// The process must be gone by now.
			if err := syscall.Kill(pid, 0); err == nil {
				t.Errorf("plugin process %d is still running", pid)
			}
		})
	}
}

func TestStopPluginsInParallel(t *testing.T) {
	const (
		count   = 4
		timeout = 500 * time.Millisecond
	)

	SetPluginStopTimeout(timeout)
	t.Cleanup(func() { SetPluginStopTimeout(DefaultPluginStopTimeout) })

	var plugins []*plugin
	for i := 0; i < count; i++ {
		plugins = append(plugins, startShellPlugin(t,
			`trap '' TERM; echo ready; while :; do sleep 0.05; done`))
	}

	start := time.Now()
	stopPlugins(plugins)
	elapsed := time.Since(start)

	if elapsed < timeout || elapsed >= 2*timeout {
		t.Errorf("stopping %d unresponsive plugins took %v, expected about %v", count, elapsed, timeout)
	}
}

func TestStopSignalConfigure(t *testing.T) {
	for _, tc := range []struct {
		name    string
		signal  int32
		want    syscall.Signal
		wantErr bool
	}{
		{name: "unset", signal: 0, want: 0},
		{name: "valid", signal: int32(syscall.SIGUSR1), want: syscall.SIGUSR1},
		{name: "negative", signal: -1, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &plugin{
				r: &Adaptation{},
				impl: &pluginType{ttrpcImpl: &configurePlugin{
					rpl: &ConfigureResponse{StopSignal: tc.signal},
				}},
			}
			err := p.configure(context.Background(), "test", "0", "")
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if p.stopSignal != tc.want {
				t.Errorf("stop signal %v, expected %v", p.stopSignal, tc.want)
			}
		})
	}
}

// configurePlugin is a plugin service which replies to Configure with a canned response.
type configurePlugin struct {
	api.PluginService
	rpl *ConfigureResponse
}

func (c *configurePlugin) Configure(context.Context, *ConfigureRequest) (*ConfigureResponse, error) {
	return c.rpl, nil
}
