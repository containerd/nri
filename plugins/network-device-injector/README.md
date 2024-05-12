## Network Device Injector Plugin

This sample plugin can inject existing network devices into containers using pod annotations.
Network devices are network namespaced, this implies that in Kubernetes they are Pod scoped
and not container scoped; all containers are able to access the network device inside the Pod.

Traditionally in Kubernetes the CNI plugin is responsible for configuring the default network
interface for Pods, but there are use cases where the Pod may need to use additional network interfaces.
A more detailed explanation of all the possible technologies to add interfaces to Pods was presented during
[SIG Network meeting 14/03/2024](https://www.youtube.com/watch?v=67UzeMEaqnM&list=PL69nYSiGNLP2E8vmnqo5MwPOY25sDWIxb&index=1),
[slides](Slides in https://docs.google.com/presentation/d/1pjDCtpdbCSWaqCbBYWgzTxAewOVbMf6rUS5SbjAJAe8/edit?usp=sharing).

The Kubernetes project is working to [provide a better API](https://docs.google.com/document/d/1VBBj8Fh0ks0_-dacpqx6kD2tlIvj0XfFxtMuSfOJ22w/edit)
introducing network device claims that would naturally provide a built in means to inject.

[Network Devices may be included in the OCI Runtime Specification](https://github.com/opencontainers/runtime-spec/issues/1239), this will allow
implementations to be more declarative offloading the low level implementation details to the runtime implementation.

Pods that run in the host network namespace can not inject any network device as those are already running on the same network namespace,
and any modification can impact the existing system networking.

### Network Device Annotations

Network devices are annotated using the `netdevices.nri.containerd.io` annotation key prefix.
Network devices are defined at the Pod level, since are part of the network namespace.

The annotation syntax for network device injection is

```
- name: enp2s2f0
  new_name: eth1
  address: 192.168.2.2
  prefix: 24
  mtu: 1500
- name: enp2s2f1
  ...
```

The parameters are based on the existing linux netdevice representation.
https://man7.org/linux/man-pages/man7/netdevice.7.html

`name` is mandatory and refers to the name of the network interface in the host,
the rest of the parameters are optional.
`new_name` is the name of the interface inside the Pod.

The plugin only injects interfaces on the Pod network namespace for which the containers are attached when created,
for more advanced networking configuration like routing, traffic redirection or dynamic address configuration new plugins can be created.

## Testing

You can test this plugin using a kubernetes cluster/node with a container
runtime that has NRI support enabled. Start the plugin on the target node
(`network-device-injector -idx 10`), create a pod with some annotated network devices or
mounts, then verify that those get injected to the containers according
to the annotations.

On the same node where the plugin is running create a dummy interface:

```
ip link add dummy0 type dummy
```

You can validate the interface state with the following command

```
$ ip link show dev dummy0
81: dummy0: <BROADCAST,NOARP> mtu 1500 qdisc noop state DOWN mode DEFAULT group default qlen 1000
    link/ether fa:57:1c:81:0b:98 brd ff:ff:ff:ff:ff:ff
```

See the [sample pod spec](sample-network-device-inject.yaml) for an example.

Once the Pod is running you'll be able to check that the `dummy0` interface is no longer
present in the node, and is now inside the Pod with the new name and network configuration
passed on the annotation.

```
kubectl exec -it bbdev0 ip a
kubectl exec [POD] [COMMAND] is DEPRECATED and will be removed in a future version. Use kubectl exec [POD] -- [COMMAND] instead.
Defaulted container "c0" out of: c0, c1
1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue qlen 1000
    link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
    inet 127.0.0.1/8 scope host lo
       valid_lft forever preferred_lft forever
    inet6 ::1/128 scope host
       valid_lft forever preferred_lft forever
2: eth0@if80: <BROADCAST,MULTICAST,UP,LOWER_UP,M-DOWN> mtu 1500 qdisc noqueue
    link/ether de:1d:9b:0f:83:b3 brd ff:ff:ff:ff:ff:ff
    inet 10.244.1.76/24 brd 10.244.1.255 scope global eth0
       valid_lft forever preferred_lft forever
    inet6 fe80::dc1d:9bff:fe0f:83b3/64 scope link
       valid_lft forever preferred_lft forever
79: eth33: <BROADCAST,NOARP> mtu 1500 qdisc noop qlen 1000
    link/ether 3a:74:86:94:75:6b brd ff:ff:ff:ff:ff:ff
    inet 192.168.2.2/24 brd 192.168.2.255 scope global eth33
       valid_lft forever preferred_lft forever
```
