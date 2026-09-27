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
