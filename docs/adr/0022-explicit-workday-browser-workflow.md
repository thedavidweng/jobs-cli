# Explicit Workday browser execution and resume inputs

Workday uses one Application Provider parameterized by tenant, career site and Job. This reopens the original v1 boundary that left the entire wizard outside jobs-cli; ADR-0001, 0002, 0009 and 0018 remain in force.

The standalone binary connects through Rod to an explicit loopback CDP endpoint and existing tab ID. The tab URL must match the employer origin and Job path. Browser credentials stay in the user's browser. Disconnecting must preserve the tab. The CLI does not create accounts or infer questionnaire answers.

JSON Resume v1.0.0 is validated using its vendored official schema. It supplies a base Candidate; explicit manifest keys override that base, including empty values. Complete names and date precision are preserved. Raw resume data remains reviewable in artifact v2. Version 1 native artifacts remain readable. Attachments are separate files with content digests; CLI file flags override the same attachment kind.

inspect and prepare never advance or save a browser application. They report login, start and unsupported steps as pending actions. Confirmed fill may start an application, reconcile repeated rows, upload files and advance known steps. It stops at review. Every step's labels, required fields, options and section fields are fingerprinted; unknown/changed steps pause for renewed preparation. --previous-artifact retains earlier reviewed steps, with all Candidate/answer/file inputs explicitly supplied again.

A separate state file binds the target, input digest, visited requirements and review digest. It stores no cookies or passwords. read-only blocks fill and submit; dry-run returns a plan. A submit attempt is recorded before clicking; absent positive confirmation yields submission_uncertain and is never automatically retried. Assessment/Candidate Home tasks are reported separately.

Greenhouse public GET remains available. Its documented POST requires an employer Job Board API key scoped to a board; no anonymous-submit capability is advertised. Indeed preparation is a local handoff with explicitly unvalidated requirements; Indeed-hosted forms are not automated.

Two real-browser local employer fixtures prove the supported DOM controls and state transitions. They do not prove production Workday submissions. See docs/browser-testing.md for prerequisites and limitations.
