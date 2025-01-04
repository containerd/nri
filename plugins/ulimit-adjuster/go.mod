module github.com/containerd/nri/plugins/ulimit-adjuster

go 1.21

replace github.com/containerd/nri => ../..

require (
	github.com/containerd/log v0.1.0
	github.com/containerd/nri v0.6.1
	github.com/sirupsen/logrus v1.9.3
	github.com/stretchr/testify v1.8.4
	sigs.k8s.io/yaml v1.3.0
)

require (
	github.com/containerd/ttrpc v1.2.7 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/knqyf263/go-plugin v0.8.1-0.20240827022226-114c6257e441 // indirect
	github.com/opencontainers/runtime-spec v1.0.3-0.20220825212826-86290f6a00fb // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/tetratelabs/wazero v1.8.2-0.20241030035603-dc08732e57d5 // indirect
	golang.org/x/net v0.25.0 // indirect
	golang.org/x/sys v0.21.0 // indirect
	golang.org/x/text v0.15.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230731190214-cbb8c96f2d6d // indirect
	google.golang.org/grpc v1.57.1 // indirect
	google.golang.org/protobuf v1.34.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/cri-api v0.25.3 // indirect
)
