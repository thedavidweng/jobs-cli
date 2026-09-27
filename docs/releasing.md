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

## Versioning

Release Please uses the `always-bump-minor` versioning strategy, so every
release bumps the minor version, whatever its commits contain. A breaking
change (`feat!:` or a `BREAKING CHANGE:` footer) is still listed under
BREAKING CHANGES in the changelog, but it does not bump the major version, and
a fix-only release gets a new minor version rather than a patch. A major bump
would also rename the Go module (`/v2`, see ADR-0019). Switch back to the
`default` strategy when the project adopts strict semantic versioning.
