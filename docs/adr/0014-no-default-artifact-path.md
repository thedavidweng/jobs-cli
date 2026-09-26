# Application Artifacts are never written by default path

`apply prepare` does not choose a silent default filesystem path. In human-readable mode, `--out <path>` is required to write an Application Artifact file; in JSON mode, the artifact may be emitted on stdout for the Agent to store. This avoids leaking answers/resume paths into cwd or the profile directory without an explicit choice.
