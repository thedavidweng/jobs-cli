# Issue #23 implementation report

## Scope

Implemented JSON Resume v1.0.0 validation and mapping, artifact v2 with complete source preservation and explicit candidate overrides, document metadata and SHA-256 validation, and the confirmed Workday browser workflow through inspect, prepare, fill, review, and submit. Repeated work and education rows, displayed select/radio labels, changing questionnaire requirements, cross-step documents, actual accepted values, and submission receipts are covered by the CLI integration tests.

Workday execution attaches to an explicitly supplied existing browser tab. Read-only, dry-run, confirmation, target/input freshness, persisted mutation state, and one-attempt submission boundaries apply. Login, verification, CAPTCHA, assessments, ambiguous rows, and unsupported custom widgets pause for user action. Greenhouse POST requires explicit board-scoped credentials; default capability claims no longer promise anonymous submission. Indeed prepares a local artifact for official handoff without claiming requirements validation.

Prior discovery work was preserved separately in `4ea0cc5`. Implementation commits are `9666235`, `ddc077d`, and `84fa168`. Review comparison: `git diff 4ea0cc5...HEAD`.

## Verification

Final checks passed on 2026-09-30:

- `go test ./...`, with `JOBS_TEST_BROWSER` pointing to the installed Chromium executable; both employer fixtures ran through the public CLI harness.
- `go build ./...`, `mise run fmt`, and `mise run lint` (0 issues).
- Workday browser integration tests under the race detector passed after replacing the browser transport with Rod.

The two local employer fixtures exercise different field wording and requirements, repeated rows, removed controls across page transitions, renewed preparation, uploaded file bytes, radio labels and checked state, explicit application start, review changes, successful receipt evidence, and uncertain submission without a second click. These are real-browser fixture checks, not production employer submissions. See [browser testing](../browser-testing.md) for the executable setup, command examples, and supported control boundary.

Public read-only checks reached CIBC and BMO Workday career shells and reviewed official Greenhouse authentication documentation. No production account registration, saved application start, document upload, or submission occurred. Authenticated production Workday end-to-end behavior remains unverified; tenant-specific custom widgets require further observed evidence before support is claimed.

## Standards

The initial review identified four actionable findings: radio review-state bypass, unverified accepted field values, lost explicit empty candidate values, and state persistence after browser mutation. Each was corrected and covered at the existing integration seam. Shared browser bindings replaced duplicate mappings. A browser teardown race found during verification was removed by using Rod and explicitly closing its CDP connection.

Standards re-review reported no remaining actionable findings. No worst remaining issue.

## Spec

The initial review identified four actionable findings: requiring attachment controls on every step, incomplete review requirement hashing, inconsistent select identity precedence, and missing radio options/state. All were corrected. Re-review found one additional radio label/value mismatch; a failing fixture reproduced it, and `84fa168` resolves a unique exact displayed label to the actual value and verifies checked state. The final re-review also confirmed explicit start requires confirmed fill and renewed preparation when the new form appears.

Spec final re-review reported no remaining actionable findings. No worst remaining issue. Review confirmation is limited to code and local fixtures.

Final findings: Standards 0; Spec 0. No merge or release performed.
