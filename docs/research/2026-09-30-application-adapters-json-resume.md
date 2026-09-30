# 通用 ATS Adapter、Indeed Apply 与 JSON Resume 调研

日期：2026-09-30。范围：现有代码、官方文档及公开招聘站的只读调查。未注册账户、登录求职者账户或提交真实申请。以下将已验证事实与实现建议分别说明。

## 结论

应该按 **ATS 系统** 实现申请 adapter，例如 Workday、PeopleSoft、Greenhouse；雇主、站点、职位、问题和会话是 adapter 的运行时输入。Discovery source 独立：Indeed 找到的岗位可以交给 Workday 申请。现有 SourceAdapter / ApplyProvider 分离已经支持这个方向。

JSON Resume 适合作为统一履历输入；申请问题答案和上传文件另行提供。实现目标应是“发现 → 解析目标 → 读取表单 → 填入资料与附件 → 补充答案 → 审阅 → 提交 → 保存回执”，并能在需要账户验证或回答新问题时继续执行。

不能把“已识别 ATS”“有公开职位 API”“能打开浏览器”当成“已经完成申请自动化”。尤其 Indeed hosted apply 与雇主 ATS 的可接入范围不同。

## 现有实现实际支持什么

| 部分 | 当前实现 | 对新目标的影响 |
| --- | --- | --- |
| Discovery / Apply 分离 | SourceAdapter 与 ApplyProvider，独立 registry | 可以复用同一个 Workday provider，不需要每家银行一个 provider |
| 申请流水线 | inspect → prepare → submit，版本化 Application Artifact，提交前重读表单 fingerprint | 继续沿用，无需重做一整套申请管线 |
| Candidate | 姓、名、邮箱、电话、location、LinkedIn、website | 尚无工作、教育、语言等可重复结构，也没有 JSON Resume importer |
| Greenhouse | 读取问题、准备 artifact、multipart submit 实现 | 有申请代码，但公开提交权限存在下述待核实矛盾 |
| Workday / 多数其他 ATS | BROWSER_REQUIRED + 目标 URL | 当前是交接链接，没有会话、表单检查、填表或 wizard 执行 |
| browser provider 的 prepare | 先调用 Inspect，收到 BROWSER_REQUIRED 即结束 | 仅加入 JSON importer 不会自动让 Workday prepare 可用 |
| LinkedIn Easy Apply | inspection 实现；native submit 被明确禁用 | 尚未完成受控的真实提交验证 |

代码依据：[provider 接口](../../internal/domain/provider.go)、[Candidate / Artifact](../../internal/domain/artifact.go)、[prepare](../../internal/cli/apply_prepare.go)、[submit](../../internal/cli/apply_submit.go)、[Workday provider](../../internal/workday/provider.go)、[Greenhouse provider](../../internal/greenhouse/provider.go)。现有 [ADR-0001](../adr/0001-source-vs-application-provider.md) 和 [ADR-0002](../adr/0002-application-artifact-handoff.md) 可保留。[ADR-0018](../adr/0018-no-candidate-profile-store.md) 允许继续采用显式文件输入，不必先建个人资料数据库。

## Workday：共用一个 adapter，可行；固定一套表单，不可行

