module github.com/containerd/nri/plugins/network-logger

go 1.22.0

require (
	github.com/containerd/nri v0.6.1
	github.com/sirupsen/logrus v1.9.3
)

require (
	github.com/containerd/log v0.1.0 // indirect
	github.com/containerd/ttrpc v1.2.7 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/knqyf263/go-plugin v0.8.1-0.20240827022226-114c6257e441 // indirect
	github.com/opencontainers/runtime-spec v1.1.0 // indirect
	github.com/tetratelabs/wazero v1.9.0 // indirect
	golang.org/x/net v0.25.0 // indirect
	golang.org/x/sys v0.21.0 // indirect
	golang.org/x/text v0.15.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230731190214-cbb8c96f2d6d // indirect
	google.golang.org/grpc v1.57.1 // indirect
	google.golang.org/protobuf v1.34.1 // indirect
	k8s.io/cri-api v0.25.3 // indirect
)

replace github.com/containerd/nri => ../..
