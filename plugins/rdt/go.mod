module github.com/containerd/nri/plugins/rdt

go 1.24.3

replace github.com/containerd/nri => ../..

require (
	github.com/containerd/log v0.1.0
	github.com/containerd/nri v0.6.1
	github.com/sirupsen/logrus v1.9.3
	github.com/stretchr/testify v1.8.4
)

require (
	github.com/containerd/ttrpc v1.2.7 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/knqyf263/go-plugin v0.9.0 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/opencontainers/runtime-spec v1.2.2-0.20250818071321-383cadbf08c0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/tetratelabs/wazero v1.9.0 // indirect
	golang.org/x/sys v0.35.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230731190214-cbb8c96f2d6d // indirect
	google.golang.org/grpc v1.57.1 // indirect
	google.golang.org/protobuf v1.34.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
