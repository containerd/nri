This document describes how to get a test setup up and running to test the NRI Identity Plugin

Note 1: that this setup uses Containerd and Kubernetes both built and run from source.

Note 2: This test setup is executed using `local-cluster-up.sh`.

# Step 1: Spiffe/Spire

The goal of this step is to have the Spire Server and Spire Agent up and running. https://spiffe.io/docs/latest/try/getting-started-k8s/ was used as the primary reference.

## Step 1.1: Setup Spiffe/Spire

**Why:** The SPIRE Server is the central authority that issues and manages X.509 SVIDs (SPIFFE Verifiable Identity Documents). The SPIRE Agent runs as a DaemonSet on every Kubernetes node and acts as the local broker between workloads and the SPIRE Server. Without these two components running, there is no identity infrastructure for the NRI Identity Plugin to delegate to.

The SPIRE Server needs a `PersistentVolume` backed by a host directory (`/tmp/spire-data`) so that its SQLite database (used to store registration entries and keys) survives pod restarts. The `chmod 777` ensures the SPIRE Server container (which runs as a non-root user) can write to that directory.

The manifests applied from the SPIRE tutorials repository install:
- A `spire` namespace to isolate all SPIRE resources.
- ServiceAccounts, ClusterRoles, and ClusterRoleBindings granting SPIRE the Kubernetes API access it needs for node attestation and workload attestation.
- A ConfigMap holding the SPIRE Server configuration (trust domain, port, etc.).
- A StatefulSet deploying a single SPIRE Server pod, and a Service exposing it to the SPIRE Agent.
- A ConfigMap holding the SPIRE Agent configuration (server address, trust bundle path, socket path, etc.).
- A DaemonSet deploying the SPIRE Agent on every node so that workloads on any node can obtain identities.

**Result:** After running these commands, the `spire` namespace will contain a running `spire-server-0` StatefulSet pod and a `spire-agent` DaemonSet pod. The SPIRE Server will have written its initial trust bundle and be ready to accept registration entries. The SPIRE Agent will have completed node attestation (proving to the server which node it is running on) and will be ready to issue SVIDs to workloads.

```
~/IdeaProjects/kubernetes$  sudo ./_output/bin/kubectl --kubeconfig=/var/run/kubernetes/admin.kubeconfig create namespace spire

sudo mkdir /tmp/spire-data

sudo chmod 777 /tmp/spire-data

cat <<EOF | sudo ./_output/bin/kubectl --kubeconfig=/var/run/kubernetes/admin.kubeconfig apply -f -
apiVersion: v1
kind: PersistentVolume
metadata:
  name: spire-data-spire-server-0
spec:
  capacity:
    storage: 1Gi
  accessModes:
    - ReadWriteOnce
  persistentVolumeReclaimPolicy: Retain
  storageClassName: standard
  hostPath:
    path: /tmp/spire-data
EOF

sudo ./_output/bin/kubectl --kubeconfig=/var/run/kubernetes/admin.kubeconfig apply -f https://raw.githubusercontent.com/spiffe/spire-tutorials/main/k8s/quickstart/spire-namespace.yaml

sudo ./_output/bin/kubectl --kubeconfig=/var/run/kubernetes/admin.kubeconfig apply -f https://raw.githubusercontent.com/spiffe/spire-tutorials/main/k8s/quickstart/server-account.yaml

sudo ./_output/bin/kubectl --kubeconfig=/var/run/kubernetes/admin.kubeconfig apply -f https://raw.githubusercontent.com/spiffe/spire-tutorials/main/k8s/quickstart/spire-bundle-configmap.yaml

sudo ./_output/bin/kubectl --kubeconfig=/var/run/kubernetes/admin.kubeconfig apply -f https://raw.githubusercontent.com/spiffe/spire-tutorials/main/k8s/quickstart/server-cluster-role.yaml

sudo ./_output/bin/kubectl --kubeconfig=/var/run/kubernetes/admin.kubeconfig apply -f https://raw.githubusercontent.com/spiffe/spire-tutorials/main/k8s/quickstart/server-configmap.yaml

sudo ./_output/bin/kubectl --kubeconfig=/var/run/kubernetes/admin.kubeconfig apply -f https://raw.githubusercontent.com/spiffe/spire-tutorials/main/k8s/quickstart/server-statefulset.yaml

sudo ./_output/bin/kubectl --kubeconfig=/var/run/kubernetes/admin.kubeconfig apply -f https://raw.githubusercontent.com/spiffe/spire-tutorials/main/k8s/quickstart/server-service.yaml

sudo ./_output/bin/kubectl --kubeconfig=/var/run/kubernetes/admin.kubeconfig apply -f https://raw.githubusercontent.com/spiffe/spire-tutorials/main/k8s/quickstart/agent-account.yaml

sudo ./_output/bin/kubectl --kubeconfig=/var/run/kubernetes/admin.kubeconfig apply -f https://raw.githubusercontent.com/spiffe/spire-tutorials/main/k8s/quickstart/agent-cluster-role.yaml

sudo ./_output/bin/kubectl --kubeconfig=/var/run/kubernetes/admin.kubeconfig apply -f https://raw.githubusercontent.com/spiffe/spire-tutorials/main/k8s/quickstart/agent-configmap.yaml

sudo ./_output/bin/kubectl --kubeconfig=/var/run/kubernetes/admin.kubeconfig apply -f https://raw.githubusercontent.com/spiffe/spire-tutorials/main/k8s/quickstart/agent-daemonset.yaml

```

