module github.com/containerd/nri/plugins/network-logger

go 1.24.0

require (
	github.com/containerd/nri v0.6.1
	github.com/sirupsen/logrus v1.9.3
)

require (
	github.com/containerd/log v0.1.0 // indirect
	github.com/containerd/ttrpc v1.2.7 // indirect
	github.com/knqyf263/go-plugin v0.9.0 // indirect
	github.com/opencontainers/runtime-spec v1.3.0 // indirect
	github.com/tetratelabs/wazero v1.11.0 // indirect
	golang.org/x/mod v0.32.0 // indirect
	golang.org/x/sys v0.40.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250811230008-5f3141c8851a // indirect
	google.golang.org/grpc v1.66.0-dev // indirect
	google.golang.org/protobuf v1.36.7 // indirect
)

replace github.com/containerd/nri => ../..
