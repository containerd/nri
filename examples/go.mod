module github.com/containerd/nri/examples

go 1.18

require (
	github.com/containerd/cgroups v1.0.3
	github.com/containerd/nri v0.1.0
	github.com/opencontainers/runtime-spec v1.0.3-0.20220825212826-86290f6a00fb
	github.com/sirupsen/logrus v1.8.1
)

require (
	github.com/coreos/go-systemd/v22 v22.3.2 // indirect
	github.com/docker/go-units v0.4.0 // indirect
	github.com/godbus/dbus/v5 v5.0.4 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	golang.org/x/sys v0.1.0 // indirect
)

replace github.com/containerd/nri => ../