Following is the output of running the above commands:

```
namespace/spire created
persistentvolume/spire-data-spire-server-0 created
namespace/spire configured
serviceaccount/spire-server created
configmap/spire-bundle created
role.rbac.authorization.k8s.io/spire-server-configmap-role created
rolebinding.rbac.authorization.k8s.io/spire-server-configmap-role-binding created
clusterrole.rbac.authorization.k8s.io/spire-server-trust-role created
clusterrolebinding.rbac.authorization.k8s.io/spire-server-trust-role-binding created
configmap/spire-server created
statefulset.apps/spire-server created
service/spire-server created
serviceaccount/spire-agent created
clusterrole.rbac.authorization.k8s.io/spire-agent-cluster-role created
clusterrolebinding.rbac.authorization.k8s.io/spire-agent-cluster-role-binding created
configmap/spire-agent created
daemonset.apps/spire-agent created

```

Verify that pods are successfully created and running. It can take up to a few minutes for the pod to be up and running:

```
~/IdeaProjects/kubernetes$  sudo ./_output/bin/kubectl --kubeconfig=/var/run/kubernetes/admin.kubeconfig -n spire get pods

NAME                READY   STATUS    RESTARTS   AGE
spire-agent-llfj4   1/1     Running   0          2m7s
spire-server-0      1/1     Running   0          2m9s


```

## Step 1.2: Expose the Admin Socket

**Why:** By default, the SPIRE Agent only exposes a standard Workload API socket (used by workloads to obtain their own SVIDs). However, the NRI Identity Plugin needs to use the SPIRE **Delegated Identity API** — a privileged API that allows a trusted intermediary (the plugin) to fetch SVIDs *on behalf of* other workloads rather than for itself. This API is only available on a separate **admin socket**, which is not exposed by the default SPIRE Agent DaemonSet configuration.

To make the admin socket accessible outside the container (so that the plugin, running in a different container, can connect to it via a host path), the socket directory must be mounted from the host into the SPIRE Agent container.

**Result:** After editing the DaemonSet, the SPIRE Agent will bind its admin socket at `/run/spire/admin-socket/admin.sock` on the host filesystem, in addition to the existing workload socket. Any process or container on the same node that mounts `/run/spire/admin-socket` from the host will be able to connect to this socket and invoke the Delegated Identity API (subject to the `authorized_delegates` allowlist configured in Step 1.3).

