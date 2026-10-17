package report

import (
	"path/filepath"

	"github.com/jayeshsarvaiya/rebuildable/internal/checks"
	"github.com/jayeshsarvaiya/rebuildable/internal/repo"
)

type Document struct {
	SchemaVersion int           `json:"schema_version"`
	Mode          string        `json:"mode"`
	Repository    string        `json:"repository"`
	Ecosystems    []string      `json:"ecosystems"`
	Score         int           `json:"score"`
	Total         int           `json:"total"`
	Passed        bool          `json:"passed"`
	MinimumScore  *int          `json:"minimum_score,omitempty"`
	MinimumMet    *bool         `json:"minimum_met,omitempty"`
	Checks        []CheckReport `json:"checks"`
}

type CheckReport struct {
	ID       string           `json:"id"`
	Name     string           `json:"name"`
	Status   string           `json:"status"`
	Findings []checks.Finding `json:"findings"`
	Fixable  bool             `json:"fixable"`
}

func BuildDocument(r *repo.Repo, results []checks.Result, minimumScore *int) Document {
	ecosystems := make([]string, 0, len(r.Ecosystems))
	for _, ecosystem := range r.Ecosystems {
		ecosystems = append(ecosystems, ecosystem.String())
	}
	checkReports := make([]CheckReport, 0, len(results))
	for _, result := range results {
		findings := result.Findings
		if findings == nil {
			findings = []checks.Finding{}
		}
		checkReports = append(checkReports, CheckReport{
			ID:       result.CheckID,
			Name:     checks.NameForID(result.CheckID),
			Status:   result.Status.String(),
			Findings: findings,
			Fixable:  result.Fixable,
		})
	}
	document := Document{
		SchemaVersion: 1,
		Mode:          "static",
		Repository:    displayName(r),
		Ecosystems:    ecosystems,
		Score:         checks.Score(results),
		Total:         len(results),
		Passed:        checks.Score(results) == len(results),
		MinimumScore:  minimumScore,
		Checks:        checkReports,
	}
	if minimumScore != nil {
		met := document.Score >= *minimumScore
		document.MinimumMet = &met
	}
	return document
}

func displayName(r *repo.Repo) string {
	if r.DisplayName != "" {
		return r.DisplayName
	}
	return filepath.ToSlash(r.Root)
}
