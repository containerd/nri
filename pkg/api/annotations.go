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

const (
	// RequiredPluginsAnnotation can be used to annotate pods with a list
	// of plugins which must process containers of the pod during creation.
	// If enabled, the default validator checks and rejects the creation
	// of containers if they fail the check.
	RequiredPluginsAnnotation = "required-plugins.nri.io"
)

// GetEffectiveAnnotation retrieves a custom annotation from a pod which
// applies to given container. The syntax allows both pod- and container-
// scoped annotations. Container-scoped annotations take precedence over
// pod-scoped ones. The key syntax defines the scope of the annotation.
//   - container-scope: <key>/container.<container-name>
//   - pod-scope: <key>/pod, or just <key>
func (x *PodSandbox) GetEffectiveAnnotation(key, container string) (string, bool) {
	if x == nil || len(x.Annotations) == 0 {
		return "", false
	}

	keys := []string{
		key + "/container." + container,
		key + "/pod",
		key,
	}

	for _, k := range keys {
		if v, ok := x.Annotations[k]; ok {
			return v, true
		}
	}

	return "", false
}
