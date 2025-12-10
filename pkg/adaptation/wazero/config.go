package wazero

import (
	"context"
	"io"
	"io/fs"

	"github.com/containerd/nri/pkg/adaptation/wazero/api"
	"github.com/containerd/nri/pkg/adaptation/wazero/sys"
)

// CompiledModule is a WebAssembly module ready to be instantiated (Runtime.InstantiateModule) as an api.Module.
//
// In WebAssembly terminology, this is a decoded, validated, and possibly also compiled module. wazero avoids using
// the name "Module" for both before and after instantiation as the name conflation has caused confusion.
// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#semantic-phases%E2%91%A0
//
// # Notes
//
//   - This is an interface for decoupling, not third-party implementations.
//     All implementations are in wazero.
//   - Closing the wazero.Runtime closes any CompiledModule it compiled.
type CompiledModule interface {
	// Name returns the module name encoded into the binary or empty if not.
	Name() string

	// ImportedFunctions returns all the imported functions
	// (api.FunctionDefinition) in this module or nil if there are none.
	//
	// Note: Unlike ExportedFunctions, there is no unique constraint on
	// imports.
	ImportedFunctions() []api.FunctionDefinition

	// ExportedFunctions returns all the exported functions
	// (api.FunctionDefinition) in this module keyed on export name.
	ExportedFunctions() map[string]api.FunctionDefinition

	// ImportedMemories returns all the imported memories
	// (api.MemoryDefinition) in this module or nil if there are none.
	//
	// ## Notes
	//   - As of WebAssembly Core Specification 2.0, there can be at most one
	//     memory.
	//   - Unlike ExportedMemories, there is no unique constraint on imports.
	ImportedMemories() []api.MemoryDefinition

	// ExportedMemories returns all the exported memories
	// (api.MemoryDefinition) in this module keyed on export name.
	//
	// Note: As of WebAssembly Core Specification 2.0, there can be at most one
	// memory.
	ExportedMemories() map[string]api.MemoryDefinition

	// CustomSections returns all the custom sections
	// (api.CustomSection) in this module keyed on the section name.
	CustomSections() []api.CustomSection

	// Close releases all the allocated resources for this CompiledModule.
	//
	// Note: It is safe to call Close while having outstanding calls from an
	// api.Module instantiated from this.
	Close(context.Context) error
}

