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
	"os"
	"path/filepath"
	"testing"

	"github.com/containerd/nri/pkg/api"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"go.podman.io/common/pkg/hooks"
)

func TestHookInjector(t *testing.T) {
	log = logrus.StandardLogger()
	log.SetFormatter(&logrus.TextFormatter{PadLevelText: true})

	t.Run("no hooks configured", func(t *testing.T) {
		testCreateContainerWithoutHooks(t)
	})

	t.Run("hooks injected correctly", func(t *testing.T) {
		testCreateContainerWithHooks(t)
	})
}

// testCreateContainerWithoutHooks validates that a container without hooks configured gets ignored
func testCreateContainerWithoutHooks(t *testing.T) {
	t.Helper()

	tempDir := t.TempDir()

	mgr, err := hooks.New(context.Background(), []string{tempDir}, []string{})
	assert.NoError(t, err)

	p := &plugin{mgr: mgr}
	pod, container := createTestPodAndContainer()

	adjust, updates, err := p.CreateContainer(context.Background(), pod, container)

	assert.NoError(t, err)
	assert.Nil(t, adjust)
	assert.Nil(t, updates)
}

// testCreateContainerWithHooks validates that OCI hooks are correctly injected
// into the container spec during creation when they are configured
func testCreateContainerWithHooks(t *testing.T) {
	t.Helper()

	tempDir := t.TempDir()

	hookJSON := []byte(`{
		"version": "1.0.0",
		"hook": {
			"path": "/bin/echo",
			"args": ["echo", "testing from hook"]
		},
		"when": {
			"always": true
		},
		"stages": ["createRuntime"]
	}`)

	hookPath := filepath.Join(tempDir, "test-hook.json")
	err := os.WriteFile(hookPath, hookJSON, 0644)
	assert.NoError(t, err)

	mgr, err := hooks.New(context.Background(), []string{tempDir}, []string{})
	assert.NoError(t, err)

	p := &plugin{mgr: mgr}
	pod, container := createTestPodAndContainer()

	adjust, updates, err := p.CreateContainer(context.Background(), pod, container)

	assert.NoError(t, err)
	assert.NotNil(t, adjust)
	assert.Nil(t, updates)

	hooks := adjust.Hooks
	assert.NotNil(t, hooks.Hooks())
	assert.NotEmpty(t, hooks.CreateRuntime, "expected createRuntime hooks to be injected")

	found := false
	for _, h := range hooks.CreateRuntime {
		if h.Path == "/bin/echo" && len(h.Args) > 0 && h.Args[0] == "echo" {
			found = true
			break
		}
	}
	assert.True(t, found, "couldn't find injected hook, or it was incorrect")
}

func createTestPodAndContainer() (*api.PodSandbox, *api.Container) {
	pod := &api.PodSandbox{
		Name:        "test-pod-hook-injector",
		Annotations: map[string]string{},
	}
	container := &api.Container{
		Name:        "test-container-hook-injector",
		Annotations: map[string]string{},
		Args:        []string{"run.sh"},
	}
	return pod, container
}
