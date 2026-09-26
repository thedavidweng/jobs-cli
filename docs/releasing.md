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

The version baseline lives in `.release-please-manifest.json` (currently
`0.0.0`; no release has shipped yet). Each Release Please pull request bumps it
together with `CHANGELOG.md`.

Until the first stable release is published, install the source-built formula:

```shell
brew install --HEAD thedavidweng/tap/jobs-cli
```

After a stable release, install the generated cask:

```shell
brew install --cask thedavidweng/tap/jobs-cli
```
