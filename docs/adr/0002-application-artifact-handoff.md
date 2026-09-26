# Application lifecycle uses an Application Artifact

`apply` is split into inspect → prepare → submit. Prepare validates candidate-supplied answers and files and writes a versioned JSON Application Artifact; submit consumes that artifact under the shared mutation gate (`--read-only` / `--dry-run` / `--confirm`). Humans default to a file path (`--out` / `--artifact`); Agents may use stdout and stdin (`--artifact -`). This keeps review on an explicit, reproducible payload instead of a one-shot `apply(job)` or a hidden profile cache of answers.
