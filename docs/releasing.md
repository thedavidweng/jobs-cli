# Releasing

This repository is configured for Release Please and GoReleaser. Adding the
workflow files does not publish a release. A release is created only after a
Release Please pull request is merged and its `v*` tag is processed.

## Repository secrets

Add these secrets in the `thedavidweng/jobs-cli` repository settings:

- `RELEASE_PLEASE_TOKEN`: a GitHub token that can create release pull requests,
  tags, and releases in this repository.
- `HOMEBREW_TAP_GITHUB_TOKEN`: a GitHub token with `contents:write` access to
  `thedavidweng/homebrew-tap`.

Use separate tokens when possible. Never commit or paste token values into the
repository, workflow files, issues, or logs.

## Release flow

1. Push conventional commits to `main`.
2. Release Please opens or updates the release pull request.
3. Merge that pull request when the release is approved.
4. The generated `v*` tag starts the Release workflow.
5. GoReleaser builds archives for macOS, Linux, and Windows, creates the GitHub
   Release, and updates `Casks/jobs-cli.rb` in `homebrew-tap`.
6. In parallel, the Release workflow requests the new tag from
   `proxy.golang.org`, so `go install …/cmd/jobs-cli@latest`, the command in
   the release notes and the README, resolves to it right away.

The version baseline lives in `.release-please-manifest.json`. Each Release
Please pull request bumps it together with `CHANGELOG.md`.

Install the released cask:

```shell
brew install --cask thedavidweng/tap/jobs-cli
```

To build the current `main` branch from source instead, use the formula:

```shell
brew install --HEAD thedavidweng/tap/jobs-cli
```

## Packaging validation

The Packaging workflow builds two unsigned snapshots from the same source,
toolchain and version metadata. The baseline omits `-trimpath` and replaces
Darwin slices with the Universal binary. The current configuration retains both
slices and Universal. Its seven archives include the unchanged
`jobs-cli_darwin_universal.tar.gz` filename, which remains the generated Cask's
download target. Native archive IDs and the Universal archive ID are separate
so GoReleaser cannot silently select a single-architecture download for the Cask.
The Cask also retains its existing Linux amd64 and arm64 downloads. Native
Darwin archives use a separate ID to exclude them from Cask selection.
GoReleaser 2.15.4 excludes non-replacing Universal binaries from Casks, even
with an explicit archive ID. A separate Darwin build feeds the replacing
Universal binary while the native build remains available for native archives.

The workflow reports raw executable and compressed archive bytes, plus MiB and
percentage deltas for identical targets. Additional Darwin downloads have no
archive baseline; comparing them with Universal would misstate the savings.
Every archive's checksum and embedded executable are verified. Six native
OS/architecture runners execute the exact archives, including Universal on both
Mac architectures, and check help, version metadata and an invalid command.
The `release-snapshot` artifact retains the archives and report for seven days.

To reproduce on a Mac with GoReleaser 2.15.4 and Python 3:

```shell
mkdir -p dist
ruby build/package-config.rb baseline dist/baseline.json
ruby build/package-config.rb current dist/current.json
HOMEBREW_TAP_GITHUB_TOKEN=unused goreleaser release --snapshot --clean --config dist/baseline.json
HOMEBREW_TAP_GITHUB_TOKEN=unused goreleaser release --snapshot --clean --config dist/current.json
python3 build/release_size.py dist/baseline dist/current dist/size-report.md
python3 build/package-smoke.py dist/current
```

UPX remains disabled. Its optional Linux amd64 experiment still needs native
startup, peak RSS, exit-status and compressed-size measurements before a
keep/reject decision. These snapshots do not prove Apple notarization, a
published release download, or an installed Homebrew Cask. Issue #26 remains
open until those acceptance requirements and the UPX decision have evidence.
