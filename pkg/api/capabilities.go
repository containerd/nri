/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package api

import (
	"slices"
	"strings"

	"github.com/containerd/nri/pkg/version"
)

// CapabilityMask represents a set of Capabilities.
type CapabilityMask []uint64

var (
	numCapabilities = len(Capability_name)
	capabilityWords = (numCapabilities + 63) / 64
)

// ValidCapability returns true if a capability is known/valid.
func ValidCapability(c Capability) bool {
	_, ok := Capability_name[int32(c)]
	return ok
}

// NewCapabilityMask creates a CapabilityMask from a list of Capabilities.
func NewCapabilityMask(capabilities ...Capability) CapabilityMask {
	mask := make(CapabilityMask, capabilityWords)
	for _, c := range capabilities {
		if !ValidCapability(c) {
			continue
		}
		idx := c / 64
		bit := uint64(1) << uint(c%64)
		mask[idx] |= bit
	}
	return mask
}

// Clone returns a copy of the capability mask.
func (m CapabilityMask) Clone() CapabilityMask {
	return slices.Clone(m)
}

// Set returns a new mask with the given extra capabilities set.
func (m CapabilityMask) Set(capabilities ...Capability) CapabilityMask {
	n := make(CapabilityMask, capabilityWords)
	copy(n, m)
	for _, c := range capabilities {
		if !ValidCapability(c) {
			continue
		}
		idx := c / 64
		bit := uint64(1) << uint(c%64)
		m[idx] |= bit
	}
	return n
}

// Clear returns a new mask with the given extra capabilities cleared.
func (m CapabilityMask) Clear(capabilities ...Capability) CapabilityMask {
	n := make(CapabilityMask, capabilityWords)
	copy(n, m)
	for _, c := range capabilities {
		if !ValidCapability(c) {
			continue
		}
		idx := c / 64
		bit := uint64(1) << uint(c%64)
		m[idx] &^= bit
	}
	return n
}

// IsSet checks if the given capabilities are set in the mask.
func (m CapabilityMask) IsSet(capabilities ...Capability) bool {
	for _, c := range capabilities {
		if !ValidCapability(c) {
			continue
		}
		idx := c / 64
		bit := uint64(1) << uint(c%64)
		if (m[idx] & bit) == 0 {
			return false
		}
	}
	return true
}

// IsSubsetOf checks if the given mask is a subset of this mask.
func (m CapabilityMask) IsSubsetOf(o CapabilityMask) bool {
	for i, w := range m {
		if (o[i] & w) != w {
			return false
		}
	}
	return true
}

// Difference returns the capabilities in this mask that are not in the other mask.
func (m CapabilityMask) Difference(o CapabilityMask) CapabilityMask {
	n := make(CapabilityMask, capabilityWords)
	for i, w := range m {
		n[i] = w &^ o[i]
	}
	return n
}

// IsEmpty checks if the mask is empty.
func (m CapabilityMask) IsEmpty() bool {
	for _, w := range m {
		if w != 0 {
			return false
		}
	}
	return true
}

// String returns the capabilities present in the mask as a comma-separated string.
func (m CapabilityMask) String() string {
	str := strings.Builder{}
	for c := 0; c < numCapabilities; c++ {
		if m.IsSet(Capability(c)) {
			if str.Len() > 0 {
				str.WriteString(",")
			}
			str.WriteString(Capability_name[int32(c)])
		}
	}
	return str.String()
}

// InferRuntimeCapabilities tries to infer the capabilities supported by a runtime
// using the runtime's name and version.
func InferRuntimeCapabilities(nriVersion, runtime, runtimeVersion string) CapabilityMask {
	mask := NewCapabilityMask()

	nriVersion = version.StripGitSuffix(nriVersion)
	runtimeVersion = version.StripGitSuffix(runtimeVersion)

	for c, vMap := range capbilityVersionMap {
		nriV, ok := vMap["nri"]
		if !ok {
			continue
		}

		// too old NRI implies unsupported capability
		if version.Compare(nriVersion, nriV) < 0 {
			continue
		}

		// new enough NRI and no runtime exception implies supported capability
		rV, ok := vMap[runtime]
		if !ok {
			mask.Set(c)
			continue
		}

		// if runtime is new enough, capability is supported
		if version.Compare(runtimeVersion, rV) >= 0 {
			mask.Set(c)
			continue
		}
	}

	return mask
}

var (
	capbilityVersionMap = map[Capability]map[string]string{
		Capability_ADJUST_POSIX_RLIMITS: {
			"nri": "v0.4.0",
		},
		Capability_ADJUST_LINUX_PID_LIMIT: {
			"nri": "v0.7.0",
		},
		Capability_ADJUST_LINUX_OOM_SCORE: {
			"nri": "v0.7.0",
		},
		Capability_ADJUST_LINUX_NAMESPACES: {
			"nri": "v0.10.0",
		},
		Capability_ADJUST_LINUX_SECCOMP_POLICY: {
			"nri": "v0.10.0",
		},
		Capability_ADJUST_LINUX_IO_PRIORITY: {
			"nri": "v0.10.0",
		},
		Capability_ADJUST_LINUX_CONTAINER_ARGS: {
			"nri": "v0.10.0",
		},
		Capability_ADJUST_LINUX_SCHEDULING_POLICY: {
			"nri": "v0.11.0",
		},
		Capability_ADJUST_LINUX_NETWORK_DEVICES: {
			"nri": "v0.11.0",
		},
		Capability_ADJUST_LINUX_RDT_CLOS: {
			"nri": "v0.11.0",
		},
		// Capability_ADJUST_LINUX_SYSCTL: {
		// 	"nri": "v0.11.0", // did not work yet
		// },
		Capability_ADJUST_CDI_DEVICES: {
			"nri":        "v0.7.0",
			"containerd": "v2.1.0",
			"cri-o":      "v1.34.0",
		},
		Capability_INPUT_IP_ADDRESSES: {
			"containerd": "v2.1.0",
			"cri-o":      "v1.32.0",
		},
		Capability_INPUT_LINUX_SECURITY_PROFILE: {
			"containerd": "v2.2.1",
			"cri-o":      "v1.34.0",
		},
		Capability_INPUT_LINUX_IO_PRIORITY: {
			"containerd": "v2.2.1",
			"cri-o":      "v1.35.0",
		},
		Capability_INPUT_LINUX_SCHEDULING_POLICY: {
			"containerd": "v2.2.1",
			"cri-o":      "v1.35.0",
		},
		Capability_INPUT_LINUX_NETWORK_DEVICES: {
			"containerd": "v2.2.1",
			"cri-o":      "v1.35.0",
		},
		Capability_INPUT_LINUX_RDT_CLOS: {
			"containerd": "v2.2.1",
			"cri-o":      "v1.35.0",
		},
	}
)
