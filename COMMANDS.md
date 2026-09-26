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
  - `--location <text>`: location filter.
  - `--radius <n>`: radius in the selected source's native units where supported
    (Indeed uses miles).
  - `--remote`: filter for remote work where supported.
  - `--sort <mode>`: source-supported sort (e.g. recency).
  - `--limit <n>` (default 25), `--offset <n>`: page sizing.
  - `--source <name>` (repeatable): `indeed`, `linkedin` in v1. Omitting it
    searches Indeed + LinkedIn (Guest) in parallel.
  - `--cursor <token>`: continue a single source's native pagination.
  - `--authenticated`: use authenticated LinkedIn Voyager instead of Guest.
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
  - `--authenticated`: for LinkedIn Easy Apply targets.
- `apply prepare <job-id>`: combine candidate data and answers, validate
  locally, and produce a versioned JSON Application Artifact. Nothing remote is
  submitted.
  - `--manifest <path>`: single JSON manifest embedding answers and attachment
    paths; or `--answers <path>` plus `--resume <path>` / `--cover-letter <path>`.
  - `--out <path>`: required in human mode (there is no default artifact path);
    in JSON mode the artifact may be emitted on stdout.
  - `--authenticated`: for LinkedIn Easy Apply targets.
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

## Utilities

- `version`: version information.
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
