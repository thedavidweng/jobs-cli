## Problem Statement

求职者需要在 jobs-cli 中完成发现 Job、解析 Application Target、复用履历、填写申请、审阅和确认提交。许多雇主使用同一 Application Provider，例如银行的 Workday；当前多数 provider 仅返回 Browser Required，求职者仍需逐项手工填写。现有 Candidate 不包含工作和教育经历，也不能直接读取 JSON Resume。公开职位 API 与获准提交申请的 API 被混淆，会导致能力声明不可靠。

此前新增的 YZi、CivicInfo 与 TransLink discovery 已在工作区实现，本次应保留这些成果。当前 spec 聚焦统一申请数据和真实 Workday 执行能力；不重新实现 discovery，也不把新 source 的解析逻辑混入申请 adapter。

## Solution

支持显式 JSON Resume 输入、针对职位的申请答案和定制 PDF；沿用 inspect → prepare → submit 与可审阅的 Application Artifact，增加明确的浏览器填表操作。实现一个可复用于不同雇主的 Workday Application Provider，动态读取当前申请的字段、选项、经历 sections 和问题，填写并在需要用户操作时暂停，完成审阅后按明确确认提交，最后返回真实回执。

Indeed 发现的 Job 继续按实际 Application Target 路由到外部 Application Provider。Indeed 自有申请保留官方界面交接及可准备的资料，不实现未经官方允许的 Indeed Apply 自动化。准确区分链接交接、表单 inspection、准备、浏览器填表和已验证提交能力。

## User Stories

1. As a job seeker, I want to provide a standard JSON Resume file, so that I can reuse my professional history across Application Providers.
2. As a job seeker, I want to provide a tailored JSON Resume for each Job, so that the application uses the history and wording I reviewed.
3. As a job seeker, I want to attach a separately tailored resume PDF, so that the employer receives a supported document.
4. As a job seeker, I want to attach an optional cover letter, so that supporting documents can follow the same application workflow.
5. As a job seeker, I want schema validation with useful errors, so that malformed resume data is rejected before remote changes.
6. As a job seeker, I want explicit manifest values to override resume values, so that application-specific corrections are deliberate.
7. As a job seeker, I want my complete name preserved and split names requested when needed, so that the CLI does not guess my identity.
8. As a job seeker, I want date precision preserved, so that missing months or days are not invented.
9. As a job seeker, I want work experience and education represented as repeated sections, so that my full history can be entered.
10. As a job seeker, I want skills, language and qualification values matched to actual form options, so that unsupported choices are reported rather than fabricated.
11. As a job seeker, I want work authorization, sponsorship, salary expectations and notice period supplied separately, so that they are not inferred from my resume.
12. As a job seeker, I want disclosures and consent questions displayed with their actual wording, so that my answers apply to this specific application.
13. As a job seeker, I want unanswered required questions listed precisely, so that I can complete the application without re-entering known information.
14. As a job seeker, I want discovery Source and Application Provider kept separate, so that a Job discovered on Indeed can be applied to through Workday.
15. As a job seeker, I want the same Workday adapter to work across different employers, so that support does not require a separate implementation for each bank.
16. As a job seeker, I want employer-specific fields and questionnaires inspected dynamically, so that the CLI follows the current Job's requirements.
17. As a job seeker, I want to connect an explicit browser session, so that the CLI uses the account and application I intended.
18. As a job seeker, I want authentication scoped to the employer, so that one employer's session is not assumed to authenticate another.
19. As a job seeker, I want login and email verification to pause the workflow clearly, so that I can complete them myself and continue.
20. As a job seeker, I want preparation to remain free of remote writes, so that I can validate an Application Artifact safely.
21. As a job seeker, I want an explicit fill operation, so that uploading documents and saving browser form values are intentional actions.
22. As a job seeker, I want existing saved application values inspected before filling, so that the CLI avoids duplicating my work history.
23. As a job seeker, I want pending steps and unsupported sections reported accurately, so that I know what remains to be done.
24. As a job seeker, I want to continue a paused workflow with the same reviewed data, so that login or missing answers do not force a fresh application.
25. As a job seeker, I want to review the completed application before submission, so that the final action uses the values I approved.
26. As a job seeker, I want changed requirements to invalidate an old Application Artifact, so that stale answers cannot be submitted.
27. As a job seeker, I want read-only and dry-run modes to block browser writes as well as API submission, so that existing safety controls remain reliable.
28. As a job seeker, I want submission to require explicit confirmation, so that filling forms cannot accidentally submit an application.
29. As a job seeker, I want a receipt or confirmation evidence before a submitted result, so that finishing the last form step is not mistaken for success.
30. As a job seeker, I want an uncertain submission result reported without automatic retry, so that a lost response does not create duplicate applications.
31. As a job seeker, I want assessments and additional Candidate Home tasks identified separately, so that a submission receipt is not confused with completion of the entire hiring process.
32. As a job seeker, I want Indeed-hosted applications routed to supported official tooling, so that the CLI does not pretend partner APIs are arbitrary candidate submission APIs.
33. As a job seeker, I want accurate Greenhouse capability declarations, so that unsupported submission credentials are detected before I rely on them.
34. As an Agent, I want machine-readable capabilities and pending-action states, so that I can orchestrate the workflow without parsing prose.
35. As an Agent, I want versioned Application Artifacts without passwords or cookies, so that I can review and transfer application data separately from authentication.
36. As a maintainer, I want existing Source and Application Provider behavior preserved, so that expanding Workday does not regress native inspection or new discovery sources.
37. As a maintainer, I want one adapter proven against two employer configurations, so that cross-employer reuse is demonstrated rather than advertised.
38. As a maintainer, I want checked documentation and examples, so that commands do not imply unimplemented or unverified functionality.

