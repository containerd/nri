module github.com/containerd/nri/plugins/device-injector

go 1.18

require (
	github.com/containerd/nri v0.2.0
	github.com/sirupsen/logrus v1.9.3
	sigs.k8s.io/yaml v1.3.0
)

require (
	github.com/containerd/ttrpc v1.2.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/opencontainers/runtime-spec v1.1.0 // indirect
	golang.org/x/net v0.14.0 // indirect
	golang.org/x/sys v0.11.0 // indirect
	golang.org/x/text v0.12.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230822172742-b8732ec3820d // indirect
	google.golang.org/grpc v1.57.0 // indirect
	google.golang.org/protobuf v1.31.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	k8s.io/cri-api v0.27.1 // indirect
)

replace github.com/containerd/nri => ../..
