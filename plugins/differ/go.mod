module github.com/containerd/nri/plugins/differ

go 1.24.0

// FIXME(thaJeztah): testing the hack from https://github.com/knqyf263/go-plugin/pull/85
replace github.com/knqyf263/go-plugin => github.com/thaJeztah/go-plugin v0.0.0-20260820145858-a377c6eaa55d

require (
	github.com/containerd/nri v0.12.2
	github.com/r3labs/diff/v3 v3.0.2
	github.com/sirupsen/logrus v1.9.4
	github.com/sters/yaml-diff v0.4.0
	sigs.k8s.io/yaml v1.5.0
)

require (
	github.com/containerd/log v0.1.0 // indirect
	github.com/containerd/ttrpc v1.2.7 // indirect
	github.com/goccy/go-yaml v1.13.7 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/knqyf263/go-plugin v0.9.0 // indirect
	github.com/opencontainers/runtime-spec v1.3.0 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	github.com/tetratelabs/wazero v1.11.0 // indirect
	github.com/vmihailenco/msgpack/v5 v5.4.1 // indirect
	github.com/vmihailenco/tagparser/v2 v2.0.0 // indirect
	go.yaml.in/yaml/v2 v2.4.4 // indirect
	golang.org/x/mod v0.32.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251202230838-ff82c1b0f217 // indirect
	google.golang.org/grpc v1.79.3 // indirect
	google.golang.org/protobuf v1.36.12 // indirect
)

replace github.com/containerd/nri => ../..