### Step 1.2.1: Edit the DaemonSet

Open the DaemonSet for live editing in your default editor:

```
~/IdeaProjects/kubernetes$ sudo ./_output/bin/kubectl --kubeconfig=/var/run/kubernetes/admin.kubeconfig edit ds spire-agent -n spire

```

### Step 1.2.2: Add a new VolumeMount

**Why:** The VolumeMount tells Kubernetes to map the host directory for the admin socket into the SPIRE Agent container at a well-known path. Without this mount, the admin socket file the agent creates would only exist inside the container's ephemeral filesystem and would not be reachable by other pods or by the NRI plugin running on the host.

There is only one container in the DaemonSet configuration and that is the container for the Spire Agent.

Inside `spec.template.spec.containers[0].volumeMounts`, add:

```
- mountPath: /run/spire/admin-socket
  name: spire-admin-socket

```

### Step 1.2.3: Add the Volume

**Why:** The Volume definition backs the VolumeMount with an actual host directory. Using `type: DirectoryOrCreate` ensures Kubernetes creates the directory on the host if it does not already exist, preventing the pod from failing to start due to a missing mount source.

Inside `spec.template.spec.volumes`, add:

```
- hostPath:
    path: /run/spire/admin-socket
    type: DirectoryOrCreate
  name: spire-admin-socket

```

### Step 1.2.4: Save and Quit

Saving the DaemonSet changes will result in the following output:

```
daemonset.apps/spire-agent edited

```

## Step 1.3: Enable the Delegated API

**Why:** Exposing the admin socket directory on the host (Step 1.2) is a necessary prerequisite, but the SPIRE Agent will only create and listen on the admin socket if it is explicitly configured to do so. Additionally, the agent must be told which SPIFFE IDs are permitted to call the Delegated Identity API — this is the `authorized_delegates` allowlist. Without this configuration, the NRI Identity Plugin would have no way to request SVIDs on behalf of workloads.

### Step 1.3.1: Open the editor

```
~/IdeaProjects/kubernetes$ sudo ./_output/bin/kubectl --kubeconfig=/var/run/kubernetes/admin.kubeconfig edit configmap spire-agent -n spire

```

### Step 1.3.2: Locate the agent.conf key

Find the `data:` section. Underneath it, you'll see a large block of text assigned to `agent.conf`. This is the HCL configuration file that the SPIRE Agent reads on startup.

### Step 1.3.3: Find the agent { ... } block

Inside that text, find where the `agent {` section starts. This top-level block contains the agent's runtime settings such as data directory, log level, server address and port, trust domain, and socket paths.

### Step 1.3.4: Insert the API configuration

Add two lines inside the `agent { }` block:

- `admin_socket_path` — tells the SPIRE Agent which path to create the admin socket at. This must match the `mountPath` added to the DaemonSet in Step 1.2.2.
- `authorized_delegates` — a list of SPIFFE IDs whose holders are permitted to call the Delegated Identity API. The NRI Identity Plugin's SPIFFE ID (`spiffe://example.org/nri-identity-plugin`, registered in Step 1.4) must be included here, otherwise the agent will reject all delegate requests from the plugin.

The completed block should look something like this:

```
agent {
  data_dir = "/run/spire"
  log_level = "debug"
  server_address = "10.x.x.x" # Your previous fix
  server_port = "8081"
  socket_path = "/run/spire/sockets/agent.sock"
  trust_bundle_path = "/run/spire/bundle/bundle.crt"
  trust_domain = "example.org"

  # ADD THESE TWO LINES:
  admin_socket_path = "/run/spire/admin-socket/admin.sock"
  authorized_delegates = ["spiffe://example.org/nri-identity-plugin"]
}

```

### Step 1.3.5: Save and exit

Saving and exiting will result in the following output:

```
configmap/spire-agent edited

```

### Step 1.3.6: Restart the pod

Deleting the pod causes the DaemonSet controller to immediately recreate it. The new pod will read the updated ConfigMap at startup and begin listening on the admin socket.

