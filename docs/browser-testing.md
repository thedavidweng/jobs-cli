# Workday browser workflow

Install a Chrome-compatible browser. jobs-cli attaches to an existing browser; it does not launch one or depend on Codex. Start a dedicated browser profile with a loopback debugging port, for example:

```sh
chromium --remote-debugging-port=9222 --user-data-dir=/tmp/jobs-browser
curl http://127.0.0.1:9222/json/list
```

Open the intended employer Job in that browser and use its tab `id`. Browser cookies remain in that profile. Use the same endpoint, tab, state and input files across commands:

```sh
jobs-cli --json apply inspect indeed:JOB --browser-endpoint http://127.0.0.1:9222 --browser-tab TAB
jobs-cli --json apply prepare indeed:JOB --resume-json tailored.json --manifest answers.json --resume tailored.pdf --out application.json --browser-endpoint http://127.0.0.1:9222 --browser-tab TAB
jobs-cli --json apply fill --artifact application.json --state application-state.json --browser-endpoint http://127.0.0.1:9222 --browser-tab TAB --confirm
jobs-cli --json apply submit --artifact application.json --state application-state.json --browser-endpoint http://127.0.0.1:9222 --browser-tab TAB --confirm
```

fill can autosave and upload. It requires confirmation; read-only blocks it and dry-run only reports a plan. prepare never starts a saved application. A pending start action can be performed by confirmed fill. Login/email/MFA/CAPTCHA and assessments are performed by the user. Rerun fill after that action; it re-inspects the existing tab.

`requirements_changed` includes the new inspection. Prepare again with `--previous-artifact application.json` plus the same resume, manifest and attachment flags, then rerun fill. Full names are not split automatically; provide explicit first_name/last_name in the manifest. Year-only history cannot populate a month/day-required field; provide the actual date rather than invented precision. Section options must match an actual option value or exact label. Ambiguous saved rows pause for manual reconciliation.

When a new questionnaire appears, add its answers to the manifest during renewed preparation and keep the same state file. New answers must belong to an inspected, uncompleted step; uploads and completed steps are retained. Previously supplied answers cannot change or disappear, and adding an answer to a completed step requires returning to that step for renewed review.

`review_ready` means review the browser and artifact before submit. State binds Candidate/answers/documents and browser review values; changes invalidate that review. Return to the affected step and prepare/fill again. Use a new state file only for deliberately changed reviewed input; it cannot submit directly from a pre-existing review page. Do not delete state to retry an uncertain write.

`submission_uncertain` means the click was attempted but no reliable receipt was observed. Further submit calls inspect for a receipt and do not click again. Check Candidate Home yourself. A receipt may still leave assessment or Candidate Home tasks.

Supported controls are labeled native inputs, textarea/select/checkbox/radio controls and the inspected Workday repeated section/navigation automation controls. Unrecognized widgets, section fields or unavailable custom-combobox options pause as `unsupported_step`; the CLI does not guess values. There is no claim that every Workday tenant template is supported.

## Run the fixtures

```sh
JOBS_TEST_BROWSER=/absolute/path/to/chrome go test ./internal/cli -run TestWorkdayBrowser -count=1 -v
```

Without JOBS_TEST_BROWSER the browser fixture test explicitly reports skipped. Browser-enabled CI must set that variable to an installed executable; Go mocks alone do not verify browser behavior. Ordinary unit/CLI tests run without a browser.

The Linux CI gate sets JOBS_TEST_BROWSER to the runner's installed Google Chrome and runs these fixtures as part of the full suite. Missing Chrome fails that setup step rather than silently skipping browser coverage.

Both employer fixtures use a real Rod CDP connection and the same adapter; prior-page form controls are removed on transition. They vary required phone fields and question wording and cover repeated work/education, option matching, file upload, explicit fixture application start, new questionnaire steps, login pause/resume, read-only/dry-run/confirmation, stale requirements, review changes, changed attachment bytes, radio values/display labels and changed selections, unwritable-state rejection before writes, positive receipt and ambiguous submit without retry.

Live checks on 2026-09-30 fetched only the public CIBC and BMO Workday career landing pages. Both are JavaScript application shells; no authenticated application form, saved application, account registration, file upload or production submission was exercised. Production end-to-end verification remains unperformed.

Schema attribution: internal/resumejson/schema.json is JSON Resume resume-schema v1.0.0, from https://github.com/jsonresume/resume-schema/tree/v1.0.0. It is vendored so preparation needs no schema network fetch.
