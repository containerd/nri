# Template NRI plugin

This is a minimal example plugin that demonstrates how to interact with
container lifecycle events with NRI. This plugin can be deployed for testing
and demonstration purposes and used as a base for implementing new NRI plugins.

## Deployment

The NRI repository contains minimal kustomize overlays for this plugin at
[contrib/kustomize/template](../../contrib/kustomize/template).

Deploy the latest release with:

```bash
kubectl apply -k https://github.com/containerd/nri/contrib/kustomize/template
```

Deploy a specific release with:

```bash
RELEASE_TAG=v0.10.0
kubectl apply -k "github.com/containerd/nri/contrib/kustomize/template?ref=${RELEASE_TAG}"
```

Deploy the latest development build from tip of the main branch with:

```bash
kubectl apply -k https://github.com/containerd/nri/contrib/kustomize/template/unstable
```
