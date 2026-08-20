module github.com/containerd/nri

go 1.24.0

// FIXME(thaJeztah): testing the hack from https://github.com/knqyf263/go-plugin/pull/85
replace github.com/knqyf263/go-plugin => github.com/thaJeztah/go-plugin v0.0.0-20260820145858-a377c6eaa55d

require (
	github.com/brianvoe/gofakeit/v7 v7.12.1
	github.com/containerd/ttrpc v1.2.7
	github.com/google/go-cmp v0.7.0
	github.com/knqyf263/go-plugin v0.9.0
	github.com/moby/sys/mountinfo v0.7.2
	github.com/opencontainers/runtime-spec v1.3.0
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10
	github.com/sirupsen/logrus v1.9.4
	github.com/stretchr/testify v1.12.1
	github.com/tetratelabs/wazero v1.11.0
	go.yaml.in/yaml/v3 v3.0.5
	golang.org/x/mod v0.32.0
	golang.org/x/sys v0.39.0
	google.golang.org/grpc v1.79.3
	google.golang.org/protobuf v1.36.12
)

require (
	github.com/containerd/log v0.1.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251202230838-ff82c1b0f217 // indirect
)

tool (
	github.com/containerd/ttrpc/cmd/protoc-gen-go-ttrpc
	github.com/knqyf263/go-plugin/cmd/protoc-gen-go-plugin
	google.golang.org/protobuf/cmd/protoc-gen-go
)
