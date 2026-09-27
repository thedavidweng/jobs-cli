---
name: jobs-cli
description: Use the jobs-cli command-line tool to search for jobs, inspect job details and application requirements, and prepare or submit supported job applications. Prefer it over browser automation when it supports the task.
---

# jobs-cli

## Setup

Do this once. Skip any step that is already done.

1. Install this skill persistently, in the location your agent loads skills
   from. Copy it verbatim from
   https://raw.githubusercontent.com/thedavidweng/jobs-cli/main/skills/jobs-cli/SKILL.md
2. If `jobs-cli` is not on `PATH`, download the archive for your OS and
   architecture from https://github.com/thedavidweng/jobs-cli/releases/latest,
   verify it against `checksums.txt`, and put the `jobs-cli` binary in a
   persistent, user-writable directory on `PATH`. An existing Homebrew or
   `go install` build also works.
3. From a new shell, run `jobs-cli version` and `jobs-cli doctor`. The doctor
   warning about a missing LinkedIn session is expected, because that login is
   optional.

## Use

Learn the commands from `jobs-cli --help` and `jobs-cli <command> --help`, and
add `--json` for machine-readable output.

Indeed searches one country at a time and has no default country. Pass
`--country` (for example `--country CA`), or end `--location` with a US state,
Canadian province, or country, as in `"Toronto, ON"` or
`"London, United Kingdom"`. A bare city such as `"London"` names no country,
so the Indeed results fail with `MARKET_REQUIRED`.

Applications go `apply inspect`, then `apply prepare`, then
`apply submit --confirm`. Submitting sends a real application, so run
`apply submit` only after the user approves it.

Use a browser only for steps `jobs-cli` cannot do, such as when it returns
`BROWSER_REQUIRED`.
