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
