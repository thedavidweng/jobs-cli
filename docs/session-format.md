# LinkedIn session file format

`jobs-cli auth linkedin login` imports the LinkedIn web session from a local
browser cookie store into a per-profile session file. The session file is the only
place Voyager authentication material is stored.

## Location and permissions

- Directory: `<config-dir>/sessions` — created with mode `0700`.
- File: `<config-dir>/sessions/<profile>.json` — written with mode `0600`.
- `<config-dir>` defaults to `~/.jobs-cli` on macOS/Windows and
  `${XDG_CONFIG_HOME:-~/.config}/jobs-cli` on Linux, overridable with
  `JOBS_CONFIG_DIR`.
- A profile may override the file path with `profiles.<name>.linkedin.session_file`.
- Session material is never embedded in `config.yaml` and is never printed by
  `auth status`, `doctor`, or any normal output path. `auth status` reports only
  presence, capture time, browser, and cookie *names*.

## JSON schema

```json
{
  "schema_version": "1",
  "provider": "linkedin",
  "profile": "default",
  "captured_at": "2026-09-26T12:00:00Z",
  "browser": "chrome",
  "csrf_token": "ajax:1234567890",
  "cookies": {
    "li_at": "<session secret>",
    "JSESSIONID": "\"ajax:1234567890\"",
    "li_atcsrf": "<optional>"
  }
}
```

### Field semantics

- `schema_version` — session-file schema, currently `1`. Bump only for breaking changes.
- `provider` — always `linkedin` in v1; reserved so the file format can host other session-backed providers.
- `profile` — the CLI profile the session belongs to.
- `captured_at` — RFC3339 UTC timestamp of the import.
- `browser` — detected or `--browser`-selected source browser (`chrome`, `safari`, `firefox`, ...). Informational.
- `cookies` — cookie name to raw stored value, at minimum:
  - `li_at` — long-lived LinkedIn authentication cookie. Required for Voyager.
  - `JSESSIONID` — LinkedIn session cookie. Its value is stored **verbatim**
    (including any surrounding double quotes) because the CSRF token semantics are
    derived from it.
  - `li_atcsrf` — optional companion cookie present in some browser stores.
- `csrf_token` — the value that adapters send in the `csrf-token` header. It is the
  `JSESSIONID` value with surrounding double quotes removed. When absent, adapters
  derive it from `cookies["JSESSIONID"]`.

Voyager requires the `li_at` and `JSESSIONID` cookies together with a `csrf-token`
header matching the unquoted `JSESSIONID`. A session with `li_at` but no
`JSESSIONID` (or vice versa) is incomplete: commands that need Voyager fail with
`LINKEDIN_SESSION_REQUIRED` rather than silently falling back to Guest.

## Consumers

- `internal/config` owns read/write/permission enforcement (`LinkedInSession`,
  `SessionStore.Load`, `SessionStore.Save`, `SessionStore.Remove`, and
  `SessionStore.Status`).
- `internal/cookieimport` produces a `LinkedInSession` from a browser cookie store.
- `internal/linkedin/session` orchestrates guided login/logout for the `auth` commands.
- `internal/linkedin/voyager` consumes the session for authenticated search/detail.
