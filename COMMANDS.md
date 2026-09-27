# jobs-cli Command Reference

The installed binary is `jobs-cli`. A **Source** is where a Job was discovered
(`indeed`, `linkedin`). An **Application Provider** is the system that accepts
the application (`greenhouse`, `linkedin`, `lever`, `ashby`, `workday`,
`smartrecruiters`, `icims`, `indeed`, `external`). They are never the same field.

## Global Flags

Available on every command:

- `--json`: emit exactly one machine-readable JSON document on stdout.
- `--pretty`: pretty-print JSON output.
- `--full`: print full payloads instead of summaries (e.g. complete job
  descriptions, provider diagnostics).
- `--read-only`: block every remote write for the invocation.
- `--dry-run`: preview a mutation as `planned_mutations` without executing it.
- `--confirm`: explicitly authorize a mutation.
- `--timeout <duration>`: per-invocation timeout (default 30s).
- `--profile <name>`: use a named profile (default `default`).
- `--config <path>`: config file (default is `<config-dir>/config.yaml`).

## Job Discovery

- `search`: search across discovery sources.
  - `--query/-q <text>`: keywords (required unless `--location` is given).
  - `--location <text>`: location filter. Indeed takes free text. Authenticated
    LinkedIn (Voyager) requires a geo URN (`urn:li:fsd_geo:<id>`, the `geoId`
    from a LinkedIn jobs search URL) or a built-in known location (for example
    `Vancouver, BC` or `remote`); anything else fails the linkedin partition
    with `INVALID_ARGUMENTS` listing the known values.
  - `--radius <n>`: radius in the selected source's native units where supported
    (Indeed uses miles).
  - `--remote`: filter for remote work where supported.
  - `--sort <mode>`: source-supported sort (e.g. recency).
  - `--limit <n>` (default 25), `--offset <n>`: page sizing.
  - `--source <name>` (repeatable): `indeed`, `linkedin` in v1. Omitting it
    searches Indeed + LinkedIn (Guest) in parallel.
  - `--cursor <token>`: continue a single source's native pagination.
  - `--authenticated`: use authenticated LinkedIn Voyager instead of Guest for
    the linkedin partition. Only LinkedIn has an authenticated variant, so
    Indeed (and any other source) keeps running its own implementation; the
    flag never turns a source off.
  - `--country <code>`, `--locale <tag>`: Indeed market override (for example
    `CA` / `en-CA`). Indeed sends these as `indeed-co` / `indeed-locale` and
    builds market source URLs (`ca.indeed.com`). Precedence: flag over
    environment (`JOBS_COUNTRY` / `JOBS_LOCALE`) over profile. Defaults are
    `US` / `en-<country>`.
  - Results are **Search Partitions**: one per source, each with its own jobs,
    pagination, or structured error. Partial success (>= 1 source ok) exits 0
    with `meta.warnings`; exit non-zero only when every requested source fails.
  - Continuation after the first page requires exactly one `--source` plus that
    source's `--cursor`/`--offset`. There is no opaque global page token.
- `show <job-id>`: full details for a Job, addressable by its compound Job ID
  (`<source>:<source-job-id>`, e.g. `indeed:517ca3fd71acddc9`).
  - `--resolve`: attach a best-effort Application Target.
  - `--authenticated`: use the authenticated LinkedIn surface for `linkedin:` IDs.
    LinkedIn Guest supports search only; `show linkedin:<id>` requires this flag.

## Applications

- `resolve <job-id | url>`: return **only** an Application Target: canonical
  URL, provider, provider identifiers where safely extractable, and
  capabilities (`inspect`, `prepare`, `native_submit`, `browser_required`,
  `auth_required`). Redirects are followed at most 5 times; HTTPS->HTTP
  downgrades and private/link-local/metadata targets are rejected.
- `apply inspect <job-id>`: fetch application requirements (fields, questions
  with ids/labels/types/options/required status, resume/cover-letter support)
  without mutation.
  - `--authenticated`: for LinkedIn Easy Apply targets. A LinkedIn job whose
    native Easy Apply is unavailable is reported as `BROWSER_REQUIRED` with the
    LinkedIn job URL; it never becomes a linkedin application artifact.
- `apply prepare <job-id>`: combine candidate data and answers, validate
  locally, and produce a versioned JSON Application Artifact. Nothing remote is
  submitted.
  - `--manifest <path>`: single JSON manifest embedding answers and attachment
    paths; or `--answers <path>` plus `--resume <path>` / `--cover-letter <path>`.
  - `--out <path>`: required in human mode (there is no default artifact path);
    in JSON mode the artifact may be emitted on stdout.
  - `--authenticated`: for LinkedIn Easy Apply targets. Non-Easy-Apply jobs
    fail with `BROWSER_REQUIRED` before an artifact is produced.