```
$ sudo ./_output/bin/kubectl --kubeconfig=/var/run/kubernetes/admin.kubeconfig delete pod -l app=spire-agent -n spire

```

Verify that the Spire Agent pod restarts successfully:

```
$ sudo ./_output/bin/kubectl --kubeconfig=/var/run/kubernetes/admin.kubeconfig -n spire get pods
NAME                READY   STATUS    RESTARTS   AGE
spire-agent-9psxb   1/1     Running   0          41s
spire-server-0      1/1     Running   0          5m29s

```

**Result:** After the agent pod restarts, the file `/run/spire/admin-socket/admin.sock` will appear on the host filesystem. The SPIRE Agent is now ready to serve the Delegated Identity API to any caller whose SVID matches an entry in `authorized_delegates`.

```
$ sudo ls /run/spire/admin-socket/admin.sock
/run/spire/admin-socket/admin.sock

```

## Step 1.4: Register the NRI Identity Plugin with Spire Server

**Why:** SPIRE uses a registration entry to describe the identity (`-spiffeID`) that should be issued to a workload, the conditions (`-selector` flags) that must all be true at runtime for the workload to receive that identity, and the parent identity (`-parentID`) under which this entry sits in the SPIRE trust hierarchy. Without a registration entry, the SPIRE Agent will refuse to issue an SVID to the plugin, and all calls from the plugin to the Delegated Identity API will fail with an "unauthorized" error.

The selectors used here (`unix:uid:0`, `unix:gid:0`, `k8s:ns:kube-system`, `k8s:container-name:nri-plugin-identity`) must collectively match the runtime attributes of the process calling the agent socket, ensuring that only the legitimate NRI Identity Plugin container can obtain this SVID.

The `-parentID` must be the SPIFFE ID of the SPIRE Agent running on the same node. You can retrieve it by inspecting the agent's attestation entry on the SPIRE Server (e.g. `spire-server agent list`) or by inspecting the logs of the Spire Agent pod.

```
$ sudo ./_output/bin/kubectl --kubeconfig=/var/run/kubernetes/admin.kubeconfig -n spire logs spire-agent-9psxb
Defaulted container "spire-agent" out of: spire-agent, init (init)
time="2026-07-31T07:41:17Z" level=warning msg="Current umask 0022 is too permissive; setting umask 0027"
time="2026-07-31T07:41:17Z" level=info msg="Starting agent" data_dir=/run/spire version=1.11.2
time="2026-07-31T07:41:17Z" level=info msg="Configured plugin" external=false plugin_name=k8s_psat plugin_type=NodeAttestor reconfigurable=false subsystem_name=catalog
time="2026-07-31T07:41:17Z" level=info msg="Plugin loaded" external=false plugin_name=k8s_psat plugin_type=NodeAttestor subsystem_name=catalog
.
.
.
time="2026-07-31T07:41:17Z" level=info msg="SVID is not found. Starting node attestation" subsystem_name=attestor trust_domain_id="spiffe://example.org"
time="2026-07-31T07:41:21Z" level=info msg="Node attestation was successful" reattestable=true spiffe_id="spiffe://example.org/spire/agent/k8s_psat/demo-cluster/ef02c136-da92-42fb-a698-65da87d9c1a9" subsystem_name=attestor trust_domain_id="spiffe://example.org"
.
.
time="2026-07-31T07:41:25Z" level=info msg="Serving health checks" address="0.0.0.0:8080" subsystem_name=health
time="2026-07-31T07:41:25Z" level=info msg="Starting Admin APIs" address=/run/spire/admin-socket/admin.sock network=unix

```

The `-x509SVIDTTL 120` sets a short 120-second certificate lifetime so that certificate rotation is exercised frequently during testing.

**Result:** A new registration entry is created in the SPIRE Server's data store. The next time the SPIRE Agent performs a sync with the server, it will receive this entry and begin issuing the SVID `spiffe://example.org/nri-identity-plugin` to any process on the node that satisfies all the specified selectors. This SVID is also what allows the plugin to pass the `authorized_delegates` check configured in Step 1.3.

