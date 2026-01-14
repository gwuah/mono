package cli

import (
	"github.com/gwuah/mono/internal/mono"
	"github.com/spf13/cobra"
)

func NewRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run [path]",
		Short: "Execute run script in tmux",
		Long:  "Send the run script from mono.yml to the tmux session.\nIf no path is provided, uses CONDUCTOR_WORKSPACE_PATH.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			absPath, err := resolvePath(args)
			if err != nil {
				return err
			}

			return mono.Run(absPath)
		},
	}

	return cmd
}
