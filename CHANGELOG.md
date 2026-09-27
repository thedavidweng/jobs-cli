# Changelog

## [2.0.0](https://github.com/thedavidweng/jobs-cli/compare/v1.0.0...v2.0.0) (2026-09-27)


### ⚠ BREAKING CHANGES

* Indeed search has no default market. Pass --country, end --location with a US state, Canadian province, or country, or set JOBS_COUNTRY or the profile country; otherwise the indeed partition fails with MARKET_REQUIRED. A market named by --location now outranks JOBS_COUNTRY and the profile, --locale no longer chooses the country, and configured locales apply only when their region is the market.

### Features

* add headless LinkedIn session import with shared validation ([94875a7](https://github.com/thedavidweng/jobs-cli/commit/94875a753c3683249917451e1f70fafc68915c3b)), closes [#17](https://github.com/thedavidweng/jobs-cli/issues/17)
* resolve the Indeed market per search with no US default ([11957c4](https://github.com/thedavidweng/jobs-cli/commit/11957c4193bd492e001f49fda135529671e93c84))


### Bug Fixes

* address two-axis code review findings from issues [#16](https://github.com/thedavidweng/jobs-cli/issues/16)-[#18](https://github.com/thedavidweng/jobs-cli/issues/18) ([9ea4345](https://github.com/thedavidweng/jobs-cli/commit/9ea4345b4fd8f9853a995f6128f31951bb3606b1))
* derive sources status provider capabilities from the resolver table ([44e5b4f](https://github.com/thedavidweng/jobs-cli/commit/44e5b4f212be369ab60af17759a0852a8f3b25a0)), closes [#18](https://github.com/thedavidweng/jobs-cli/issues/18)


### Documentation

* add agent setup skill and one-line README prompt ([837f6e0](https://github.com/thedavidweng/jobs-cli/commit/837f6e048e936f77180837b5a1ac551265eb7c6b))
* add Indeed no-login note to sources status and fix spec config paths ([c06a63e](https://github.com/thedavidweng/jobs-cli/commit/c06a63efc3d727b85445b28fc5a906f9f953188c))
* backfill the 1.0.0 changelog with the initial release content ([b28a1d1](https://github.com/thedavidweng/jobs-cli/commit/b28a1d1ba9ae8a8115ccf502b4bf5121e67a5c38))
* clarify Go prerequisite, Indeed discovery auth, version provenance, and config paths ([5eab9dc](https://github.com/thedavidweng/jobs-cli/commit/5eab9dcf6bf45ecc0fe755698e29ce9b5e83d321)), closes [#16](https://github.com/thedavidweng/jobs-cli/issues/16)
* update install instructions for the v1.0.0 release ([dad9f7d](https://github.com/thedavidweng/jobs-cli/commit/dad9f7da1e3619d93ecae64eb4ff1ada8cf04c4d))

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

* assert session file permissions only on Unix platforms ([7011e9d](https://github.com/thedavidweng/jobs-cli/commit/7011e9db4294eaa0168d3e2c00077db7021f6ec5))
