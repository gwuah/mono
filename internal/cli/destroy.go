package cli

import (
	"github.com/gwuah/mono/internal/mono"
	"github.com/spf13/cobra"
)

func NewDestroyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "destroy [path]",
		Short: "Destroy an environment",
		Long:  "Stop containers, kill tmux session, and clean up data.\nIf no path is provided, uses CONDUCTOR_WORKSPACE_PATH.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			absPath, err := resolvePath(args)
			if err != nil {
				return err
			}

			return mono.Destroy(absPath)
		},
	}

	return cmd
}
