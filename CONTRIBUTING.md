# Contributing

## Getting Started

```bash
git clone https://github.com/thedavidweng/jobs-cli.git
cd jobs-cli
mise install  # install tools pinned in mise.toml
go build ./cmd/jobs-cli
go test ./...
```

Requires Go 1.27.1 or later.

## Development

```bash
# Run all gates: fmt, build, test, lint
mise run check

# Individual tasks
mise run build
mise run test
mise run lint
mise run fmt
```

CI runs `mise run check` on Linux, macOS, and Windows, then fails if the run left uncommitted changes. Run `mise run fmt` before committing.

## Project Structure

```
cmd/jobs-cli/         Entry point
internal/
  cli/                Cobra commands
  config/             Config file, profiles, and LinkedIn session files
  domain/             Domain types: jobs, applications, artifacts, capabilities
  errors/             Error codes and categories
  output/             JSON envelope and rendering
  safety/             Mutation gates: --read-only, --dry-run, --confirm
  indeed/             Indeed source
  linkedinguest/      LinkedIn guest source
  linkedin/           Signed-in LinkedIn: Voyager API, session, Easy Apply
  market/             Indeed market (country and locale) resolution
  resolver/           Application provider resolution
  registry/           Source and application provider registry
  greenhouse/         Greenhouse application provider (native submit)
  lever/, ashby/, workday/, smartrecruiters/, icims/
                      Other application providers
  artifact/           Application artifact files
  cookieimport/       Browser cookie import for LinkedIn login
  httpclient/         HTTP client with retries
  testutil/           Test helpers
  version/            Build version metadata
tests/e2e/            End-to-end tests against the built binary
skills/jobs-cli/      Agent skill
docs/                 Spec, ADRs, and reference docs
```

See [docs/spec.md](docs/spec.md) for the product spec, [CONTEXT.md](CONTEXT.md) for domain vocabulary, and [docs/adr/](docs/adr/) for architecture decisions.

## Code Style

- Standard Go formatting (`gofumpt -extra`)
- Table-driven tests
- No CGO dependencies
- Errors carry a stable code from `internal/errors`, so JSON output and exit codes stay predictable
- Remote mutations go through the shared gate in `internal/safety`
- Session secrets never printed in any output mode

## Testing

```bash
# All tests
go test ./...

# Specific package
go test ./internal/cli/

# With race detection (the Release workflow runs this before every release)
go test -race ./...

# With coverage
go test -coverprofile=coverage.out ./...
```

`internal/testutil/` provides fake HTTP transports and a binary builder for the end-to-end tests in `tests/e2e/`.

## Pull Requests

1. Fork the repository
2. Create a feature branch
3. Write tests for new functionality
4. Ensure `mise run check` passes
5. Use [Conventional Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`, `docs:`, and so on), because Release Please builds the changelog and the next version from them
6. Submit a pull request

## License

By contributing, you agree that your contributions will be licensed under the [Apache License 2.0](LICENSE).
