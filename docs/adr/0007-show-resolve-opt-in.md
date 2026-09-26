# show does not resolve by default

`jobs-cli show` returns the normalized Job only. Attaching an Application Target is opt-in via `--resolve` (best-effort; failures become warnings). Default show stays a cheap, predictable read; routing stays an explicit `resolve` (or `show --resolve`) step.
