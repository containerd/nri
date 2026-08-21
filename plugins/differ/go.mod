module github.com/containerd/nri/plugins/differ

go 1.24.0

require (
	github.com/containerd/nri v0.12.2
	github.com/r3labs/diff/v3 v3.0.2
	github.com/sirupsen/logrus v1.9.4
	github.com/sters/yaml-diff v0.4.0
	sigs.k8s.io/yaml v1.5.0
)

require (
	github.com/containerd/log v0.1.0 // indirect
	github.com/containerd/ttrpc v1.2.7 // indirect
	github.com/goccy/go-yaml v1.13.7 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/knqyf263/go-plugin v0.9.0 // indirect
	github.com/opencontainers/runtime-spec v1.3.0 // indirect
	github.com/tetratelabs/wazero v1.11.0 // indirect
	github.com/vmihailenco/msgpack/v5 v5.4.1 // indirect
	github.com/vmihailenco/tagparser/v2 v2.0.0 // indirect
	go.yaml.in/yaml/v2 v2.4.4 // indirect
	golang.org/x/mod v0.32.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240528184218-531527333157 // indirect
	google.golang.org/grpc v1.65.1 // indirect
	google.golang.org/protobuf v1.34.1 // indirect
)

replace github.com/containerd/nri => ../..
