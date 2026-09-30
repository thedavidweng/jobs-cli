<h1 align="center">jobs-cli</h1>

<p align="center">
  Agent-friendly CLI for finding jobs on Indeed and LinkedIn and applying from the terminal.
</p>

<p align="center">
  <a href="https://github.com/thedavidweng/jobs-cli/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/thedavidweng/jobs-cli/ci.yml?branch=main&style=flat-square&label=ci" alt="CI"></a>
  <a href="https://github.com/thedavidweng/jobs-cli/releases"><img src="https://img.shields.io/github/v/release/thedavidweng/jobs-cli?style=flat-square" alt="Release"></a>
  <a href="https://github.com/thedavidweng/jobs-cli/blob/main/LICENSE"><img src="https://img.shields.io/github/license/thedavidweng/jobs-cli?style=flat-square" alt="License"></a>
  <img src="https://img.shields.io/badge/go-%3E%3D1.27-blue?style=flat-square" alt="Go">
</p>

`jobs-cli` is a single-binary CLI that searches Indeed, LinkedIn, YZi Talent, CivicInfo BC, and TransLink, finds the application provider behind each job (Greenhouse, Lever, Workday, and others), and submits your application from the terminal when the provider supports it. When it doesn't, you get the link to apply in a browser.

## Highlights

- Agent-first: stable JSON output with `--json`, distinct stdout/stderr, and predictable exit and error codes
- Safety-first: `--read-only`, `--dry-run`, and `--confirm` gates, so no application is sent without `--confirm`
- Search without logging in: Indeed and LinkedIn in one command, and if one source fails, the other still returns results
- Finds where to apply: Greenhouse, Lever, Ashby, Workday, SmartRecruiters, iCIMS, or the employer's own site
- Applies from the terminal: inspect the form, prepare an application you can review, fill Workday in an explicitly connected browser, stop at review, then confirm submission; Greenhouse API submission requires an employer key
- Single binary for discovery and API operations; Workday requires a running Chrome-compatible browser with an explicit local CDP connection

## Why

Job boards and application forms are built for browsers, and every employer's form is different. `jobs-cli` treats where you find a job and where you apply as separate steps, so scripts and agents can search, find the right application provider, and apply through one stable set of commands.

## Quickstart

### Install

Run the following on macOS or Linux:

```shell
curl -fsSL https://raw.githubusercontent.com/thedavidweng/jobs-cli/main/install.sh | sh
```

Run the following on Windows:

```shell
powershell -ExecutionPolicy ByPass -c "irm https://raw.githubusercontent.com/thedavidweng/jobs-cli/main/install.ps1 | iex"
```

On macOS and Linux, the installer detects Homebrew and uses it when available (recommended for easy upgrades). Otherwise it downloads the binary to `~/.local/bin`. On Windows, it installs to `%LOCALAPPDATA%\jobs-cli\bin`.

**AI agents:** paste this into an agent that has its own persistent computer. It installs the [`jobs-cli` skill](skills/jobs-cli/SKILL.md) and the CLI.

```text
Read https://github.com/thedavidweng/jobs-cli/blob/main/skills/jobs-cli/SKILL.md and set up jobs-cli
```

<details>
<summary>Other installation methods</summary>

**Homebrew Cask (macOS/Linux):**

```shell
brew install --cask thedavidweng/tap/jobs-cli
```