Remember to replace the `-parentID` with the SPIFFE ID of the SPIRE Agent:

```
$ sudo ./_output/bin/kubectl --kubeconfig=/var/run/kubernetes/admin.kubeconfig exec -n spire spire-server-0 -- /opt/spire/bin/spire-server entry create -parentID spiffe://example.org/spire/agent/k8s_psat/demo-cluster/ef02c136-da92-42fb-a698-65da87d9c1a9 -spiffeID spiffe://example.org/nri-identity-plugin -selector unix:uid:0 -selector unix:gid:0 -selector k8s:ns:kube-system -selector k8s:container-name:nri-plugin-identity -x509SVIDTTL 120

Entry ID         : 7df39966-be54-4c18-9d70-ad2873103f9a
SPIFFE ID        : spiffe://example.org/nri-identity-plugin
Parent ID        : spiffe://example.org/spire/agent/k8s_psat/demo-cluster/ef02c136-da92-42fb-a698-65da87d9c1a9
Revision         : 0
X509-SVID TTL    : 120
JWT-SVID TTL     : default
Selector         : k8s:container-name:nri-plugin-identity
Selector         : k8s:ns:kube-system
Selector         : unix:gid:0
Selector         : unix:uid:0

```

## Step 1.5: Register Sample Test Workload with the Spire Server

**Why:** In order for the Spire Server to know the existence of a workload and issue an SVID for that workload, that workload has to be registered with the Spire Server. The Registration entry in the Spire Server must have:

1. Has `-parentID spiffe://example.org/nri-identity-plugin` — meaning the plugin's own SVID is the attesting authority for this entry. This is the "delegated" trust relationship: the plugin vouches for the workload. //TODO is this mandatory?
2. Has `-selector` flags that match the container's runtime attributes (namespace, pod name, container name), so the server can confirm the plugin is requesting an identity for the correct workload.

Without these entries, any call the NRI plugin makes to the Delegated Identity API for a workload container will be rejected by the SPIRE Agent with "no registration entries found".

The test workload pod `bbid0` has three containers (`c0`, `c1`, `c2`), but only `c0` and `c1` are registered here intentionally — `c2` is left without an entry to demonstrate that the plugin gracefully skips containers for which no identity is configured.

**Result:** Two registration entries are created in the SPIRE Server. When the test pod `bbid0` is deployed (Step 3) and the NRI Identity Plugin intercepts its container start events, the plugin will use these entries to fetch X.509 SVIDs for `c0` and `c1` and inject them as certificate files into each container's filesystem. `c2` will not receive any SVID.

The First Container

```
$ sudo ./_output/bin/kubectl --kubeconfig=/var/run/kubernetes/admin.kubeconfig exec -n spire spire-server-0 -- /opt/spire/bin/spire-server entry create -parentID spiffe://example.org/nri-identity-plugin -spiffeID spiffe://example.org/nri-identity-plugin/bbid0/c0 -selector k8s:container-name:c0 -selector k8s:pod-name:bbid0 -selector k8s:ns:default -x509SVIDTTL 120

Entry ID         : 3f9cb6bd-2446-43d3-9989-7da3a5fa2991
SPIFFE ID        : spiffe://example.org/nri-identity-plugin/bbid0/c0
Parent ID        : spiffe://example.org/nri-identity-plugin
Revision         : 0
X509-SVID TTL    : 120
JWT-SVID TTL     : default
Selector         : k8s:container-name:c0
Selector         : k8s:ns:default
Selector         : k8s:pod-name:bbid0


```

The Second Container

