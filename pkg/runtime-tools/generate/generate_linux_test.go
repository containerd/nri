//go:build linux

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

package generate_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"

	rspec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"github.com/containerd/nri/pkg/api"
	xgen "github.com/containerd/nri/pkg/runtime-tools/generate"
	"github.com/containerd/nri/pkg/runtime-tools/internal/ocigen"
)

func TestAdjustMountsPropagation(t *testing.T) {
	const childEnv = "NRI_TEST_MOUNT_NAMESPACE"
	if os.Getenv(childEnv) != "1" {
		cmd := exec.Command(os.Args[0], "-test.run=^TestAdjustMountsPropagation$", "-test.v")
		cmd.Env = append(os.Environ(), childEnv+"=1")
		cmd.SysProcAttr = &syscall.SysProcAttr{Cloneflags: unix.CLONE_NEWNS}
		output, err := cmd.CombinedOutput()
		if errors.Is(err, unix.EPERM) {
			t.Skip("mount namespaces require CAP_SYS_ADMIN")
		}
		require.NoError(t, err, "%s", output)
		t.Logf("%s", output)
		return
	}

	namespace, err := os.Readlink("/proc/self/ns/mnt")
	require.NoError(t, err)
	parentNamespace, err := os.Readlink("/proc/" + strconv.Itoa(os.Getppid()) + "/ns/mnt")
	require.NoError(t, err)
	require.NotEqual(t, parentNamespace, namespace, "fixture requires a separate mount namespace")

	// Prevent fixture mounts from propagating back to the parent namespace.
	require.NoError(t, unix.Mount("", "/", "", unix.MS_PRIVATE|unix.MS_REC, ""))
	fixture := t.TempDir()
	shared := filepath.Join(fixture, "shared")
	private := filepath.Join(fixture, "private")
	for _, path := range []string{shared, private} {
		require.NoError(t, os.Mkdir(path, 0o700))
		require.NoError(t, unix.Mount(path, path, "", unix.MS_BIND, ""))
		t.Cleanup(func() { require.NoError(t, unix.Unmount(path, 0)) })
	}
	require.NoError(t, unix.Mount("", shared, "", unix.MS_SHARED, ""))

	for _, tc := range []struct {
		name  string
		mount *api.Mount
	}{
		{name: "private bind", mount: &api.Mount{Source: private, Destination: "/private", Type: "bind", Options: []string{"bind", "ro"}}},
		{name: "tmpfs", mount: &api.Mount{Source: "tmpfs", Destination: "/scratch", Type: "tmpfs", Options: []string{"nosuid", "nodev"}}},
	} {
		for _, propagation := range []string{"rshared", "rslave"} {
			t.Run(propagation+"/"+tc.name, func(t *testing.T) {
				spec := &rspec.Spec{Process: &rspec.Process{}, Linux: &rspec.Linux{}}
				generator := xgen.SpecGenerator(ocigen.New(spec))
				mounts := []*api.Mount{
					{Source: shared, Destination: "/shared", Type: "bind", Options: []string{"bind", propagation}},
					tc.mount,
				}
				require.NoError(t, generator.Adjust(&api.ContainerAdjustment{Mounts: mounts}))
				require.Len(t, spec.Mounts, 2)
				require.Equal(t, propagation, spec.Linux.RootfsPropagation)
				require.Contains(t, spec.Mounts, tc.mount.ToOCI(nil))
			})
		}
	}
}
