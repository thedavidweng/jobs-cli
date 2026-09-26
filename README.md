# jobs-cli

A single-binary, agent-friendly CLI for discovering jobs across sources and
applying through application providers, with stable JSON envelopes and explicit
mutation safety.

`jobs-cli` keeps two concepts separate: the **Source** where a Job was discovered
(for example `indeed` or `linkedin`) and the **Application Provider** that actually
accepts the application (for example `greenhouse` or `lever`). Discovery and
application execution are routed independently.

- Human-readable output by default; `--json` emits exactly one JSON document on stdout.
- Diagnostics and errors go to stderr.
- `--read-only`, `--dry-run`, and `--confirm` gate every remote application mutation.
- No embedded AI, no MCP server, no browser automation, no CAPTCHA bypass.

## Install

```bash
# placeholder: Homebrew and GitHub Release download are wired up in a later ticket
go install github.com/thedavidweng/jobs-cli/cmd/jobs-cli@latest
```

## Commands

```
jobs-cli search        # partitioned multi-source discovery
jobs-cli show          # normalized Job detail (--resolve for an Application Target)
jobs-cli resolve       # Application Target only
jobs-cli apply inspect | prepare | submit
jobs-cli auth status | auth linkedin login | auth logout
jobs-cli sources status
jobs-cli doctor
jobs-cli version
jobs-cli completion
```

Full flag reference: [`COMMANDS.md`](COMMANDS.md). Machine-readable contract:
[`JSON_SCHEMA.md`](JSON_SCHEMA.md).

Product spec: [`docs/spec.md`](docs/spec.md). Domain glossary:
[`CONTEXT.md`](CONTEXT.md). Architecture: [`docs/adr/`](docs/adr/).

## Development

```bash
mise run check   # fmt + build + test + lint + conventions (the quality gate)
```

