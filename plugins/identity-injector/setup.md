This document describes how to get a test setup up and running to test the NRI Identity Plugin

Note: that this setup uses Containerd and Kubernetes both built and run from source

# Step 1: Spiffe/Spire

## Step 1.1: Setup Spiffe/Spire

```
sudo ./_output/bin/kubectl --kubeconfig=/var/run/kubernetes/admin.kubeconfig create namespace spire

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


## Step 1.2: Expose the Admin Socket

### Step 1.2.1: Edit the DaemonSet:

```
sudo ./_output/bin/kubectl --kubeconfig=/var/run/kubernetes/admin.kubeconfig edit ds spire-agent -n spire

```

### Step 1.2.2: Add a new VolumeMount: Inside spec.template.spec.containers[0].volumeMounts, add:

```
- mountPath: /run/spire/admin-socket
  name: spire-admin-socket

```

### Step 1.2.3: Add the Volume: Inside spec.template.spec.volumes, add:

```
- hostPath:
    path: /run/spire/admin-socket
    type: DirectoryOrCreate
  name: spire-admin-socket

```

## Step 1.3: Enable the Delegated API

### Step 1.3.1: open the editor

```
sudo ./_output/bin/kubectl --kubeconfig=/var/run/kubernetes/admin.kubeconfig edit configmap spire-agent -n spire

```

### Step 1.3.2: Locate the agent.conf key: Find the data: section. Underneath it, you'll see a large block of text assigned to agent.conf.

### Step 1.3.3: Find the agent { ... } block: Inside that text, find where the agent { section starts.

### Step 1.3.4: Insert the API configuration: Add the authorized_delegates line inside that block. It should look something like this:

```
agent {
  data_dir = "/run/spire"
  log_level = "debug"
  server_address = "10.x.x.x" # Your previous fix
  server_port = "8081"
  socket_path = "/run/spire/sockets/agent.sock"
  trust_bundle_path = "/run/spire/bundle/bundle.crt"
  trust_domain = "example.org"

  # ADD THIS LINE:
  admin_socket_path = "/run/spire/admin-socket/admin.sock"
  authorized_delegates = ["spiffe://example.org/nri-identity-plugin"]
}

```

### Step 1.3.5: save and exit

### Step 1.3.6: restart the pod

```
sudo ./_output/bin/kubectl --kubeconfig=/var/run/kubernetes/admin.kubeconfig delete pod -l app=spire-agent -n spire

```


## Step 1.4: Register the NRI Identity Plugin with Spire Server

Remember to replace the parentId with the spiffeId of the Spire Agent

```
sudo ./_output/bin/kubectl --kubeconfig=/var/run/kubernetes/admin.kubeconfig exec -n spire spire-server-0 -- /opt/spire/bin/spire-server entry create -parentID [replace-me-with-the-spiffe-id-of-the-spire-agent] -spiffeID spiffe://example.org/nri-identity-plugin -selector unix:uid:0 -selector unix:gid:0 -selector k8s:ns:kube-system -selector k8s:container-name:nri-plugin-identity -x509SVIDTTL 120

```


## Step 1.5: Register Sample Test Workload with the Spire Server

The First Container

```
sudo ./_output/bin/kubectl --kubeconfig=/var/run/kubernetes/admin.kubeconfig exec -n spire spire-server-0 -- /opt/spire/bin/spire-server entry create -parentID spiffe://example.org/nri-identity-plugin -spiffeID spiffe://example.org/nri-identity-plugin/bbid0/c0 -selector k8s:container-name:c0 -selector k8s:pod-name:bbid0 -selector k8s:ns:default -x509SVIDTTL 120

```

The Second Container

```
sudo ./_output/bin/kubectl --kubeconfig=/var/run/kubernetes/admin.kubeconfig exec -n spire spire-server-0 -- /opt/spire/bin/spire-server entry create -parentID spiffe://example.org/nri-identity-plugin -spiffeID spiffe://example.org/nri-identity-plugin/bbid0/c1 -selector k8s:container-name:c1 -selector k8s:pod-name:bbid0 -selector k8s:ns:default -x509SVIDTTL 120

```



# Step 2: NRI Identity Plugin

## Step 2.1: Containerize the plugin and push to local registry


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

## Step 2.2: Deploy the NRI Identity Plugin

```
sudo ./_output/bin/kubectl --kubeconfig=/var/run/kubernetes/admin.kubeconfig apply -k ../nri/contrib/kustomize/identity-injector/

```

# Step 3: Deploy Test Workload

```
sudo ./_output/bin/kubectl --kubeconfig=/var/run/kubernetes/admin.kubeconfig apply -f busyboxpodspec.yaml

```

Using the test podspec with busybox containers

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