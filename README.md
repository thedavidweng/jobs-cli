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

### Homebrew

There is not a tagged release yet. Install the current `main` build from the
Homebrew tap:

```shell
brew tap thedavidweng/tap
brew install --HEAD thedavidweng/tap/jobs-cli
```

After the first tagged release, GoReleaser will publish a stable Homebrew Cask:

```shell
brew install --cask thedavidweng/tap/jobs-cli
```

### Go

```shell
go install github.com/thedavidweng/jobs-cli/cmd/jobs-cli@main
```

### Build from source

```shell
git clone https://github.com/thedavidweng/jobs-cli.git
cd jobs-cli
mise install
mise run build
```

GitHub Release downloads are enabled by the release workflow after a version
tag is created. No release has been published by this repository yet.

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

Product specification: [`docs/spec.md`](docs/spec.md). Domain glossary:
[`CONTEXT.md`](CONTEXT.md). Architecture decisions: [`docs/adr/`](docs/adr/).

## Development

```shell
mise run check
```

The quality gate runs formatting, build, unit tests, lint, and repository
convention checks. Release configuration and required repository secrets are
documented in [`docs/releasing.md`](docs/releasing.md).