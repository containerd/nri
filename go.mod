module github.com/containerd/nri

go 1.24.0

require (
	github.com/brianvoe/gofakeit/v7 v7.12.1
	github.com/containerd/ttrpc v1.2.7
	github.com/google/go-cmp v0.7.0
	github.com/knqyf263/go-plugin v0.9.0
	github.com/moby/sys/mountinfo v0.7.2
	github.com/onsi/ginkgo/v2 v2.28.1
	github.com/onsi/gomega v1.39.1
	github.com/opencontainers/runtime-spec v1.3.0
	github.com/opencontainers/runtime-tools v0.9.1-0.20251114084447-edf4cb3d2116
	github.com/sirupsen/logrus v1.9.3
	github.com/stretchr/testify v1.11.1
	github.com/tetratelabs/wazero v1.11.0
	golang.org/x/sys v0.40.0
	google.golang.org/grpc v1.75.0
	google.golang.org/protobuf v1.36.7
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/Masterminds/semver/v3 v3.4.0 // indirect
	github.com/containerd/log v0.1.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-task/slim-sprig/v3 v3.0.0 // indirect
	github.com/google/pprof v0.0.0-20260115054156-294ebfa9ad83 // indirect
	github.com/moby/sys/capability v0.4.0 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/mod v0.32.0 // indirect
	golang.org/x/net v0.49.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/text v0.33.0 // indirect
	golang.org/x/tools v0.41.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250811230008-5f3141c8851a // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
)

tool (
	github.com/containerd/ttrpc/cmd/protoc-gen-go-ttrpc
	github.com/knqyf263/go-plugin/cmd/protoc-gen-go-plugin
	google.golang.org/protobuf/cmd/protoc-gen-go
)
