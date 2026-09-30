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

## Independent PR review

A second review against remote main found one Spec P1: adding answers from a newly discovered questionnaire invalidated the entire workflow state, losing completed-step and upload evidence. The regression reproduced on both browser fixtures. The fix retains unchanged candidate/document bindings, permits new answers only for inspected uncompleted steps, rejects changed or removed prior answers and additions to completed questions, and invalidates review when new answers are added. Both fixtures pass after the fix. Standards re-review: 0 remaining findings; Spec re-review: 0 remaining findings.

Discovery Search tests verify the maximum of four simultaneous detail requests and stopping later batches after a failed detail request. The callers already enforce four-job batches and immediately return on failure, so an initially added redundant semaphore was removed after tracing both callers. Linux CI now explicitly enables real-browser fixtures rather than leaving them skipped.

After these fixes, `mise run check` passed with JOBS_TEST_BROWSER set to installed Chromium, including the full test suite, build, formatting and lint (0 issues). `go test -race ./internal/boards` passed. No production application was performed. Remote PR gates are checked separately before merge.

Remote CodeQL identified potentially unsafe quoting in a browser navigation expression. Browser functions now receive artifact data, selectors and step names through Rod's structured arguments; dynamic JSON/string interpolation and the quoting helper were removed. The two fixtures include step titles containing single and double quotes and pass with the parameterized implementation. Standards review found no new issues. The alert is addressed in code rather than suppressed.

CodeRabbit review reproduced stale nested values when merging a manifest into existing candidate maps/slices; merged data is now decoded into a fresh Candidate. CLI regression checks replacement of address, work and education. LinkedIn profile network mapping is now case-insensitive, and the Greenhouse test transport counts rejected POST attempts. The spec's Out of Scope list now expressly permits the declared Workday browser workflow. The proposed discovery failure-scheduling issue does not reproduce through Search: both callers bound batches to four and return immediately on error; the integration check confirms no later batch is sent.

Kilo review led to a shared completeness guard requiring matching completion evidence for each prepared input step before review readiness or submission. Submitted-but-unrecognized pages remain uncertain even when they also require login or another user action. Review hashing uses rendered text rather than hidden script/style content. Both employer browser fixtures pass the skipped-step, post-attempt login and hidden-content regressions. Rod v0.116.2 supports the structured Eval arguments; no alternate wait implementation is needed. Workday already rejects v1 artifacts and checks live step fingerprints; unvalidated pending-start artifacts remain intentional, while final review and submit require validated requirements. Partial discovery results and nil-request defensive guards were not added.

Greenhouse now diagnoses invalid targets before checking credentials. Prepare explicitly rejects an embedded resume_json input member and directs users to --resume-json; the artifact member remains output data. Ambiguous attachment kinds within a manifest are rejected, while explicit resume/cover-letter flags retain documented override precedence. Targeted regressions reproduced all three findings before the fixes and pass afterward.
