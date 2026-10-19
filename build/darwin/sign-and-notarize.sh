#!/usr/bin/env bash
set -euo pipefail

artifact="${1:?provide the macOS artifact path}"
if [[ "${3:-darwin}" != darwin ]]; then
  exit 0
fi
if [[ "${2:?provide the GoReleaser snapshot flag}" == true && "${APPLE_SIGNING_ACCEPTANCE:-false}" != true ]]; then
  exit 0
fi
: "${APPLE_SIGNING_IDENTITY:?}" "${APPLE_SIGNING_KEYCHAIN:?}" "${APPLE_TEAM_ID:?}"
: "${APPLE_API_KEY_PATH:?}" "${APPLE_API_KEY_ID:?}" "${APPLE_API_ISSUER:?}"
case "$APPLE_SIGNING_IDENTITY" in
  "Developer ID Application: "*) ;;
  *) echo 'A Developer ID Application identity is required.' >&2; exit 1 ;;
esac

codesign --force --timestamp --options runtime --sign "$APPLE_SIGNING_IDENTITY" \
  --keychain "$APPLE_SIGNING_KEYCHAIN" "$artifact"
codesign --verify --strict --verbose=2 "$artifact"
details="$(codesign --display --verbose=4 "$artifact" 2>&1)"
grep -Fq 'Authority=Developer ID Application: ' <<< "$details"
grep -Fxq "TeamIdentifier=$APPLE_TEAM_ID" <<< "$details"
grep -q '^Timestamp=' <<< "$details"
grep -q 'flags=.*(runtime)' <<< "$details"

work_dir="$(mktemp -d)"
trap 'rm -rf "$work_dir"' EXIT
ditto -c -k --keepParent "$artifact" "$work_dir/release.zip"
submit_status=0
xcrun notarytool submit "$work_dir/release.zip" \
  --key "$APPLE_API_KEY_PATH" --key-id "$APPLE_API_KEY_ID" --issuer "$APPLE_API_ISSUER" \
  --wait --timeout 30m --output-format json > "$work_dir/result.json" || submit_status=$?
cat "$work_dir/result.json"
if [[ "$submit_status" != 0 ]] || ! jq -e '.status == "Accepted"' "$work_dir/result.json" > /dev/null; then
  submission_id="$(jq -r '.id // empty' "$work_dir/result.json")"
  if [[ -n "$submission_id" ]]; then
    xcrun notarytool log "$submission_id" \
      --key "$APPLE_API_KEY_PATH" --key-id "$APPLE_API_KEY_ID" --issuer "$APPLE_API_ISSUER"
  fi
  exit 1
fi
codesign --verify --strict --check-notarization -R '=notarized' --verbose=2 "$artifact"
