module github.com/containerd/nri

go 1.24.0

require (
	github.com/brianvoe/gofakeit/v7 v7.12.1
	github.com/containerd/ttrpc v1.2.7
	github.com/google/go-cmp v0.7.0
	github.com/knqyf263/go-plugin v0.9.0
	github.com/moby/sys/mountinfo v0.7.2
	github.com/opencontainers/runtime-spec v1.3.0
	github.com/sirupsen/logrus v1.9.4
	github.com/stretchr/testify v1.12.1
	github.com/tetratelabs/wazero v1.11.0
	go.yaml.in/yaml/v3 v3.0.5
	golang.org/x/mod v0.32.0
	golang.org/x/sys v0.39.0
	google.golang.org/grpc v1.57.1
	google.golang.org/protobuf v1.36.12
)

require (
	github.com/containerd/log v0.1.0 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/planetscale/vtprotobuf v0.4.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230731190214-cbb8c96f2d6d // indirect
)

tool (
	github.com/containerd/ttrpc/cmd/protoc-gen-go-ttrpc
	github.com/knqyf263/go-plugin/cmd/protoc-gen-go-plugin
	google.golang.org/protobuf/cmd/protoc-gen-go
)
