# Spec - dbdlp 数据库敏感数据扫描发现平台 v2026.11.13.1

> 生成日期：2026-11-13
> 状态：**已确认**（用户 Phase 0 已确认四项决策；本文件为团队内部契约）
> 作用：设计、编码、测试、部署以本文件为唯一依据。范围／接口／数据／页面／令牌／验收标准全部锁定；变更走变更流程。

---

## 1. 产品定义

- **一句话**：离线只读、多源抽样扫描、识别敏感字段、形成可审计台账并输出仅含掩码样例报告。
- **目标用户**：安全合规专员（覆盖全、要可审计报告）、数据库管理员（可控可中断、不冲击生产）；次要为业务负责人、审计、运维研发。
- **核心问题**：用户要的是「不影响生产、可审计可复核可持续维护的敏感资产底数」，交付闭环工具而非单点引擎。

## 2. MVP 范围（锁定，不在此表的能力一律不做）

功能编号 F1～F15 完整定义见 `01-项目产品说明书PRD.md` 第 7 章。本表按能力域归并，为范围锁定索引；「主要依据」只标文档编号，精确章节以各文档目录为准。

| 能力域 | 锁定内容 | 优先级 | 主要依据 |
|---|---|---|---|
| 数据源纳管 | 统一数据源抽象；Oracle／达梦首批，MySQL／PostgreSQL 预留，openGauss 暂缓；数据源落库增删改查＋excel/yaml 批量导入 | P0 | 01、02、06 |
| 凭据与连接门禁 | ENC(base64) 加密存储、自动加解密、密钥失败即终止；只读连接与连接池；**连接测试通过才保存** | P0 | 02、06 |
| 元数据与扫描调度 | 模式／表／字段／注释／加密列采集；每列默认抽样 20 行、主键范围分页、分批限流、限速、低峰、断点续扫、流式读取 | P0 | 01、02 |
| 敏感识别 | 自研四层流水线（字节扫描器→正则→校验→自定义 YAML），godlp 仅参考、零依赖 | P0 | 02、05 |
| 聚合分级 | 命中率、置信度、存储形态、可配置风险分级、规则版本快照 | P0 | 02、05 |
| 台账与复核 | 任务、字段台账、命中明细、复核结论分存；复核不覆盖原始识别结论 | P0 | 02 |
| 报告 | Excel／HTML／JSON 三格式全出；只放掩码样例；导出记审计 | P0 | 01、02 |
| 网页控制台 | 12 个页面、资产数据源增删改查与批量导入、任务与报告可视化 | P0 | 04 |
| 配置与日志 | dbdlp.toml 三级优先级；五级日志默认 INFO、按日轮转保留 32 天 | P0 | 01、02、09 |
| 任务调度与审计（P1） | 定时触发、失败重试、跳过记录、操作留痕 | P1 | 01 |
| 视图／过程扫描（P1） | 视图定义与过程源码中的敏感字段引用 | P1 | 01 |
| 语义增强（P2） | 人名／地址／公司名以自研规则层覆盖（中低置信度＋强制人工复核），语义模型列后续 | P2 | 05 |

## 3. 明确不做（Out-of-Scope，锁定）

| 不做的能力 | 原因 | 何时考虑 |
|---|---|---|
| 实时流量拦截／网关代理 | 离线盘点，不接入业务流量 | 有实时管控需求另立项目 |
| 在线动态脱敏与数据改写 | 全程只读，识别为主 | 后续可选算子 |
| 替代数据库权限体系／访问审批 | 不替代 DBA 权限 | 有工单集成需求时 |
| BLOB 图片／文件 OCR | 二进制默认跳过，OCR 另立链路 | 专项立项 |
| 任何 UPDATE／DELETE／DDL | 只读硬约束 | 永不 |
| 主备／数仓自动血缘遍历 | 需额外元数据与权限 | 后续迭代，本期靠实例清单手动纳入 |
| dexp 备份集扫描 | 非在线库范围 | 后续独立采集器 |
| openGauss 适配器 | 官方驱动 import 路径／驱动名／DSN 未核实 | U-01 解除后排期 |
| SM4-GCM-SIV | 库能力已具备但导出签名未核实；本期 0x01 用已核实的 SM4-GCM | U-02 解除后新 ADR＋新标签＋旧密文兼容回归 |
| SSO／LDAP 集成 | 本期单管理员＋会话 | 有企业身份源后 |
| 自适应采样 | 缺真实库表分布数据 | 有线上扫描数据后 |
| NER 语义模型 | 破坏离线单二进制部署约束，且缺基线评测 | 后续专项 |