**Go:** requires the Go version in [`go.mod`](go.mod). Distro packages often lag behind, so get Go from [go.dev/dl](https://go.dev/dl/).

```shell
go install github.com/thedavidweng/jobs-cli/v2/cmd/jobs-cli@latest
```

`go install` builds may show `commit: none` in `jobs-cli version`. Release archives and the Homebrew cask carry full build metadata.

**Manual download:** grab the archive for your platform, including Windows, from the [latest GitHub Release](https://github.com/thedavidweng/jobs-cli/releases/latest), check it against `checksums.txt`, extract it, and place the `jobs-cli` binary on your `PATH`.

**Build from source:**

```shell
# Latest main through Homebrew
brew install --HEAD thedavidweng/tap/jobs-cli

# Or build it yourself
git clone https://github.com/thedavidweng/jobs-cli.git
cd jobs-cli
go build ./cmd/jobs-cli
```

</details>

### Set up

```shell
jobs-cli doctor
```

`doctor` checks your installation, config, and session, and `jobs-cli sources status` shows what each source and application provider supports. Search works without an account. To turn on the signed-in LinkedIn features (job details, signed-in search, and Easy Apply forms), log in once:

```shell
jobs-cli auth linkedin login
```

Then try it:

```shell
# Search Indeed and LinkedIn
jobs-cli search --query "backend engineer" --location "Vancouver, BC"

# Show a job from the results and find its application provider
jobs-cli show indeed:<id> --resolve

# Read the application form, then prepare an application (nothing is sent)
jobs-cli apply inspect indeed:<id>
jobs-cli apply prepare indeed:<id> --manifest application.json --out artifact.json

# Submit it (this sends a real application)
jobs-cli apply submit --artifact artifact.json --confirm
```

Indeed searches one country at a time. End `--location` with a US state, Canadian province, or country, pass `--country`, or set a default in the [configuration](COMMANDS.md#configuration). Otherwise the Indeed results fail with `MARKET_REQUIRED`. Add `--json` to any command for machine-readable output.

Indeed discovery requires no login. Indeed-hosted applications resolve with `browser_required=true` and an application URL for you to continue manually in a browser; native submission reports `BROWSER_REQUIRED`. [Indeed's Job Seeker Terms](https://www.indeed.com/legal?co=US&hl=en) prohibit automating Indeed Apply outside its official vendors and tooling. Any proposal to automate this flow must first establish a new architecture decision (ADR) and review the terms; browser extensions, native-messaging hosts, and DOM auto-submit are outside the current product scope.

### Portfolio and BC sources

```shell
jobs-cli search --source yzi --remote
jobs-cli search --source civicinfo --query "analyst"
jobs-cli search --source translink --query "engineer"
jobs-cli show translink:<id> --resolve
```

YZi and TransLink discovery have been checked live. CivicInfo's parser is tested,
but direct requests currently receive Cloudflare HTTP 403. YZi and PeopleSoft
applications return browser routing; CivicInfo jobs route to employer application
sites. See the [board reference](COMMANDS.md#portfolio-and-bc-boards) for dates,
filters, pagination, and verification limits.

### Uninstall

```shell
# Homebrew Cask
brew uninstall --cask thedavidweng/tap/jobs-cli

# install.sh
curl -fsSL https://raw.githubusercontent.com/thedavidweng/jobs-cli/main/install.sh | sh -s uninstall

# install.ps1 (Windows)
powershell -ExecutionPolicy ByPass -c "& ([scriptblock]::Create((irm https://raw.githubusercontent.com/thedavidweng/jobs-cli/main/install.ps1))) uninstall"

# Go
rm "$(go env GOPATH)/bin/jobs-cli"
```

Remove local data if desired: `rm -rf ~/.jobs-cli` (on Linux, `~/.config/jobs-cli`, and on Windows, `%APPDATA%\jobs-cli`). This deletes your config and LinkedIn session.

## Documentation

**Reference:**

- [Command Reference](COMMANDS.md): every command, flag, and exit code
- [Configuration](COMMANDS.md#configuration): config file, profiles, and `JOBS_*` environment variables
- [JSON Schema](JSON_SCHEMA.md): success and error envelopes, common objects, and error codes
- [Session Format](docs/session-format.md): the LinkedIn session file
- [Agent Skill](skills/jobs-cli/SKILL.md): setup and usage guide for AI agents
- [Releasing](docs/releasing.md): how Release Please and GoReleaser publish a release
- [Contributing](CONTRIBUTING.md): development setup and contribution guidelines

**Explanation:**

- [Safety Model](COMMANDS.md#safety-model): how `--read-only`, `--dry-run`, and `--confirm` gate every submission
- [Specification](docs/spec.md): product scope and behavior
- [Glossary](CONTEXT.md): domain terms such as Source and Application Provider
- [Architecture Decisions](docs/adr/): the ADRs behind the design

## Disclaimer

`jobs-cli` is an independent, community-maintained project and is **not affiliated with, sponsored by, or endorsed by LinkedIn, Indeed, or any application provider it supports.**

## Infrastructure

- **CI/CD:** GitHub Actions + [mise](https://mise.jdx.dev/)
- **Releases:** [Release Please](https://github.com/googleapis/release-please) + [GoReleaser](https://goreleaser.com/), published to GitHub Releases and the [Homebrew tap](https://github.com/thedavidweng/homebrew-tap)

## License

[Apache License 2.0](LICENSE)
