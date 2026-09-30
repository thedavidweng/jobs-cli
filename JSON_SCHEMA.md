# JSON Output Schema

All commands support `--json` and wrap their results in a standard envelope.
stdout carries exactly one JSON document; diagnostics and human-readable errors
go to stderr.

## Success Envelope

```json
{
  "ok": true,
  "data": { ... },
  "meta": {
    "command": "search",
    "profile": "default",
    "duration_ms": 123,
    "schema_version": "2026-09-26",
    "request_id": "uuid",
    "warnings": []
  }
}
```

## Error Envelope

```json
{
  "ok": false,
  "error": {
    "code": "ERROR_CODE",
    "message": "Human readable message",
    "category": "validation",
    "retryable": false,
  },
  "meta": { ... }
}
```

## Common Objects

### Job

```json
{
  "id": "indeed:517ca3fd71acddc9",
  "source": "indeed",
  "source_job_id": "517ca3fd71acddc9",
  "title": "Backend Engineer",
  "employer": "Acme",
  "location": "Vancouver, BC",
  "workplace": "remote",
  "remote": true,
  "description": "...",
  "posted_date": "2026-09-20",
  "closing_date": "2026-10-20",
  "employment_type": "Full-Time",
  "employment_length": "Regular",
  "level": "Senior",
  "tags": ["AI"],
  "compensation": {
    "amounts": [{ "kind": "range", "min": 100000, "max": 140000 }],
    "currency": "CAD",
    "interval": "year",
    "summary": "CA$100k-140k a year"
  },
  "source_url": "https://...",
  "application_url": "https://...",
  "application": { }
}
```

`closing_date`, `employment_type`, `employment_length`, `level`, and `tags`
are optional. CivicInfo expiry and TransLink closing dates use `closing_date`;
fixed dates are `YYYY-MM-DD`, and open-ended deadlines retain their source text
(for example `Open until filled`). YZi search returns level and tags. TransLink
compensation retains the source salary text in `compensation.summary`.

`application` (an Application Target) is attached only when explicitly resolved.
Provider-specific raw payloads never appear in the top-level contract; with
`--full`, `diagnostics.source_payload` carries them for debugging. An Indeed
Job's `source_url` points at the job's own country site (for example
`ca.indeed.com` for a job in Canada).

### Search Result

`search` data partitions results by Source; there is no cross-source merge or
ranking:

```json
{
  "partitions": [
    {
      "source": "indeed",
      "market": { "country": "CA", "locale": "en-CA", "origin": "location" },
      "jobs": [ { "id": "indeed:...", "title": "...", "...": "..." } ],
      "pagination": {
        "limit": 25,
        "offset": 0,
        "total": 120,
        "has_more": true,
        "next_cursor": "cursor-or-empty",
        "native": { "kind": "cursor", "cursor": "..." }
      }
    },
    {
      "source": "linkedin",
      "jobs": [ ],
      "error": { "code": "SOURCE_UNAVAILABLE", "message": "...", "category": "network", "retryable": true }
    }
  ]
}
```

The indeed partition carries `market`, the country and locale it searched.
`origin` names the input that chose the country: `flag` (`--country`),
`location` (the end of `--location`), `env` (`JOBS_COUNTRY`), or `profile`.
Indeed has no default market, so when none resolves, the indeed partition has
no `market` and fails with `MARKET_REQUIRED`. Other Sources never carry
`market`.

`meta.partitions` mirrors per-partition pagination. Native pagination models
(cursor, offset, and start/count) are preserved under `pagination.native`
so each source continues with its own primitive; continuation always names a
single source.

### Capability Flags

Every Application Target carries a `capabilities` object; `sources status`
reports the same flags per Application Provider. Both derive from one canonical
table, so `sources status --json` and `resolve`/`apply inspect` never disagree.

- `inspect`: the CLI can fetch application requirements natively.
- `prepare`: the CLI can build a validated Application Artifact natively.
- `native_submit`: the CLI can submit through a supported native interface.
- `browser_required`: native submission is not available; hand the returned
  application URL to a browser-capable Agent.
- `auth_required`: the **CLI-native operation requires CLI-managed
  authentication** (a CLI session or login, for example the LinkedIn Session).
  It does **not** mean that a login may exist somewhere in the flow's UI. A
  provider whose browser flow happens to use a candidate account that jobs-cli
  does not manage — Indeed Apply, for example — reports `auth_required: false`.

### Application Target

Returned by `resolve` (and attached to Jobs with `show --resolve`):

```json
{
  "url": "https://boards.greenhouse.io/acme/jobs/12345",
  "provider": "greenhouse",
  "provider_job_id": "12345",
  "board_token": "acme",
  "tenant": "",
  "site": "",
  "capabilities": {
    "inspect": true,
    "prepare": true,
    "native_submit": true,
    "browser_required": false,
    "auth_required": false
  },
  "verification": "VERIFIED WORKING",
  "resolved_from": "https://..."
}
```

Providers with no native public submission (Indeed, Lever, Ashby, Workday,
SmartRecruiters, iCIMS, external) report `browser_required: true` plus a usable
application URL. A LinkedIn job URL resolves to the `linkedin` provider (Easy
Apply may be available); `apply inspect`/`prepare` return `BROWSER_REQUIRED`
with the LinkedIn job URL when the job's native Easy Apply is unavailable, so
non-Easy-Apply jobs never produce a linkedin application artifact.

