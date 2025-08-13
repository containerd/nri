module github.com/containerd/nri/plugins/network-device-injector

go 1.24.3

require (
	github.com/containerd/nri v0.6.1
	github.com/containernetworking/plugins v1.4.1
	github.com/sirupsen/logrus v1.9.3
	github.com/vishvananda/netlink v1.2.1-beta.2
	sigs.k8s.io/yaml v1.4.0
)

require (
	github.com/containerd/log v0.1.0 // indirect
	github.com/containerd/ttrpc v1.2.7 // indirect
	github.com/knqyf263/go-plugin v0.9.0 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/opencontainers/runtime-spec v1.2.0 // indirect
	github.com/tetratelabs/wazero v1.9.0 // indirect
	github.com/vishvananda/netns v0.0.4 // indirect
	golang.org/x/sys v0.35.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240509183442-62759503f434 // indirect
	google.golang.org/grpc v1.63.2 // indirect
	google.golang.org/protobuf v1.34.1 // indirect
)

replace github.com/containerd/nri => ../..