## 4. 技术架构（锁定，版本锚定）

| 层 | 技术 | 锁定版本／取值 | 锁定原因 |
|---|---|---|---|
| 后端语言 | Go | 1.25.10+ | 总纲锚定；不因依赖提升到 1.26 |
| 架构 | 四层 DDD ＋ 整洁架构 ＋ TDD | domain／application／infrastructure／interfaces，依赖只向内 | 总纲；单文件 ≤ 300 行，入口只装配 |
| 数据库访问 | database/sql ＋ 统一 Driver 端口 | 方言适配器 | 多源统一抽象 |
| 首批驱动 | sijms/go-ora/v2（Oracle）、gitee.com/chunanyong/dm（达梦） | 已联网核实签名 | 总纲优先落地 Oracle 与达梦 |
| 预留驱动 | jackc/pgx/v5/stdlib、go-sql-driver/mysql | 已核实 | 后续扩展 |
| 存储 | SQLite（WAL）／modernc.org/sqlite 纯 Go | `data/dbdlp.db`；可选 PostgreSQL | 离线单二进制、免 CGO |
| 前端 | TypeScript／Next.js／React | 7.0+／16.2+ App Router／19.2+ | 总纲锁定，不混用 |
| 前端配套 | Tailwind CSS、ECharts、Zustand、shadcn/ui、@tanstack/react-table | ECharts 6.0.0；react-table 8.21.3（v9 破坏性变更不追新）；Zustand 精确版本「待验证」 | 已核实部分锁定 |
| 图标 | lucide-react | **1.72.0**，v1 canonical 名，16／20／24px | npm 实时版本＋包内类型声明核验；全项目唯一，禁 emoji 功能图标 |
| 报告 | excelize/v2、HTML 模板、JSON | excelize tag「待验证」，须选 Go 要求 ≤ 1.25.10 | 工具链不提升 |
| 配置 | BurntSushi/toml ＋ dbdlp.toml | 命令行 ＞ 配置文件 ＞ 默认值 | 总纲 |
| 加密 | AES-256-GCM（0x00）／SM4-GCM（0x01） | ENC(base64)，1B 标签＋12B nonce＋密文＋16B tag；32 字节密钥 | 总纲 |
| 认证 | 单管理员＋会话 | token 哈希存 `web_session` | 本期边界 |
| 部署 | Go 单进程同源托管前端静态资源 | 内网／信创离线 | 总纲 |
| 版本 | CalVer `YYYY.MM.DD.MICRO` | 2026.11.13.1 | 总纲 |

## 5. API 端点清单（锁定）

**唯一依据**：`02-架构设计文档.md` 第 13.2 节表格。前端页面接口期望（04 文档 2.4 节，50 余条）是页面级细化与参数变体，**不是新增端点**，须映射回本清单。

通用约定：`/api/v1/` 前缀；统一响应 `{ "code": 0, "data": {}, "message": "" }`，`code=0` 成功、`message` 一律中文；分页 `data` 固定 `{ "items": [], "total": 100, "page": 1, "limit": 20, "hasMore": true }`；除登录外全部需 `Authorization: Bearer <token>`；**凭据不出后端**（响应中 `password` 恒为 `"******"`，host／IP 按脱敏规则输出）；静态资源同源部署，不引入跨域。

| 分组 | 端点（路径 · 方法） | 条数 |
|---|---|---|
| 认证 | `/auth/login` POST、`/auth/logout` POST、`/auth/me` GET | 3 |
| 数据源 | `/datasources` GET／POST、`/datasources/{id}` GET／PATCH／DELETE、`/datasources/{id}/test` POST、`/datasources/import` POST、`/datasources/import/template` GET、`/datasources/{id}/metadata` GET | 9 |
| 扫描任务 | `/tasks` GET／POST、`/tasks/{id}` GET、`/tasks/{id}/progress` GET、`/tasks/{id}/pause` POST、`/tasks/{id}/resume` POST、`/tasks/{id}/retry` POST | 7 |
| 结果台账 | `/findings` GET、`/findings/{id}` GET、`/findings/{id}/hits` GET | 3 |
| 复核 | `/reviews` POST／GET | 2 |
| 规则 | `/rules` GET、`/rules/snapshots` GET | 2 |
| 报告 | `/reports` POST／GET、`/reports/{id}/download` GET | 3 |
| 仪表盘 | `/dashboard/summary` GET | 1 |
| 审计 | `/audit-logs` GET | 1 |
| 配置 | `/config` GET（只读回显生效配置，不含密钥与 DSN） | 1 |

