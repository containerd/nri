module github.com/containerd/nri/examples

go 1.24.3

require (
	github.com/containerd/cgroups/v3 v3.1.3
	github.com/containerd/log v0.1.0
	github.com/containerd/nri v0.12.2
	github.com/opencontainers/runtime-spec v1.3.0
)

require (
	github.com/cilium/ebpf v0.16.0 // indirect
	github.com/containerd/ttrpc v1.2.7 // indirect
	github.com/coreos/go-systemd/v22 v22.5.0 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/godbus/dbus/v5 v5.1.0 // indirect
	github.com/knqyf263/go-plugin v0.9.0 // indirect
	github.com/moby/sys/userns v0.1.0 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	github.com/sirupsen/logrus v1.10.1 // indirect
	github.com/tetratelabs/wazero v1.11.0 // indirect
	golang.org/x/exp v0.0.0-20241108190413-2d47ceb2692f // indirect
	golang.org/x/mod v0.32.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251202230838-ff82c1b0f217 // indirect
	google.golang.org/grpc v1.79.3 // indirect
	google.golang.org/protobuf v1.36.12 // indirect
)

replace github.com/containerd/nri => ..
