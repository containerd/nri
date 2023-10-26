# Build clearcfs
```shell
make
```
# Install
```shell
make install
```
# Run container with pod sandbox
```shell
crictl run misc/container.yaml misc/pod.yaml
```
# Check the containerd logs
```shell
journalctl -xeu containerd | grep "clearcfs"
```
Expected output:
```
Oct 26 21:44:37 10.37.7.188 containerd[2849150]: time="2023-10-26T21:44:37+08:00" level=info msg="[clearcfs] Invoked NRI plugin, state: create"
Oct 26 21:44:37 10.37.7.188 containerd[2849150]: time="2023-10-26T21:44:37+08:00" level=info msg="[clearcfs] Container ID: 40dabdba54c52d5a3efc7325348e8c5ef06a4ad80a0d241a1152bd1399ce8d03, PID: 2853941, Sandbox ID: 40dabdba54c52d5a3efc7325348e8c5ef06a4ad80a0d241a1152bd1399ce8d03"
Oct 26 21:44:37 10.37.7.188 containerd[2849150]: time="2023-10-26T21:44:37+08:00" level=info msg="[clearcfs] Labels: map[]"
Oct 26 21:44:37 10.37.7.188 containerd[2849150]: time="2023-10-26T21:44:37+08:00" level=info msg="[clearcfs] Annotations: map[qos.class:ls]"
Oct 26 21:44:37 10.37.7.188 containerd[2849150]: time="2023-10-26T21:44:37+08:00" level=info msg="[clearcfs] Spec.Annotations: map[io.kubernetes.cri.container-type:sandbox io.kubernetes.cri.sandbox-id:40dabdba54c52d5a3efc7325348e8c5ef06a4ad80a0d241a1152bd1399ce8d03 io.kubernetes.cri.sandbox-log-directory: io.kubernetes.cri.sandbox-name:sandbox io.kubernetes.cri.sandbox-namespace:default io.kubernetes.cri.sandbox-uid:hdishd83djaidwnduwk28bcsb]"
Oct 26 21:44:37 10.37.7.188 containerd[2849150]: time="2023-10-26T21:44:37+08:00" level=info msg="[clearcfs] Spec.CgroupsPath: /k8s.io/40dabdba54c52d5a3efc7325348e8c5ef06a4ad80a0d241a1152bd1399ce8d03"
Oct 26 21:44:37 10.37.7.188 containerd[2849150]: time="2023-10-26T21:44:37+08:00" level=info msg="[clearcfs] Spec.Namespaces: map[ipc: mount: network:/var/run/netns/cni-58bdab5c-3c90-0d79-88c6-58775cce3207 pid: uts:]"
Oct 26 21:44:37 10.37.7.188 containerd[2849150]: time="2023-10-26T21:44:37+08:00" level=info msg="[clearcfs] clearing cfs for 40dabdba54c52d5a3efc7325348e8c5ef06a4ad80a0d241a1152bd1399ce8d03"
Oct 26 21:44:40 10.37.7.188 containerd[2849150]: time="2023-10-26T21:44:40+08:00" level=info msg="[clearcfs] Invoked NRI plugin, state: create"
Oct 26 21:44:40 10.37.7.188 containerd[2849150]: time="2023-10-26T21:44:40+08:00" level=info msg="[clearcfs] Container ID: 1bb3edcd4eb4cc491bd216635069e2245883374932af4590d7badf82963e7808, PID: 2854103, Sandbox ID: 40dabdba54c52d5a3efc7325348e8c5ef06a4ad80a0d241a1152bd1399ce8d03"
Oct 26 21:44:40 10.37.7.188 containerd[2849150]: time="2023-10-26T21:44:40+08:00" level=info msg="[clearcfs] Labels: map[]"
Oct 26 21:44:40 10.37.7.188 containerd[2849150]: time="2023-10-26T21:44:40+08:00" level=info msg="[clearcfs] Annotations: map[qos.class:ls]"
Oct 26 21:44:40 10.37.7.188 containerd[2849150]: time="2023-10-26T21:44:40+08:00" level=info msg="[clearcfs] Spec.Annotations: map[io.kubernetes.cri.container-name:nginx io.kubernetes.cri.container-type:container io.kubernetes.cri.image-name:docker.io/library/nginx:latest io.kubernetes.cri.sandbox-id:40dabdba54c52d5a3efc7325348e8c5ef06a4ad80a0d241a1152bd1399ce8d03 io.kubernetes.cri.sandbox-name:sandbox io.kubernetes.cri.sandbox-namespace:default io.kubernetes.cri.sandbox-uid:hdishd83djaidwnduwk28bcsb]"
Oct 26 21:44:40 10.37.7.188 containerd[2849150]: time="2023-10-26T21:44:40+08:00" level=info msg="[clearcfs] Spec.CgroupsPath: /k8s.io/1bb3edcd4eb4cc491bd216635069e2245883374932af4590d7badf82963e7808"
Oct 26 21:44:40 10.37.7.188 containerd[2849150]: time="2023-10-26T21:44:40+08:00" level=info msg="[clearcfs] Spec.Namespaces: map[ipc:/proc/2853941/ns/ipc mount: network:/proc/2853941/ns/net pid:/proc/2853941/ns/pid uts:/proc/2853941/ns/uts]"
Oct 26 21:44:40 10.37.7.188 containerd[2849150]: time="2023-10-26T21:44:40+08:00" level=info msg="[clearcfs] clearing cfs for 1bb3edcd4eb4cc491bd216635069e2245883374932af4590d7badf82963e7808"
```