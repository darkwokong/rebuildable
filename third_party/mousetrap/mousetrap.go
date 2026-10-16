package mousetrap

// StartedByExplorer is retained for Cobra's Windows build. Rebuildable is
// distributed and tested on Linux, where this helper is never called.
func StartedByExplorer() bool { return false }
