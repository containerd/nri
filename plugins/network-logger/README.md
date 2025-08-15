## NRI Network logger Plugin

The network logger plugin logs the network parameters of Pods, specifically
the Pod IPs assigned by the CNI plugin and the network namespace of the Pod.

The provided network logger NRI plugin serves as a reference implementation,
showcasing non-disruptive integration with container runtimes and CNI plugins.

### The Kubernetes network model

The Kubernetes network model is implemented by the container runtime on each node.
The most common container runtimes use Container Network Interface (CNI) plugins
to manage their network and security capabilities.

Before starting a pod, kubelet calls RuntimeService.RunPodSandbox to create the environment.
This includes setting up networking for a pod (e.g., allocating an IP).
Once the PodSandbox is active, individual containers can be created/started/stopped/removed independently.
To delete the pod, kubelet will stop and remove containers before stopping and removing the PodSandbox
and releasing the network resources (e.g., releasing the IP).

### NRI Pod Lifecycle events

NRI plugins can subscribe to the following Pod lifecycle events that are of interest to
integrations with Kubernetes and depend on the Pod networking characteristics.

- RunPodSandbox: It happens after the Linux network namespace has been created and the CNI
plugin has created the network interface and allocated the IPs.

- StopPodSandbox: It happens before the CNI plugin removes the network interface and releases the IPs.

- RemovePodSandbox: It happens after all the network resources were released.

## Deployment

The NRI repository contains minimal kustomize overlays for this plugin at
[contrib/kustomize/network-logger](../../contrib/kustomize/network-logger).

Deploy the latest release with:

```bash
kubectl apply -k https://github.com/containerd/nri/contrib/kustomize/network-logger
```

Deploy a specific release with:

```bash
RELEASE_TAG=v0.10.0
kubectl apply -k "github.com/containerd/nri/contrib/kustomize/network-logger?ref=${RELEASE_TAG}"
```

Deploy the latest development build from tip of the main branch with:

```bash
kubectl apply -k https://github.com/containerd/nri/contrib/kustomize/network-logger/unstable
```

## Testing

You can test this plugin using a kubernetes cluster/node with a container
runtime that has NRI support enabled.
Start the plugin on the target node (`network-logger -idx 10`) and it will start
logging all the networking related events and the Pod IPs and network namespaces.

```
./network-logger -idx 10
INFO   [0000] Created plugin 10-network-logger (network-logger, handles RunPodSandbox,StopPodSandbox,RemovePodSandbox)
INFO   [0000] Registering plugin 10-network-logger...
INFO   [0000] Configuring plugin 10-network-logger for runtime v2/v2.0.0-rc.5-110-g5e084bdc6...
INFO   [0000] Started plugin 10-network-logger...
INFO   [0000] Synchronized state with the runtime (6 pods, 5 containers)...
INFO   [0000] pod default/webapp: namespace=/var/run/netns/cni-d3f41d35-dde9-382d-9502-3cb05c99543a ips=[10.244.1.22]
INFO   [0000] pod ingress-nginx/ingress-nginx-controller-7d8d8c7b4c-dz7f8: namespace=<host-network> ips=[10.244.1.7]
INFO   [0000] pod kube-system/kube-proxy-cvlfm: namespace=<host-network> ips=[]
INFO   [0000] pod default/test3: namespace=/var/run/netns/cni-a2492a06-3efc-bec3-b180-0a83411b7311 ips=[10.244.1.20]
INFO   [0000] pod kube-system/kube-network-policies-996zs: namespace=<host-network> ips=[]
INFO   [0000] pod ingress-nginx/ingress-nginx-controller-7d8d8c7b4c-dz7f8: namespace=/var/run/netns/cni-7a78a743-c4db-7790-0f38-7e6f8f945d42 ips=[10.244.1.4]

INFO   [0027] Started pod default/test-nri-1: namespace=/var/run/netns/cni-2b46640c-8073-214b-77ae-fdebdcd7c223 ips=[10.244.1.24]
INFO   [0038] Stopped pod default/test-nri-1: ips=[10.244.1.24]
INFO   [0047] Removed pod default/test-nri-1: ips=[10.244.1.24]
```
