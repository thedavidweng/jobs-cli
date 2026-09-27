# jobs-cli Specification

Canonical product and implementation spec for `jobs-cli`, updated after the September 2026 grilling session. Domain vocabulary lives in [`CONTEXT.md`](../CONTEXT.md). Architectural decisions live in [`docs/adr/`](./adr/). Historical feasibility research informed the implementation but is intentionally not part of the public source tree.

**Binary name:** `jobs-cli` (not `jobs` — shell builtin conflict; users may alias locally).  
**Module:** `github.com/thedavidweng/jobs-cli`  
**Entry:** `cmd/jobs-cli`

---

## Problem Statement

Job search and application automation is fragmented across LinkedIn, Indeed, employer career sites, and multiple Applicant Tracking Systems (ATS). Existing automation commonly depends on browser scraping, site-specific DOM logic, or isolated scripts that are difficult for humans, shell scripts, and coding agents to compose reliably.

The project already completed a technical feasibility investigation in September 2026. That investigation established that much of the required structured network surface already exists and has either been exercised live or is available in current reusable source implementations.

The remaining problem is therefore not primarily API discovery or reverse engineering. It is to turn those confirmed capabilities into a cohesive, predictable, single-binary CLI that follows the same design language as the existing `qualtrics-cli` and `monarchmoney-cli` projects.

The resulting tool must let a user or external Agent:

- search jobs across supported discovery sources;
- retrieve normalized job details;
- resolve a discovered listing to the actual application provider;
- inspect structured application requirements where possible;
- prepare and validate an application without submitting it;
- explicitly submit applications only through supported native interfaces;
- return the application URL and a machine-readable `browser_required` capability when native submission is not supported.

The CLI itself must not contain an LLM, MCP server, browser automation engine, CAPTCHA solver, or autonomous mass-application loop. Reasoning belongs to the external Agent; deterministic network interaction belongs to the CLI.

The installed binary name is **`jobs-cli`**.

---

## Solution

Build a Go-based, single-binary `jobs-cli` using the established CLI family conventions from `qualtrics-cli` and `monarchmoney-cli`.

The CLI will separate two concepts that must never be conflated:

- **Source** — where a job was discovered, such as Indeed or LinkedIn.
- **Application Provider** — the system that actually accepts the application, such as Greenhouse, LinkedIn Easy Apply, Lever, Ashby, Workday, SmartRecruiters, iCIMS, Indeed, or an unrecognized external site.

For example, a job may have:

- source: `indeed`
- application provider: `greenhouse`

The first implementation must use the already-verified structured interfaces instead of repeating feasibility research.

Core functionality will include:

- Indeed structured GraphQL discovery and job detail retrieval;
- LinkedIn anonymous Guest discovery (default for `--source linkedin`);
- LinkedIn authenticated Voyager integration after guided session login and explicit `--authenticated`;
- canonical normalized job objects;
- ATS/application-provider resolution;
- Greenhouse structured application inspection and native submission;
- LinkedIn Easy Apply form inspection; native submission only after live authenticated verification;
- read-only structured support for Lever, Ashby, Workday, and SmartRecruiters;
- explicit browser-required results for providers whose public candidate submission cannot be completed natively;
- stable JSON envelopes and exit codes for Agent use;
- human-readable output by default;
- mutation safety through `--read-only`, `--dry-run`, and `--confirm`.

---

## User Stories

1. As a job seeker, I want to search for jobs by keyword, so that I can discover relevant openings from the terminal.

2. As a job seeker, I want to search by location, so that I can restrict results to places where I can work.

3. As a job seeker, I want to filter for remote work when the source supports it, so that I can focus on remote opportunities.

4. As a job seeker, I want to filter by recency where supported, so that I can prioritize newly posted jobs.

5. As a job seeker, I want to choose one or more discovery sources, so that I can search Indeed, LinkedIn, or future providers selectively.

6. As a job seeker, I want the default command output to be concise and human-readable, so that the CLI is pleasant to use interactively.

7. As an Agent, I want `--json` to return one stable JSON document on stdout, so that I can parse results deterministically.

8. As an Agent, I want diagnostics to go to stderr, so that stdout remains safe for pipelines.

9. As an Agent, I want every successful JSON response to use the same success envelope, so that I do not need provider-specific parsers.

10. As an Agent, I want every failure to contain a machine-readable error code and category, so that I can decide whether to retry, stop, authenticate, or request human intervention.

11. As an Agent, I want stable exit-code categories shared with the user's other CLIs, so that orchestration logic can be reused.

12. As a user, I want an Indeed result and a LinkedIn result to normalize into the same Job shape, so that I can compare them without knowing the originating API.

13. As a user, I want a stable compound job identifier such as `indeed:<source-id>` or `linkedin:<source-id>`, so that I can pass jobs between commands.

14. As a user, I want to retrieve complete details for a known job, so that I can inspect description, employer, location, compensation, posting date, and application destination.

15. As a user, I want compensation normalized into amount, currency, and interval fields when the source exposes it, so that values are machine-comparable.

16. As a user, I want source-specific data to be preserved when useful without contaminating the normalized contract, so that advanced debugging remains possible.

17. As a user, I want `jobs-cli resolve` to determine the actual application provider, so that a listing discovered on Indeed or LinkedIn can be routed to the correct application implementation.

18. As a user, I want redirect-based application links such as Greenhouse tracking links to resolve to a canonical target, so that provider detection remains reliable.

19. As an Agent, I want the resolved application target to expose provider capabilities, so that I know whether application inspection, preparation, and submission are supported natively.

20. As an Agent, I want unsupported native application flows to return the original application URL plus `browser_required`, so that I can hand the task to a separate browser-capable Agent.

21. As a user, I want Greenhouse application questions returned as structured fields, so that required answers can be prepared before submission.

22. As an Agent, I want question identifiers, labels, types, options, and required status preserved, so that I can map candidate information to exact form fields.

23. As a user, I want to prepare a Greenhouse application with a resume, cover letter, identity fields, and question answers, so that the CLI can validate the application locally.

24. As a user, I want missing required application fields to fail validation before any remote submission, so that incomplete applications are not sent.

