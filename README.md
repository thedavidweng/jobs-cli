# jobs-cli

`jobs-cli` is a single-binary, agent-friendly CLI for discovering jobs and
applying through supported application providers.

It keeps the **Source** where a job was discovered separate from the
**Application Provider** that accepts the application. Discovery and application
execution are routed independently.

## Highlights

- Indeed GraphQL and LinkedIn Guest discovery with partitioned results
- Authenticated LinkedIn Voyager search and detail behind an explicit flag
- ATS resolution for Greenhouse, Lever, Ashby, Workday, SmartRecruiters, iCIMS,
  and external application pages
- Greenhouse native application inspection, preparation, and submission
- Stable JSON envelopes, exit codes, and machine-readable diagnostics
- `--read-only`, `--dry-run`, and `--confirm` gates for remote mutations
- No embedded AI, MCP server, CAPTCHA bypass, or hidden browser automation

## Install

### AI agents

Paste this into an AI agent that has its own persistent computer. It installs
the [`jobs-cli` skill](skills/jobs-cli/SKILL.md) and the CLI:

```text
Read https://github.com/thedavidweng/jobs-cli/blob/main/skills/jobs-cli/SKILL.md and set up jobs-cli
```

### Homebrew

Install the stable release from the Homebrew tap (Cask, macOS and Linux):

```shell
brew tap thedavidweng/tap
brew install --cask thedavidweng/tap/jobs-cli
```

To build the current `main` branch from source instead:

```shell
brew install --HEAD thedavidweng/tap/jobs-cli
```

### Go

```shell
go install github.com/thedavidweng/jobs-cli/v2/cmd/jobs-cli@latest
```

This requires the Go toolchain version declared in [`go.mod`](go.mod) (currently
Go 1.27.1); install it from [go.dev/dl](https://go.dev/dl/) rather than a distro
package, which can lag behind.

`go install` builds carry reduced version metadata: the toolchain records the
module version but not full VCS provenance, so `jobs-cli version` may print
`(commit: none, date: unknown, built by: unknown)`. Release archives and the
Homebrew cask are built with complete commit, date, and builder metadata.

### Build from source

```shell
git clone https://github.com/thedavidweng/jobs-cli.git
cd jobs-cli
mise install
mise run build
```

Release archives and `checksums.txt` for each tagged version are published on
the [GitHub Releases](https://github.com/thedavidweng/jobs-cli/releases) page.

## Quickstart

```shell
# Discover jobs from Indeed and LinkedIn Guest
jobs-cli search --query "backend engineer" --location "Vancouver, BC" --json

# Inspect a normalized job and resolve its application provider
jobs-cli show indeed:<source-job-id> --resolve --json
jobs-cli resolve https://boards.greenhouse.io/acme/jobs/12345 --json

# Inspect and prepare an application without submitting it
jobs-cli apply inspect indeed:<source-job-id> --json
jobs-cli apply prepare indeed:<source-job-id> --manifest application.json --out artifact.json

# Check local configuration and provider capabilities
jobs-cli doctor
jobs-cli sources status
```

Indeed discovery needs no login: `search` reaches Indeed and LinkedIn Guest
anonymously, and `sources status` reports the Indeed discovery Source with
`auth_required=false`. The one optional login is `auth linkedin login`, which
imports a LinkedIn Session for authenticated LinkedIn (Voyager) search and
LinkedIn Easy Apply; `auth status` reports only that per-profile LinkedIn
Session. Applying through Indeed is a separate Application Provider path and
stays browser-required.

## Command surface

```text
jobs-cli search
jobs-cli show
jobs-cli resolve
jobs-cli apply inspect | prepare | submit
jobs-cli auth status | auth linkedin login | auth logout
jobs-cli sources status
jobs-cli doctor
jobs-cli version
jobs-cli completion
```

Full flags and behavior: [`COMMANDS.md`](COMMANDS.md). Machine-readable
contract: [`JSON_SCHEMA.md`](JSON_SCHEMA.md).

## Configuration

The config file is `<config-dir>/config.yaml`, where `<config-dir>` defaults to:

- macOS: `~/.jobs-cli`
- Linux: `${XDG_CONFIG_HOME:-~/.config}/jobs-cli`
- Windows: `%APPDATA%\jobs-cli`, falling back to `~/.jobs-cli` when `%APPDATA%`
  is unavailable

`JOBS_CONFIG_DIR` overrides the default directory on every platform.

It carries named profiles. Indeed searches one country at a time (its market),
and there is no default market. `search` takes the market from `--country`,
else from the end of `--location` (a US state, Canadian province, or country,
such as `"Austin, TX"` or `"London, United Kingdom"`), else from
`JOBS_COUNTRY`, else from the profile:

```yaml
profiles:
  default:
    country: CA
    locale: en-CA
```

If none of these names a market, the Indeed results fail with
`MARKET_REQUIRED` and the other sources still run.

Product specification: [`docs/spec.md`](docs/spec.md). Domain glossary:
[`CONTEXT.md`](CONTEXT.md). Architecture decisions: [`docs/adr/`](docs/adr/).

## Development

```shell
mise run check
```

The quality gate runs formatting, build, unit tests, and lint. Release
configuration and required repository secrets are documented in
[`docs/releasing.md`](docs/releasing.md).
