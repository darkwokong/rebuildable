package report

import (
	"encoding/json"
	"io"

	"github.com/jayeshsarvaiya/rebuildable/internal/checks"
	"github.com/jayeshsarvaiya/rebuildable/internal/repo"
)

func RenderJSON(w io.Writer, r *repo.Repo, results []checks.Result, minimumScore *int) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(BuildDocument(r, results, minimumScore))
}