25. As a user, I want `--dry-run` on any supported application mutation to show the planned action without submitting it, so that I can review it safely.

26. As a user, I want actual application submission to require `--confirm`, so that an Agent cannot accidentally submit a job application merely by calling a normal command.

27. As a user, I want `--read-only` to block all remote writes globally, so that I can give an Agent safe exploratory access.

28. As a user, I want LinkedIn Guest search to work without authentication, so that basic discovery does not depend on an account.

29. As a user, I want `jobs-cli auth linkedin login` to guide me through browser login and import my existing LinkedIn web session on macOS, Linux, and Windows, so that I do not have to paste cookies manually.

30. As a user, I want `jobs-cli auth status` to tell me whether authenticated LinkedIn capabilities are available, so that I understand which commands will work.

31. As a user, I want authenticated LinkedIn job search and job details to use the current Voyager implementation rather than legacy Voyager REST endpoints, so that the CLI builds on the currently supported reverse-engineered surface.

32. As a user, I want the CLI to inspect LinkedIn Easy Apply state and fields before submission, so that I can review what LinkedIn intends to send.

33. As a user, I want LinkedIn Easy Apply submission to remain disabled until the current Voyager implementation has been successfully exercised with a real authenticated test session, so that source-level evidence is not mistaken for end-to-end verification.

34. As a user, I want Lever jobs to be searchable and inspectable through their public structured API, so that browser scraping is unnecessary for discovery.

35. As a user, I want Lever application targets to report browser-required status when hCaptcha prevents public native submission, so that the CLI does not pretend to support an unreliable bypass.

36. As a user, I want Ashby jobs and structured compensation to be available through its public job-board API, so that rich salary data is retained.

37. As a user, I want public Ashby candidate submissions to return browser-required status, so that the CLI does not misuse employer-only credentials.

38. As a user, I want Workday job discovery and details to use CXS JSON endpoints where available, so that the read path avoids browser automation.

39. As a user, I want Workday application targets to report browser-required status, so that candidate-account creation, email verification, session state, and multi-step wizard handling stay outside this CLI.

40. As a user, I want SmartRecruiters public listings and details to be available through the same normalized interface, so that they can participate in cross-source job discovery.

41. As a user, I want iCIMS jobs to be identified even when only public HTML/Schema.org data is available, so that the application URL is still useful.

42. As a user, I want iCIMS native submission to be explicitly unsupported rather than implemented as fragile browser scraping.

43. As an Agent, I want pagination metadata to be represented consistently even when providers use different pagination models, so that I can iterate without provider-specific logic.

44. As an Agent, I want rate-limit failures to be explicitly retryable and expose `retry_after_ms` when available, so that I can back off correctly.

45. As a user, I want idempotent network reads to retry transient failures according to existing CLI-family conventions, so that temporary service failures do not unnecessarily break workflows.

46. As a user, I do not want application POST requests automatically retried, so that a transient response cannot accidentally create duplicate submissions.

47. As a user, I want a `doctor` command, so that configuration, source availability, authentication state, and local prerequisites can be inspected in one place.

48. As a user, I want a `sources status` command, so that I can see which discovery and application providers are available and which require authentication or a browser.

49. As a user, I want shell completions for supported shells, so that `jobs-cli` behaves consistently with the user's other CLI tools.

50. As a maintainer, I want command names, help, documentation, JSON schema, and implementation to change together, so that public behavior never drifts from reference documentation.

51. As a maintainer, I want protocol assumptions that came from reverse engineering to be clearly isolated from generic domain logic, so that provider breakage is easy to diagnose and update.

52. As a maintainer, I want current endpoint references and provenance recorded in project documentation, so that a future Agent knows which upstream implementation or verification supports each integration.

53. As a maintainer, I want provider schema changes to fail as `API_SCHEMA_CHANGED` rather than silently dropping data, so that external API drift is visible.

54. As a maintainer, I want the existing feasibility research retained as evidence rather than re-created during implementation, so that engineering effort is spent on the product rather than repeating discovery work.

55. As a user, I want the project distributed as a single Go binary with the same release conventions as the user's other CLI projects, so that it is easy to install locally and invoke from any Agent.

56. As an Agent, I want multi-source search results partitioned by Source, so that I do not depend on invented cross-source ranking or deduplication.

57. As an Agent, I want a versioned Application Artifact from `apply prepare` that `apply submit` consumes, so that I can review and replay a validated payload.

58. As a user, I want multi-source search to succeed partially when at least one Source works, so that a single provider outage does not fail the whole search.

---

## Implementation Decisions

### 1. CLI family conventions

`jobs-cli` will follow the established architecture and public behavior of `qualtrics-cli` and `monarchmoney-cli`.

The implementation should copy conventions, not code blindly.

The following behavior is considered established project-family policy:

- Go implementation.
- Cobra command tree.
- Single native binary named `jobs-cli`.
- Module path `github.com/thedavidweng/jobs-cli`; entry `cmd/jobs-cli`.
- Human-readable output by default.
- `--json` for machine-readable output.
- `--pretty` for formatted JSON.
- `--full` for expanded payloads where a summary is otherwise shown.
- `--read-only` to block all writes.
- `--dry-run` to preview mutations (plan shape: Monarch-style `planned_mutations`).
- `--confirm` to authorize mutations.
- `--timeout`, `--profile`, and `--config` global controls.
- Config directory: macOS `~/.jobs-cli`; Linux `${XDG_CONFIG_HOME:-~/.config}/jobs-cli`; Windows `%APPDATA%\jobs-cli` (fallback `~/.jobs-cli`); override with `JOBS_CONFIG_DIR`; env prefix `JOBS_*`.
- Multi-profile `config.yaml` (Qualtrics-style); LinkedIn session stored in a separate per-profile session file (0700/0600), not in YAML.
- Exactly one JSON document on stdout in normal JSON mode.
- Diagnostics and human-readable errors go to stderr.
- Dotted command identifiers are used consistently in metadata and error reporting.
- Stable success/error envelopes (`ok` / `data`|`error` / `meta`).
- Stable error taxonomy and exit-code categories (family exits including 3/4/5/6/7/10).
- `doctor`, `version`, and `completion` commands.
- Documentation changes move with command/flag/schema changes.
- Architectural rationale belongs in ADRs rather than code comments.
- `mise run check` is the pre-commit/pre-push quality gate.

