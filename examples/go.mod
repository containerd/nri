module github.com/containerd/nri/examples

go 1.16

require (
	github.com/containerd/cgroups v1.0.1
	github.com/containerd/nri v0.1.0
	github.com/opencontainers/runtime-spec v1.0.3-0.20200929063507-e6143ca7d51d
	github.com/sirupsen/logrus v1.8.1
)

replace github.com/containerd/nri => ../
