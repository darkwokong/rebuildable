package verify

import (
	"fmt"
	"io"
)

func RenderText(w io.Writer, report Report) error {
	if _, err := fmt.Fprintf(w, "Rebuildable verify · image %s\n\n", report.ImageTag); err != nil {
		return err
	}
	for _, stage := range report.Stages {
		icon := "✗"
		if stage.Passed {
			icon = "✓"
		}
		message := "passed"
		if !stage.Passed {
			message = stage.Error
		}
		if _, err := fmt.Fprintf(w, "  %s %-8s %s (%.1fs)\n", icon, stage.Name, message, stage.Duration.Seconds()); err != nil {
			return err
		}
		if stage.LogPath != "" {
			if _, err := fmt.Fprintf(w, "      log: %s\n", stage.LogPath); err != nil {
				return err
			}
		}
	}
	if report.LogDir != "" {
		_, err := fmt.Fprintf(w, "\n  Logs: %s\n", report.LogDir)
		return err
	}
	return nil
}