The implementation Agent should use `thedavidweng/qualtrics-cli` as the primary architecture donor and `thedavidweng/monarchmoney-cli` as the mature behavior donor.

### 2. No MCP

MCP is explicitly out of scope.

The integration boundary for Agents is the CLI process plus JSON.

Agents call `jobs-cli` using shell execution and consume the stdout contract.

There must be no MCP server, embedded tool server, long-running daemon, or alternate RPC protocol.

### 3. No embedded AI

The CLI performs deterministic operations only.

The CLI does not:

- rank jobs using an LLM;
- generate screening-question answers;
- tailor resumes;
- generate cover letters;
- decide whether a user should apply;
- autonomously apply in a loop.

An external Agent may consume job/application JSON, reason about it, create answer files, and call the next CLI command.

### 4. Core command surface

The initial public command domains are:

- `search`
- `show`
- `resolve`
- `apply inspect`
- `apply prepare`
- `apply submit`
- `auth status`
- `auth linkedin login` / LinkedIn session logout under `auth`
- `auth logout` (as applicable)
- `sources status`
- `doctor`
- `version`
- `completion`

Do not create a redundant `jobs-cli jobs ...` hierarchy.

The binary name already scopes the product; subcommands are the job domain verbs.

### 5. Normalized Job domain model

All discovery providers normalize into one canonical Job contract.

The normalized object must cover at least:

- stable CLI ID;
- source;
- original source job ID;
- title;
- employer/company;
- location;
- remote/workplace information when known;
- full description when retrieved;
- posting date when known;
- compensation when known;
- source URL;
- application target fields when attached (only when explicitly resolved);
- provider-specific raw data where required for diagnostics or later operations.

Provider-specific network shapes must not leak into the common top-level contract.

### 6. Source and Application Provider are separate concepts

This is a core architectural decision (ADR-0001).

`source` describes discovery provenance.

`application.provider` describes the system responsible for the actual candidate application.

Supported provider values should be capable of representing at least:

- `greenhouse`
- `linkedin`
- `indeed`
- `lever`
- `ashby`
- `workday`
- `smartrecruiters`
- `icims`
- `external`
- `unknown`

Application metadata must include enough information to expose capabilities such as:

- inspect supported;
- prepare supported;
- native submit supported;
- browser required.

**v1 discovery Sources** for `search` are Indeed and LinkedIn only. Greenhouse, Lever, Ashby, Workday, and SmartRecruiters are Application Providers (and read adapters) in v1—not first-class `search --source` values. Keep a Source extension point for later board search without exposing it in the v1 command surface.

### 7. Multi-source search behavior

Defaults and contracts (ADR-0003, 0005, 0011, 0015):

- Default sources when `--source` is omitted: `indeed` + `linkedin` (Guest).
- `--authenticated` selects the authenticated variant **only for Sources that have one** (LinkedIn Voyager). Indeed has no authenticated variant, so `search --authenticated` with default sources must still run the Indeed partition through Indeed and only switch the linkedin partition to Voyager; the flag never turns a Source off.
- Results are **partitioned by Source** (each Search Partition has its own jobs, pagination, or structured error). No cross-source flat merge or fingerprint dedup in the CLI.
- The same real-world opening found on two Sources yields two Job IDs; Agents may dedupe after `resolve` if they choose.
- Source requests fan out **in parallel** under `--timeout`.
- **Partial success:** if at least one Source succeeds, exit 0; failed partitions carry structured errors and `meta.warnings`. Exit non-zero only when every requested Source fails.
- **Pagination continue:** after the first multi-source page, continuation requests must name a **single** Source plus that Source’s native cursor/offset. No opaque global page token.

### 8. Pagination abstraction

Providers retain their native pagination internally:

- Indeed: cursor.
- LinkedIn Voyager: offset/count.
- LinkedIn Guest: start offset.
- Lever: limit/skip.
- Workday: limit/offset.
- SmartRecruiters: offset/limit.
- Greenhouse/Ashby board APIs may return the full board.

The public JSON envelope normalizes pagination metadata without pretending every provider supports an identical primitive.

Provider-specific cursor state may be exposed when required to continue a query.

### 9. Indeed implementation

Indeed discovery must use the already-verified internal mobile GraphQL service.

Do not start with HTML scraping.

Do not perform new APK reverse engineering.

Do not use the obsolete Indeed Publisher APIs.

Required functionality:

- job search;
- location;
- radius where appropriate;
- sorting;
- structured compensation;
- attributes;
- cursor pagination;
- batch job detail retrieval;
- employer information;
- description;
- application URL extraction through `recruit.viewJobUrl`.

The required mobile client identity/header bundle must be sourced from the pinned working JobSpy implementation referenced under Further Notes.

Market parameters are profile-driven, not guessed from the `--location` string. `indeed-co`, `indeed-locale`, and `accept-language` derive from the profile market (`country` / `locale`, overridable via `JOBS_COUNTRY` / `JOBS_LOCALE` and `search --country` / `--locale`). The canonical source URL host follows the same market (`www.indeed.com` for US, `ca.indeed.com` for CA, `uk.indeed.com` for GB). Defaults are `US` / `en-US` — the live-verified values of the pinned bundle; the September 2026 Vancouver verification used `indeed-locale: en-CA` with `indeed-co: CA`. The locale defaults to `en-<country>` when only a country is set.

Do not invent a new Indeed query if the verified query is sufficient.

### 10. Indeed application behavior

For external-ATS jobs, Indeed is discovery only.

The CLI resolves `recruit.viewJobUrl`, detects the application provider, and hands the job to that provider.

Direct Indeed Apply submission is not part of the initial native application surface because it requires candidate OAuth/session/resume flows that were not implemented by the completed research.

When a listing is Indeed Apply-only and no native implementation exists, the CLI must expose this clearly (`NATIVE_APPLY_UNSUPPORTED` / `BROWSER_REQUIRED` as appropriate) instead of attempting hidden browser automation.