## Implementation Decisions

- Preserve Source / Application Provider separation and existing resolver identifiers. Use one Workday Application Provider parameterized by tenant, career site and Job identifiers; do not create company-specific providers or a dynamic plugin framework.
- Add explicit `--resume-json` input to application preparation. Retain `--resume` for an upload document and existing manifest / answers input; do not reuse global `--profile` or add a persistent candidate-profile database.
- Pin the supported JSON Resume schema to an identified version. Validate JSON Resume structure first, then validate the current application's requirements. Preserve unmapped resume sections in the reviewed data or explicitly report unsupported fields; do not silently drop required information.
- Extend shared applicant data only for the sections actually needed by the Workday implementation: complete identity and address, repeated work / education, and relevant skill / language / qualification sections. Preserve date precision and full names; require explicit values when a form demands data that the resume cannot supply.
- Define deterministic precedence: JSON Resume is the base, manifest candidate data overrides it, and explicit application answers remain bound to inspected question identifiers. Preserve existing manifest and attachment override behavior. Never infer authorization, salary, consent or demographic answers.
- Evolve the Application Artifact schema deliberately. Retain existing native workflows and readable previously supported artifacts where their semantics remain valid; unsupported artifact versions fail explicitly. Authentication material remains outside the Application Artifact.
- Reuse the current prepare validation and artifact-freshness checks. Browser inspection must describe current sections, required status, available options and conditional questions sufficiently for preparation; do not introduce an all-ATS form language before actual Workday forms require it.
- Add an explicit `apply fill` operation consuming a prepared Application Artifact and an explicit browser connection. Browser connection options must be available consistently to Workday inspection, preparation, filling and submission. Do not require Codex-specific tools for normal installed jobs-cli execution.
- Use a real browser execution path for Workday rather than promising a public candidate submission API. Prefer connection to the user's already running browser through an explicit local endpoint; select one maintained Go browser-control dependency only if existing dependencies cannot provide the capability. Do not build a browser automation framework or undocumented HTTP submission client.
- Inspect / prepare do not create remote applications, upload files, change profiles or advance steps that save data. If an application must first be opened or started, expose a precise pending action rather than quietly mutating during inspection. Browser fill can start or advance an application and may autosave data, so apply the mutation gate to fill as well as final submission.
- Browser fill requires explicit confirmation for writes; read-only blocks it and dry-run only reports planned actions. Filling must stop at review and never activate the final submission control.
- Workday runtime reads labels, section structure and options from current pages. Handle repeats deliberately: inspect existing rows before inserting, reconcile only clear identity matches and surface ambiguous rows rather than duplicating or destructively replacing history.
- Represent authentication, missing answers, unsupported steps, review readiness, submission confirmation and uncertain results in stable machine-readable outcomes. Keep browser workflow state outside credentials-bearing artifacts; resumed execution re-inspects the current page and uses the same target and reviewed data.
- A session belongs to the relevant employer tenant / origin; do not assume a global Workday candidate account. Users perform registration, email verification, MFA and assessments themselves. Do not save browser credentials or dump cookies into logs.
- Preserve `apply submit --artifact ... --confirm`. Add explicitly separate browser-fill and browser-submit capability semantics; keep `native_submit` truthful. A provider that only returns a URL must never claim browser fill or submit support.
- Before submission, verify the target, current questions and reviewed values. If the form or answers changed, require renewed preparation / review rather than a force bypass. Dynamic multi-step requirements require freshness checks at the relevant steps, not only the initial page.
- Record submitted only from reliable confirmation or receipt evidence. Never retry an application write automatically after an ambiguous failure; expose uncertainty and recovery instructions without asserting success.
- Audit the existing Greenhouse native-submit claim against official authentication requirements and prior receipts. Public GET and absence of candidate login do not establish permission for anonymous POST. Require appropriate employer credentials for the documented API path, or correct capabilities and errors if a candidate submission path is not independently verified; do not test this by sending real applications.
- Preserve Indeed discovery and external Application Target resolution. For Indeed-hosted apply, provide accurate browser handoff and locally reviewable application inputs where possible, clearly distinguishing unvalidated provider requirements. Do not automate Indeed forms without a specifically permitted official integration.
- Preserve ADR-0001, ADR-0002, ADR-0009 and ADR-0018. The browser workflow deliberately reopens the current product boundary that keeps Workday wizard execution outside the CLI. Update the domain definitions of Browser Required / Session / Capability and the spec with a focused ADR for the new browser runtime; do not silently reinterpret existing flags or claim browser handoff was automation.
- Keep the existing YZi, CivicInfo and TransLink work intact. Their browser handoff providers do not gain automatic filling merely because Workday is implemented. No new fallback, speculative compatibility layer or ignored-error success path.

