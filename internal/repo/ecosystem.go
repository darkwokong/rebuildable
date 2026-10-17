package repo

import "sort"

type Ecosystem string

const (
	EcosystemGo     Ecosystem = "go"
	EcosystemNode   Ecosystem = "node"
	EcosystemPython Ecosystem = "python"
)

func (e Ecosystem) String() string {
	return string(e)
}

// DetectEcosystems intentionally looks at root manifests. Monorepo traversal
// is a later feature; this keeps Session 1 deterministic and easy to explain.
func DetectEcosystems(files []string) []Ecosystem {
	present := make(map[string]bool, len(files))
	for _, file := range files {
		present[file] = true
	}

	ecosystems := make([]Ecosystem, 0, 3)
	if present["go.mod"] {
		ecosystems = append(ecosystems, EcosystemGo)
	}
	if present["package.json"] {
		ecosystems = append(ecosystems, EcosystemNode)
	}
	if present["pyproject.toml"] || present["requirements.txt"] {
		ecosystems = append(ecosystems, EcosystemPython)
	}
	sort.Slice(ecosystems, func(i, j int) bool { return ecosystems[i] < ecosystems[j] })
	return ecosystems
}

func HasFile(files []string, target string) bool {
	for _, file := range files {
		if file == target {
			return true
		}
	}
	return false
}

func HasTrackedFile(r *Repo, target string) bool {
	return r.GitAvailable && HasFile(r.TrackedFiles, target)
}