Indeed resolves as the `indeed` Application Provider with `auth_required=false, browser_required=true`. The browser flow may require an authenticated Indeed session, but jobs-cli manages no session for it, so the provider must not advertise `auth_required=true`.

### 11. LinkedIn Guest implementation

Anonymous LinkedIn discovery must use the current Guest jobs endpoint.

It provides zero-auth job cards and pagination.

Guest search is the **default** for `--source linkedin` and is a fallback/basic discovery provider. It is not the Easy Apply implementation.

HTML parsing is acceptable here because the current public Guest interface itself returns HTML fragments and was live-verified.

### 12. LinkedIn Voyager implementation

Authenticated LinkedIn capabilities must be based on the current `yashiels/linkedin-cli` Voyager implementation, not legacy Voyager REST libraries.

Reuse/adapt the known:

- Rest.li variable encoding semantics;
- Android-style request headers;
- current persisted query IDs;
- search parameter shapes;
- job detail query;
- Easy Apply form query;
- Easy Apply submission action.

Authenticated Voyager is currently classified as `VERIFIED SOURCE IMPLEMENTATION`, not yet `VERIFIED WORKING` in this project.

`--source linkedin` must **not** silently upgrade to Voyager when a session exists. Callers pass an explicit authenticated switch (for example `--authenticated`) to use Voyager (ADR-0008). The switch applies to the linkedin partition only; Sources without an authenticated variant keep their single implementation.

Authenticated location filtering follows the donor's geo URN mechanism: `--location` resolves to `urn:li:fsd_geo:<id>` (raw `urn:li:` passthrough or the built-in known-locations table), then the search encodes `query.locationUnion.geoUrn` — never a raw location string. An unknown location fails the linkedin partition with `INVALID_ARGUMENTS` listing the known values and the raw URN escape hatch.

Before enabling native Easy Apply submission as a supported user-facing capability, perform a controlled live integration verification using a user-supplied valid LinkedIn session. Until then: Easy Apply **inspect** may work; **submit** returns a machine-readable not-yet-verified / unsupported outcome (`LINKEDIN_EASY_APPLY_UNVERIFIED`) without changing the command tree.

Easy Apply availability is checked per job: when inspection shows LinkedIn-native apply is not available (`onsiteApply: false`), the job reports `BROWSER_REQUIRED` with the LinkedIn job URL. A non-Easy-Apply job must never be represented as `application.provider: linkedin` in an Application Artifact — the Source stays LinkedIn while the actual application happens on the employer's ATS in a browser.

### 13. LinkedIn authentication

Do not implement automated username/password login or checkpoint bypass.

Primary UX (ADR-0010): `jobs-cli auth linkedin login`

1. Open LinkedIn’s login page in the user’s browser.
2. Wait for the user to finish login and confirm in the terminal.
3. Import Voyager session cookies from the local browser cookie store into the per-profile session file.

This is **session import**, not LinkedIn OAuth (Voyager has no official token grant for this CLI).

- Supported platforms in v1: **macOS, Linux, and Windows**.
- Browser auto-detect order: Chrome → Safari → Firefox (with platform-appropriate equivalents); override with `--browser`.
- If cookies cannot be read, fail with a clear actionable error rather than making manual cookie flags the primary path.
- Credentials/session material must never be printed in normal output.
- Authentication state must be inspectable through `auth status` and `doctor`.

Local storage follows CLI-family security conventions (directory `0700`, session file `0600`).

### 14. ATS Resolver

A resolver is required between discovery and application.

It accepts the job's application URL and determines:

- final canonical URL;
- application provider;
- provider-specific identifiers where they can be safely extracted;
- native application capabilities.

The resolver must understand at least:

- Greenhouse board URLs;
- Greenhouse tracking/short redirect URLs;
- Lever job URLs;
- Ashby job URLs;
- Workday tenant/site URLs;
- SmartRecruiters URLs;
- iCIMS URLs;
- LinkedIn-native Easy Apply;
- Indeed-native Apply;
- generic external URLs.

Redirect policy (ADR-0013):

- at most **5** redirects;
- cross-host redirects allowed;
- reject HTTPS→HTTP downgrades;
- reject private/link-local/metadata targets.

`jobs-cli resolve` returns **only** an Application Target (ADR-0006). Use `jobs-cli show` for Job fields.

`jobs-cli show` does **not** resolve by default; `--resolve` attaches a best-effort Application Target (ADR-0007).

The resolver must not become a general browser.

### 15. Greenhouse native application support

Greenhouse is the first fully native end-to-end application provider.

The implementation must support:

- board job discovery where needed for apply flows (not a v1 `search` Source);
- job details;
- application question retrieval;
- identity/contact fields;
- resume file upload;
- cover letter where accepted;
- custom question answers;
- local validation;
- dry-run representation;
- explicit confirmed submission.

Greenhouse native application submission requires no candidate authentication on the verified public Board API surface.

### 16. Application lifecycle

Do not implement application as one opaque `apply(job)` call.

The public workflow is split into:

- inspect;
- prepare;
- submit.

`apply inspect` and `apply prepare` accept a **Job ID** and resolve the Application Target as needed (ADR-0016).

`inspect` fetches requirements and questions without mutation.

`prepare` combines user-supplied candidate data/documents/answers and validates the resulting application without remote submission. It produces a versioned JSON **Application Artifact**.

Artifact I/O (ADR-0002, 0014):

- Human mode: `--out <path>` required to write a file (no silent default path under cwd or profile).
- JSON/Agent mode: artifact may be emitted on stdout; submit may read `--artifact -` (stdin) or a file path.
- Prepare inputs: human-friendly answers file + `--resume` / `--cover-letter` flags, and also a single manifest JSON that embeds answers and attachment paths.

`submit` executes the supported remote mutation, consumes the Application Artifact, and requires safety authorization.

Before confirmed native submit, re-inspect remote requirements and compare a schema fingerprint to the artifact; on mismatch fail with `ARTIFACT_STALE` (must prepare again). No `--force` bypass of freshness (ADR-0009).

### 17. Application input

Application preparation must be driven by explicit user/Agent-provided data.

v1 does **not** store a local candidate profile (name/email/default resume) in the CLI profile directory (ADR-0018). Reuse is an Agent/workspace concern.