## Testing Decisions

- Primary seam: the existing CLI command integration harness, exercising public commands and JSON output through injected provider / HTTP boundaries; extend it to deterministic browser-backed Workday pages. Tests assert observable artifact content, pending actions, browser changes, mutation gating and receipts, not private helpers or selector spelling.
- Use two local Workday-shaped employer fixture configurations, served to a real browser connection: different required fields / labels, repeated work and education, options, conditional questions and review / confirmation steps. Exercise the same adapter code for both. These are controlled fixtures, not proof of production tenant submission.
- TDD at this agreed seam: cover valid and invalid JSON Resume, explicit override precedence, names and dates requiring clarification, missing required questions, attachment selection, cross-employer reuse, resume-after-login, no duplicate repeated rows, review stopping, stale requirements, read-only / dry-run / confirmation gates, successful receipt and ambiguous submission without retry.
- Preserve existing prior art: CLI prepare / submit round trips, provider contract tests, Greenhouse application tests, artifact freshness tests and discovery-source tests. Prefer expanding those tests over adding internal testing interfaces.
- A browser-enabled fixture check must be runnable locally and in an explicitly documented test environment. Missing required browser prerequisites must be clearly reported; do not report browser behavior verified solely from mocks or silently skipped browser tests.
- Run Go build / type checks and targeted changed tests regularly. At completion run the full test suite once along with formatting and lint checks; repeat only if a new change or failure warrants it.
- Live verification is limited to public read-only pages. Do not register accounts, start real saved applications, upload user documents or submit real Jobs during implementation. Production authenticated tests require a later explicitly authorized environment; publish exactly which behavior is fixture-verified versus live-verified.
- Review the finished change using the code-review skill, address actionable findings, and commit the implementation on the current branch as required by implement. Existing conversation-owned source changes must be preserved; inspect and document the baseline rather than resetting, stashing away or silently discarding them.

## Out of Scope

- Unapproved Indeed hosted-apply automation, speculative partner integration, or reverse-engineered Indeed submission endpoints.
- Native Workday candidate writes through private HTTP or employer integration APIs without suitable authorization.
- Automatic account creation, email/MFA/CAPTCHA bypass, fabricated questionnaire answers, assessment completion, mass application or auto-retry of writes.
- Browser fill for every other Application Provider in the same change. Lever, Ashby, PeopleSoft and YZi keep accurate existing capabilities and remain future adapters.
- New discovery implementations, Workday discovery aggregation, fixing CivicInfo Cloudflare with fallback scraping, or replacing the earlier three-source work.
- A profile database, automatic PDF / cover-letter generation, dynamic plugins, a generalized workflow language, or per-employer hard-coded adapters.
- Real applications to production Jobs, claiming production end-to-end verification from fixture results, merging or publishing a release.

## Further Notes

Research basis: the conversation's application-adapter / JSON Resume report and the official references linked there. Key primary references are [Workday application templates](https://doc.workday.com/admin-guide/en-us/human-capital-management/recruiting/job-applications/ijz1499291475964.html), [Workday Recruiting API](https://community.workday.com/sites/default/files/file-hosting/productionapi/Recruiting/v47.0/Recruiting.html), [JSON Resume](https://jsonresume.org/schema), [Greenhouse authentication](https://docs.greenhouse.io/job-board.html#authentication), [Indeed terms](https://co.indeed.com/legal?hl=en), and [Indeed Send Candidates scope](https://docs.indeed.com/legal-terms/send-candidates).

Implement in observable increments: accurate capability / credential handling and JSON Resume input; Workday inspection / fill / pause / review across two employer configurations; confirmed browser submission and receipt states. Deliver the complete scoped implementation rather than stopping after only the importer. Do not broaden to other providers until this shared path has passed its acceptance checks.

The implementation session must use GPT-6.1 Sol with medium reasoning and the implement skill, including TDD at the agreed seam, final code review and a commit. No real application is authorized by this spec.

Testing-seam confirmation: user approved the existing CLI integration seam, two local employer browser fixtures and read-only live checks on 2026-09-30. No production application tests are authorized.
