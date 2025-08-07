## NRI Differ Plugin

The differ plugin can be injected before, after or between other NRI plugins
to track and show what changes these other plugins request to containers.
The plugin can register itself multiple times at multiple indices, so a single
differ instance can be used to track and show step-by-step all the changes
made to a container.

## Deployment

The NRI repository contains minimal kustomize overlays for this plugin at
[contrib/kustomize/differ](../../contrib/kustomize/differ).

Deploy the latest release with:

```bash
kubectl apply -k https://github.com/containerd/nri/contrib/kustomize/differ
```

Deploy a specific release with:

```bash
RELEASE_TAG=v0.10.0
kubectl apply -k "github.com/containerd/nri/contrib/kustomize/differ?ref=${RELEASE_TAG}"
```

Deploy the latest development build from tip of the main branch with:

```bash
kubectl apply -k https://github.com/containerd/nri/contrib/kustomize/differ/unstable
```

## Testing

You can test this plugin by registering it to the desired indices (for
instance `nri-differ --indices 00,20,99 --yaml`) in addition to your other
plugins that make changes to containers, then starting some containers
and examining the results. You should see container modifications printed
as yaml-diffs. Make sure you properly inject/register `differ` both at the
front of the plugin chain and after any other plugin.