The CLI must validate:

- required fields;
- supported field types;
- selected option values;
- referenced files;
- provider-specific required identifiers.

The CLI must not infer subjective answers.

### 18. Lever

Use the public Postings API for structured read support.

Native public candidate submission is not implemented because the public application flow is hCaptcha-protected and programmatic submission requires non-public/employer authentication.

Resolved Lever applications must therefore expose a usable application URL and `browser_required`.

Do not attempt CAPTCHA bypass.

### 19. Ashby

Use the public job-board API for job discovery/details.

Request compensation data where supported.

Candidate-side public native submission is not implemented because programmatic application submission requires employer credentials while the public candidate flow is handled by the web application.

Expose `browser_required` for public application.

### 20. Workday

Use Workday CXS JSON endpoints for search and job-detail reads where the target tenant exposes them.

Do not implement public native candidate submission in the initial version.

Candidate account creation, verification, authentication, session state, and multi-step application wizard remain browser responsibilities.

Expose `browser_required`.

### 21. SmartRecruiters

Use the public company postings API for job discovery and details.

Initial scope is read support and resolution.

Do not invent candidate submission support unless it is separately verified.

### 22. iCIMS

Treat iCIMS as a low-structure/browser fallback.

Public Schema.org/JSON-LD may be used for job detail extraction where available.

Native search and application submission are not claimed.

Expose the application URL and browser-required capability.

### 23. Safety model

Remote application submission is a mutation.

The established CLI-family safety model applies.

- `--read-only` blocks all application submissions.
- `--dry-run` shows the plan (`planned_mutations`) and does not submit.
- Submission without explicit confirmation fails with `CONFIRMATION_REQUIRED`.
- `--confirm` authorizes the mutation.
- POST submission requests are never automatically retried.

No provider is allowed to bypass the shared mutation gate.

### 24. Output contract

Use the same conceptual envelope as the current CLI family.

Success contains:

- `ok: true`
- command-specific `data`
- `meta`

Error contains:

- `ok: false`
- structured `error`
- `meta`

Metadata must include the established common fields such as:

- command identifier;
- active profile where relevant;
- duration;
- schema version;
- per-invocation request ID;
- warnings where applicable;
- pagination where applicable (including per-partition pagination for multi-source search).

### 25. Error taxonomy

Reuse the existing CLI-family exit-code classes rather than creating a jobs-specific exit-code system.

The established categories include:

- internal;
- invalid arguments / validation;
- authentication;
- safety/read-only;
- network/rate limit;
- remote API/schema/resource errors;
- confirmation required.

Jobs-specific machine error codes (ADR-0017) — v1 normative set:

- `SOURCE_UNAVAILABLE`
- `ATS_RESOLUTION_FAILED`
- `NATIVE_APPLY_UNSUPPORTED`
- `BROWSER_REQUIRED`
- `APPLICATION_INCOMPLETE`
- `ARTIFACT_STALE`
- `LINKEDIN_SESSION_REQUIRED`
- `LINKEDIN_EASY_APPLY_UNVERIFIED`

Jobs-specific codes may be added later without renaming these once shipped.

### 26. Network behavior

Idempotent reads may retry transient transport errors, rate limits, and server failures according to existing family behavior and `Retry-After`.

Application POSTs must not be automatically retried.

Provider clients must enforce reasonable response-size limits.

Unexpected remote response shapes must surface as schema-change errors rather than silently producing incomplete normalized objects.

### 27. Documentation discipline

Public command/flag/output behavior must remain synchronized with:

- Cobra help;
- command reference;
- JSON contract documentation.

New architectural choices require ADRs according to the existing CLI-family convention.

The implementation evidence and protocol history are represented by the typed
adapters, transport-pinned tests, fixtures, ADRs, and external provenance links
in this repository. Private research notes are not runtime dependencies and are
not tracked in the public source tree.

### 28. Distribution

The expected final product is a native Go CLI following the same release/distribution conventions as the user's other CLI projects.

Binary name on PATH: **`jobs-cli`**.

The design should remain suitable for:

- direct GitHub Release download;
- Homebrew distribution (formula installs `jobs-cli`);
- `go install`;
- shell scripting;
- coding Agent invocation.

### 29. doctor vs sources status

`doctor` inspects local installation, config, session files, and optional connectivity (`--connect` where aligned with family).

`sources status` reports discovery Sources and Application Providers with capabilities (auth required, browser required, native submit, verification posture). Its provider capability flags derive from the same canonical table the resolver uses (`resolve`, `show --resolve`, `apply inspect`), so the two surfaces cannot disagree.

`auth_required` means the CLI-native operation requires CLI-managed authentication (a CLI session, for example the LinkedIn Session). It does not mean that a login may exist somewhere in the flow's UI: an Application Provider whose browser flow uses a candidate account that jobs-cli does not manage reports `auth_required=false`. A provider with no CLI-managed auth must never advertise `auth_required=true`.

Do not merge these commands (ADR-0012).

---

## Testing Decisions

### Primary seam

The primary test seam is the `jobs-cli` binary itself.

Tests should invoke the real command layer against controlled HTTP transports and assert externally visible behavior:

- arguments and flags;
- stdout;
- stderr;
- JSON envelopes;
- exit codes;
- mutation safety;
- normalized domain results;
- outgoing HTTP behavior when externally significant.

This follows the project's established preference for the highest practical E2E seam.

Avoid creating low-level seams solely to make internal implementation details easier to unit test.

### What makes a good test

A good test proves observable behavior that a user or Agent depends on.

Examples:

- a search command returns normalized jobs from a recorded provider response;
- multi-source search returns Search Partitions and partial-success warnings;
- pagination continuation requires a single Source;
- an Indeed external application URL resolves to Greenhouse;
- a Greenhouse application question schema appears in `apply inspect`;
- a missing required answer causes validation failure;
- `apply submit` without `--confirm` cannot make the POST;
- `--dry-run` never makes the POST;
- a confirmed Greenhouse submission sends exactly one POST;
- stale artifact fingerprint fails with `ARTIFACT_STALE`;
- an unexpected provider response returns `API_SCHEMA_CHANGED`;
- a 429 read request respects retry policy;
- a submission POST is not retried after an ambiguous failure;
- unsupported Workday application returns `browser_required` and the URL;
- JSON mode emits no diagnostics on stdout;
- LinkedIn defaults to Guest unless `--authenticated`;
- Easy Apply submit before live verification returns `LINKEDIN_EASY_APPLY_UNVERIFIED`.

