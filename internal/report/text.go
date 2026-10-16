package report

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/jayeshsarvaiya/rebuildable/internal/checks"
	"github.com/jayeshsarvaiya/rebuildable/internal/repo"
)

func RenderText(w io.Writer, r *repo.Repo, results []checks.Result, color bool) error {
	return RenderTextWithMinimum(w, r, results, color, nil)
}

func RenderTextWithMinimum(w io.Writer, r *repo.Repo, results []checks.Result, color bool, minimumScore *int) error {
	ecosystems := make([]string, 0, len(r.Ecosystems))
	for _, ecosystem := range r.Ecosystems {
		ecosystems = append(ecosystems, ecosystem.String())
	}
	label := strings.Join(ecosystems, ", ")
	if label == "" {
		label = "unknown"
	}

	name := r.DisplayName
	if name == "" {
		name = filepath.ToSlash(r.Root)
	}
	if _, err := fmt.Fprintf(w, "Rebuildable · %s · %s\n\n", name, label); err != nil {
		return err
	}

	for _, result := range results {
		icon := statusIcon(result.Status)
		icon = colorize(color, result.Status, icon)
		name := checks.NameForID(result.CheckID)
		message := firstMessage(result)
		if _, err := fmt.Fprintf(w, "  %s %-30s %s\n", icon, name, message); err != nil {
			return err
		}
		for _, finding := range result.Findings {
			if finding.Message == message && finding.File == "" && finding.Hint == "" {
				continue
			}
			location := ""
			if finding.File != "" {
				location = finding.File
				if finding.Line > 0 {
					location = fmt.Sprintf("%s:%d", location, finding.Line)
				}
			}
			if location != "" {
				if _, err := fmt.Fprintf(w, "      %s — %s\n", location, finding.Message); err != nil {
					return err
				}
			} else if finding.Message != message {
				if _, err := fmt.Fprintf(w, "      %s\n", finding.Message); err != nil {
					return err
				}
			}
			if finding.Hint != "" {
				if _, err := fmt.Fprintf(w, "      → %s\n", finding.Hint); err != nil {
					return err
				}
			}
		}
	}

	if _, err := fmt.Fprintf(w, "\n  Score %d/%d", checks.Score(results), len(results)); err != nil {
		return err
	}
	if minimumScore != nil {
		if _, err := fmt.Fprintf(w, " · minimum %d", *minimumScore); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	return nil
}

func statusIcon(status checks.Status) string {
	switch status {
	case checks.StatusPass:
		return "✓"
	case checks.StatusWarn:
		return "!"
	case checks.StatusFail:
		return "✗"
	case checks.StatusSkip:
		return "○"
	default:
		return "?"
	}
}

func colorize(color bool, status checks.Status, value string) string {
	if !color {
		return value
	}
	code := ""
	switch status {
	case checks.StatusPass:
		code = "32"
	case checks.StatusWarn:
		code = "33"
	case checks.StatusFail:
		code = "31"
	case checks.StatusSkip:
		code = "90"
	}
	if code == "" {
		return value
	}
	return "\x1b[" + code + "m" + value + "\x1b[0m"
}

func firstMessage(result checks.Result) string {
	if len(result.Findings) == 0 {
		return ""
	}
	return result.Findings[0].Message
}