> 门禁核对：**以 02 文档 13.2 节实列 32 条为准**（架构师回传 33 条为计数误差）；不得据此私自新增端点。

数据创建硬规则：`POST /datasources` 先只读连接测试、通过才保存、失败整条不落库，含凭据变更的 PATCH 须重新测试；批量导入逐条连接测试＋逐条独立事务，通过行入库、失败行不入库并逐行中文回显（不含明文凭据），返回 total/succeeded/failed/errors，允许部分成功、不整批回滚，重复导入幂等去重、失败行可导出修正重试。依据：总纲「当且仅当连接测试验证成功时保存」为逐条门禁语义，且 02 文档 13.2 的 import 响应体即部分成功结构（DIFF-01）。

## 6. 数据库表清单（锁定）

完整 DDL 见 `02-架构设计文档.md` 第 10.3 节（`internal/infrastructure/persistence/sqlite/schema.sql`）。共 14 张表：

| 表名 | 用途 | 关键关联 |
|---|---|---|
| `data_source` | 资产数据源实例（增删改查、批量导入落库） | 1:N → `scan_task` |
| `scan_task` | 扫描任务记录、状态、规则版本、摘要 | 1:N → `schema_meta`／`scan_checkpoint`／`report_artifact` |
| `schema_meta` / `table_meta` / `column_meta` | 模式／表／字段元数据与注释、加密标记 | 逐级 1:N |
| `finding_column` | 字段级台账（敏感类型、命中率、置信度、存储形态、风险等级、复核状态） | 1:N → `finding_hit`／`review_record` |
| `finding_hit` | 命中明细（仅掩码值、规则 ID、来源形态） | FK → `finding_column` |
| `scan_checkpoint` | 断点续扫位置 | FK → `scan_task` |
| `rule_snapshot` | 规则版本快照，保证历史报告可复现 | 按 `rule_version` 关联任务 |
| `report_artifact` | 报告产物与生成时间、路径 | FK → `scan_task` |
| `review_record` | 人工复核结论（不覆盖原始识别结论） | FK → `finding_column` |
| `web_user` / `web_session` | 控制台账号与会话（token 存哈希） | 1:N |
| `audit_log` | 所有写操作留痕（数据源变更、任务启停、复核、报告导出） | FK → `web_user` |

约定：所有表含 `id`／`created_at`／`updated_at`；软删除用 `deleted_at`（NULL 为未删除）；时间统一 `TEXT`，格式 `YYYY-MM-DD HH:MM:SS.SSS` 本地时间不含时区，与日志时间戳一致；`PRAGMA journal_mode=WAL`、`foreign_keys=ON`；`SetMaxOpenConns(1)` 避免 SQLite 单写者锁冲突。

## 7. 页面清单（锁定）

唯一依据：`04-前端控制台与报告设计规格.md` 第 2.1 节，共 **13 个页面**，Next.js App Router。

| 编号 | 页面 | 路由 | 对应 API 分组 | 优先级 |
|---|---|---|---|---|
| P01 | 总览工作台 | `/` | 仪表盘／任务／台账 | P0 |
| P02 | 数据源管理 | `/datasources` | 数据源 | P0 |
| P03 | 数据源新建／编辑 | `/datasources/new`、`/datasources/[id]/edit` | 数据源／连接测试 | P0 |
| P04 | 批量导入 | `/datasources/import` | 导入与模板 | P0 |
| P05 | 扫描任务列表 | `/tasks` | 任务 | P0 |
| P06 | 扫描任务详情 | `/tasks/[id]` | 任务进度／重试 | P0 |
| P07 | 结果台账 | `/findings` | 台账（TanStack Table 服务端分页） | P0 |
| P08 | 字段明细与复核 | `/findings/[id]` | 命中明细／复核 | P1 |
| P09 | 报告中心 | `/reports` | 报告 | P0 |
| P10 | 报告预览 | `/reports/[id]` | 报告下载 | P0 |
| P11 | 规则管理 | `/rules` | 规则清单与版本快照（只读） | P0 |
| P12 | 系统设置 | `/config` | 生效配置只读回显、审计日志、外观偏好 | P1 |
| P13 | 登录与会话 | `/login` | 认证／会话（单管理员＋会话） | P0 |

P03 用独立路由而非弹窗（刷新不丢草稿）；P08 桌面端右侧抽屉呈现并保留可分享深链。全站共用应用壳层；路由与后端资源名对齐，P13 对应本期单管理员＋会话认证范围。

## 8. 设计 Token（锁定）

