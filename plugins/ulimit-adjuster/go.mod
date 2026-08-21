module github.com/containerd/nri/plugins/ulimit-adjuster

go 1.24.0

replace github.com/containerd/nri => ../..

require (
	github.com/containerd/log v0.1.0
	github.com/containerd/nri v0.12.2
	github.com/sirupsen/logrus v1.9.4
	github.com/stretchr/testify v1.12.1
	sigs.k8s.io/yaml v1.5.0
)

require (
	github.com/containerd/ttrpc v1.2.7 // indirect
	github.com/knqyf263/go-plugin v0.9.0 // indirect
	github.com/opencontainers/runtime-spec v1.3.0 // indirect
	github.com/tetratelabs/wazero v1.11.0 // indirect
	go.yaml.in/yaml/v2 v2.4.4 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/mod v0.32.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240528184218-531527333157 // indirect
	google.golang.org/grpc v1.65.1 // indirect
	google.golang.org/protobuf v1.34.1 // indirect
)
