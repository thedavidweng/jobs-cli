# Jobs CLI

Domain language for discovering jobs across sources and applying through application providers, without conflating those two roles. The installed binary is `jobs-cli`.


## Language

**Job**:
A normalized opening as seen by the CLI after discovery or detail retrieval, addressable by a compound Job ID.
_Avoid_: Listing (except in prose about a raw source card), posting (except when quoting a provider API), opportunity

**Job ID**:
A stable compound identifier of the form `<source>:<source-job-id>` that is passed between commands.
_Avoid_: URL-as-ID, opaque internal UUID without source prefix

**Source**:
The system where a Job was discovered (for example `indeed` or `linkedin`).
_Avoid_: Provider (when meaning discovery), platform, board (when meaning discovery origin)

**Application Provider**:
The system that accepts the candidate application (for example `greenhouse`, `linkedin`, `lever`, `external`).
_Avoid_: Source, ATS (as the only term—ATS may describe a provider but is not the glossary word), destination site

**Application Target**:
The resolved application endpoint for a Job: canonical URL, Application Provider, provider-specific identifiers, and capability flags.
_Avoid_: Apply link (alone), redirect, ATS URL (alone)

**Application Artifact**:
The validated, reviewable payload produced by `apply prepare` and consumed by `apply submit`.
_Avoid_: Draft application (ambiguous), form state, cached answers

**Capability**:
A machine-readable flag on an Application Target describing what the CLI can do natively (inspect, prepare, submit) versus what requires a browser.
_Avoid_: Feature flag (reserve for build/runtime gates), support level (prose only)

**Browser Required**:
A Capability outcome meaning native submission is not available; the CLI returns the application URL for an external browser-capable Agent.
_Avoid_: Unsupported (too vague—prefer explicit capability + error code), scrape fallback

**Search Partition**:
The per-Source slice of a multi-source `search` result, carrying that Source’s jobs, pagination, or structured error.
_Avoid_: Page (reserve for pagination within a Source), batch, group

**Market**:
The country whose Indeed index a search runs in, plus the locale sent with it. Each search resolves it from `--country`, the end of `--location`, `JOBS_COUNTRY`, or the profile; there is no default, and detail retrieval needs none.
_Avoid_: Region, geo (LinkedIn's location URN), site

**Session**:
The locally stored LinkedIn web authentication material used for Voyager, obtained via guided browser import—not an OAuth access token.
_Avoid_: OAuth token, API key, password

**Error Code**:
A stable jobs-specific `error.code` string (for example `BROWSER_REQUIRED`, `ARTIFACT_STALE`) inside the shared CLI-family error envelope and exit-code categories.
_Avoid_: Exit code (the numeric process status), exception type

**jobs-cli**:
The installed CLI binary name. Command invocations in docs are `jobs-cli <command>`.
_Avoid_: `jobs` as the binary name (shell builtin conflict)
