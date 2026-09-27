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
  - `--location <text>`: location filter. Indeed takes free text, and its ending
    can also choose the Indeed market (see `--country`). Authenticated
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
  - `--country <code>`: the Indeed market as an ISO country code (for example
    `US`, `CA`, `GB`; `UK` is accepted for `GB`). Indeed searches one country
    per request and there is no default market. The indeed partition takes its
    market from the first of these that names one:
    1. `--country`;
    2. the last comma-separated part of `--location`, when it is a US state,
       Canadian province, or country (code or name: `Austin, TX`,
       `Toronto, ON`, `London, United Kingdom`). Two-letter parts other than
       `US` and `UK` are read as states or provinces, so `San Francisco, CA` is
       California and `Berlin, DE` is Delaware (write `Berlin, Germany` or
       pass `--country DE`). A bare city names no market;
    3. `JOBS_COUNTRY`;
    4. the profile `country`.

    With none of these, the indeed partition fails with `MARKET_REQUIRED` and
    other sources still run. A country that is not an Indeed market fails the
    indeed partition with `INVALID_ARGUMENTS` listing the markets. The indeed
    partition reports the market it searched as `market` (`country`, `locale`,
    and `origin`: `flag`, `location`, `env`, or `profile`).
  - `--locale <tag>`: the Indeed locale as language-REGION (for example
    `fr-CA`). Without it, `JOBS_LOCALE` or the profile `locale` applies when
    its region is the market country; otherwise the locale is `en-<country>`.
    The locale never chooses the market.
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

- `auth status`: whether an authenticated LinkedIn session is available. A
  session file that exists but fails validation is reported as `present but
  invalid` with the reason, not as absent.
- `auth linkedin login`: guided session import — opens LinkedIn's login page,
  waits for you to finish, then imports Voyager cookies from your local browser
  cookie store into the per-profile session file.
  - `--browser chrome|safari|firefox`: override auto-detection
    (Chrome -> Safari -> Firefox). Session secrets are never printed.
  - `--no-open`: skip the browser attempt; print the login URL and wait for you
    to sign in manually, then continue with the import.
  - When no browser opener is available (`open`, `xdg-open`, `start`), the
    command prints the login URL and keeps waiting instead of failing.
- `auth linkedin import --from-json -`: advanced/headless session import — read
  the documented session JSON (see `docs/session-format.md`) from stdin
  (`-`), validate it, and store it for the active profile with the CLI's
  session-storage protections (0700 directory / 0600 file where the platform
  supports Unix permission bits). For machines with no local browser cookie
  store (servers, containers, agents); `auth linkedin login` remains the
  primary path for humans. Invalid input fails with a validation error and
  writes nothing.
- `auth logout`: remove the local LinkedIn session.
- `sources status`: discovery Sources and Application Providers with their
  capabilities (auth required, browser required, native submit) and
  verification posture (`VERIFIED WORKING`, `VERIFIED SOURCE IMPLEMENTATION`,
  `PARTIALLY VERIFIED`, `BROWSER REQUIRED`). Provider capability flags are
  derived from the same canonical table `resolve` and `apply inspect` use.
  `auth_required` means the CLI-native operation needs CLI-managed
  authentication (a CLI session such as the LinkedIn Session); a browser flow
  that uses a login jobs-cli does not manage reports `auth_required: false`.
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
    country: CA      # Indeed market fallback (see search --country)
    locale: en-CA    # used when its region is the market
    timeout: 30s
    read_only: false
    linkedin:
      session_file: /path/to/session.json
```

Environment overrides: `JOBS_PROFILE`, `JOBS_CONFIG`, `JOBS_TIMEOUT`,
`JOBS_READ_ONLY`, `JOBS_SOURCES`, `JOBS_COUNTRY`, `JOBS_LOCALE`.
`JOBS_COUNTRY` ranks below `search --country` and a market named by
`--location`, and above the profile `country`. `JOBS_LOCALE` and the profile
`locale` apply only when their region is the market country, and
`search --locale` overrides both. The market drives Indeed's `indeed-co` /
`indeed-locale` headers for search. `show`, `resolve`, and `apply` need no
market. An Indeed Job's `source_url` points at the job's own country site
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
| 2 | invalid arguments / validation (e.g. `MARKET_REQUIRED`) |
| 3 | authentication (e.g. `LINKEDIN_SESSION_REQUIRED`) |
| 4 | safety / read-only violation |
| 5 | network / rate limit / source unavailable |
| 6 | remote API, schema, resource, resolution, unsupported, browser-required |
| 7 | application validation (`APPLICATION_INCOMPLETE`, `ARTIFACT_STALE`) |
| 10 | confirmation required |

See `JSON_SCHEMA.md` for the machine-readable error codes and envelopes.