Workday 官方允许配置履历、工作、教育、语言、技能等 section，改变标签和必填状态，并关闭简历解析；因此不能假定每个职位都有 Autofill with Resume，也不能用固定字段列表覆盖所有雇主。[Job Application Templates](https://doc.workday.com/admin-guide/en-us/human-capital-management/recruiting/job-applications/ijz1499291475964.html)

同一租户的不同 requisition 可以配置不同问卷；申请时和后续阶段都可能出现问题。工签、sponsorship、搬迁、薪资和 notice period 属于显式申请答案。[Recruiting Questionnaires](https://doc.workday.com/admin-guide/en-us/human-capital-management/recruiting/questionnaires/san1392075077658.html)

是否必须建立 Candidate Home 账户由租户配置；账户还可能收到验证邮件、补充问卷、电子签名及 assessment 等任务。工程上按雇主 tenant / browser origin 隔离会话，不能假定存在跨雇主通用账户；也不能断言所有站点都必须注册新账户。[Candidate Home Accounts](https://doc.workday.com/admin-guide/en-us/human-capital-management/recruiting/career-sites/gtv1538650489786.html)

Workday 确实有 Recruiting SOAP API，包括 Put_Candidate 和附件接口；但 Put_Candidate 有 contextual security。此次没有找到可供普通外部求职者跨任意雇主使用的官方公共提交 API。雇主授权的 integration 与求职者 Candidate Home 登录不是同一权限体系。[Recruiting API v47.0](https://community.workday.com/sites/default/files/file-hosting/productionapi/Recruiting/v47.0/Recruiting.html)、[Put_Candidate](https://community.workday.com/sites/default/files/file-hosting/productionapi/Recruiting/v47.0/Put_Candidate.html)

公开 discovery 已做只读实测：CIBC 官网链接到 `cibc.wd3.myworkdayjobs.com/search`；其页面配置 tenant=cibc、siteId=search。POST `/wday/cxs/cibc/search/jobs`，body 为 `{"appliedFacets":{},"limit":1,"offset":0,"searchText":""}`，调查时返回 HTTP 200、total=483 和 jobPostings/facets。这是网站前端使用的公开接口，**不是官方支持的公共 API 合约，也不是申请写入验证**。[CIBC 招聘流程及官网入口](https://www.cibc.com/en/about-cibc/careers/hiring-process.html)

推荐：一个 Workday 浏览器 adapter，共用履历 section 填写、文件上传、当前步骤检查、review 和回执验证；读取每份职位的真实字段、选项与条件问题。账户验证和 assessment 返回明确的待用户处理状态。CIBC 明确提交之后无法编辑，故 review 必须发生在提交前。[CIBC Hiring Process](https://www.cibc.com/en/about-cibc/careers/hiring-process.html)

这里是可行性判断，尚无跨多个雇主的登录后填表和提交实测。应先用两个真实租户验证复用，再扩大覆盖。

## Indeed Quick Apply：需区分三个表面相似的能力

1. **Indeed Apply / Easily Apply**：Indeed 自己用求职者资料提供的申请体验；官方集成文档主要面向发布职位、配置问题和接收申请的 ATS / 雇主合作方。[Indeed Apply](https://docs.indeed.com/indeed-apply)
2. **Send Candidates API 的 application.submit**：将 ATS 的候选人数据同步到 Indeed，要求合作方配置、注册雇主及 sendApplications 权限。这个名字不意味着任意求职者可以用它投递任意 Indeed 职位。[API guide](https://docs.indeed.com/send-candidates-api/send-candidates-api-guide)、[官方用途说明](https://docs.indeed.com/legal-terms/send-candidates)
3. **Apply For Me**：Indeed 自己的有限测试。2026-08-04 的更新说明已暂停自动申请模式，转向用户审阅草稿后提交；最初测试面向有限美国求职者。不能承诺在加拿大可用，也不是已公布的 jobs-cli 接口。[Indeed 官方更新](https://www.indeed.com/news/releases/indeed-tests-apply-for-me-job-search)

Indeed 的求职者条款明确禁止官方 vendors / tooling 之外的脚本、机器人自动化 Indeed Apply。条款同时提到可能提供 Permitted AI Connector，但可用功能由 Indeed 限定；这不是对任意 CLI 自动填写或提交的普遍授权。本次没有找到适用于 jobs-cli 的已开放求职者提交接入方式。[Indeed Terms，Job Seekers 与 Third Party Automations](https://co.indeed.com/legal?hl=en)

因此当前可落地的是：Indeed discovery → 外部 ATS adapter 完成申请；Indeed hosted apply → 准备资料并交给官方申请界面。若要自动填写 Indeed 自有表单，应先确认官方合作方或允许的 connector 接入范围。不能把“只填、不点击 Submit”默认当成条款已允许。

## 其他 ATS：公开读取与提交授权必须分开验证

| ATS | 官方证据 | 求职者 CLI 的结论 |
| --- | --- | --- |
| Greenhouse | GET 职位/问题公开；同一 Board API 的 POST 提交要求 Basic Auth + Job Board API key | 普通求职者不能默认拥有雇主 API key；优先核实已有实现 |
| Lever | postings API 可读；程序化 POST 要 API key，由雇主账户 Super Admin 创建；该 API 不暴露 custom questions | 不能把公开 listing API 推导成完整的无凭据 apply API |
| Ashby | public posting API 提供 applyUrl；applicationForm.submit 要 candidatesWrite，API Basic Auth 使用 API key | 求职者路径应研究 hosted form；雇主授权 API 是另一种部署场景 |

来源：[Greenhouse Authentication](https://docs.greenhouse.io/job-board.html#authentication)、[Lever 官方 Postings API](https://github.com/lever/postings-api/blob/master/README.md)、[Ashby Public Postings](https://developers.ashbyhq.com/docs/public-job-posting-api)、[Ashby applicationForm.submit](https://developers.ashbyhq.com/reference/applicationformsubmit)、[Ashby Authentication](https://developers.ashbyhq.com/reference/authentication)。

**现有 Greenhouse 存在需要纠正或补充验证的证据差异**：代码向上述 Board API 直接 POST，没有设置 Basic Auth；spec 却称这个 surface 无需 candidate authentication、已经 verified。无需“候选人登录”不等于无需“雇主 API key”。仅依据本次文档与代码，不能判断每个生产 board 的实际行为，但不能继续据此承诺无凭据通用提交。本次没有向真实职位发送测试申请；应先审查之前的成功回执证据，或在雇主授权的测试 board 上验证，而不是用真实申请来探测。

## JSON Resume 输入设计

JSON Resume 标准包含 basics、work、education、skills、languages、certificates、projects 等履历数据。它的 name 是完整姓名，不提供可靠的 given/family name 分拆；日期允许不同精度。履历 schema 校验通过也不代表满足某个岗位的申请必填项。[标准示例](https://jsonresume.org/schema)、[官方 schema](https://github.com/jsonresume/resume-schema/blob/master/schema.json)

建议保留标准 resume.json，不把各家 ATS 的问题 ID 塞进履历文件。Importer 读取并验证约定版本；保留原始履历，映射共享申请数据。相同字段以显式申请输入覆盖 resume.json 的值，未知必填项列为待补充。

| 输入 | 用途 | 不应自动推断的内容 |
| --- | --- | --- |
| resume.json | 姓名、联系方式、地址、工作、教育、技能、语言、项目 | 姓名拆分、日期缺失的日/月、ATS 选项枚举 |
| application answers / manifest | 针对职位的问题答案及显式覆盖 | 工签、sponsorship、薪资期望、披露与同意 |
| resume.pdf / cover-letter.pdf | 上传附件，可按职位定制 | JSON 文件不能默认代替 ATS 接受的简历附件 |

尤其 work 的 name / position、education 的 studyType / area、languages 的 fluency 不一定能直接匹配某雇主的下拉选项。读取真实选项，只有明确匹配才填写；不默默丢失经历，也不选择“看起来接近”的答案。

建议增加显式 `--resume-json`，保留已有 `--resume` 文件上传含义及 `--manifest` / `--answers`；不占用现有全局 `--profile`。PDF 渲染不是 importer 的前置依赖，初版接受用户已有 PDF。既有 prepare-input 合并逻辑可以复用。

以下仅是建议的未来命令形态，**当前尚未实现**：

```sh
jobs-cli apply prepare <job-id> \
  --resume-json tailored-resume.json \
  --resume tailored-resume.pdf \
  --manifest application-input.json \
  --out application.json

jobs-cli apply submit --artifact application.json --confirm
```

## 最小架构调整

保留 source、resolver、provider registry、artifact 和既有命令。按 ATS 新增真实的浏览器执行能力，不创建按公司分叉的 providers，也不先建动态插件平台。

浏览器路径需要读取当前步骤、填写与上传、暂停及继续；在 inspection 中表达可重复经历 section 和条件字段。用两个租户的实际表单确定最小数据结构，避免先设计一个覆盖所有 ATS 的巨大表单语言。浏览器会话与 artifact 分开：artifact 保留资料、答案、目标、附件和可审阅的计划，不存密码或 cookies。

能力不能只有 native_submit / browser_required：应明确区分“只能打开网页”“能 inspect”“能 fill”“能提交”；具体接口名在实现时再定。输出区分准备完成、待登录/补充答案、待审阅、已提交及提交结果不确定。填完最后一页不等于申请成功；只有确认页或可靠回执才能记录 submitted。网络失败后的写入不能盲目重试，沿用现有提交约束。

要满足独立 jobs-cli 的一站式目标，浏览器执行必须成为产品明确支持的运行路径，不能依赖每次 Codex 手动操作。可先使用用户现有浏览器及显式会话连接；再根据两个租户的试验确定执行依赖。当前 spec 刻意把浏览器申请排除在 CLI 外，真正实施时应更新该产品边界；这次调研没有改动 spec 或 ADR。[当前 Workday 边界与 Indeed 边界](../spec.md)

## 推荐顺序与验收

1. **先核实 Greenhouse 权限声明，并加入 JSON Resume importer。** 验收 schema 验证、显式覆盖、姓名和日期不猜测、附件路径及未回答问题；不需个人资料数据库或 PDF 渲染引擎。
2. **实现一个 Workday adapter 的填表到 review。** 在两个不同雇主租户使用同一份代码，覆盖履历、教育、附件、问卷、账户验证后继续。跨租户复用才证明 adapter 模块化有效；固定一家银行的 selectors 不算完成。
3. **补显式确认后的提交与回执。** 在授权测试环境验证；无法确认结果必须返回不确定状态。随后将已验证的浏览器机制复用于 PeopleSoft / YZi / 其他 ATS，仍逐个验证其流程。
4. **Indeed hosted apply 独立处理。** 保持 discovery → external ATS 路线；有官方允许的接入资格和明确能力后，再研究 Quick Apply 自动填写/提交。

本次成果是可行性、权限边界与最小实施路径；未实现 JSON Resume importer 或浏览器填表，也未证明任意 ATS 的端到端提交成功率。
