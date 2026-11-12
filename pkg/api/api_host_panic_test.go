//go:build !wasip1

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

package api

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/tetratelabs/wazero"
)

// A failing host function must panic with an error that carries
// context (function name and failed step), so that the wazero engine
// can recover it and report a descriptive error back to the runtime
// instead of a bare, context-free panic value.
func TestHostFunctionErrorCarriesContext(t *testing.T) {
	ctx := context.Background()

	r := wazero.NewRuntime(ctx)
	defer r.Close(ctx)

	// A guest module with a single page (64KiB) of memory: an offset
	// beyond that makes wasm.ReadMemory fail inside the generated
	// host function. Note we instantiate the real "env" module from
	// _hostFunctions.Instantiate; the guest has the memory.
	c, err := r.CompileModule(ctx, guestWasm(t))
	if err != nil {
		t.Fatal(err)
	}
	guest, err := r.InstantiateModule(ctx, c, wazero.NewModuleConfig())
	if err != nil {
		t.Fatal(err)
	}

	h := _hostFunctions{HostFunctions: failingHost{}}
	caught := func() (v any) {
		defer func() { v = recover() }()
		h._Log(ctx, guest, []uint64{0x100000, 16})
		return nil
	}()
	if caught == nil {
		t.Fatal("expected a panic from the out-of-range host memory read")
	}
	msg := fmt.Sprintf("%v", caught)
	if !strings.Contains(msg, "host function") || !strings.Contains(msg, "read memory") {
		t.Fatalf("panic value lacks host function context: %v", caught)
	}
	t.Logf("panic value: %s", msg)
}

// guestWasm returns a minimal binary WebAssembly module with one page
// of memory and a single exported no-op function.
func guestWasm(t *testing.T) []byte {
	t.Helper()
	// (module (memory 1) (func (export "nop")))
	return []byte{
		0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00, // magic + version
		0x01, 0x04, 0x01, 0x60, 0x00, 0x00, // type section: ()->()
		0x03, 0x02, 0x01, 0x00, // function section: 1 func, type 0
		0x05, 0x03, 0x01, 0x00, 0x01, // memory section: 1 page min
		0x07, 0x07, 0x01, 0x03, 'n', 'o', 'p', 0x00, 0x00, // export "nop"
		0x0a, 0x04, 0x01, 0x02, 0x00, 0x0b, // code section: empty body
	}
}

type failingHost struct{}

func (failingHost) Log(context.Context, *LogRequest) (*Empty, error) {
	return nil, nil
}