### Application Inspection

Returned by `apply inspect`:

```json
{
  "provider": "greenhouse",
  "application": { "url": "...", "provider": "greenhouse", "capabilities": { } },
  "fingerprint": "sha256:...",
  "fields": [ { "name": "first_name", "label": "First Name", "type": "text", "required": true } ],
  "questions": [
    {
      "id": "provider question id",
      "label": "Why do you want to work here?",
      "type": "input",
      "required": true,
      "options": [ { "value": "yes", "label": "Yes" } ],
      "max_length": 2000
    }
  ],
  "accepts_resume": true,
  "accepts_cover_letter": true,
  "capabilities": { }
}
```

### Application Artifact

Versioned JSON produced by `apply prepare` and consumed by `apply submit`
(`--artifact <file>` or `--artifact -` for stdin). `schema_version`:
`2026-09-26`, `artifact_version`: `1`.

```json
{
  "schema_version": "2026-09-26",
  "artifact_version": 1,
  "generated_at": "2026-09-26T12:00:00Z",
  "job_id": "indeed:517ca3fd71acddc9",
  "job_title": "Backend Engineer",
  "employer": "Acme",
  "provider": "greenhouse",
  "application": { "url": "...", "provider": "greenhouse", "capabilities": { } },
  "fingerprint": "sha256:...",
  "candidate": {
    "first_name": "Ada",
    "last_name": "Lovelace",
    "email": "ada@example.com",
    "phone": "",
    "location": "",
    "linkedin": "",
    "website": ""
  },
  "answers": [ { "question_id": "provider question id", "value": "..." } ],
  "attachments": [
    { "kind": "resume", "path": "resume.pdf", "filename": "resume.pdf", "content_type": "application/pdf", "size": 90000 }
  ]
}
```

`submit` re-inspects remote requirements and compares `fingerprint`; a mismatch
fails with `ARTIFACT_STALE`.

## Error Codes

Jobs-specific v1 codes (stable once shipped; new codes may be added later
without renaming these):

- `SOURCE_UNAVAILABLE` (5): a requested discovery source is unavailable.
- `MARKET_REQUIRED` (2): an Indeed search has no market, because `--country`,
  the end of `--location`, `JOBS_COUNTRY`, and the profile `country` all name
  none. It fails only the indeed partition.
- `ATS_RESOLUTION_FAILED` (6): the application target could not be resolved.
- `NATIVE_APPLY_UNSUPPORTED` (6): no native application implementation exists
  for the provider.
- `BROWSER_REQUIRED` (6): native submission is not available; use the returned
  application URL in a browser.
- `APPLICATION_INCOMPLETE` (7): required application data is missing or
  invalid; the application was not sent.
- `ARTIFACT_STALE` (7): remote requirements changed after `apply prepare`;
  prepare again.
- `LINKEDIN_SESSION_REQUIRED` (3): authenticated LinkedIn was requested but no
  complete, valid session is stored (absent, or present but invalid).
- `LINKEDIN_EASY_APPLY_UNVERIFIED` (6): Easy Apply submission is disabled until
  the current Voyager implementation is verified with a live session.

Shared family codes also in use: `INVALID_ARGUMENTS` (2),
`READ_ONLY_VIOLATION` (4), `CONFIRMATION_REQUIRED` (10), `VALIDATION_FAILED` (7),
`API_ERROR`, `API_SCHEMA_CHANGED`, `API_ACCESS_FORBIDDEN`, `RESOURCE_NOT_FOUND`,
`NETWORK_UNREACHABLE`, `NETWORK_TIMEOUT`, `RATE_LIMITED` (5, retryable, exposes
`retry_after_ms` when the provider supplies one), `AUTH_REQUIRED` (3),
`NOT_IMPLEMENTED`, `INTERNAL_ERROR` (1).

`API_SCHEMA_CHANGED` is used whenever a provider response no longer matches the
verified shape — drift is never silently normalized.

## Application workflow additions (2026-09-30)

Artifact v2 adds full_name, address, repeated work/education/skills/languages/certificates, raw resume_json, requirements_validated, pending_action, browser_steps, and attachment sha256 content digests. Resume entry keys keep JSON Resume spelling (startDate, studyType, etc.). Version 1 native artifacts remain readable; unknown versions fail validation.

Capabilities distinguish browser_fill/browser_submit from native_submit. Workday exposes browser execution with an explicit local CDP connection; its production submission remains unverified. Greenhouse's default capability is public inspection/preparation with auth_required; native_submit requires runtime board-scoped employer credentials.

Workday results always include submitted and status. Pending statuses include start_application_required, start_uncertain, login_required, verification_required, user_action_required, missing_answers, unsupported_step, requirements_changed, review_required, review_ready and submission_uncertain. requirements_changed includes the current inspection under receipt.inspection. submitted is true only after positive confirmation evidence; receipt.tasks is separate from submission success. Review and mutation state live in the separately supplied --state file, without browser credentials.

Indeed-hosted artifacts carry requirements_validated=false and pending_action=official_handoff. They represent locally prepared inputs, not validated Indeed form answers or permission to automate submission.
