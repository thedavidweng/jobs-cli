# LinkedIn auth uses guided browser session import on all major OSes

`jobs-cli auth linkedin login` follows a GitHub-CLI-like interaction: open LinkedIn’s login page, wait for the user to finish in their browser, then import the Voyager session from the local browser cookie store into a per-profile session file (0700/0600). This is session import, not LinkedIn OAuth—Voyager has no official token grant for this CLI. The CLI does not automate username/password entry or checkpoint bypass.

Browser selection auto-detects (Chrome → Safari → Firefox, with platform-appropriate equivalents) and may be overridden with `--browser`. v1 supports macOS, Linux, and Windows. If a supported OS/browser combo cannot read cookies, the command fails with a clear, actionable error rather than making manual cookie flags the primary path.
