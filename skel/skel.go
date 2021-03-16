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

package skel

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	types "github.com/containerd/nri/types/v1"
	"github.com/pkg/errors"
)

// Plugin for modifications of resources
type Plugin interface {
	// Type or plugin name
	Type() string
	// Invoke the plugin
	Invoke(context.Context, *types.Request) (*types.Result, error)
}

// Run the plugin from a main() function
func Run(ctx context.Context, plugin Plugin) error {
	enc := json.NewEncoder(os.Stdout)
	var request types.Request
	if err := json.NewDecoder(os.Stdin).Decode(&request); err != nil {
		return err
	}
	switch os.Args[1] {
	case "invoke":
		result, err := plugin.Invoke(ctx, &request)
		if err != nil {
			// if the plugin sets ErrorMessage we ignore it
			result = request.NewResult(plugin.Type())
			result.Error = err.Error()
		}
		if err := enc.Encode(result); err != nil {
			return errors.Wrap(err, "unable to encode plugin error to stdout")
		}
	default:
		result := request.NewResult(plugin.Type())
		result.Error = fmt.Sprintf("invalid arg %s", os.Args[1])
		if err := enc.Encode(result); err != nil {
			return errors.Wrap(err, "unable to encode invalid parameter error to stdout")
		}
	}
	return nil
}
