module github.com/containerd/nri

go 1.24.0

require (
	github.com/containerd/ttrpc v1.2.7
	github.com/google/go-cmp v0.7.0
	github.com/knqyf263/go-plugin v0.9.0
	github.com/moby/sys/mountinfo v0.7.2
	github.com/onsi/ginkgo/v2 v2.19.1
	github.com/onsi/gomega v1.34.0
	github.com/opencontainers/runtime-spec v1.3.0
	github.com/opencontainers/runtime-tools v0.9.1-0.20251114084447-edf4cb3d2116
	github.com/sirupsen/logrus v1.9.3
	github.com/stretchr/testify v1.8.4
	github.com/tetratelabs/wazero v1.11.0
	golang.org/x/sys v0.38.0
	google.golang.org/grpc v1.57.1
	google.golang.org/protobuf v1.34.1
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/containerd/log v0.1.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-task/slim-sprig/v3 v3.0.0 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/pprof v0.0.0-20240424215950-a892ee059fd6 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/moby/sys/capability v0.4.0 // indirect
	github.com/planetscale/vtprotobuf v0.4.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rogpeppe/go-internal v1.10.0 // indirect
	golang.org/x/net v0.38.0 // indirect
	golang.org/x/text v0.23.0 // indirect
	golang.org/x/tools v0.21.1-0.20240508182429-e35e4ccd0d2d // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230731190214-cbb8c96f2d6d // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
)

tool (
	github.com/containerd/ttrpc/cmd/protoc-gen-go-ttrpc
	github.com/knqyf263/go-plugin/cmd/protoc-gen-go-plugin
	google.golang.org/protobuf/cmd/protoc-gen-go
)