- `apply submit`: execute the supported remote mutation.
  - `--artifact <path | ->`: the Application Artifact from `apply prepare`
    (file, or `-` for stdin).
  - Requires `--confirm`. Before submission the remote requirements are
    re-inspected and compared to the artifact's schema fingerprint; a mismatch
    fails with `ARTIFACT_STALE` (prepare again; there is no `--force` bypass).
  - Application POSTs are never automatically retried.

## Account & Sources

- `auth status`: whether an authenticated LinkedIn session is available.
- `auth linkedin login`: guided session import — opens LinkedIn's login page,
  waits for you to finish, then imports Voyager cookies from your local browser
  cookie store into the per-profile session file.
  - `--browser chrome|safari|firefox`: override auto-detection
    (Chrome -> Safari -> Firefox). Session secrets are never printed.
- `auth logout`: remove the local LinkedIn session.
- `sources status`: discovery Sources and Application Providers with their
  capabilities (auth required, browser required, native submit) and
  verification posture (`VERIFIED WORKING`, `VERIFIED SOURCE IMPLEMENTATION`,
  `PARTIALLY VERIFIED`, `BROWSER REQUIRED`).
- `doctor [--connect]`: local installation, config, and session checks;
  `--connect` adds optional connectivity checks for Indeed, LinkedIn, and
  Greenhouse.

Notes:

- Indeed discovery needs no login: `search` and `show` query Indeed anonymously,
  and `sources status` reports the Indeed discovery Source with
  `auth_required=false`. Applying through the Indeed Application Provider is a
  separate, browser-required path.
- `auth status` reports the local LinkedIn Session for the active profile only
  (`profile` + `linkedin`); it is not a per-provider auth posture report. The
  capability matrix, including which Sources and Application Providers need
  auth, is `sources status` (see ADR-0012).

## Configuration

Config file: `<config-dir>/config.yaml` (default dir `~/.jobs-cli` on macOS,
`%APPDATA%\jobs-cli` on Windows, `${XDG_CONFIG_HOME:-~/.config}/jobs-cli` on
Linux; `JOBS_CONFIG_DIR` overrides it), with named profiles:

```yaml
default_profile: default
profiles:
  default:
    sources:
      - indeed
      - linkedin
    country: CA      # Indeed market; default US
    locale: en-CA    # Indeed locale; defaults to en-<country>
    timeout: 30s
    read_only: false
    linkedin:
      session_file: /path/to/session.json
```

Environment overrides: `JOBS_PROFILE`, `JOBS_CONFIG`, `JOBS_TIMEOUT`,
`JOBS_READ_ONLY`, `JOBS_SOURCES`, `JOBS_COUNTRY`, `JOBS_LOCALE`. The search
`--country` / `--locale` flags override both. The market drives Indeed's
`indeed-co` / `indeed-locale` headers and the canonical source URL host
(`www.indeed.com` for US, `ca.indeed.com` for CA, `uk.indeed.com` for GB).

## Utilities

- `version`: version information — module version, commit, date, builder, Go
  version, and build info. Release archives and the Homebrew cask are built with
  complete commit, date, and builder metadata; `go install` builds may report
  `(commit: none, date: unknown, built by: unknown)` because the toolchain does
  not record full VCS provenance for module installs.
- `completion [bash|zsh|fish|powershell]`: shell completions.

## Safety Model

Remote application submission is a mutation:

- `--read-only` blocks all remote writes.
- `--dry-run` emits `planned_mutations` and never submits.
- Submission without `--confirm` fails with `CONFIRMATION_REQUIRED`.
- No provider bypasses the shared mutation gate.

## Exit Codes

| Code | Category |
|------|----------|
| 1 | internal error / not implemented |
| 2 | invalid arguments / validation |
| 3 | authentication (e.g. `LINKEDIN_SESSION_REQUIRED`) |
| 4 | safety / read-only violation |
| 5 | network / rate limit / source unavailable |
| 6 | remote API, schema, resource, resolution, unsupported, browser-required |
| 7 | application validation (`APPLICATION_INCOMPLETE`, `ARTIFACT_STALE`) |
| 10 | confirmation required |

See `JSON_SCHEMA.md` for the machine-readable error codes and envelopes.