```
$ sudo ./_output/bin/kubectl --kubeconfig=/var/run/kubernetes/admin.kubeconfig exec -n spire spire-server-0 -- /opt/spire/bin/spire-server entry create -parentID spiffe://example.org/nri-identity-plugin -spiffeID spiffe://example.org/nri-identity-plugin/bbid0/c1 -selector k8s:container-name:c1 -selector k8s:pod-name:bbid0 -selector k8s:ns:default -x509SVIDTTL 120

Entry ID         : 7162fe96-2bf5-4516-87e5-626bb72f7e3a
SPIFFE ID        : spiffe://example.org/nri-identity-plugin/bbid0/c1
Parent ID        : spiffe://example.org/nri-identity-plugin
Revision         : 0
X509-SVID TTL    : 120
JWT-SVID TTL     : default
Selector         : k8s:container-name:c1
Selector         : k8s:ns:default
Selector         : k8s:pod-name:bbid0

```

# Step 2: NRI Identity Plugin

## Step 2.1: Containerize the plugin and push to local registry

**Why:** The test cluster is an isolated local Kubernetes cluster (started via `local-cluster-up.sh`) that does not have access to any external image registry. A local Docker registry (listening on `localhost:5000`) is used as a substitute. The plugin image must be built from source and pushed to this registry so that the Kubernetes node can pull it when the DaemonSet pod is scheduled.

The host directories `/var/run/spiffe/` and `/var/run/spiffe/secrets/` are created in advance because the plugin container mounts them to store the SVIDs it fetches from SPIRE. The `chmod 777` ensures the plugin process (which may run as a non-root user) can write certificate files into the secrets directory.

`make build/bin/identity-injector` compiles the plugin binary. The `docker build` step packages it into a container image using the shared plugin `Dockerfile`. The `docker tag` and `docker push` steps make the image available in the local registry.

**Result:** The image `localhost:5000/nri-identity-injector:latest` is available in the local registry and can be pulled by Kubernetes. The host paths for SVID storage exist with the correct permissions.

```
sudo mkdir /var/run/spiffe/
sudo mkdir /var/run/spiffe/secrets/
sudo chmod 777 /var/run/spiffe/secrets/
sudo make build/bin/identity-injector

sudo docker build \
  -f plugins/Dockerfile \
  --build-arg PLUGIN=identity-injector \
  -t nri-identity-injector:latest \
  .

sudo docker tag nri-identity-injector:latest localhost:5000/nri-identity-injector:latest

sudo docker push localhost:5000/nri-identity-injector:latest

```

Running the above commands will result in the following output:

```
Building build/bin/identity-injector...
[+] Building 1.6s (12/12) FINISHED                                                                                                                   docker:default
.
.
.
.
The push refers to repository [localhost:5000/nri-identity-injector]
22d392cf586d: Layer already exists
4fc62ec696ca: Pushed
latest: digest: sha256:d1c16b291d569ceaaa1a7732c1bfeb058c00f2268ccbf382880dfb6413cccef1 size: 855

```

## Step 2.2: Deploy the NRI Identity Plugin

**Why:** The kustomize overlay in `contrib/kustomize/identity-injector/` contains all of the Kubernetes resources needed to run the NRI Identity Plugin as a DaemonSet in the `kube-system` namespace: the DaemonSet definition (which mounts the SPIRE admin socket and the SVID secrets directory), the NRI registration ConfigMap (so that the Containerd NRI runtime knows to invoke the plugin), and the necessary RBAC resources. Applying it as a single `kubectl apply -k` command ensures all resources are created in the correct order and with the correct labels.

```
~/IdeaProjects/kubernetes$ sudo ./_output/bin/kubectl --kubeconfig=/var/run/kubernetes/admin.kubeconfig apply -k ../nri/contrib/kustomize/identity-injector/

daemonset.apps/nri-plugin-identity created

```

Verify that the DaemonSet was deployed successfully:

```

~/IdeaProjects/kubernetes$ sudo ./_output/bin/kubectl --kubeconfig=/var/run/kubernetes/admin.kubeconfig -n kube-system get pods

NAME                        READY   STATUS    RESTARTS   AGE
coredns-6ff6468495-shgh8    1/1     Running   0          15m
nri-plugin-identity-7r6xv   1/1     Running   0          62s

```

