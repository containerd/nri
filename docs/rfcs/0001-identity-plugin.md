- Created:
- Current Version:
- Status: WIP
- Contributors: Harshit Gupta (hg2t4e@gmail.com, harshit.gupta@ibm.com), Michael Brown (brownwm@us.ibm.com)
- Approvers:

# Summary

The identity plugin, running as a daemonset, fetches identity artifacts (X.509 SVID certificate bundle) for each annotated container and mounts the artifact volume into the container.

# Motivation

The current way how we mount identity artifacts (X.509 SVID certificate bundle) into a container is to use a initContainer / sidecar. There is a sidecar for each pod. This sidecar fetches the artifacts from identity providers, injects the volume containing identity artifacts into the pod and mounts the volume into the container. Using a sidecar in each pod for fetching identity artifacts is too resource intensive. Using a initContainer sidecar comes with an additional overhead of increased container start latency. The motivation for using a NRI Identity Plugin is that only one process is enough to fetch the identity artifacts for all of the annotated containers in all the pods on a single host. Therefore the identity Plugin as a daemonset can fetch the identity artifacts for all the containers in all the pods on all thee hosts in the cluster that have the appropriate annotations. Running as a single process per host instead of single process per pod we save on host resources.

# Proposed Implementation

## Setup Steps

WIP

1. Create a static spiffe-id for the plugin.

```
spire-server entry create \
    -spiffeID spiffe://example.org/nri/identity-injector \
    -selector k8s:ns:backend \
    -selector k8s:sa:db-writer
```

PENDING - what selector's will the plugin have?


2. https://spiffe.io/docs/latest/deploying/spire_agent/#delegated-identity-api Configure the Plugin's Spiffe ID in the Agent configuration.

```
agent {
    trust_domain = "example.org"
    ...
    admin_socket_path = "/tmp/spire-agent/private/admin.sock"
    authorized_delegates = [
        "spiffe://example.org/nri/identity-injector"
    ]
}

```


## How the plugin will work

WIP

1. Plugin registers itself to the agent and get its spiffe id.

2. Containerd calls the NRI plugin on CreateContainer event - PENDING fix the correct event, add details on when and which volume is mounted. The plugin receives the pod and container metadata. Using this metadata, the plugin executes step 3 below. The plugin has to ensure that the pid is a stable identifier, that the pid is not a recycled identifier. This is a platform dependent step. (e.g. by using pidfds on Linux). https://spiffe.io/docs/latest/deploying/spire_agent/#delegated-identity-api

3. The plugin reads the podspec and parses the annotations. Using spire-api-sdk, the plugin fetches identity artifacts and mounts the volume containing the identity artifacts into the container.  The spire-api-sdk streams artifact updates, the plugin will on receiving updates will overwrite the artifact on the host volume. Since the host volume is mounted into the container, the artifacts in the container are automatically updated. The plugin does not need a separate mount annotation in the podspec yaml. The daemonset specifies the volume that the plugin will use to fetch and download the identity artifacts. The plugin will then mount the host volume to the container volume using the cert_dir from the podspec.



## DaemonSet yaml

WIP

```
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: nri-plugin-identity-injector
spec:
  template:
    spec:
      priorityClassName: system-node-critical
      containers:
        - name: plugin
          image: plugin:latest
          args:
            - "-idx"
            - "10"
          resources:
            requests:
              cpu: "2m"
              memory: "5Mi"
          securityContext:
            allowPrivilegeEscalation: false
            capabilities:
              drop:
                - ALL
          volumeMounts:
            - name: nri-socket
              mountPath: /var/run/nri/nri.sock
            - name: identity-artifacts                      # Artifact volume
              mountPath: /var/identity/
      volumes:
        - name: nri-socket
          hostPath:
            path: /var/run/nri/nri.sock
            type: Socket
        - name: identity-artifacts                          # Artifact volume
          hostPath:
            path: /var/identity/
            type: Directory

```



## Podspec example

WIP PENDING - are annotations immutable? can we support mutable annotations?
WIP PENDING - supporting pod certificate KEP 4317

WIP

```
apiVersion: v1
kind: Pod
metadata:
  name: bbid0
  labels:
    app: bbid0
  annotations:
    identity.noderesource.dev/container.c0: |+
      - spire_agent_address: /tmp/agent.sock
        cert_dir: /var/certs
        svid_file_name: svid.pem
        svid_key_file_name: svid_key.pem
        svid_bundle_file_name: svid_bundle.pem
    identity.noderesource.dev/container.c1: |+
      - spire_agent_address: /tmp/agent.sock
        cert_dir: /var/certs
        svid_key_file_name: svid_key.pem
        svid_bundle_file_name: svid_bundle.pem
spec:
  containers:
  - name: c0
    image: busybox
    imagePullPolicy: IfNotPresent
    command:
      - sh
      - -c
      - |
        if [ -f /var/certs/svid.pem ]; then
          echo "svid exists!"
        else
          echo "svid does NOT exist."
        fi
        sleep inf
    resources:
      requests:
        cpu: 500m
        memory: '100M'
      limits:
        cpu: 500m
        memory: '100M'
  - name: c1
    image: busybox
    imagePullPolicy: IfNotPresent
    command:
      - sh
      - |
        if [ -f /var/certs/svid.pem ]; then
          echo "svid exists!"
        else
          echo "svid does NOT exist."
        fi
        sleep inf
    resources:
      requests:
        cpu: 1
        memory: '100M'
      limits:
        cpu: 1
        memory: '100M'
  - name: c2
    image: busybox
    imagePullPolicy: IfNotPresent
    command:
      - sh
      - -c
      - |
        if [ -f /var/certs/svid.pem ]; then
          echo "svid exists!"
        else
          echo "svid does NOT exist."
        fi
        sleep inf
    resources:
      requests:
        cpu: 1
        memory: '100M'
      limits:
        cpu: 1
        memory: '100M'
  terminationGracePeriodSeconds: 1

```

## Handling Openshift / cri-o

Openshift / cri-o specific challenges that the plugin resolves.

WIP

## Handling VMs

WIP


# Alternatives

## Sidecars / InitContainers

An alternative to identity plugin is to use sidecar containers or initContainers in every pod for fetching the artifacts and mounting the artifact volume.

Advantage of using NRI Plugin over sidecars: SideCars / initContainers require additional resources. NRI Plugin does not need sidecar containers or initContainers therefore the NRI Plugin is resource efficient.


## CSI Driver

Another alternative is to use a CSI Driver. The CSI Driver uses the Pod's PID to fetch identity artifacts before the container is created/started.

Advantage of using NRI Plugin over CSI Driver: 
  - CSI drivers require extra sidecars for their functioning. Although these sidecars per node (not per pod), they still consume resources. NRI Plugin does not require any sidecars to support its function and is therefore resource efficient.
  - Since the container is actually using the identity artifacts, the actual PID that the identity artifacts should be fetched for is the container PID. CSI Driver does not have the container PID at the time it fetches the identity artifacts therefore the CSI Driver uses the Pod PID. CSI Driver could use the container PID as soon as it is available after container start, but by that time a container might have executed commands without identity artifacts, breaking security. 

## Advantages of using NRI Identity Plugin over K8s KEP 4317 Pod Certificates

WIP

- Adds supports for stacks not using the Kubelet.
- Must also be using the Pod's PID to fetch identity artifacts.


## Advantages of using NRI Identity Plugin over Container Lifecycle Hooks

WIP


# Open Questions

How can the container runtime not fetch the certs if the kubelet is also fetching them? or the other way around