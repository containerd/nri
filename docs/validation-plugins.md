# NRI Validation Plugins

NRI plugins operate as trusted extensions of the container runtime, granting them significant privileges to alter container specs. While this extensibility is powerful, it also comes with the risk that a plugin could inadvertently or maliciously weaken a container's isolation or security posture.

To mitigate this risk, NRI offers a mechanism for fine-grained control over what changes plugins are allowed to make. This is achieved through **Validation Plugins**.

## Requirements and Versioning

Validation plugins were introduced in **NRI v0.10.0**. To use them, you must be using a container runtime that supports this version of NRI:

- **containerd**: v2.2.0 or newer.
- **CRI-O**: v1.34.0 or newer.

Older versions of these runtimes do not support the validation phase and will ignore any validation-related configuration or plugin handlers.

## What are Validation Plugins?

Validation plugins are NRI plugins that approve or deny adjustments made by other (mutating) plugins. They act as a second phase in the NRI container adjustment process:

1.  **Mutation Phase**: One or more plugins propose changes to a container (e.g., adding mounts, changing resource limits, injecting OCI hooks).
2.  **Validation Phase**: Validating plugins are invoked with the combined set of all proposed changes. They receive information about which plugin requested which change.

### Timing and Context

Validation plugins are invoked during container creation after all mutations from other plugins have been collected, but before the container is actually created by the runtime.

The validation request (`ValidateContainerAdjustment`) includes:
- The **Pod** and **Container** in their pristine state (as received from the orchestrator/runtime).
- The **Pending Adjustments** (mounts, environment variables, resource limits, etc.).
- **Ownership Information**: A map indicating which plugin is responsible for each specific change.
- **Consulted Plugins**: A list of all plugins that have already processed the container.

### Transactional Nature

Validation has transactional semantics. If **any** validating plugin rejects an adjustment:
- Container creation **fails**.
- None of the other proposed adjustments are applied.
- The runtime returns an error to the orchestrator (e.g., Kubelet).

This "fail-closed" behavior ensures that no unauthorized or inconsistent state can be reached if a validator is active and disapproves of the changes.

## How to Deploy Validation Plugins

Validation plugins are regular NRI plugins that implement the `ValidateContainerAdjustment` handler. They can be implemented using the [NRI plugin stub library](../pkg/stub).

### 1. Implement the Interface

Your plugin must implement the `ValidateContainerAdjustmentInterface` defined in the stub library:

```go
type ValidateContainerAdjustmentInterface interface {
    // ValidateContainerAdjustment validates the container adjustment.
    ValidateContainerAdjustment(context.Context, *api.ValidateContainerAdjustmentRequest) error
}
```

If your plugin returns an error from this function, the adjustment will be rejected.

### 2. Subscribe to the Event

When using the stub library, the library automatically subscribes your plugin to the `VALIDATE_CONTAINER_ADJUSTMENT` event if you implement the required method.

### 3. Initialize and Run

Initialize your plugin and start the stub:

```go
type myValidator struct {}

func (v *myValidator) ValidateContainerAdjustment(ctx context.Context, req *api.ValidateContainerAdjustmentRequest) error {
    // Perform validation logic here
    return nil
}

func main() {
    v := &myValidator{}
    s, err := stub.New(v, stub.WithPluginName("my-validator"), stub.WithPluginIdx("10"))
    if err != nil {
        panic(err)
    }
    s.Run(context.Background())
}
```

Validation plugins are usually given a high index (e.g., `90` or `99`) to ensure they run after most mutating plugins, although the NRI framework ensures they are called in a separate phase regardless of their index relative to mutating plugins.

## Kinds of Validation

Validation plugins can implement several types of checks, depending on the goals of the cluster administrator:

### 1. Functional Validators
Functional validators ensure that the final combined state of the container is consistent and valid. They perform "sanity checks" such as:
- Are resource limits (CPU/memory) within an acceptable range?
- Are the requested mounts compatible with the container's environment?
- Are the OCI hooks valid and present on the host?

### 2. Security Validators
Security validators focus on **who** is making the changes. They use the `owners` information passed in the `ValidateContainerAdjustmentRequest` to:
- Prevent untrusted or third-party plugins from modifying sensitive fields like Linux namespaces, seccomp profiles, or sysctls.
- Enforce an "approved list" of plugins allowed to modify specific container properties.
- Reject any modification to security-sensitive settings that should be controlled only by the orchestrator (e.g., K8s).

