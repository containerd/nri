module github.com/containerd/nri/plugins/hook-injector

go 1.24.3

require (
	github.com/containerd/nri v0.6.1
	github.com/containers/common v0.64.1
	github.com/opencontainers/runtime-spec v1.2.1
	github.com/sirupsen/logrus v1.9.3
	sigs.k8s.io/yaml v1.5.0
)

require (
	github.com/containerd/log v0.1.0 // indirect
	github.com/containerd/ttrpc v1.2.7 // indirect
	github.com/containers/storage v1.59.1 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/knqyf263/go-plugin v0.9.0 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/tetratelabs/wazero v1.9.0 // indirect
	go.yaml.in/yaml/v2 v2.4.2 // indirect
	golang.org/x/sys v0.35.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250313205543-e70fdf4c4cb4 // indirect
	google.golang.org/grpc v1.72.2 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
)

replace github.com/containerd/nri => ../..
