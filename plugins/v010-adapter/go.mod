module github.com/containerd/nri/plugins/v010-adapter

go 1.21

require (
	github.com/containerd/containerd v1.6.36
	github.com/containerd/nri v0.6.1
	github.com/opencontainers/runtime-spec v1.1.0
	github.com/sirupsen/logrus v1.9.3
)

require (
	github.com/Microsoft/go-winio v0.5.3 // indirect
	github.com/Microsoft/hcsshim v0.9.12 // indirect
	github.com/containerd/cgroups v1.0.4 // indirect
	github.com/containerd/continuity v0.3.0 // indirect
	github.com/containerd/errdefs v0.1.0 // indirect
	github.com/containerd/fifo v1.0.0 // indirect
	github.com/containerd/log v0.1.0 // indirect
	github.com/containerd/ttrpc v1.2.7 // indirect
	github.com/containerd/typeurl v1.0.2 // indirect
	github.com/docker/go-events v0.0.0-20190806004212-e31b211e4f1c // indirect
	github.com/gogo/googleapis v1.4.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/uuid v1.3.1 // indirect
	github.com/klauspost/compress v1.15.9 // indirect
	github.com/knqyf263/go-plugin v0.8.1-0.20240827022226-114c6257e441 // indirect
	github.com/moby/locker v1.0.1 // indirect
	github.com/moby/sys/mountinfo v0.6.2 // indirect
	github.com/moby/sys/signal v0.6.0 // indirect
	github.com/moby/sys/user v0.1.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.0 // indirect
	github.com/opencontainers/selinux v1.10.2 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/tetratelabs/wazero v1.8.2-0.20241030035603-dc08732e57d5 // indirect
	go.opencensus.io v0.24.0 // indirect
	golang.org/x/net v0.25.0 // indirect
	golang.org/x/sync v0.7.0 // indirect
	golang.org/x/sys v0.21.0 // indirect
	golang.org/x/text v0.15.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20231002182017-d307bd883b97 // indirect
	google.golang.org/grpc v1.59.0 // indirect
	google.golang.org/protobuf v1.34.1 // indirect
	k8s.io/cri-api v0.25.3 // indirect
)

replace github.com/containerd/nri => ../..