- **唯一机器可读源**：`docs/design-tokens.json`（v1.0.0）与 `docs/design-tokens.css`，前端 import 引用，禁止组件写死色值。
- **分层**：primitive → semantic → component → extension；仅 primitive 段与 `value` 字段允许字面色值，组件样式一律 `var(--token)`，例外仅 `#fff`／`#000`。
- **主题**：浅色（light）；深色预留扩展，本期未实现。
- **主色**：navy 系为主干，accent 蓝 `#1D4ED8`；风险高用红 `red_600 #B42318`（中国用户习惯）；风险等级与任务状态色彩语义见 04 文档 4.4／4.5 节。
- **字体**：以令牌文件 `font` 段与 04 文档 1.7 节选型结论为准，全项目不混用第二套字体。
- **图标**：`lucide-react@1.72.0`，24×24 网格、2px 描边、currentColor；尺寸 16（行内）／20（按钮内）／24（独立）；**一律 v1 canonical 名**（`Trash`、`Ellipsis`、`CircleCheck`、`TriangleExclamationPoint`、`LoaderCircle`、`ChartColumn`），禁 0.x 旧别名；门禁用 04 文档 12.4 节 CI 校验脚本扫描导出名。
- **对标品牌**：见 04 文档 1.4／1.5 节；主视觉禁紫→粉渐变、禁千篇一律 Hero，直接展示真实产品内容。

## 9. 验收标准（锁定）

QA 执行以 `07-测试设计与验收规格.md` 为唯一用例依据；本 Spec 锁定其索引与放行门槛。

| 类别 | 依据 | 判定 |
|---|---|---|
| 功能验收 | 01 文档第 9 章 EARS 20 条 ＋ 第 10 章 Given/When/Then 15 条 | 逐条通过，不得跳过 |
| 规则有效性 | 05 文档正负样本集与召回／误报基线 | 有基线记录，自定义 YAML 规则可生效 |
| 只读安全 | 代码评审确认无写操作；报告与日志无完整明文 | 零明文出口 |
| 强校验基线回归 | 默认 INFO、`项目根目录/logs/`、日志格式串、保留 32 天，在 01／02／09 三份文档取值一致 | 任一处回退即判失败（见第 11 章） |
| 生产就绪 | 07 文档评分卡 | P0 缺陷归零方可放行 |

验收口径要点：报告样例一律掩码；IPv4 `192.168.10.3 → *.*.10.3`，`127.0.0.1` 与 `localhost` 保留完整；SQL 字面量、运行日志、报告三处强制脱敏；密钥获取失败必须直接终止且绝不回退默认密钥；单表失败不阻断整任务。

## 10. 边界与约束

- **只读**：仅只读账号，代码层禁止拼装写操作；连接凭据隔离于报告。
- **低侵入**：每列默认 20 行抽样、主键范围分页（禁深偏移）、分批 100～500 行、并发 1～4、批次间隔 0～200ms、低峰窗口、大表降级、CLOB 截断、BLOB 跳过、流式读取。
- **离线**：内网／信创环境单二进制，规则与配置随包，不依赖外网；`CGO_ENABLED=0` 可交叉编译。
- **配置**：`dbdlp.toml` 唯一入口，三级优先级；路径命名统一 `.dbdlp`（已对齐 `AGENTS.md` 总纲）。
- **日志**：五级 TRACE／DEBUG／INFO／WARN／ERROR，默认 INFO（FATAL/PANIC 内部保留）；时间戳 `YYYY-MM-DD HH:MM:SS.SSS` 本地时间；行格式 `YYYY-MM-DD HH:MM:SS.SSS  日志级别  trace_id  线程ID或进程ID  模块:行号 - 日志内容`；`项目根目录/logs/`；`dbdlp-YYYYMMDD.log`；按日轮转保留 32 天自动清理；UTF-8 中文；禁记密码、完整身份证号、手机号。
- **响应式与无障碍**：断点、对比度、键盘与焦点态按 04 文档「响应式与无障碍」章节执行。
- **待验证（不得凭记忆写签名）**：U-01 openGauss 驱动；U-02 国密 GCM-SIV 导出签名；U-05 excelize 与 Go 最低版本；U-07 Zustand 精确版本。
- **需真机验证**：目标 Oracle／达梦小版本的字典视图与分页语法，不得直接照抄进生产代码。

## 11. 内嵌已知坑（源自 `.workbuddy/memory/pitfalls.jsonl` 与 Phase 1 核实）

