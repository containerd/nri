## ulimit Adjuster Plugin

This sample plugin can adjust ulimits for containers using pod annotations.

### Annotations

ulimits are annotated using the key
`ulimits.noderesource.dev/container.$CONTAINER_NAME`, which adjusts ulimits
for `$CONTAINER_NAME`. For compatibility, this plugin also accepts annotations
named `ulimits.nri.containerd.io/container.$CONTAINER_NAME`. The ulimit names
are the valid names of Linux resource limits, which can be seen on the
[`setrlimit(2)` manual page](https://linux.die.net/man/2/setrlimit).

The annotation syntax for ulimit adjustment is

```
- type: RLIMIT_NOFILE
  soft: 1024
  hard: 4096
- path: RLIMIT_MEMLOCK
  soft: 1073741824
  hard: 1073741824
  ...
```

All fields are mandatory (`soft` and `hard` will be interpreted as 0 if
missing). The `type` field accepts names in uppercase letters
("RLIMIT_NOFILE"), lowercase letters ("rlimit_memlock"), and omitting the
"RLIMIT_" prefix ("nproc").

## Testing

You can test this plugin using a kubernetes cluster/node with a container
runtime that has NRI support enabled. Start the plugin on the target node
(`ulimit-adjuster -idx 10`), create a pod with some annotated ulimits, then
verify that those get adjusted in the container. See the
[sample pod spec](sample-ulimit-adjust.yaml) for an example.