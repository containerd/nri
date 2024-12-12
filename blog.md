# The Node Resource Interface says _"hi"_ to WebAssembly

The [Node Resource Interface (NRI)][nri] allows users to write plugins for [Open
Container Initiative (OCI)][oci] compatible runtimes like [CRI-O][crio] and
[containerd][containerd]. These plugins are capable of making controlled changes
to containers at dedicated points in their life cycle. For example, by using the
NRI it is possible to allocate extra node resources on container creation, which
can be released again after the container got removed.

[nri]: https://github.com/containerd/nri
[oci]: https://opencontainers.org
[crio]: https://cri-o.io
[containerd]: https://containerd.io

A plugin is written as daemon-like process which serves [a predefined][api]
API based on [ttRPC][ttrpc] ([gRPC][grpc] for low-memory environments). This
means in detail, that the NRI implementation in the runtime (CRI-O, containerd)
will communicate using a UNIX Domain Socket (UDS) with each plugin and provide
them with all required event data. For example, events can be container or pod
sandbox _creation_, _stopping_ or _removal_, while corresponding data are the
_name_, _namespace_ or corresponding _annotations_.

[api]: https://github.com/containerd/nri/blob/eaf78a9/pkg/api/api.proto
[ttrpc]: https://github.com/containerd/ttrpc
[grpc]: https://grpc.io

On one hand plugins written as daemons have the benefit of persisting the
current state out of the box, while on the other hand they come with a
performance and management overhead. For that reason, the NRI also supports
[OCI hook][hook]-like [binary plugins][v010] which get executed for each event.
Combining the concept of small binary plugins with a universal standard like
[WebAssembly (Wasm)][wasm] empowers the NRI to run on the edge and universally
on all imaginable platforms.

[hook]: https://github.com/opencontainers/runtime-spec/blob/9ceba9f/config.md#posix-platform-hooks
[v010]: https://github.com/containerd/nri/tree/693d64e/plugins/v010-adapter
[wasm]: https://webassembly.org

## How it works

