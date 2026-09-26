# Submit re-inspects Application Artifact freshness

Before a confirmed native submit, the CLI re-inspects remote application requirements and compares a schema fingerprint to the Application Artifact. On mismatch, submit fails and the user must prepare again. Submit does not offer a force bypass of that check: stale answers must not be sent under a confirmation that referred to an older form.
