package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jayeshsarvaiya/rebuildable/internal/checks"
	"github.com/jayeshsarvaiya/rebuildable/internal/repo"
	"github.com/jayeshsarvaiya/rebuildable/internal/report"
	"github.com/spf13/cobra"
)

func main() {
	if err := newRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(exitCode(err))
	}
}

type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string { return e.err.Error() }
func (e *exitError) Unwrap() error { return e.err }

func exitCode(err error) int {
	var coded *exitError
	if errors.As(err, &coded) {
		return coded.code
	}
	return 2
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
	var asJSON bool
	var markdownPath string
	var minimumScore int
	command := &cobra.Command{
		Use:   "audit [path]",
		Short: "Audit a local repository or Git URL",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if minimumScore < -1 || minimumScore > 8 || (minimumScore == -1 && cmd.Flags().Changed("min-score")) {
				return fmt.Errorf("--min-score must be between 0 and 8")
			}
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
			var minimum *int
			if minimumScore >= 0 {
				minimum = &minimumScore
			}

			if markdownPath != "" {
				var markdownOutput strings.Builder
				if err := report.RenderMarkdown(&markdownOutput, loaded, results, minimum); err != nil {
					return fmt.Errorf("render Markdown report: %w", err)
				}
				if err := os.WriteFile(markdownPath, []byte(markdownOutput.String()), 0o644); err != nil {
					return fmt.Errorf("write Markdown report: %w", err)
				}
			}

			var renderErr error
			if asJSON {
				renderErr = report.RenderJSON(cmd.OutOrStdout(), loaded, results, minimum)
			} else {
				renderErr = report.RenderTextWithMinimum(cmd.OutOrStdout(), loaded, results, terminalColor(cmd.OutOrStdout()), minimum)
			}
			if renderErr != nil {
				return renderErr
			}
			if minimum != nil && checks.Score(results) < *minimum {
				return &exitError{code: 1, err: fmt.Errorf("score %d is below minimum %d", checks.Score(results), *minimum)}
			}
			return nil
		},
	}
	command.Flags().BoolVar(&asJSON, "json", false, "write a JSON report to stdout")
	command.Flags().StringVar(&markdownPath, "md", "", "write a Markdown report to this file")
	command.Flags().IntVar(&minimumScore, "min-score", -1, "fail with exit code 1 below this score (0-8)")
	return command
}

func terminalColor(w io.Writer) bool {
	file, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
