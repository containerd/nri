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

package skel

import (
	"context"
	"os"
	"testing"

	v1 "github.com/containerd/nri/types/v1"
)

type dummyPlugin struct{}

func (dummyPlugin) Type() string { return "dummy" }

func (dummyPlugin) Invoke(_ context.Context, _ *v1.Request) (*v1.Result, error) { //nolint:staticcheck
	return &v1.Result{}, nil //nolint:staticcheck
}

// Run() must not panic when the binary is started without any arguments.
func TestRunWithoutArguments(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"plugin-binary"}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Run() panicked: %v", r)
		}
	}()

	if err := Run(context.Background(), dummyPlugin{}); err == nil {
		t.Fatal("expected an error when no command argument is given")
	}
}