Tests should not assert private helper names, internal struct layouts, or incidental implementation decomposition.

### Provider transport tests

Provider tests should use real request/response shapes captured from the verified research or upstream reference projects.

Do not invent simplified fake schemas that omit the quirks being relied upon.

For each structured provider, pin the externally meaningful HTTP contract:

- method;
- host/path;
- important headers;
- query/body shape;
- parsing of the verified response shape.

### Indeed tests

Cover:

- search request;
- keyword/location propagation;
- cursor pagination;
- salary parsing;
- employer parsing;
- `recruit.viewJobUrl`;
- batch details;
- schema drift;
- market propagation (US and CA at minimum: `indeed-co`, `indeed-locale`, `accept-language`, market source URL host, from profile, environment, and flags);
- anti-bot response detection as an API failure rather than HTML parsing success.

Fixture shape should be derived from the verified `speedyapply/JobSpy` implementation and the September 2026 live investigation.

### LinkedIn Guest tests

Cover:

- search parameters;
- offset pagination;
- job-card parsing;
- job IDs;
- company/location/date extraction;
- malformed/changed HTML handling.

### LinkedIn Voyager tests

Adapt the behavioral coverage demonstrated by `yashiels/linkedin-cli`, especially:

- Rest.li variable serialization;
- authenticated header construction;
- persisted query invocation;
- search parsing;
- detail parsing;
- location resolution: the exact `locationUnion.geoUrn` Rest.li shape, raw `urn:li:fsd_geo` passthrough, and unknown-location `INVALID_ARGUMENTS`;
- Easy Apply inspection, including `BROWSER_REQUIRED` for jobs whose native Easy Apply is unavailable;
- submission gating (`LINKEDIN_EASY_APPLY_UNVERIFIED` until live-verified).

Add a command-seam regression: default multi-source search with `--authenticated` keeps Indeed on Indeed and switches only the linkedin partition to Voyager.

Do not claim live success from fixture tests.

A separate manually triggered live test may validate an explicitly supplied session.

### LinkedIn auth login tests

Cover (with fakes/stubs for browser cookie DBs where needed):

- browser auto-detect and `--browser` override;
- successful session file write permissions;
- clear failure when cookies are missing;
- never-print of session secrets in stdout/stderr on success paths.

### Resolver tests

Resolver behavior should be tested at its public resolution seam.

Include real representative URLs for:

- direct Greenhouse;
- Greenhouse short/tracking redirect;
- Lever;
- Ashby;
- Workday;
- SmartRecruiters;
- iCIMS;
- unknown external provider.

Assert canonical target and capabilities, not internal regex details.

Also cover redirect limits, HTTPS downgrade rejection, and private-target rejection.

### Greenhouse application tests

This is the most important native application E2E path.

Test:

- question retrieval;
- field normalization;
- resume attachment preparation;
- custom answers;
- required field validation;
- dry-run `planned_mutations`;
- confirmation gate;
- multipart submission shape;
- exactly-once POST behavior;
- success receipt/result;
- remote validation failure;
- artifact freshness re-inspect.

Automated tests must never submit to a real employer.

### Live smoke tests

Live tests are separate from normal CI.

They may validate non-destructive read operations against public services.

They must be opt-in.

At minimum, useful smoke tests include:

- Indeed search;
- LinkedIn Guest search;
- Greenhouse job retrieval;
- Lever posting retrieval;
- Ashby board retrieval;
- SmartRecruiters posting retrieval.

Authenticated LinkedIn live tests require an explicitly configured test session.

Live automated application submission to real jobs is prohibited.

### Prior art

Use the testing and command-contract conventions in:

- `thedavidweng/qualtrics-cli`
- `thedavidweng/monarchmoney-cli`

as the project's primary prior art.

The existing `yashiels/linkedin-cli` tests are prior art specifically for Voyager protocol behavior, not for the public `jobs-cli` contract.

---

## Out of Scope

The following are explicitly out of scope for this spec:

- MCP server support.
- MCP tool definitions.
- Long-running daemon or HTTP server.
- Embedded LLM functionality.
- Job recommendation/ranking AI.
- Resume rewriting or tailoring.
- Cover-letter generation.
- Automatic screening-question reasoning.
- Autonomous mass application.
- Scheduled/background job monitoring.
- Browser automation inside the CLI (except opening the system browser for LinkedIn login guidance).
- Playwright dependency or Node sidecar.
- CAPTCHA solving or bypass.
- Anti-bot bypass beyond reproducing already-verified normal official-client request behavior.
- Automated LinkedIn username/password login.
- Automated LinkedIn challenge/checkpoint bypass.
- Direct Indeed Apply implementation in the first version.
- Native Lever public candidate submission.
- Native Ashby public candidate submission without an appropriate public candidate API.
- Native Workday candidate-account/application automation.
- Native iCIMS application automation.
- Scraping as a replacement for an already-working structured API.
- New Indeed APK reverse engineering unless the currently verified structured interface actually stops working.
- Reverse engineering merely to expand speculative future functionality.
- An internal universal browser abstraction.
- v1 local candidate profile store for identity/resume defaults.
- v1 first-class `search --source` for ATS boards (Greenhouse/Lever/Ashby/Workday/SmartRecruiters).
- Installing a binary named `jobs` (shell builtin conflict).

When a provider requires a browser, the CLI returns structured capability information and the application URL. An external Agent may choose to continue in its own browser environment.

---

## Further Notes

### Verified research status

The feasibility investigation was performed on September 25–26, 2026.

The following results are established implementation evidence. Do not repeat
the research phase unless an endpoint fails during implementation.

#### Indeed — VERIFIED WORKING

Purpose:

- structured discovery;
- details;
- salary;
- pagination;
- external ATS resolution.

Endpoint:

`POST https://apis.indeed.com/graphql`

Verified GraphQL fields/operations include:

- `jobSearch`
- `jobData`
- `pageInfo.nextCursor`
- `compensation.baseSalary`
- `recruit.viewJobUrl`

`recruit.viewJobUrl` is particularly important because it returns the canonical or redirecting application destination and allows Indeed to be removed from the application execution path for external ATS jobs.

Search and details were exercised live on September 25–26, 2026.

The existing working native proof of concept successfully searched Vancouver jobs and returned structured results.

Do not perform new Indeed APK reverse engineering for search/detail/resolution.

Reference implementation:

- Repository: `speedyapply/JobSpy`
- Relevant provider: Indeed
- Primary reference: `jobspy/indeed/constant.py`
- Verified investigation commit: `4ec308a302e35b2a765a6bb73cee659c4011ff91`

That implementation contains:

- the currently working Indeed mobile client identity/header bundle;
- GraphQL search text;
- job detail query patterns;
- compensation normalization.

The CLI should implement the requests natively in Go rather than depending on the Python library.

The client identity value should be taken from the pinned verified source rather than copied from prose documentation, so there is one auditable upstream reference.

The older `jamerst/JobHunt` Indeed implementation is useful for query-shape context but is not the preferred authentication/client path because it relied on an employer-dashboard key.

#### Indeed Apply

Direct Indeed Apply is not solved by the current native client.

The research found that direct Indeed candidate submission requires authenticated candidate/session/OAuth and resume flows.

For the first implementation:

- external ATS jobs resolve and continue through their ATS provider;
- Indeed-native applications report unsupported/browser-required as appropriate.

The investigation estimated that a large majority of relevant technical listings encountered during testing resolve externally, but this percentage must not be treated as a guaranteed platform invariant.

#### LinkedIn Guest — VERIFIED WORKING

Purpose:

- anonymous job discovery;
- zero-auth fallback.

Endpoint:

`GET https://www.linkedin.com/jobs-guest/jobs/api/seeMoreJobPostings/search`

Relevant parameters include:

- `keywords`
- `location`
- `start`
- remote/Easy Apply filters where supported by the endpoint.

Response format:

HTML job-card fragments.

Reference implementation:

- Repository: `speedyapply/JobSpy`
- LinkedIn provider implementation
- Investigation reference commit: `fda080a`

This endpoint was live-verified in 2026.

It is appropriate for anonymous discovery but not for Easy Apply.

#### LinkedIn Voyager — VERIFIED SOURCE IMPLEMENTATION

Primary endpoint:

`https://www.linkedin.com/voyager/api/graphql`

Protocol:

- persisted GraphQL queries;
- LinkedIn Rest.li variable syntax;
- authenticated session cookies;
- CSRF header;
- Android-client style request metadata.

Primary reusable implementation:

- Repository: `yashiels/linkedin-cli`
- Investigation commit: `87d0b8e`

The project compiled and its test suite passed during research.

Its generated API catalog contained 481 GraphQL queries across 261 resources.

Important current operations discovered in that implementation:

Job search:

- query name: `JobCardsByJobSearch`
- query ID: `voyagerJobsDashJobCards.c7c69fb8e8f054fed088918d714be58a`

Job detail:

- query name: `JobPostingDetailSectionsByCardSectionTypesV2`
- query ID: `voyagerJobsDashJobPostingDetailSections.8195171dc4c610f8c1551eaef6546bd8`

Easy Apply inspection/prefill:

- query name: `JobsOnsiteApplyApplicationByJobPosting`
- query ID: `voyagerJobsDashOnsiteApplyApplication.34ac512c4fd87baec02c710aef4f563b`

Easy Apply submission action:

`POST /voyager/api/voyagerJobsDashOnsiteApplyApplication?action=submitApplication`

Authenticated requests require the established LinkedIn session/CSRF combination, including `li_at`, `JSESSIONID`, and the matching CSRF token semantics.

The investigation verified that the Voyager gateway remains active and enforces the expected auth flow, but did not complete a real authenticated application submission.

Therefore:

- reusing the implementation is approved;
- representing it as already end-to-end live verified is not approved;
- native Easy Apply should be enabled only after controlled authenticated integration verification.

Do not use `tomquirk/linkedin-api` as the primary implementation reference for this work. Its legacy Voyager REST search surface is obsolete relative to the current GraphQL implementation.

#### Greenhouse — VERIFIED WORKING

Greenhouse is the first target for complete native application support.

Base:

`https://boards-api.greenhouse.io`

Verified public interfaces:

List jobs:

`GET /v1/boards/{board_token}/jobs`

Job details and application questions:

`GET /v1/boards/{board_token}/jobs/{job_id}?questions=true`

Submit candidate:

`POST /v1/boards/{board_token}/jobs/{job_id}`

Submission:

- public;
- no candidate API authentication required on the verified Board API flow;
- multipart form data;
- resume supported;
- cover letter supported where configured;
- custom questions represented using provider question identifiers.

The research live-tested Greenhouse read behavior with a real public board in September 2026.

The investigation established the submission endpoint and request surface; automated project tests must use a stub and must not send candidate applications to real employers.

Greenhouse is the required first end-to-end path for:

discovery → resolution → application inspection → preparation → dry-run → confirm → submit.

#### Lever — VERIFIED WORKING FOR READ

Public base:

`https://api.lever.co`

Postings:

`GET /v0/postings/{company}`

Single posting:

`GET /v0/postings/{company}/{posting_id}`

Supports structured discovery and details.

Public candidate submission is not part of this implementation because the web candidate flow uses hCaptcha and programmatic submission requires privileged API credentials.

Result:

- read support;
- application URL;
- `browser_required`.

#### Ashby — VERIFIED WORKING FOR READ

Public endpoint pattern:

`GET https://api.ashbyhq.com/posting-api/job-board/{organization}?includeCompensation=true`

Useful structured data includes:

- jobs;
- compensation tiers;
- minimum/maximum values;
- currency;
- interval;
- equity indicators.

Public read was live-verified.

Programmatic application submission uses an employer-side authenticated API; this must not be repurposed for public candidate automation.

Result:

- read support;
- rich compensation normalization;
- application URL;
- `browser_required`.

