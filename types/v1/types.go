package v1

import "encoding/json"

// Plugin type and configuration
type Plugin struct {
	// Type of plugin
	Type string `json:"type"`
	// Conf for the specific plugin
	Conf json.RawMessage `json:"conf,omitempty"`
}

// ConfigList for the global configuration of NRI
//
// Normally located at /etc/nri/conf.json
type ConfigList struct {
	// Verion of the list
	Version string `json:"version"`
	// Plugins
	Plugins []*Plugin `json:"plugins"`
}

// Spec for the container being processed
type Spec struct {
	// Resources struct from the OCI specification
	//
	// Can be WindowsResources or LinuxResources
	Resources json.RawMessage `json:"resources"`
	// Namespaces for the container
	Namespaces map[string]string `json:"namespaces,omitempty"`
	// CgroupsPath for the container
	CgroupsPath string `json:"cgroupsPath,omitempty"`
	// Annotations passed down to the OCI runtime specification
	Annotations map[string]string `json:"annotations,omitempty"`
}

// State of the request
type State string

const (
	// Create the initial resource for the container
	Create State = "create"
	// Delete any resources for the container
	Delete State = "delete"
	// Update the resources for the container
	Update State = "update"
	// Pause action of the container
	Pause State = "pause"
	// Resume action for the container
	Resume State = "resume"
)

// Request for a plugin invocation
type Request struct {
	// Conf specific for the plugin
	Conf json.RawMessage `json:"conf"`

	// Version of the plugin
	Version string `json:"version"`
	// State action for the request
	State State `json:"state"`
	// ID for the container
	ID string `json:"id"`
	// SandboxID for the sandbox that the request belongs to
	//
	// If ID and SandboxID are the same, this is a request for the sandbox
	// SandboxID is empty for a non sandboxed container
	SandboxID string `json:"sandboxID"`
	// Pid of the container
	//
	// -1 if there is no pid
	Pid int `json:"pid,omitempty"`
	// Spec generated from the OCI runtime specification
	Spec *Spec `json:"spec"`
}

// IsSandbox returns true if the request is for a sandbox
func (r *Request) IsSandbox() bool {
	return r.ID == r.SandboxID
}

// NewResult returns a result from the original request
func (r *Request) NewResult() *Result {
	return &Result{
		ID:          r.ID,
		State:       r.State,
		Pid:         r.Pid,
		Version:     r.Version,
		CgroupsPath: r.Spec.CgroupsPath,
	}
}

// Result of the plugin invocation
type Result struct {
	// Version of the plugin
	Version string `json:"version"`
	// State of the invocation
	State State `json:"state"`
	// ID of the container
	ID string `json:"id"`
	// Pid of the container
	Pid int `json:"pid"`
	// CgroupsPath of the container
	CgroupsPath string `json:"cgroupsPath"`
}
