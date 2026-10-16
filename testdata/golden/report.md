# Rebuildable report

- Repository: `fixture`
- Ecosystems: `go`
- Score: **1/3**

| # | Check | Status | Fixable |
| ---: | --- | --- | :---: |
| 1 | Git repo with commits | **pass** | no |
|  |  |  | `.git` — 3 commit(s) found |
| 2 | Dependency lockfile | **fail** | no |
|  |  |  | `go.mod` — go.sum is missing |
|  |  |  | Hint: run: go mod tidy |
| 3 | Runtime version pinned | **skip** | no |
|  |  |  | not implemented in Session 1 |
