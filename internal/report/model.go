package report

import (
	"path/filepath"

	"github.com/jayeshsarvaiya/rebuildable/internal/checks"
	"github.com/jayeshsarvaiya/rebuildable/internal/repo"
)

type Document struct {
	Repository   string        `json:"repository"`
	Ecosystems   []string      `json:"ecosystems"`
	Score        int           `json:"score"`
	Total        int           `json:"total"`
	MinimumScore *int          `json:"minimum_score,omitempty"`
	Checks       []CheckReport `json:"checks"`
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
	return Document{
		Repository:   displayName(r),
		Ecosystems:   ecosystems,
		Score:        checks.Score(results),
		Total:        len(results),
		MinimumScore: minimumScore,
		Checks:       checkReports,
	}
}

func displayName(r *repo.Repo) string {
	if r.DisplayName != "" {
		return r.DisplayName
	}
	return filepath.ToSlash(r.Root)
}
