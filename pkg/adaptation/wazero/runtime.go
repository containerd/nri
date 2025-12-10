package wazero

import (
	"context"

	"github.com/containerd/nri/pkg/adaptation/wazero/api"
)

// Runtime allows embedding of WebAssembly modules.
//
// The below is an example of basic initialization:
//
//	ctx := context.Background()
//	r := wazero.NewRuntime(ctx)
//	defer r.Close(ctx) // This closes everything this Runtime created.
//
//	mod, _ := r.Instantiate(ctx, wasm)
//
// # Notes
//
//   - This is an interface for decoupling, not third-party implementations.
//     All implementations are in wazero.
//   - Closing this closes any CompiledModule or Module it instantiated.
type Runtime interface {
	// Instantiate instantiates a module from the WebAssembly binary (%.wasm)
	// with default configuration, which notably calls the "_start" function,
	// if it exists.
	//
	// Here's an example:
	//	ctx := context.Background()
	//	r := wazero.NewRuntime(ctx)
	//	defer r.Close(ctx) // This closes everything this Runtime created.
	//
	//	mod, _ := r.Instantiate(ctx, wasm)
	//
	// # Notes
	//
	//   - See notes on InstantiateModule for error scenarios.
	//   - See InstantiateWithConfig for configuration overrides.
	Instantiate(ctx context.Context, source []byte) (api.Module, error)

	// InstantiateWithConfig instantiates a module from the WebAssembly binary
	// (%.wasm) or errs for reasons including exit or validation.
	//
	// Here's an example:
	//	ctx := context.Background()
	//	r := wazero.NewRuntime(ctx)
	//	defer r.Close(ctx) // This closes everything this Runtime created.
	//
	//	mod, _ := r.InstantiateWithConfig(ctx, wasm,
	//		wazero.NewModuleConfig().WithName("rotate"))
	//
	// # Notes
	//
	//   - See notes on InstantiateModule for error scenarios.
	//   - If you aren't overriding defaults, use Instantiate.
	//   - This is a convenience utility that chains CompileModule with
	//     InstantiateModule. To instantiate the same source multiple times,
	//     use CompileModule as InstantiateModule avoids redundant decoding
	//     and/or compilation.
	InstantiateWithConfig(ctx context.Context, source []byte, config ModuleConfig) (api.Module, error)

	// NewHostModuleBuilder lets you create modules out of functions defined in Go.
	//
	// Below defines and instantiates a module named "env" with one function:
	//
	//	ctx := context.Background()
	//	hello := func() {
	//		fmt.Fprintln(stdout, "hello!")
	//	}
	//	_, err := r.NewHostModuleBuilder("env").
	//		NewFunctionBuilder().WithFunc(hello).Export("hello").
	//		Instantiate(ctx, r)
	//
	// Note: empty `moduleName` is not allowed.
	NewHostModuleBuilder(moduleName string) HostModuleBuilder

	// CompileModule decodes the WebAssembly binary (%.wasm) or errs if invalid.
	// Any pre-compilation done after decoding wasm is dependent on RuntimeConfig.
	//
	// There are two main reasons to use CompileModule instead of Instantiate:
	//   - Improve performance when the same module is instantiated multiple times under different names
	//   - Reduce the amount of errors that can occur during InstantiateModule.
	//
	// # Notes
	//
	//   - The resulting module name defaults to what was binary from the custom name section.
	//   - Any pre-compilation done after decoding the source is dependent on RuntimeConfig.
	//
	// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#name-section%E2%91%A0
	CompileModule(ctx context.Context, binary []byte) (CompiledModule, error)

	// InstantiateModule instantiates the module or errs for reasons including
	// exit or validation.
	//
	// Here's an example:
	//	mod, _ := n.InstantiateModule(ctx, compiled, wazero.NewModuleConfig().
	//		WithName("prod"))
	//
	// # Errors
	//
	// While CompiledModule is pre-validated, there are a few situations which
	// can cause an error:
	//   - The module name is already in use.
	//   - The module has a table element initializer that resolves to an index
	//     outside the Table minimum size.
	//   - The module has a start function, and it failed to execute.
	//   - The module was compiled to WASI and exited with a non-zero exit
	//     code, you'll receive a sys.ExitError.
	//   - RuntimeConfig.WithCloseOnContextDone was enabled and a context
	//     cancellation or deadline triggered before a start function returned.
	InstantiateModule(ctx context.Context, compiled CompiledModule, config ModuleConfig) (api.Module, error)

	// CloseWithExitCode closes all the modules that have been initialized in this Runtime with the provided exit code.
	// An error is returned if any module returns an error when closed.
	//
	// Here's an example:
	//	ctx := context.Background()
	//	r := wazero.NewRuntime(ctx)
	//	defer r.CloseWithExitCode(ctx, 2) // This closes everything this Runtime created.
	//
	//	// Everything below here can be closed, but will anyway due to above.
	//	_, _ = wasi_snapshot_preview1.InstantiateSnapshotPreview1(ctx, r)
	//	mod, _ := r.Instantiate(ctx, wasm)
	CloseWithExitCode(ctx context.Context, exitCode uint32) error

	// Module returns an instantiated module in this runtime or nil if there aren't any.
	Module(moduleName string) api.Module

	// Closer closes all compiled code by delegating to CloseWithExitCode with an exit code of zero.
	api.Closer
}