// ModuleConfig configures resources needed by functions that have low-level interactions with the host operating
// system. Using this, resources such as STDIN can be isolated, so that the same module can be safely instantiated
// multiple times.
//
// Here's an example:
//
//	// Initialize base configuration:
//	config := wazero.NewModuleConfig().WithStdout(buf).WithSysNanotime()
//
//	// Assign different configuration on each instantiation
//	mod, _ := r.InstantiateModule(ctx, compiled, config.WithName("rotate").WithArgs("rotate", "angle=90", "dir=cw"))
//
// While wazero supports Windows as a platform, host functions using ModuleConfig follow a UNIX dialect.
// See RATIONALE.md for design background and relationship to WebAssembly System Interfaces (WASI).
//
// # Notes
//
//   - This is an interface for decoupling, not third-party implementations.
//     All implementations are in wazero.
//   - ModuleConfig is immutable. Each WithXXX function returns a new instance
//     including the corresponding change.
type ModuleConfig interface {
	// WithArgs assigns command-line arguments visible to an imported function that reads an arg vector (argv). Defaults to
	// none. Runtime.InstantiateModule errs if any arg is empty.
	//
	// These values are commonly read by the functions like "args_get" in "wasi_snapshot_preview1" although they could be
	// read by functions imported from other modules.
	//
	// Similar to os.Args and exec.Cmd Env, many implementations would expect a program name to be argv[0]. However, neither
	// WebAssembly nor WebAssembly System Interfaces (WASI) define this. Regardless, you may choose to set the first
	// argument to the same value set via WithName.
	//
	// Note: This does not default to os.Args as that violates sandboxing.
	//
	// See https://linux.die.net/man/3/argv and https://en.wikipedia.org/wiki/Null-terminated_string
	WithArgs(...string) ModuleConfig

	// WithEnv sets an environment variable visible to a Module that imports functions. Defaults to none.
	// Runtime.InstantiateModule errs if the key is empty or contains a NULL(0) or equals("") character.
	//
	// Validation is the same as os.Setenv on Linux and replaces any existing value. Unlike exec.Cmd Env, this does not
	// default to the current process environment as that would violate sandboxing. This also does not preserve order.
	//
	// Environment variables are commonly read by the functions like "environ_get" in "wasi_snapshot_preview1" although
	// they could be read by functions imported from other modules.
	//
	// While similar to process configuration, there are no assumptions that can be made about anything OS-specific. For
	// example, neither WebAssembly nor WebAssembly System Interfaces (WASI) define concerns processes have, such as
	// case-sensitivity on environment keys. For portability, define entries with case-insensitively unique keys.
	//
	// See https://linux.die.net/man/3/environ and https://en.wikipedia.org/wiki/Null-terminated_string
	WithEnv(key, value string) ModuleConfig

	// WithFS is a convenience that calls WithFSConfig with an FSConfig of the
	// input for the root ("/") guest path.
	WithFS(fs.FS) ModuleConfig

	// WithFSConfig configures the filesystem available to each guest
	// instantiated with this configuration. By default, no file access is
	// allowed, so functions like `path_open` result in unsupported errors
	// (e.g. syscall.ENOSYS).
	WithFSConfig(FSConfig) ModuleConfig

	// WithName configures the module name. Defaults to what was decoded from
	// the name section. Duplicate names are not allowed in a single Runtime.
	//
	// Calling this with the empty string "" makes the module anonymous.
	// That is useful when you want to instantiate the same CompiledModule multiple times like below:
	//
	// 	for i := 0; i < N; i++ {
	//		// Instantiate a new Wasm module from the already compiled `compiledWasm` anonymously without a name.
	//		instance, err := r.InstantiateModule(ctx, compiledWasm, wazero.NewModuleConfig().WithName(""))
	//		// ....
	//	}
	//
	// See the `concurrent-instantiation` example for a complete usage.
	//
	// Non-empty named modules are available for other modules to import by name.
	WithName(string) ModuleConfig

	// WithStartFunctions configures the functions to call after the module is
	// instantiated. Defaults to "_start".
	//
	// Clearing the default is supported, via `WithStartFunctions()`.
	//
	// # Notes
	//
	//   - If a start function doesn't exist, it is skipped. However, any that
	//     do exist are called in order.
	//   - Start functions are not intended to be called multiple times.
	//     Functions that should be called multiple times should be invoked
	//     manually via api.Module's `ExportedFunction` method.
	//   - Start functions commonly exit the module during instantiation,
	//     preventing use of any functions later. This is the case in "wasip1",
	//     which defines the default value "_start".
	//   - See /RATIONALE.md for motivation of this feature.
	WithStartFunctions(...string) ModuleConfig

	// WithStderr configures where standard error (file descriptor 2) is written. Defaults to io.Discard.
	//
	// This writer is most commonly used by the functions like "fd_write" in "wasi_snapshot_preview1" although it could
	// be used by functions imported from other modules.
	//
	// # Notes
	//
	//   - The caller is responsible to close any io.Writer they supply: It is not closed on api.Module Close.
	//   - This does not default to os.Stderr as that both violates sandboxing and prevents concurrent modules.
	//
	// See https://linux.die.net/man/3/stderr
	WithStderr(io.Writer) ModuleConfig

	// WithStdin configures where standard input (file descriptor 0) is read. Defaults to return io.EOF.
	//
	// This reader is most commonly used by the functions like "fd_read" in "wasi_snapshot_preview1" although it could
	// be used by functions imported from other modules.
	//
	// # Notes
	//
	//   - The caller is responsible to close any io.Reader they supply: It is not closed on api.Module Close.
	//   - This does not default to os.Stdin as that both violates sandboxing and prevents concurrent modules.
	//
	// See https://linux.die.net/man/3/stdin
	WithStdin(io.Reader) ModuleConfig

	// WithStdout configures where standard output (file descriptor 1) is written. Defaults to io.Discard.
	//
	// This writer is most commonly used by the functions like "fd_write" in "wasi_snapshot_preview1" although it could
	// be used by functions imported from other modules.
	//
	// # Notes
	//
	//   - The caller is responsible to close any io.Writer they supply: It is not closed on api.Module Close.
	//   - This does not default to os.Stdout as that both violates sandboxing and prevents concurrent modules.
	//
	// See https://linux.die.net/man/3/stdout
	WithStdout(io.Writer) ModuleConfig

	// WithWalltime configures the wall clock, sometimes referred to as the
	// real time clock. sys.Walltime returns the current unix/epoch time,
	// seconds since midnight UTC 1 January 1970, with a nanosecond fraction.
	// This defaults to a fake result that increases by 1ms on each reading.
	//
	// Here's an example that uses a custom clock:
	//	moduleConfig = moduleConfig.
	//		WithWalltime(func(context.Context) (sec int64, nsec int32) {
	//			return clock.walltime()
	//		}, sys.ClockResolution(time.Microsecond.Nanoseconds()))
	//
	// # Notes:
	//   - This does not default to time.Now as that violates sandboxing.
	//   - This is used to implement host functions such as WASI
	//     `clock_time_get` with the `realtime` clock ID.
	//   - Use WithSysWalltime for a usable implementation.
	WithWalltime(sys.Walltime, sys.ClockResolution) ModuleConfig

	// WithSysWalltime uses time.Now for sys.Walltime with a resolution of 1us
	// (1000ns).
	//
	// See WithWalltime
	WithSysWalltime() ModuleConfig

	// WithNanotime configures the monotonic clock, used to measure elapsed
	// time in nanoseconds. Defaults to a fake result that increases by 1ms
	// on each reading.
	//
	// Here's an example that uses a custom clock:
	//	moduleConfig = moduleConfig.
	//		WithNanotime(func(context.Context) int64 {
	//			return clock.nanotime()
	//		}, sys.ClockResolution(time.Microsecond.Nanoseconds()))
	//
	// # Notes:
	//   - This does not default to time.Since as that violates sandboxing.
	//   - This is used to implement host functions such as WASI
	//     `clock_time_get` with the `monotonic` clock ID.
	//   - Some compilers implement sleep by looping on sys.Nanotime (e.g. Go).
	//   - If you set this, you should probably set WithNanosleep also.
	//   - Use WithSysNanotime for a usable implementation.
	WithNanotime(sys.Nanotime, sys.ClockResolution) ModuleConfig

	// WithSysNanotime uses time.Now for sys.Nanotime with a resolution of 1us.
	//
	// See WithNanotime
	WithSysNanotime() ModuleConfig

	// WithNanosleep configures the how to pause the current goroutine for at
	// least the configured nanoseconds. Defaults to return immediately.
	//
	// This example uses a custom sleep function:
	//	moduleConfig = moduleConfig.
	//		WithNanosleep(func(ns int64) {
	//			rel := unix.NsecToTimespec(ns)
	//			remain := unix.Timespec{}
	//			for { // loop until no more time remaining
	//				err := unix.ClockNanosleep(unix.CLOCK_MONOTONIC, 0, &rel, &remain)
	//			--snip--
	//
	// # Notes:
	//   - This does not default to time.Sleep as that violates sandboxing.
	//   - This is used to implement host functions such as WASI `poll_oneoff`.
	//   - Some compilers implement sleep by looping on sys.Nanotime (e.g. Go).
	//   - If you set this, you should probably set WithNanotime also.
	//   - Use WithSysNanosleep for a usable implementation.
	WithNanosleep(sys.Nanosleep) ModuleConfig

	// WithOsyield yields the processor, typically to implement spin-wait
	// loops. Defaults to return immediately.
	//
	// # Notes:
	//   - This primarily supports `sched_yield` in WASI
	//   - This does not default to runtime.osyield as that violates sandboxing.
	WithOsyield(sys.Osyield) ModuleConfig

	// WithSysNanosleep uses time.Sleep for sys.Nanosleep.
	//
	// See WithNanosleep
	WithSysNanosleep() ModuleConfig

	// WithRandSource configures a source of random bytes. Defaults to return a
	// deterministic source. You might override this with crypto/rand.Reader
	//
	// This reader is most commonly used by the functions like "random_get" in
	// "wasi_snapshot_preview1", "seed" in AssemblyScript standard "env", and
	// "getRandomData" when runtime.GOOS is "js".
	//
	// Note: The caller is responsible to close any io.Reader they supply: It
	// is not closed on api.Module Close.
	WithRandSource(io.Reader) ModuleConfig
}
