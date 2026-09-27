# Changelog

## 1.0.0 (2026-09-27)

Initial release of `jobs-cli`: a single-binary, agent-friendly CLI for discovering jobs and applying through supported application providers. Discovery (the Source) and the application provider are routed independently.

### Discovery

- Indeed GraphQL search with market- and locale-aware queries and partitioned results
- LinkedIn Guest search and job detail without authentication
- Authenticated LinkedIn Voyager search and job detail behind an explicit flag

### Application providers

- ATS resolution for Greenhouse, Lever, Ashby, Workday, SmartRecruiters, iCIMS, and external application pages, with a redirect policy for shortened job links
- Greenhouse native apply: `apply inspect`, `apply prepare` with an artifact manifest, and `apply submit` with remote validation
- LinkedIn Voyager client with Easy Apply gating
- Read adapters for Lever, Ashby, Workday, SmartRecruiters, and iCIMS

### Auth and sessions

- Guided LinkedIn session import from local browser cookies (`auth linkedin login`), stored with `0600` permissions; `auth status` and `auth logout` complete the session lifecycle

### CLI contract

- Stable JSON envelopes, exit codes, and machine-readable diagnostics for every command
- Safety gates for remote mutations: `--read-only`, `--dry-run`, and `--confirm`
- Named profiles in `~/.jobs-cli/config.yaml` driving Indeed market, country, and locale defaults
- Command surface: `search`, `show`, `resolve`, `apply inspect | prepare | submit`, `auth status | linkedin login | logout`, `sources status`, `doctor`, `version`, `completion`

### Engineering

- The full quality gate (formatting, build, tests, lint, conventions) runs on Linux, macOS, and Windows
- Release automation with Release Please, GoReleaser archives and checksums, and a Homebrew tap cask

### Bug Fixes

* assert session file permissions only on Unix platforms ([7011e9d](https://github.com/thedavidweng/jobs-cli/commit/7011e9db4294eaa0168d3e2c00077db7021f6ec5))
