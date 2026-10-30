# NRI Pod Sandbox Lifecycle Hooks

## Relationship to CRI API

This specification defines how NRI plugins interact with pod sandbox lifecycle events. The underlying pod sandbox operations are defined by the [Kubernetes CRI API](https://github.com/kubernetes/cri-api):

- **RunPodSandbox (CRI)**: Creates and starts a pod-level sandbox. Runtimes must ensure the sandbox is in the ready state on success.
- **StopPodSandbox (CRI)**: Stops any running process that is part of the sandbox and directs the runtime to reclaim certain pod resources (e.g. Network Namespace, CNI teardown, and image mounts). May be called multiple times, and is idempotent.
- **RemovePodSandbox (CRI)**: Removes the sandbox. If there are any running containers, they must be forcibly terminated and removed.

This NRI specification details when and under what conditions NRI plugins receive notifications for these events, ensuring plugins can reliably depend on consistent sandbox state across different runtime implementations.

## Overview

The pod sandbox lifecycle consists of three distinct phases, each with a corresponding NRI event that plugins can subscribe to:

1. **RunPodSandbox**: Fired during the runtime CRI RunPodSandbox execution, after the PodSandbox is created but before setting the pod to running and then replying success to CRI RunPodSandbox request.
2. **StopPodSandbox**: Fired when the runtime initiates CRI StopPodSandbox
3. **RemovePodSandbox**: Fired when the runtime performs CRI RemovePodSandbox

For each event, this specification defines:

- **Sandbox State Contract**: What sandbox infrastructure conditions runtimes MUST satisfy when firing the NRI event
- **Plugin Responsibilities and Capabilities**: What plugins can safely do in response to the event

## RunPodSandbox

**CRI Operation**: RunPodSandbox - Creates and starts a pod-level sandbox.

**NRI Event Timing**: The RunPodSandbox NRI event is fired after the runtime has successfully executed most of the CRI RunPodSandbox operation; NRI plugin execution is the final step before the sandbox reaches a "Ready" state. The Kubelet does not start workload containers until after the sandbox becomes "Ready".

### Sandbox State Contract

When the runtime fires the RunPodSandbox NRI event, it guarantees:

- The Pod-level cgroup hierarchy has been established
- The Sandbox namespaces (IPC, Network, UTS) are created and active
- The Sandbox will not be reused
- Network setup has been fully configured (network interfaces are up and assigned addressing)
- The pod IP address (if applicable) is assigned and available
- The "pause" container (if the runtime uses one) is running
- All prerequisite operations for workload container startup are complete, the pod is in the "unknown state" and will become "Ready" once the NRI event is processed. This guarantees the NRI plugin has a window to allocate resources for the pod before any workload containers are started.

### Plugin Responsibilities and Capabilities

Upon receiving the RunPodSandbox event, plugins can safely:

- Access the network namespace and inspect network configuration
- Perform network-level operations or monitoring
- Inject sandbox-level hardware configurations (e.g., RDMA, RoCEv2)
- Establish plugin-specific tracking or monitoring for the pod
- Store initial state or baseline metrics for later reference

Plugins should treat this as an initialization phase. The sandbox infrastructure will remain accessible throughout the pod's lifetime until StopPodSandbox is called.

## StopPodSandbox

**CRI Operation**: StopPodSandbox - Stops any running process that is part of the sandbox and reclaims certain pod resources (e.g. Network Namespace, CNI teardown, and image mounts).

**NRI Event Timing**: The StopPodSandbox NRI event is fired when the runtime initiates the CRI StopPodSandbox operation.

### Sandbox State Contract

When the runtime fires the StopPodSandbox NRI event, it guarantees:

- Workload containers within the sandbox are stopped or are stopping
- **CRITICAL**: The sandbox infrastructure still exists and remains fully accessible during this hook
- The pod resources allocated by the runtime; such as network namespace, CNI networks, and image mounts; are not unmounted or deleted until this hook completes
- The pod's cgroups remain accessible
- All pod-level resources remain stable until this hook returns

### Plugin Responsibilities and Capabilities

StopPodSandbox is the designated cleanup and observation phase for plugins. Upon receiving this event, plugins can:

- Access the pod's network namespace to read final telemetry or metrics
- Collect final state for observability or troubleshooting
- Detach hardware interfaces or reconfigure resources
- Clean up custom firewall configurations, routing rules, or other network-level state
- Perform graceful cleanup or resource release before sandbox teardown

**Important**: Plugin processing must complete within the configured request timeout. Do not assume sandbox access persists after this hook returns or times out.

## RemovePodSandbox

**CRI Operation**: RemovePodSandbox - Removes the sandbox and forcibly terminates any remaining containers.

**NRI Event Timing**: The RemovePodSandbox NRI event is fired when the runtime initiates the CRI RemovePodSandbox operation, just prior to removing the pod from the pod list.

### Sandbox State Contract

When the runtime fires the RemovePodSandbox NRI event:

- All workload containers have been removed
- The StopPodSandbox operation has completed
- Network setup teardown may be underway or complete
- The pod's namespaces (Network, IPC, UTS) may have already been deleted
- Pod-level cgroups may be destroyed
- Sandbox infrastructure access is **not guaranteed**

### Plugin Responsibilities and Capabilities

RemovePodSandbox is strictly for plugin-internal cleanup. Plugins MUST NOT attempt to access pod infrastructure (namespaces, cgroups, network configuration) during this hook, as their existence is not guaranteed.

Plugins receiving this event should only:

- Clean up plugin-internal memory caches or object tracking associated with the podSandboxID
- Remove host-level tracking files, database entries, or other locally stored pod references
- Release any plugin resources held for this specific pod
- Perform final accounting or bookkeeping

**Important**: This hook is informational only. Plugins should not assume any pod infrastructure exists. Only clean up information the plugin created or stored internally.

## Event Ordering and Guarantees

Runtimes MUST guarantee the following ordering:

1. **RunPodSandbox** NRI event fires after successful CRI RunPodSandbox execution, but before the pod is set to the "Ready" state.
2. **StopPodSandbox** NRI event fires during CRI StopPodSandbox execution, just prior to removing the runtime pod resources allocated by the runtime; such as network namespace, CNI networks, and image mounts
3. **RemovePodSandbox** NRI event fires during CRI RemovePodSandbox execution
4. These events MUST fire in strict order: RunPodSandbox → StopPodSandbox → RemovePodSandbox
5. No workload containers will be started until after RunPodSandbox hook completes
6. All workload containers will be stopped before StopPodSandbox hook is called
7. No network resource reclamation should occur during StopPodSandbox hook execution

See the [CRI API specification](https://github.com/kubernetes/cri-api) for details on each CRI operation.

## Plugin Implementation Guidance

### Subscribing to Events

Plugins subscribe to these events during the Configure phase by returning the appropriate event flags in the ConfigureResponse:

- `Event_RUN_POD_SANDBOX` (1 << 0) for RunPodSandbox
- `Event_STOP_POD_SANDBOX` (1 << 1) for StopPodSandbox
- `Event_REMOVE_POD_SANDBOX` (1 << 2) for RemovePodSandbox

These events are delivered to plugins using the RunPodSandbox, StopPodSandbox and RemovePodSandbox event handlers.

### Timeout Handling

All plugin processing must complete within the configured request timeout. A plugin timeout is treated as an error by the runtime:

- **RunPodSandbox**: Failure may result in pod creation failure
- **StopPodSandbox**: Non-blocking for subsequent operations; the plugin should not depend on completion of subsequent teardown
- **RemovePodSandbox**: Non-blocking; removal will proceed regardless of plugin timeout

### Error Handling

On the teardown path, plugin errors MUST NOT prevent the operation from proceeding. Runtimes MUST ensure that a failing plugin cannot block pod or container teardown:

- **RunPodSandbox errors**: A plugin error may prevent the pod from being created, depending on runtime policy. Plugins bear responsibility for errors they return at this phase.
- **StopPodSandbox errors**: A plugin error MUST NOT prevent the sandbox from being stopped. The runtime MUST proceed with teardown regardless of plugin failures.
- **RemovePodSandbox errors**: A plugin error MUST NOT prevent the sandbox from being removed. The runtime MUST proceed with removal regardless of plugin failures.


### Multi-Plugin Coordination

When multiple plugins are active:

- All RunPodSandbox hooks complete before first workload container starts
- Hooks execute in plugin index order; later plugins should not assume earlier plugins' modifications will persist
- RemovePodSandbox hooks are independent; plugins should not rely on side effects from other plugins
