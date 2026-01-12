package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gwuah/mono/internal/mono"
	"github.com/spf13/cobra"
)

func NewInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init <path>",
		Short: "Initialize a new environment",
		Long:  "Register an environment, start containers, and create a tmux session.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := args[0]

			absPath, err := filepath.Abs(path)
			if err != nil {
				return fmt.Errorf("invalid path: %w", err)
			}

			if _, err := os.Stat(absPath); err != nil {
				return fmt.Errorf("path does not exist: %s", absPath)
			}

			return mono.Init(absPath)
		},
	}

	return cmd
}