### 3. Mandatory Plugin Validators
Mandatory plugin validators ensure that specific plugins have processed the container. They can:
- Check the `plugins` list in the validation request to ensure that all "mandatory" plugins are present.
- Verify that a mandatory plugin "owns" specific fields it is supposed to control (e.g., ensuring a resource-management plugin successfully set the CPU quota).
- Fail-closed if a required plugin is missing or was not consulted.

## Default Validation Plugin

NRI includes a **Default Validation Plugin** as part of the runtime adaptation. It provides a set of configurable, minimal validation rules designed to protect the security and isolation of containers.

### Validation Rules

The default validator implements the following rules, which can be enabled or disabled via configuration:

1.  **Reject OCI Hook Injection**: Rejects any adjustment that attempts to inject OCI hooks into a container.
2.  **Reject Linux Seccomp Policy Adjustment**: Rejects adjustments that try to set or override the Linux seccomp policy. This can be fine-tuned based on the type of profile (runtime default, unconfined, or custom).
3.  **Reject Linux Namespace Adjustment**: Rejects any adjustment that alters the Linux namespaces of a container.
4.  **Reject Linux Sysctl Adjustment**: Rejects any adjustment that alters Linux sysctl settings.
5.  **Verify Required Plugins**: Ensures that a specific set of "mandatory" plugins have processed the container.

### Configuration

The default validator is configured via the `DefaultValidatorConfig` structure. In most container runtimes (like `containerd`), this configuration is provided as part of the NRI section in the runtime configuration file (e.g., `config.toml`).

| Field | Type | Description |
| :--- | :--- | :--- |
| `enable` | bool | Enable the default validator plugin. |
| `reject_oci_hook_adjustment` | bool | Fail if OCI hooks are adjusted. |
| `reject_runtime_default_seccomp_adjustment` | bool | Fail if a runtime default seccomp policy is adjusted. |
| `reject_unconfined_seccomp_adjustment` | bool | Fail if an unconfined seccomp policy is adjusted. |
| `reject_custom_seccomp_adjustment` | bool | Fail if a custom seccomp policy (LOCALHOST) is adjusted. |
| `reject_namespace_adjustment` | bool | Fail if any plugin adjusts Linux namespaces. |
| `reject_sysctl_adjustment` | bool | Fail if any plugin adjusts sysctls. |
| `required_plugins` | list(string) | List of globally required plugins. |
| `tolerate_missing_plugins_annotation` | string | Optional annotation key to tolerate missing required plugins. |

Example configuration in `containerd`'s `config.toml`:

```toml
[plugins."io.containerd.nri.v1.runtime"]
  [plugins."io.containerd.nri.v1.runtime".default_validator]
    enable = true
    reject_oci_hook_adjustment = true
    reject_namespace_adjustment = true
    required_plugins = ["my-auth-plugin"]
```

## Dealing with Required Plugins

The default validator allows you to specify a list of "required" plugins. These plugins **must** have processed a container during its creation phase, or the default validator will reject the container.

There are two ways to specify required plugins:

### 1. Globally Required (Always)
Globally required plugins are specified in the runtime configuration (e.g., `containerd`'s `config.toml`) using the `required_plugins` field. These plugins are mandatory for **every** container created on the node.

**Warning**: If you configure globally required plugins, you must ensure that your **static pods** (like those for the required plugins themselves) are either already running or are annotated to tolerate missing plugins. Otherwise, they may fail to start, leading to a deadlock.

### 2. Workload-Specific Required (via Annotations)
You can also specify required plugins for a particular pod or container using the `required-plugins.noderesource.dev` annotation.

The value of the annotation should be a JSON-encoded list of plugin names.

#### Pod-Scoped
To require a plugin for all containers in a pod:
```yaml
annotations:
  required-plugins.noderesource.dev: '["my-special-plugin"]'
```
Alternatively, you can use the `/pod` suffix:
```yaml
annotations:
  required-plugins.noderesource.dev/pod: '["my-special-plugin"]'
```

#### Container-Scoped
To require a plugin for a specific container within a pod:
```yaml
annotations:
  required-plugins.noderesource.dev/container.my-container: '["my-container-plugin"]'
```

### Tolerating Missing Plugins
If the `tolerate_missing_plugins_annotation` is configured in the default validator, you can use that annotation to allow a pod or container to skip the check for required plugins.

For example, if `tolerate_missing_plugins_annotation` is set to `tolerate-missing-nri-plugins.noderesource.dev`:

```yaml
annotations:
  tolerate-missing-nri-plugins.noderesource.dev: "true"
```

This is particularly useful for:
- **Static pods** that are part of the NRI infrastructure itself.
- **Sidecars** that might be created before the required NRI plugin is fully initialized.
