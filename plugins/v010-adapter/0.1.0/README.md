# Node Resource Interface v0.1.0

*This version of NRI is supported through the included v010-adapter plugin.*

Refer to the original documentation: [NRI v0.1.0](https://github.com/containerd/nri/blob/v0.9.0/README-v0.1.0.md).
If you're still using *nri v0.1.0*, We recommend refactoring to the latest NRI.

## Background

NRI v0.1.0 used an OCI hook-like one-shot plugin invocation mechanism where a separate instance of a plugin was spawned for every NRI event. This instance then used its standard input and output to receive a request and provide a response, both as JSON data.

## Other Packages

> other 0.1.0 files useful for v0.1.0 cll invoke style NRI plugins

* [v0.1.0 clearcfs example](https://github.com/containerd/nri/blob/v0.9.0/examples)
* [v0.1.0 skel example](https://github.com/containerd/nri/tree/v0.9.0/skel)