| 坑 | 触发条件 | 根因 | 修法／护栏 |
|---|---|---|---|
| 文档契约默认值冲突 | 总纲原文不可读时凭通用习惯写配置 | PRD 曾把日志默认级别写 INFO、目录归入 `data/`，与总纲冲突；写死默认值被当施工依据 | 字段级订正；默认 ERROR／logs/ 路径／格式串／32 天列为跨 01、02、09 强校验基线，07 文档落持久回归用例 |
| 凭记忆写第三方库接口 | 引用 SDK 签名时 | 旧文档写 godlp `Close() error`，真实签名无返回值；旧第 1.4 节整体未核实 | 一律联网核实后写入 02 文档「第三方库核实结论表」，未核实标「待验证」；本项目自研引擎已零依赖 godlp |
| 达梦驱动连接异常 | 使用 chunanyong/dm | `COMM_ENCRYPT_NAME=AES128_ECB` 致连接异常；Go 三段位与 DM 四段位版本不兼容 | 连接测试门禁拦截；版本组合列入部署检查单 |
| pgx 占位符差异 | 切换 PostgreSQL | pgx v5 用 `$1`，不支持命名参数 | 方言适配层统一处理占位符 |
| SQLite 锁冲突 | modernc.org/sqlite | 单写者；`_pragma` 须写入 DSN 对每个新连接生效 | `SetMaxOpenConns(1)`、WAL、`busy_timeout` |
| 图标旧别名失效 | lucide-react 升 v1 | v1 移除 0.x 别名（Trash2→Trash 等） | 全量改用 canonical 名，CI 校验脚本扫描导出名 |

## 12. 端到端验证步骤（Spec 锁定的验收流程）

> 下列命令为锁定流程，待实现完成后执行；含中文的请求体一律 UTF-8 显式编码发送（避免终端代码页造成伪 400）。

```bash
# 1. 构建（Go 1.25.10+，CGO_ENABLED=0 可交叉编译）
CGO_ENABLED=0 go build -o dbdlp ./cmd/scanner
npm ci && npm run build

# 2. 启动（dbdlp.toml；首次运行自动生成密钥）— 断言：启动成功；密钥存在且权限受控；logs/ 生成当日日志；默认 INFO
./dbdlp serve --config dbdlp.toml

# 3. 登录 — 断言：code=0 且返回 token；连续失败触发中文错误与锁定
curl -s -X POST http://127.0.0.1:8080/api/v1/auth/login \
  -H "Content-Type: application/json; charset=utf-8" \
  -d '{"username":"admin","password":"<占位>"}'

# 4. 数据源创建（先连接测试，通过才保存）— 断言：测试失败整条不落库；GET 中 password 恒为 "******"
curl -s -X POST http://127.0.0.1:8080/api/v1/datasources/{id}/test -H "Authorization: Bearer <token>"
curl -s -X POST http://127.0.0.1:8080/api/v1/datasources -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json; charset=utf-8" -d @datasource.json

# 5. 批量导入（逐条门禁 + 部分成功，不整批回滚）— 断言：返回 total/succeeded/failed/errors；失败行不入库、不含明文凭据；重复导入幂等
curl -s -X POST http://127.0.0.1:8080/api/v1/datasources/import \
  -H "Authorization: Bearer <token>" -F "file=@datasources.xlsx"

# 6. 发起扫描并轮询 — 断言：每列抽样 20 行；主键分页无深偏移；单表失败不阻断任务
curl -s -X POST http://127.0.0.1:8080/api/v1/tasks -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" -d '{"dataSourceId":"<id>","sampleRows":20}'
curl -s http://127.0.0.1:8080/api/v1/tasks/{id}/progress -H "Authorization: Bearer <token>"

# 7. 台账与报告 — 断言：报告只含掩码样例；命中明细无完整明文；导出写入审计日志
curl -s "http://127.0.0.1:8080/api/v1/findings?taskId=<id>" -H "Authorization: Bearer <token>"
curl -s -X POST http://127.0.0.1:8080/api/v1/reports -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" -d '{"taskId":"<id>","formats":["excel","html","json"]}'

# 8. 安全与日志复核 — 断言：logs/dbdlp-YYYYMMDD.log 无完整手机号/身份证/银行卡/密码；
#       日志行格式与时间戳符合第 10 章；轮转保留 32 天；data/ 下不含日志；grep 全库无明文凭据
```

## 变更流程

小改（不新增端点、不新增表、不影响超过 2 个页面、不改变核心流程）就地更新后继续；大改（新增端点 ≥ 2、新增表、影响超过 2 个页面、改变核心流程）须回到需求澄清并重走文档链。`docs/decisions/OPEN-DECISIONS.md` 未决项关闭时就地翻 `RESOLVED` 并补 `Resolution`，可升格为 ADR。
