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
2. If `jobs-cli` is not on `PATH`, install it with the install script. Set
   `JOBS_INSTALL_DIR` to a persistent, user-writable directory that is
   already on `PATH`, because the `PATH` change the script makes may not
   reach your shell.

   macOS or Linux:

   ```sh
   curl -fsSL https://raw.githubusercontent.com/thedavidweng/jobs-cli/main/install.sh | JOBS_INSTALL_DIR=<dir> sh
   ```

   Windows, from PowerShell:

   ```powershell
   $env:JOBS_INSTALL_DIR = "<dir>"
   powershell -ExecutionPolicy ByPass -c "irm https://raw.githubusercontent.com/thedavidweng/jobs-cli/main/install.ps1 | iex"
   ```

   An existing Homebrew or `go install` build also works.
3. From a new shell, run `jobs-cli version` and `jobs-cli doctor`. A `WARN`
   line is a problem to fix or report to the user. An `info` line, such as an
   absent LinkedIn session, is optional setup and needs no action.

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
