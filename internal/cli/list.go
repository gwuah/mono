package cli

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/anthropics/mono/internal/mono"
	"github.com/spf13/cobra"
)

func NewListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all environments",
		Long:  "Show all registered environments with their status.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			statuses, err := mono.List()
			if err != nil {
				return err
			}

			if len(statuses) == 0 {
				fmt.Println("No environments found.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tPATH\tSTATUS")

			for _, s := range statuses {
				status := getStatus(s.TmuxRunning, s.DockerRunning)

				path := s.Path
				if home, err := os.UserHomeDir(); err == nil {
					path = strings.Replace(path, home, "~", 1)
				}

				fmt.Fprintf(w, "%s\t%s\t%s\n", s.Name, path, status)
			}

			return w.Flush()
		},
	}

	return cmd
}

func getStatus(tmux, docker bool) string {
	if tmux && docker {
		return "running"
	}
	if tmux {
		return "running (no docker)"
	}
	if docker {
		return "docker only"
	}
	return "stopped"
}
