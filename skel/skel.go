package skel

import (
	"context"
	"encoding/json"
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
	var (
		enc = json.NewEncoder(os.Stdout)
		out interface{}
	)
	var request types.Request
	if err := json.NewDecoder(os.Stdin).Decode(&request); err != nil {
		return err
	}
	switch os.Args[1] {
	case "invoke":
		result, err := plugin.Invoke(ctx, &request)
		if err != nil {
			pe := types.NewPluginError(plugin.Type(), err)
			if err := enc.Encode(pe); err != nil {
				return err
			}
			return pe
		}
		out = result
	default:
		return errors.New("undefined arg")
	}
	return enc.Encode(out)
}
