# Changelog

## [2.0.1](https://github.com/thedavidweng/jobs-cli/compare/v2.0.0...v2.0.1) (2026-09-27)


### Bug Fixes

* use the /v2 module path so go install finds v2 releases ([a31a20e](https://github.com/thedavidweng/jobs-cli/commit/a31a20e02f5b3ab81abfd862d211aadb2a7405b6))

## [2.0.0](https://github.com/thedavidweng/jobs-cli/compare/v1.0.0...v2.0.0) (2026-09-27)


### ⚠ BREAKING CHANGES

* Indeed search has no default market. Pass --country, end --location with a US state, Canadian province, or country, or set JOBS_COUNTRY or the profile country; otherwise the indeed partition fails with MARKET_REQUIRED. A market named by --location now outranks JOBS_COUNTRY and the profile, --locale no longer chooses the country, and configured locales apply only when their region is the market.

### Features

* add headless LinkedIn session import with shared validation ([6c5ed81](https://github.com/thedavidweng/jobs-cli/commit/6c5ed81a5be0b28447023747bf2b26431209c08c)), closes [#17](https://github.com/thedavidweng/jobs-cli/issues/17)
* resolve the Indeed market per search with no US default ([5f49e5c](https://github.com/thedavidweng/jobs-cli/commit/5f49e5cbce5f9c718573d4db2800249c481ce2f4))


### Bug Fixes

* address two-axis code review findings from issues [#16](https://github.com/thedavidweng/jobs-cli/issues/16)-[#18](https://github.com/thedavidweng/jobs-cli/issues/18) ([ca3ce44](https://github.com/thedavidweng/jobs-cli/commit/ca3ce44c1477579cac33b0a56c174e28b0c64fed))
* derive sources status provider capabilities from the resolver table ([6286472](https://github.com/thedavidweng/jobs-cli/commit/62864729827bbd9e144567e20baad457abb419a4)), closes [#18](https://github.com/thedavidweng/jobs-cli/issues/18)


### Documentation

* add agent setup skill and one-line README prompt ([a1171fa](https://github.com/thedavidweng/jobs-cli/commit/a1171fad75df2d8e8c0bba265ed7beb55810f42b))
* add Indeed no-login note to sources status and fix spec config paths ([2bfcba4](https://github.com/thedavidweng/jobs-cli/commit/2bfcba4c2985aecf5bfb357755213c1e4af260be))
* backfill the 1.0.0 changelog with the initial release content ([3806021](https://github.com/thedavidweng/jobs-cli/commit/38060214cb097b6c52544aa6533031ee6e670288))
* clarify Go prerequisite, Indeed discovery auth, version provenance, and config paths ([2b6169d](https://github.com/thedavidweng/jobs-cli/commit/2b6169dffab9743cdb10cb67129613bd7d09dbc8)), closes [#16](https://github.com/thedavidweng/jobs-cli/issues/16)
* update install instructions for the v1.0.0 release ([336978c](https://github.com/thedavidweng/jobs-cli/commit/336978c4335b34c92878fa6b03e04eaf20292f83))

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

* assert session file permissions only on Unix platforms ([e7596f8](https://github.com/thedavidweng/jobs-cli/commit/e7596f86aa05b40f1ff8896a46b095fba3cc8985))