The required change for the NRI landed with [Pull Request
containerd/nri#121][pr121]. This change adds a [go-plugin][go-plugin] mechanism
to the NRI. Each plugin gets compiled to Wasm, which means that it is
size-efficient, memory-safe, automatically sandboxed and highly portable out of
the box! The plugin system works in the same way as the NRI by using [Protocol
Buffers][protobuf]. This means, that the NRI can reuse the existing API for
ttRPC, while the communication will happen in memory and not over the Remote
Procedure Call (RPC).

[pr121]: https://github.com/containerd/nri/pull/121
[go-plugin]: https://github.com/knqyf263/go-plugin
[protobuf]: https://protobuf.dev
[tinygo]: https://tinygo.org

One key benefit is that WebAssembly is designed as a portable compilation target
for programming languages. Plugins compiled to Wasm can be used anywhere, which
means that there is no requirement for multi architecture binaries. Beside that,
the Wasm stack machine is designed to be encoded in a size and time efficient
binary format, which make them great targets for binary execution.

## Demo

Unfortunately, the native golang (`go`) compiler does not have full WebAssembly
support yet, which means the plugins have to be compiled using the alternative
[tinygo][tinygo] compiler. An [example Wasm plugin][example] within the NRI
repository, can be compiled locally using:

```bash
make $(pwd)/build/bin/wasm
```

Or within a container image:

```bash
make $(pwd)/build/bin/wasm TINYGO_DOCKER=1
```

In the future it may be possible to cross compile plugins using `GOOS=wasip1
GOARCH=wasm go build`, but that is not implemented yet (see
[knqyf263/go-plugin#58][go-plugin-58]).

[example]: https://github.com/containerd/nri/blob/dd57194/plugins/wasm/plugin.go
[go-plugin-58]: https://github.com/knqyf263/go-plugin/issues/58

The resulting file should be a valid WebAssembly binary:

```bash
file build/bin/wasm
```

```text
build/bin/wasm: WebAssembly (wasm) binary module version 0x1 (MVP)
```

To try out the binary, we have to put it into the default local NRI directory.
We also need to prefix the binary by a chosen index, which later refers to the
plugin execution order:

```bash
sudo mkdir -p /opt/nri/plugins
sudo cp build/bin/wasm /opt/nri/plugins/10-wasm
```

[CRI-O][crio-gh] v1.32 (which has been not released yet as time of writing) or
it's recent [`main`][crio-gh-main] branch can be used to verify that the plugin
got loaded successfully:

[crio-gh]: https://github.com/cri-o/cri-o
[crio-gh-main]: https://github.com/cri-o/cri-o/commits/main

```bash
sudo ./bin/crio
```

```text
…
INFO[…] Create NRI interface
INFO[…] runtime interface created
INFO[…] Registered domain "k8s.io" with NRI
INFO[…] runtime interface starting up...
INFO[…] starting plugins...
INFO[…] discovered plugin 10-wasm
INFO[…] starting pre-installed NRI plugin "wasm"...
INFO[…] Found WASM plugin: /opt/nri/plugins/10-wasm
INFO[…] WASM: Got configure request
INFO[…] Synchronizing NRI (plugin) with current runtime state
INFO[…] synchronizing plugin 10-wasm
INFO[…] WASM: Got synchronize request
INFO[…] pre-installed NRI plugin "10-wasm" synchronization success
INFO[…] plugin invocation order
INFO[…]   #1: "10-wasm" (external:10-wasm[0])
…
```

The partial logs above outline that the `10-wasm` plugin got loaded and the
WebAssembly plugin received a `configure` and `synchronize` request. Log lines
prefixed with `WASM:` are directly invoked [from the plugin itself][wasm-log]:

[wasm-log]: https://github.com/containerd/nri/blob/d138684/plugins/wasm/plugin.go#L39

```go
func (p *plugin) Configure(ctx context.Context, req *api.ConfigureRequest) (*api.ConfigureResponse, error) {
	log(ctx, "Got configure request")
	return nil, nil
}
```

The logging itself is achieved by a so-called _host function_. This function can
be used to pass data back to the host (the NRI) and process them there (log to
`stderr`). The plugin just has to fulfill the host [`log`
function][wasm-log-plugin]:

[wasm-log-plugin]: https://github.com/containerd/nri/blob/d138684/plugins/wasm/plugin.go#L31-L36

```go
func log(ctx context.Context, msg string) {
	api.NewHostFunctions().Log(ctx, &api.LogRequest{
		Msg:   "WASM: " + msg,
		Level: api.LogRequest_LEVEL_INFO,
	})
}
```

And the NRI can fulfill the [logging functionality][wasm-log-nri]:

[wasm-log-nri]: https://github.com/containerd/nri/blob/d138684/pkg/adaptation/plugin.go#L699-L715

```go
func (wasmHostFunctions) Log(ctx context.Context, request *api.LogRequest) (*api.Empty, error) {
	switch request.GetLevel() {
	case api.LogRequest_LEVEL_INFO:
		log.Infof(ctx, request.GetMsg())
	case api.LogRequest_LEVEL_WARN:
		log.Warnf(ctx, request.GetMsg())
	case api.LogRequest_LEVEL_ERROR:
		log.Errorf(ctx, request.GetMsg())
	default:
		log.Debugf(ctx, request.GetMsg())
	}

	return &api.Empty{}, nil
}
```

If the plugin is loaded into memory and CRI-O now creates [an example
sandbox][crio-sb], then the WebAssembly instance will get executed accordingly
by invoking the correct entry point:

[crio-sb]: https://github.com/cri-o/cri-o/blob/e83973d/test/testdata/sandbox_config.json

```bash
sudo crictl runp test/testdata/sandbox_config.json
```

```text
…
INFO[…] Running pod sandbox: test.crio/podsandbox1/POD  id=…
…
INFO[…] WASM: Got state change request with event: RUN_POD_SANDBOX
INFO[…] WASM: Got run pod sandbox request
…
INFO[…] Ran pod sandbox … with infra container: test.crio/podsandbox1/POD  id=…
…
```

WebAssembly NRI plugins allow to distribute functionality independently from the
target platform in a secure and performant way. That makes them awesome for edge
scenarios or for being distributed as OCI artifacts. For the future, it is
imaginable to have a (semi) automatic reload functionality for the loaded
in-memory plugins, but that is something we are currently elaborating.

Thank you for reading this blog post! If you have any questions or comments
feel free to open an issue in the [NRI repository][nri-issue].

[nri-issue]: https://github.com/containerd/nri/issues/new
