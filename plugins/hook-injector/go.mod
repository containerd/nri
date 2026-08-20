module github.com/containerd/nri/plugins/hook-injector

go 1.24.2

// FIXME(thaJeztah): testing the hack from https://github.com/knqyf263/go-plugin/pull/85
replace github.com/knqyf263/go-plugin => github.com/thaJeztah/go-plugin v0.0.0-20260820145858-a377c6eaa55d

require (
	github.com/containerd/nri v0.6.1
	github.com/opencontainers/runtime-spec v1.3.0
	github.com/sirupsen/logrus v1.9.4
	github.com/stretchr/testify v1.12.1
	go.podman.io/common v0.66.1
	sigs.k8s.io/yaml v1.6.0
)

require (
	github.com/containerd/log v0.1.0 // indirect
	github.com/containerd/ttrpc v1.2.7 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/knqyf263/go-plugin v0.9.0 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	github.com/tetratelabs/wazero v1.11.0 // indirect
	go.podman.io/storage v1.61.0 // indirect
	go.yaml.in/yaml/v2 v2.4.4 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/mod v0.32.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251202230838-ff82c1b0f217 // indirect
	google.golang.org/grpc v1.79.3 // indirect
	google.golang.org/protobuf v1.36.12 // indirect
)

replace github.com/containerd/nri => ../..
