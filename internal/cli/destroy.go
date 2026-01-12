package cli

import (
	"fmt"
	"path/filepath"

	"github.com/gwuah/mono/internal/mono"
	"github.com/spf13/cobra"
)

func NewDestroyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "destroy <path>",
		Short: "Destroy an environment",
		Long:  "Stop containers, kill tmux session, and clean up data.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := args[0]

			absPath, err := filepath.Abs(path)
			if err != nil {
				return fmt.Errorf("invalid path: %w", err)
			}

			return mono.Destroy(absPath)
		},
	}

	return cmd
}