#### Workday — VERIFIED SOURCE IMPLEMENTATION FOR READ

Typical search pattern:

`POST https://{tenant}.{sub}.myworkdayjobs.com/wday/cxs/{tenant}/{site}/jobs`

Typical detail pattern:

`GET https://{tenant}.{sub}.myworkdayjobs.com/wday/cxs/{tenant}/{site}/job/{job_id}`

Search commonly uses:

- `appliedFacets`
- `limit`
- `offset`
- `searchText`

CXS is the preferred read surface.

Candidate application requires stateful account/login/verification/multi-step web flows and is not a native CLI submit target in this spec.

#### SmartRecruiters — VERIFIED WORKING FOR READ

Public patterns:

`GET https://api.smartrecruiters.com/v1/companies/{company}/postings`

`GET https://api.smartrecruiters.com/v1/companies/{company}/postings/{posting_id}`

Read behavior was live-verified in September 2026.

Initial implementation is discovery/detail/resolution only.

#### iCIMS

Public partner API access is gated.

Public candidate pages may expose Schema.org `JobPosting` JSON-LD.

Treat the public candidate path as browser-required and use structured page metadata only when useful for normalization.

Do not present iCIMS as a native structured submit provider.

### Reuse inventory

High-value existing work:

1. `yashiels/linkedin-cli`
   - Go.
   - Current LinkedIn Android Voyager GraphQL.
   - Rest.li encoder.
   - persisted query registry.
   - job search/details.
   - Easy Apply inspection/submission implementation.
   - high reuse value.

2. `speedyapply/JobSpy`
   - Python.
   - working Indeed mobile GraphQL client configuration.
   - working Indeed search/detail queries.
   - LinkedIn Guest discovery.
   - high protocol-reference value.
   - do not take a runtime dependency on the Python library.

3. `jamerst/JobHunt`
   - useful historical/structural Indeed GraphQL examples.
   - lower implementation priority than JobSpy.

4. `tomquirk/linkedin-api`
   - legacy Voyager REST.
   - not a primary source for the new implementation.

5. Existing ATS-board projects discovered during research
   - useful as parsing/provider-identification references;
   - public API documentation and the project's verified research remain the authority for the implemented surface.

### Existing research artifacts

The implementation was preceded by research covering:

- master capability matrix;
- repository inventory;
- Indeed technical investigation;
- LinkedIn technical investigation;
- Greenhouse investigation;
- Lever investigation;
- Ashby investigation;
- Workday investigation;
- working Indeed proof of concept.

The private research artifacts are not tracked in this public repository.
Public evidence is preserved in the adapters, fixtures, transport-pinned tests,
ADR documents, and upstream provenance references.

If implementation encounters a contradiction between a live endpoint and the September 2026 research, document the new evidence and update the research rather than silently changing assumptions.

### Verification vocabulary

Use the following terms consistently:

- `VERIFIED WORKING`
  - exercised successfully against the real service during the investigation.

- `VERIFIED SOURCE IMPLEMENTATION`
  - current implementation/source was inspected and validated, but this project has not completed a full authenticated live path.

- `PARTIALLY VERIFIED`
  - some important parts are verified but a significant surface remains unconfirmed.

- `BROWSER REQUIRED`
  - public native application submission is not available through the supported deterministic CLI path.

Do not promote a provider from source-verified to live-verified based only on fixtures or unit tests.

### Implementation order

Recommended execution order:

1. Reproduce the common CLI skeleton and contracts from the existing CLI family (`jobs-cli` binary, envelopes, safety, mise check).
2. Define normalized Job, Source, ApplicationTarget, capability, pagination, output, and error contracts.
3. Implement Indeed search/details in native Go.
4. Implement LinkedIn Guest discovery.
5. Implement provider resolution.
6. Implement Greenhouse inspect/prepare/submit (Application Artifact handoff).
7. Complete the first full real-world path:
   Indeed discovery → Greenhouse resolution → application inspection → preparation → dry-run → confirmed submission using non-production test infrastructure/stubs for automated tests.
8. Add authenticated LinkedIn Voyager session handling (`auth linkedin login`) and live search/details verification behind `--authenticated`.
9. Verify and enable LinkedIn Easy Apply submit after live verification.
10. Add structured read adapters for Lever, Ashby, Workday, and SmartRecruiters.
11. Add iCIMS/generic external resolution behavior.
12. Finish installation, release, docs, and Homebrew integration (`jobs-cli`).

### Definition of success

The project is successful when an external coding Agent can safely perform a deterministic workflow such as:

- search for jobs through `jobs-cli --json`;
- inspect normalized Search Partitions;
- choose a result itself;
- resolve the actual ATS;
- obtain application requirements;
- generate answers outside the CLI;
- feed those explicit answers back to the CLI;
- dry-run the application;
- submit only with explicit confirmation when native submission is supported;
- receive a machine-readable browser-required result and URL when it is not.

At no point should the Agent need to understand provider-specific HTML, GraphQL, Rest.li, ATS URL conventions, or safety semantics unless it deliberately requests low-level diagnostics.

### Grilling decisions index

| ADR | Decision |
|-----|----------|
| 0001 | Source ≠ Application Provider |
| 0002 | Application Artifact handoff (file + stdin) |
| 0003 | Partitioned multi-source search |
| 0004 | Multi-profile config + per-profile session files; `planned_mutations` dry-run |
| 0005 | Multi-source partial success |
| 0006 | `resolve` returns only Application Target |
| 0007 | `show` resolves only with `--resolve` |
| 0008 | LinkedIn Guest default; `--authenticated` for Voyager |
| 0009 | Submit re-inspects artifact freshness |
| 0010 | Guided browser session import on macOS/Linux/Windows |
| 0011 | Single-Source pagination continue |
| 0012 | `doctor` ≠ `sources status` |
| 0013 | Resolver redirect policy (≤5, safe) |
| 0014 | No default artifact filesystem path |
| 0015 | Parallel multi-source fan-out |
| 0016 | inspect/prepare take Job ID; submit takes artifact |
| 0017 | v1 jobs-specific error codes |
| 0018 | No local candidate profile store in v1 |
| 0019 | Go module path |
| 0020 | Binary name `jobs-cli` |
