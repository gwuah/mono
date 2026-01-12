package cli

import (
	"fmt"
	"path/filepath"

	"github.com/gwuah/mono/internal/mono"
	"github.com/spf13/cobra"
)

func NewRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run <path>",
		Short: "Execute run script in tmux",
		Long:  "Send the run script from mono.yml to the tmux session.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := args[0]

			absPath, err := filepath.Abs(path)
			if err != nil {
				return fmt.Errorf("invalid path: %w", err)
			}

			return mono.Run(absPath)
		},
	}

	return cmd
}
