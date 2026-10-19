#!/usr/bin/env bash
set -euo pipefail

for archive in "${1:?provide the archive directory}"/jobs-cli_darwin_*.tar.gz; do
  directory="$(mktemp -d)"
  trap 'rm -rf "$directory"' EXIT
  tar -xzf "$archive" -C "$directory"
  architectures="$(lipo -archs "$directory/jobs-cli")"
  for architecture in $architectures; do
    echo "Verify notarized trust: $archive ($architecture)"
    codesign --verify --strict --check-notarization -R '=notarized' \
      --architecture "$architecture" --verbose=2 "$directory/jobs-cli"
  done
  rm -r "$directory"
  trap - EXIT
done