# Step 3: Deploy Test Workload

**Why:** This step deploys the pod `bbid0` that exercises the full end-to-end path. The pod's annotations (`identity.noderesource.dev/container.c0` and `identity.noderesource.dev/container.c1`) are the signal the NRI Identity Plugin reads to decide which containers should receive an SVID and what filenames to use for the certificate, key, and trust bundle. Containers without a matching annotation (such as `c2`) are silently skipped by the plugin.

**Result:** The NRI plugin intercepts the `RunPodSandbox` and `CreateContainer` events for `bbid0`. For `c0` and `c1`, it fetches their respective SVIDs from SPIRE via the Delegated Identity API and writes the certificate files (`svid.pem`, `svid_key.pem`, `svid_bundle.pem`) into `/var/run/spiffe/secrets` inside each container. The startup script in each container checks for the existence of `/var/certs/svid.pem` and prints `svid exists!` to confirm successful injection. Container `c2` prints `svid does NOT exist.` as expected.

```
~/IdeaProjects/kubernetes$ sudo ./_output/bin/kubectl --kubeconfig=/var/run/kubernetes/admin.kubeconfig apply -f busyboxpodspec.yaml

pod/bbid0 created

```

Verify that the test workload was deployed successfully:

```
~/IdeaProjects/kubernetes$ sudo ./_output/bin/kubectl --kubeconfig=/var/run/kubernetes/admin.kubeconfig get pods -n default
NAME    READY   STATUS    RESTARTS   AGE
bbid0   3/3     Running   0          21s

```

Using the test podspec with busybox containers:

```
apiVersion: v1
kind: Pod
metadata:
  name: bbid0
  labels:
    app: bbid0
  annotations:
    identity.noderesource.dev/container.c0: |+
      cert_file_name: svid.pem
      key_file_name: svid_key.pem
      bundle_file_name: svid_bundle.pem
    identity.noderesource.dev/container.c1: |+
      cert_file_name: svid.pem
      key_file_name: svid_key.pem
      bundle_file_name: svid_bundle.pem
spec:
  containers:
  - name: c0
    image: busybox
    imagePullPolicy: IfNotPresent
    command:
      - sh
      - -c
      - |
        if [ -f /var/run/spiffe/secrets/svid.pem ]; then
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
      - -c
      - |
        if [ -f /var/run/spiffe/secrets/svid.pem ]; then
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
        if [ -f /var/run/spiffe/secrets/svid.pem ]; then
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



# Step 4: Verify Existence of Identity Artifacts

Verify that the identity artifacts have been successfully fetched:

```
~/IdeaProjects/kubernetes$ ls /var/run/spiffe/secrets/
4101ec79-32d0-45d2-aee9-03d6a57d9993

~/IdeaProjects/kubernetes$ ls /var/run/spiffe/secrets/4101ec79-32d0-45d2-aee9-03d6a57d9993/
c0  c1

~/IdeaProjects/kubernetes$ ls /var/run/spiffe/secrets/4101ec79-32d0-45d2-aee9-03d6a57d9993/c0/
svid_bundle.pem  svid_key.pem  svid.pem

~/IdeaProjects/kubernetes$ ls /var/run/spiffe/secrets/4101ec79-32d0-45d2-aee9-03d6a57d9993/c1
svid_bundle.pem  svid_key.pem  svid.pem

~/IdeaProjects/kubernetes$ sudo ./_output/bin/kubectl --kubeconfig=/var/run/kubernetes/admin.kubeconfig exec bbid0 -n default -it -- /bin/sh
Defaulted container "c0" out of: c0, c1, c2

/ # ls
bin    dev    etc    home   lib    lib64  proc   root   sys    tmp    usr    var

/ # ls var/run/
secrets  spiffe

/ # ls var/run/spiffe/
secrets

/ # ls var/run/spiffe/secrets/
svid.pem         svid_bundle.pem  svid_key.pem

```