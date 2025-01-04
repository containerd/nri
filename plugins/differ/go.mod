module github.com/containerd/nri/plugins/differ

go 1.21

require (
	github.com/containerd/nri v0.6.1
	github.com/r3labs/diff/v3 v3.0.0
	github.com/sirupsen/logrus v1.9.3
	github.com/sters/yaml-diff v0.4.0
	sigs.k8s.io/yaml v1.3.0
)

require (
	github.com/containerd/log v0.1.0 // indirect
	github.com/containerd/ttrpc v1.2.7 // indirect
	github.com/fatih/color v1.12.0 // indirect
	github.com/goccy/go-yaml v1.8.10 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/knqyf263/go-plugin v0.8.1-0.20240827022226-114c6257e441 // indirect
	github.com/mattn/go-colorable v0.1.8 // indirect
	github.com/mattn/go-isatty v0.0.14 // indirect
	github.com/opencontainers/runtime-spec v1.1.0 // indirect
	github.com/tetratelabs/wazero v1.8.2-0.20241030035603-dc08732e57d5 // indirect
	github.com/vmihailenco/msgpack v4.0.4+incompatible // indirect
	golang.org/x/net v0.25.0 // indirect
	golang.org/x/sys v0.21.0 // indirect
	golang.org/x/text v0.15.0 // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230731190214-cbb8c96f2d6d // indirect
	google.golang.org/grpc v1.57.1 // indirect
	google.golang.org/protobuf v1.34.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	k8s.io/cri-api v0.25.3 // indirect
)

replace github.com/containerd/nri => ../..
