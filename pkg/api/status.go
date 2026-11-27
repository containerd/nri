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

// Well-known reasons for a plugin status update. A plugin can tell these
// apart without parsing prose, but the reason is informational: it never
// asks the plugin to do anything.
const (
	// StatusReasonRegistered is the reason a plugin becomes live after it
	// has registered and synchronized.
	StatusReasonRegistered = "plugin registered and synchronized"
	// StatusReasonReplacedOther is the reason a plugin becomes live after
	// it has replaced another instance of its name.
	StatusReasonReplacedOther = "replaced another instance of this plugin"
	// StatusReasonReplaced is the reason a plugin is retired: another
	// instance of its name registered and took over.
	StatusReasonReplaced = "another instance of this plugin took over"
)
