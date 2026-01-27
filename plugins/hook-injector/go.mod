module github.com/containerd/nri/plugins/hook-injector

go 1.24.2

require (
	github.com/containerd/nri v0.6.1
	github.com/opencontainers/runtime-spec v1.3.0
	github.com/sirupsen/logrus v1.9.3
	github.com/stretchr/testify v1.11.1
	go.podman.io/common v0.66.1
	sigs.k8s.io/yaml v1.6.0
)

require (
	github.com/containerd/log v0.1.0 // indirect
	github.com/containerd/ttrpc v1.2.7 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/knqyf263/go-plugin v0.9.0 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/tetratelabs/wazero v1.11.0 // indirect
	go.podman.io/storage v1.61.0 // indirect
	go.yaml.in/yaml/v2 v2.4.2 // indirect
	golang.org/x/sys v0.38.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250414145226-207652e42e2e // indirect
	google.golang.org/grpc v1.72.2 // indirect
	google.golang.org/protobuf v1.36.9 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/containerd/nri => ../..
