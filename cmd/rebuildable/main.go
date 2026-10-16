package main

import (
	"fmt"
	"io"
	"os"

	"github.com/jayeshsarvaiya/rebuildable/internal/checks"
	"github.com/jayeshsarvaiya/rebuildable/internal/repo"
	"github.com/jayeshsarvaiya/rebuildable/internal/report"
	"github.com/spf13/cobra"
)

func main() {
	if err := newRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}
}

func newRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:           "rebuildable",
		Short:         "Audit a repository for reproducibility",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(newAuditCommand())
	return root
}

func newAuditCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "audit [path]",
		Short: "Audit a local repository",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "."
			if len(args) == 1 {
				path = args[0]
			}

			loaded, cleanup, err := repo.Load(cmd.Context(), path)
			if err != nil {
				return err
			}
			defer cleanup()

			results := checks.RunAll(cmd.Context(), loaded)
			return report.RenderText(cmd.OutOrStdout(), loaded, results, terminalColor(cmd.OutOrStdout()))
		},
	}
}

func terminalColor(w io.Writer) bool {
	file, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
