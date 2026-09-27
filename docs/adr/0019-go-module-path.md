# Go module path follows the GitHub repo

The Go module path matches the GitHub repo (`github.com/thedavidweng/jobs-cli`), with the main package under `cmd/jobs-cli`, following the qualtrics-cli / monarchmoney-cli layout. The installed binary is `jobs-cli` (see ADR-0020).

Supplement (v2): Go requires a module at major version 2 or higher to end its path in that major version, so from v2 the path is `github.com/thedavidweng/jobs-cli/v2`, still at the repo root. v2.0.0 shipped without the suffix: `go install` rejects that tag, and `@latest` on the unsuffixed path stops at v1.0.0. v2.0.1 is the first v2 release `go install` accepts. GoReleaser's version ldflags, the release-notes install line, and the Homebrew HEAD formula read the path from `go.mod`, so a later major release changes only `go.mod`, the imports, and the docs.
