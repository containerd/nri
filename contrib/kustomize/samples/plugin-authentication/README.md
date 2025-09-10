# Containerized Plugins With Authentication

This example uses the template plugin to demonstrate a kustomize overlay
to enable authentication for our containerized sample plugins.

> [!NOTE] In addition to the kustomize overlay here, you will need a runtime
> with support for NRI plugin authentication to test this.

## Generate And Store Plugin Keys in a Secret

```bash
$ mkdir tmp
$ wget https://raw.githubusercontent.com/klihub/nri/refs/heads/hacking/plugin-authentication/examples/keygen/keygen.go
$ go run ./keygen.go > plugin-key
$ private=$(head -2 plugin-key | tail -1)
$ public=$(tail -1 plugin-key)
$ kubectl -n kube-system create secret generic test-auth \
      --from-literal=private="$(echo private)" \--from-literal=public="$(echo public)"
```

> [!NOTE] Now you need to update your runtime's NRI configuration for authentication
> with these generated keys.

## Customized Deployment

```bash
$ kubectl apply -k https://github.com/containerd/nri/contrib/kustomize/samples/plugin-authentication
```

If you check the plugin's logs, you should see it getting authenticated:

```bash
$ kubectl -n kube-system logs daemonset/nri-plugin-template
time="2025-09-10T17:12:12Z" level=info msg="Created plugin 10-template (plugin, handles RunPodSandbox,StopPodSandbox,RemovePodSandbox,CreateContainer,PostCreateContainer,StartContainer,PostStartContainer,UpdateContainer,PostUpdateContainer,StopContainer,RemoveContainer)"
time="2025-09-10T17:12:12Z" level=info msg="Authenticated with role test1 (tags: map[role:test1])..."
time="2025-09-10T17:12:12Z" level=info msg="Registering plugin 10-template..."
time="2025-09-10T17:12:12Z" level=info msg="Configuring plugin 10-template for runtime containerd/v2.1.0-372-g7b052529d.m..."
```
