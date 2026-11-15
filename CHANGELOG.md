# CHANGELOG

> 版本规范：CalVer `YYYY.MM.DD.MICRO`（年.月.日.迭代号）
> 说明：2026.11.14 起进入**代码实施**阶段，此前条目为文档体系迭代记录。

## [ 2026.11.15.1 ] - 0.5.0

### ✨ New Features 新增功能

- 【MySQL 方言驱动】新增 `internal/infrastructure/datasource/mysql.go`：基于 go-sql-driver/mysql 的只读适配器，覆盖 `Connect`/`ListSchemas`/`ListColumns`/`TableStats`/`SampleRows`；DSN 仅保留 `charset=utf8mb4&parseTime=true&loc=Local`（只读性由 `roConn` 守卫保障，不依赖驱动参数）；系统库排除 information_schema/mysql/performance_schema/sys。
- 【PostgreSQL 方言驱动】新增 `internal/infrastructure/datasource/postgres.go`：基于 lib/pq 的只读适配器，覆盖同组接口；`ListSchemas` 排除 pg_catalog/information_schema 与 `pg_%` 模式，`TableStats` 经 `pg_class`/`pg_index` 取主键，`SampleRows` 用 `$N` 占位符与 `LIMIT $N` 主键分页。
- 【方言白名单放开】`datasource.NewDriver` 注册 mysql/postgres 分支（`registry.go`）；CLI `add`/`test` 的 `--dialect` 帮助文本更新为 `oracle|dameng|mysql|postgres|sqlite`，应用层经工厂直接放行。
- 【Web 数据源导入错误处理】导入失败链路增强：前端 `web/components/business/import.tsx`/`stores/useImportStore.ts`/`services/datasources.ts` 归一化逐行错误并可读展示失败清单；后端 `internal/interfaces/http/task_handlers.go`/`views.go` 补齐契约字段；新增 `web/scripts/verify_import_errors.py` 校验导入错误响应。

### 🐛 Bug Fixes 问题修复

- 【HTML 报告非标准编码（中文预览乱码）】根因在 `internal/domain/mask/mask.go`：脱敏函数原按**字节**切片（`v[:1]`/`v[len-1:]`），中文「京A12345」(UTF-8 9 字节) 掩码后保留孤立首字节 `0xe4`，破坏多字节序列致整份 HTML 含非法 UTF-8 字节（与第八轮观测 0xe4/0xae/0x8a 解码失败吻合）。已改为按 `[]rune` 处理（`京A12345`→`京*****5`，`utf8.ValidString` 通过）；`reporter.go`/`excel.go` 加 `toValidUTF8`（`strings.ToValidUTF8(s,"\uFFFD")`）写出防御层，与 JSON 降级行为对齐。新增 `TestMaskGenericMultibyte`/`TestMaskEmailMultibyteLocal`/`TestReportersInvalidUTF8`（注入 `[]byte{0xe4,0x41,0x42}` 断言 HTML 整文件 `utf8.Valid`、xlsx 单元格内容 `utf8.ValidString`）。
- 【PostgreSQL 采样 LIMIT 占位符缺失】`SampleRows` 生成 `LIMIT $N` 但未把 `q.BatchSize` 追加进参数切片，首页（args 为空）触发 `pq: there is no parameter $1` 致采样失败、表 0 命中。修复为先 `args = append(args, q.BatchSize)` 再按 `len(args)` 生成占位符，与 `WHERE pk > $1` 自然衔接。
- 【MySQL DSN 非法参数】旧 DSN 含 `readOnly=true`，go-sql-driver/mysql 不支持（`Error 1193 Unknown system variable 'readOnly'`）。已移除，只读性由 `roConn` 守卫保障（与 PG 一致）。

### 📈 Improvements 性能/体验优化

- 端到端验证：在 `192.168.10.180` 容器矩阵（Oracle/达梦/MySQL/PG）完成登记与扫描；PG（`dbdlp_test`）与 MySQL（`dbdlp_test`）均 `状态=done | 表1 | 字段7 | 命中4 | 失败0`，HTML 报告 UTF-8 严格解码通过、中文脱敏正常显示（`138****8000`、`z****@example.com`、`6*****************3`）。报告产物 `reports/e2e_pg_20261114/`、`reports/e2e_mysql_20261114/`。
- 依赖新增 `github.com/go-sql-driver/mysql`、`github.com/lib/pq`（`go.mod`/`go.sum`）。

### 📚 Docs 文档更新

- `docs/01`~`docs/10` 多份修订（版本口径收敛、设计令牌内联、契约对齐等，详见各文档差异）；新增 `docs/11-前端设计复核报告.md`。
- 【文档收敛/删除】删除决策记录 `docs/decisions/ADR-001.md`、`ADR-002.md`、`ADR-003.md`、`OPEN-DECISIONS.md` 及 `docs/design-tokens.css`、`docs/design-tokens.json`（相关结论已合并/收敛至 `docs/04` 等正文，按本轮评审裁定不再单列）。

### 核验结果（可复现）

- `go build ./...`、`go vet ./...`、`go test ./...` 全绿（TEST_EXIT=0）；`gofmt -l .` 为空。

---

## [ 2026.11.14.5 ] - 0.4.2

### ✨ New Features 新增功能

- 【IT-06 单表失败不阻断】扫描编排由「全库一次跑完再统一提交」重构为「模式 → 表」分组逐表扫描（`internal/application/scan/pipeline.go`）：单表采样/识别失败只记 `failed` 断点并继续下一张表，**失败表的台账整体作废不入库**（避免重试产生重复行），台账改为**按表增量落库**（进程中断不再整批丢失）。任务状态按结果判定：有失败表 → `partial`，一张都没扫成 → `failed`，否则 `done`。
- 【失败表可重试 / 断点续扫】新增 `RetryFailed`、`RetryFailedTables`（支持 `targetIds` 定向重试「模式/表」）、`Resume`（重新枚举目标库、跳过 `done` 断点、覆盖中断后未触及的表）；`scan_checkpoint` 唯一键 `(task_id, table_meta_id, column_name)` 承载 `pending`/`failed`/`done` 三态。`port.Store` 新增 `ListCheckpoints`（返回全部断点，含 `done`），`ListPendingCheckpoints` 复用同一查询助手。
- 【接口与 CLI 接线】`POST /api/v1/tasks/{id}/retry` 由状态迁移存根改为**同步真实重扫**，响应 `{retried, newFindings, remaining, status}` 并写审计与 INFO 日志；错误按哨兵映射（`ErrTaskNotRetryable`→3001、`ErrNoFailedTable`/`ErrTargetNotFailed`→1004、其余→5000）。新增 `dbdlp retry --task` 与 `dbdlp resume --task`（均支持 `--report` 多格式、`--out`）。
- 【元数据幂等】`SaveMetadata` 改为先复用任务内已有的 `schema_meta`/`table_meta` ID，`column_meta` 走 `ON CONFLICT ... DO UPDATE`，支撑续扫补齐元数据而不撞唯一约束。
- 【内置规则补齐】新增 IPv6／MAC／域名三条内置规则与 `ipv6`／`mac`／`domain` 三个校验器（`internal/domain/rule`）：IPv6 覆盖完整 8 组／`::` 压缩／内嵌 IPv4 映射形态（`net.ParseIP` 精确判定，拒绝全压缩空地址 `::` 与双 `::`）；MAC 要求分隔符一致；域名校验标签字符集／长度／TLD 纯字母并排除 IPv4 形态。正负样本单测覆盖（负样本：双 `::`／9 组／含 `G`／5 组／混合分隔符／无 TLD 等），对应 `testdata` 预留列，复扫验证命中。
- 【HTML 审计报告八章节】`internal/infrastructure/reporter/html_report.go`（新文件，保持单文件 ≤300 行门禁）按 `docs/02` §14.1 与 `docs/04` §4 原型实现八章节 HTML 报告：扫描概况（KPI＋条形）→ 风险汇总（C1 环形 SVG＋C3 类型 TOP）→ 高/中/待复核清单 → 加密脱敏现状 → 整改建议 → 附录（规则清单＋任务记录）；风险徽章三重编码、掩码样例弱底、四级色彩语义与后端六值收敛。浏览器渲染验收通过。

### 🐛 Bug Fixes 问题修复

- 【前后端任务状态枚举错位】前端 `TaskStatus` 误把 `docs/04 §4.5` 的**设计令牌名** `--state-idle-*` / `--state-success-*` 当成接口枚举值（`idle`/`success`），而后端 `port.TaskStatus` 实际为 `pending`/`done`。后果：成功任务的进度轮询永不停止、状态徽章与筛选下拉值全部失配。已把 `web/types/api.ts` 对齐为 `pending/running/paused/done/partial/failed`，同步修正 `lib/format.ts`（标签与 `isTerminalStatus`）、`components/business/tasks.tsx`（徽章与筛选值）、`components/business/reports.tsx`（报告状态徽章）、两个 Zustand 切片的轮询终止判断（统一走 `isTerminalStatus`）。
- 【任务汇总字段名双轨导致页面静默失配】后端 `TaskSummaryView` 下发 `tableCount/columnCount/hitColumnCount/failedCount`，前端 `TaskSummary` 却读 `totalTables/totalColumns/hitColumns/failedTables`，导致「命中列恒显示 0 / 0」「失败表恒 0」「重试按钮恒禁用」「失败对象清单恒空」。已在后端补齐与原始计数同源的控制台契约字段，并在 `handleGetTask` 用 `scan_checkpoint` 真实统计 `doneTables`/`failedTables` 与 `failedObjects`（`id` 即断点键「模式/表」，可直接作为 `targetIds` 回传）；列表接口不查断点，避免 N+1。
- 【重试响应语义】前端 `retryTask` 原声明 `{ retried }` 且提示「已重新提交」，与后端真实重扫语义不符；现声明 `{retried, newFindings, remaining, status}` 并提示实际重扫表数、新增命中与剩余失败表，确认按钮文案改为「重试失败对象」。
- 【前端既有类型错误】`web/services/datasources.ts` 缺失 `requestEnvelope`/`ApiError`/`ApiErrorCode`/`ImportResult`/`ImportErrorRow` 导入，且 `normalizeImportResult` 把未知结构误断言为 `ImportErrorRow`，`tsc --noEmit` 报 18 处错误。已补齐导入并把逐行原始值按 `unknown[]` 归一化。

### 📈 Improvements 性能/体验优化

- 重试/续扫共用 `scanEnv` 装配（`env.go`）与 `datasourceConfig`/`splitTableKey`（`datasource.go`），消除重复代码并满足单文件 ≤300 行门禁；删除被取代的 `failed.go`（事后过滤方案）。
- 新增 `internal/application/scan/retry_test.go`（3 用例：单表失败不阻断且可重试、指定对象重试不重跑成功表、续扫跳过已完成表）与 `internal/interfaces/http/task_retry_test.go`（2 用例：部分失败→修复→定向重试→done 且台账两表各一条无明文；404/1004、401/1002、409/3001 错误码）。后者额外断言控制台契约字段与 `targetIds` 闭环。
- CLI 端到端脚本 `.tmp/e2e_retry.py` 走通 `init → datasource add → scan → 三格式报告 → retry/resume`，台账 2 条、断点全 `done`、任务 `done`。
- 【CLI 任务审计补齐（DEF-01 收尾）】`internal/application/scan/audit.go`：用例层可选审计主体（`WithAuditActor("cli")`，HTTP 形态保持空串由 handler 留痕避免双写）；`Run/RunConfig` 成功/失败（含报告导出失败分支）、`RetryFailedTables`、`Resume` 统一记 `task.start`/`task.retry`/`task.resume`，detail 含状态/表数/命中数与重扫统计；真机验证 CLI 扫描审计留痕。至此 datasource 与 scan 两条 CLI 链路的审计缺口全部关闭。
- 【限速与低峰执行落地】`internal/application/scan/scheduling.go`（新文件）：批次间歇 `batch_interval_ms` 在采样循环批间生效（`paceBatch`，ctx 可取消）；低峰窗口 `low_peak_window` 由编排消费——窗口外暂停采样查询并轮询等待（`waitLowPeak`，支持跨午夜窗口，非法/空窗口视为不限）；`assembly` 经 `WithScheduling` 注入。`inLowPeakWindow`／`parseHM` 单测覆盖跨午夜与边界。

### 📚 Docs 文档更新

- `.tmp/gate05_check.py`（临时校验脚本，非交付物）按正确口径复核 GATE-05：单文件代码行 ≤300、入口 <100、domain/application/interfaces 依赖方向。

### 核验结果（可复现）

- `go build ./...`、`go vet ./...`、`go test ./...` 全绿（含 `internal/interfaces/http` 与 `internal/application/scan` 定向复跑 `-count=1`）；`gofmt -l .` 为空。
- `npx tsc --noEmit` 通过；`npm run build`（Next.js 16.4.6 / Turbopack）编译成功，16 个静态页面全部生成。
- GATE-05：最大单文件 274 代码行（`internal/interfaces/http/views.go`）、`cmd/dbdlp/main.go` 32 代码行、分层依赖方向检查通过。

### 已知未实现／如实标注

- `skippedTables` 本期固定返回 0：后端尚未统计 `excludeTables`/`excludeColumns` 过滤掉的表数，UI 保留该字段但数值不代表真实跳过量。
- `GET /datasources/{id}/metadata?refresh=true` 仍读上次扫描元数据（缺应用层 `CollectMetadata`）；`POST /tasks` 的 `sampleRows/batchSize/concurrency` 仍仅校验未生效（审计已注明「本次未生效」）；`pause` 仍为状态迁移（无执行体中断钩子，`resume` 已能按表续扫）；前端 `Dialect` 枚举（`dm`/`postgresql`/`kingbase`）与后端产品方言（`dameng`/`postgres`/`opengauss`）仍有偏差。
- HTTP 端到端脚本因 API 只接受 5 种产品方言、SQLite 目标适配器属测试专用，创建数据源处被 1001 拒绝；HTTP 成功路径改由 Go 测试覆盖。

---

## [ 2026.11.14.4 ] - 0.4.1

### ✨ New Features 新增功能

- 【端到端验收】按 `docs/03` §12 锁定流程完成 **Spec 12 第 1~8 步 API 主链**真机走通（serve → 登录 → API 建源 → multipart YAML 导入部分成功 → API 扫描+进度轮询 → 台账 → API 报告导出+审计 → 日志/页面复核），MVP（M1）交付判据全部达成；新增 `docs/10-端到端验证测试报告.md`（15 项测试矩阵、缺陷清单与处理状态）。
- 【报告原型】新增 `docs/prototypes/report-html-prototype.html`：按 `docs/04` §4 设计规格与 `docs/02` §14.1 八章节契约制作的 **HTML 审计报告输出原型**（自包含零依赖，真实扫描脱敏数据填充）——①扫描概况（KPI＋C2 识别率）②风险汇总（C1 环形＋C3 类型 TOP，图例含数量占比）③④高/中风险清单 ⑤待复核 ⑥加密脱敏现状（掩码规则表＋存储形态）⑦整改建议 ⑧附录（规则清单 13 条＋任务记录）；风险徽章三重编码（色块＋图标＋中文）、掩码样例 `--sample-*` 等宽弱底、数据源 IP 脱敏 `*.*.10.180`、打印分页样式，供评审后照原型补齐 `reporter` 八章节实现。视觉验收通过（四区段逐页核查）。

### 🐛 Bug Fixes 问题修复

- 【CLI 审计缺失（DEF-01）】`datasource add/test/import` 原不写 `audit_log`（`AppendAudit` 仅 HTTP 层调用），纯 CLI 部署形态下数据源操作不可审计——docs/06 契约要求 CLI 同构留痕。已在 `application/datasource` 补 `audit` 助手：创建成功记 `datasource.create/success`、门禁拒绝记 `datasource.test/failed`（含中文原因）、导入记 `datasource.import`（partial 语义＋失败行明细）；actor=`cli`、审计写入失败仅记 WARN 不影响主流程。真机验证成功/失败双路径均留痕。
- 【负样本池误判高风险（DEF-03，EV-03/EV-04 违约）】列级聚合以「任一 hit 过校验」置 `VerifyPassed=true`，负样本池中 3 个碰巧通过校验器的纯数字串（订单号过 Luhn／全 0 卡号）把整列抬为 `high`，掩盖列内大量校验未过的格式命中——违反 EV-03「NG-01~06 判高风险数必须 0」。`domain/aggregator` 的 `ClassifyRisk` high 分支收紧为「校验过 **且** 列名/注释吻合」（对齐 §14.4 置信度维度），校验过但缺上下文吻合降为 `medium` 交人工确认；列风险为 `pending_review` 时回写 `ReviewStatus=pending_review`（EV-04 分流）。复扫验证：NG 池 high→medium，全库 high 字段 13→11 且全部列名吻合。
- 【样例规则 bytescan 自相矛盾（DEF-02）】`configs/rules.yaml` 的 `CONTRACT_NO` 正则实际匹配 18 字符而 `bytescan.min_len/max_len=16`，18 字符值被字节扫描预过滤致自定义规则静默不命中；修正为 18 并经正负样本复扫验证。
- 【命中率超 100%（DEF-04）】同一采样值命中多条规则（如 18 位数字串同命中身份证/银行卡/信用代码）时 `HitRatio = len(hits)/sampleTotal` 可达 280%，违反 docs/04 §4.3 口径；`domain/aggregator` 按采样行数封顶 100%，`命中/样本` 列保留多类型命中明细数语义，复扫验证超 100% 字段为 0。

### 📈 Improvements 性能/体验优化

- 【复核分流可验证】聚合器新增 `pending_review` → `ReviewStatus` 回写链路（单测 `TestAggregatePendingReview` 覆盖），复核 API/台账/报告可按 `verify_passed=false` 过滤格式命中，M2「误报复核」闭环的数据基础补齐。

### 📚 Docs 文档更新

- `docs/10-端到端验证测试报告.md` 缺陷状态更新（DEF-01/02/03 全部关闭、OBS-01/02 留痕）；`testdata/README.md` 数据源信息与核验结论维持同步。
- 新增 `archify/architecture-dbdlp-20261114-112500/dbdlp-architecture.html`：**项目架构总览图**（Archify 可探索 HTML，深/浅主题、动效路由、搜索/透镜/导出）——四层 DDD 依赖倒置拓扑、扫描主旅程、只读防线安全边界、crypto/store/reporter 分支与三张要点卡片；validate/deliver/check/browser-check 四门禁通过，感知验收通过，候选与回执留痕于 `.archify/`。

---

## [ 2026.11.14.3 ] - 0.4.0

### ✨ New Features 新增功能

- 【测试数据集】新增 `testdata/`（Oracle／达梦双版本 SQL，可重复执行、幂等清理）：自研测试集 8 表——客户（混合正负样本行 5/10/15/20）、人员（语义类+中低置信类）、企业、网络（含 `127.0.0.1` 脱敏保留场景）、出行（新能源车牌）、CLOB 文本边界（JSON 内嵌／超长截断／中文上下文／空值族）、通用负样本池（NG-01～NG-06 逐行标注）、千行分页大表（`CONNECT BY` 生成，验证主键分页/断点续扫）；正样本校验位均按 `internal/domain/rule/verifier.go` 同款算法生成并独立复核（身份证加权模11、Luhn、信用代码 GB32100）。
- 【openDLP 来源测试集】引入 [hitsz-ids/openDLP](https://github.com/hitsz-ids/openDLP)（Apache-2.0）测试数据并转换落库：长表 `T_DBDLP_TEST_OPENDLP_PAIRS`（14 类 × 500 对正负样本，14000 行，按类轮转交错——前 28 行即覆盖全部类型正负）、宽表 `T_DBDLP_TEST_OPENDLP_WIDE`（17 列综合基准 1000 行，源列 `DATE` 因 Oracle 保留字改 SQL 列名 `SAMPLE_DATE`）；附转换器 `testdata/tools/opendlp_to_sql.py`（参数可调、支持全量再生）。
- 【e2e 数据源登记】`ORA-E2E-TEST`（oracle/XE）与 `DM-E2E-TEST`（dameng/5236）两数据源经连接测试后加密落库，扫描模式统一为专用业务用户 `DBDLP_TEST`（测试表由此前 SYSTEM/SYSDBA 迁入——扫描器按设计强制排除系统模式，`include-schemas` 不覆盖）。

### 🐛 Bug Fixes 问题修复

- 【Oracle 驱动占位符】`internal/infrastructure/datasource/oracle.go` 全部绑定参数误用 `?`（go-ora 需 `:N`），`ListColumns`／`TableStats` 触发 `ORA-00911: invalid character`——连接测试仅 Ping 未暴露；`Placeholder(i)` 同步修正为 `:N` 对齐接口契约。e2e 真机暴露后修复。
- 【Oracle 11g 分页兼容】采样分页原用 12c 语法 `FETCH FIRST n ROWS ONLY`（11g 报 `ORA-00933`），改为 ROWNUM 三明治分页；同时修正行号别名 `__RN` 未加引号（未加引号标识符须字母开头，`ORA-00911`）。修复后 11g 全表扫描通过。
- 【测试集数据缺陷】`T_DBDLP_TEST_NETWORK` 行 8 补 `127.0.0.1`（README 已声明脱敏保留场景但数据缺失）；openDLP 宽表 `DATE` 列名撞 Oracle 保留字（建表 `ORA-00904` 连锁失败），转换器加 `WIDE_SQL_NAMES` 映射改为 `SAMPLE_DATE`。
- 【达梦脚本编码】实测确认达梦 `disql` 读脚本文件固定按 GB18030 解码后再转服务端字符集（UTF-8 实例上装载 UTF-8 脚本致中文双重编码乱码，100 处 `[-2007]`/`[-70037]`），达梦版脚本改为 **GB18030 编码交付**并在头部注明，转换器同步输出 GB18030。

### 📈 Improvements 性能/体验优化

- 【端到端扫描核验】e2e 环境（Oracle 11.2.0.4 EE + 达梦 DM8 容器）完成「装载 → 登记 → 扫描 → 报告 → 核验」闭环：两库各 10 表／66 字段／命中字段 28，**逐字段结果完全一致**（交叉验证）。正样本召回符合设计（混合列负样本行精确不命中 16/20，千行表全命中 20/20）；报告脱敏合规（`139****0001`／`110101********1006`／`*.*.10.3`，`127.0.0.1` 原样保留）；对照列（MEMO/REMARK/ORDER_NO 等）零误报。
- 【边界暴露】负样本池命中明细定位三项引擎待迭代点（测试集价值留痕）：① 校验未过的仅格式命中未进 `pending_review` 分流（EV-04 口径差距）；② 纯数字串与信用代码字符集重叠（`202611140000123456` 碰巧过模31校验被判 high，需字段上下文降误报）；③ openDLP 手机号变形样本（`+86`/分隔符）整串正则仅命中 4/20（RS-07/BD-03 预期差距）。

### 📚 Docs 文档更新

- 新增 `testdata/README.md`：执行方式（Oracle sqlplus／达梦 disql 及 **GB18030 编码要求**）、8 张自研表逐列期望识别结果对照表（对齐 docs/07 RS/NG/BD 编号）、验收检查清单（EV-02/03/04、IT-03、脱敏特例）、openDLP 测试集类型映射与全量再生方法、e2e 数据源信息（IP 按脱敏策略写为 `*.*.10.180`，口令仅指向容器环境变量不落盘）与扫描核验结论。

---

## [ 2026.11.14.2 ] - 0.3.0

### ✨ New Features 新增功能

- 【日志系统】落地 `internal/infrastructure/logging`（`redact.go`／`writer.go`／`logging.go`），对齐 `AGENTS.md` §日志系统：五级（TRACE/DEBUG/INFO/WARN/ERROR，默认 INFO）、时间戳 `YYYY-MM-DD HH:MM:SS.SSS`、格式 `时间戳  级别  trace_id  PID  模块:行号 - 内容`、文件落 `项目根目录/logs/`、按日轮转、保留 32 天自动清理、文件名 `dbdlp-YYYYMMDD.log`、UTF-8 中文。脱敏层按敏感键（password/token/secret/dsn/cookie/口令/密码…）整体替换为 `******`，自由文本按身份证 → 手机号 → 银行卡 → IPv4 顺序掩码。应用层（scan／datasource／cli）全面注入 `port.Logger`；测试替身按 `docs/07` §1.3 放 `internal/application/testing`（`NopLogger`），不污染 domain。
- 【HTTP API】落地 `internal/interfaces/http`，**32 个端点与 `docs/02` §13 逐条对齐（零缺失零多余）**：统一响应 `{code,data,message}`、分页 `{items,total,page,limit,hasMore}`、Bearer 鉴权（`POST /api/v1/auth/login` 除外）、bcrypt 口令校验与连续失败锁定、会话只存 token 的 sha256 散列、数据源响应 `password` 恒为 `******`、`GET /api/v1/config` 只回显五键且不含密钥/DSN、批量导入逐条连接测试＋部分成功、导入模板下载、审计留痕、`/api/v1/*` 优先于静态资源的 SPA 回落。
- 【Web 控制台】落地 `web/`（Next.js 16 App Router 静态导出 → `web/out`，`trailingSlash: true` 与 Go 托管逻辑对齐）：14 个路由页（总览／数据源列表·新建·编辑·导入／任务列表·详情／字段台账·明细／报告列表·预览／规则／配置／登录），统一 API 封装层（token 只存 sessionStorage，统一抛 `ApiError`），状态层用 Zustand，图表用 ECharts，表格用 TanStack Table，样式走 `docs/design-tokens.css` 的 311 个令牌变量。Go 进程同端口托管前端，无独立 Node 服务、无跨域配置。
- 【CLI】新增 `dbdlp serve` 子命令：启动 HTTP API 并托管前端静态资源，`--listen`／`--static-dir` 可覆盖，SIGINT/SIGTERM 优雅停机（15s 超时）。新增控制台管理员引导：`DBDLP_ADMIN_USER`／`DBDLP_ADMIN_PASSWORD` 环境变量首次创建 bcrypt 账号，账号已存在不覆盖口令，口令不落配置不进日志。
- 【规则加载】`internal/domain/rule/loader.go` 改为真正的 YAML 解析，补齐 `docs/02` §6.3 定义但此前缺失的语义：`globals.default_threshold`（缺省阈值）、`globals.disabled_rules`（禁用≠删除，仍进规则快照）、YAML 规则与内置同 `rule_id` 时覆盖内置定义（本文件内 `rule_id` 重复仍拒绝启动）；`rule_version` 摘要扩展为覆盖文件声明版本与规则全字段（含 bytescan 参数）。新增 `configs/rules.yaml` 样例（内置覆盖 + EMPLOYEE_NO／CONTRACT_NO／CUSTOMER_NO 三类自定义）与 `internal/domain/rule/loader_test.go`。
- 【配置】新增根目录 `dbdlp.toml` 全字段样例（`DisallowUnknownFields` 校验通过）。

### 🐛 Bug Fixes 问题修复

- 【规则加载器误用格式】`rules.yaml` 原用 `toml.Unmarshal` ＋ `toml:` 标签解析，与文档锁定的 YAML 格式矛盾——自定义规则静默不生效、`globals` 语义完全无法配置。已改为 `gopkg.in/yaml.v3` ＋ `yaml:` 标签。
- 【报告导出静默失败】`--report json,html,excel` 原把整串当单一格式传给 `ReportFormat`（不支持多格式），且 `exportReport` 把错误直接吞掉且不记日志——扫描显示「完成」、台账入库，但报告目录里一个文件都没有，成功信号与交付物完全脱节。已改为按逗号拆分逐个导出、失败记 ERROR 并向上返回错误，CLI 仍打印任务 ID 不丢失；`report` 子命令同步支持多格式。
- 【日志过度脱敏】银行卡正则 `\b\d{16,19}\b` 把系统长数字标识（数据源 ID／任务 ID／流水号）当成银行卡掩码，日志里 ID 变成 `1*****************0-...`，排障信息尽失。改为「先候选再验 Luhn」：只有 Luhn 校验位通过的 16~19 位数字才按银行卡脱敏（非 Luhn 有效者视为系统标识原样保留）。
- 【配置目录相对路径】`--config` 指向其他目录时，存储与报告目录原先仍写到进程 CWD；现统一按配置目录解析。

### 📈 Improvements 性能/体验优化

- 【架构门禁 GATE-05 修复】拆分 3 个超 300 行的文件：`internal/domain/port` 拆为 `port.go`（端口接口，199 行）＋ `values.go`（值类型，207 行）；`internal/infrastructure/store/methods.go`（479 行）拆为 `datasources.go`／`tasks.go`／`findings.go`／`sessions.go`；`internal/application/scan/service.go`（362 行）拆为 `service.go`／`sampling.go`／`report.go`。入口 `cmd/dbdlp/main.go` 由 184 行降至 39 行，组合根迁至 `cmd/dbdlp/assembly.go`（满足「入口 < 100 行」）。拆分后 `gofmt -l` 为空、`go build`／`go vet`／`go test ./...` 全绿、CLI 端到端回归通过。
- 【契约审计】新增三项自动化核对并留痕：① 文档 §13 端点表 ↔ 实际注册路由 **32/32 完全对齐**；② 前端服务层 21 个调用点 **100% 命中**后端路由、缺失 0；③ 前端 P0 红线扫描（emoji 0、硬编码色值 0、渐变 0、令牌经 `globals.css` 接入）。
- 【安全验证】原始 socket 绕过客户端 URL 归一化，对 8 种目录穿越向量（字面 `..`、URL 编码、反斜杠、`....//` 等）实测**零泄露**：字面 `..` 被 Go `ServeMux` 以 307 规范化重定向，编码变体返回 404。

### 🔧 Dependencies 依赖更新

- `gopkg.in/yaml.v3` 由间接依赖转为直接依赖（规则加载器改用 YAML 解析）。
- 本地 `goimports` 版本过旧，无法解析泛型语法（`Page[T any]`），在本仓库不可用；改用 `gofmt` ＋ 按标识符引用精确计算导入的脚本。

### 📚 Docs 文档更新

- 文档体系不变；本条记录代码实施阶段的第二批落地范围，以及对文档契约的三处实现取舍（规则加载器的 YAML 覆盖语义、报告多格式、入口文件拆分）。

---

## [ 2026.11.14.1 ] - 0.2.0

### ✨ New Features 新增功能

- 【工程骨架】按 `AGENTS.md` 四层 DDD ＋ clean architecture 落地 Go 工程：`internal/domain`（port／mask／rule／detector／aggregator）、`internal/application`（scan／datasource 用例）、`internal/infrastructure`（config／crypto／datasource／store／reporter）、`internal/interfaces/cli`（cobra），入口 `cmd/dbdlp`。
- 【领域契约】`internal/domain/port` 定义全部端口与值类型：`Driver`／`ReadOnlyConn`／`Rows`／`Store`／`Detector`／`Aggregator`／`Reporter`／`Cipher`／`KeyStore`，无循环依赖。
- 【识别引擎】自研四层流水线（字节扫描 → 正则 → 校验器 → 自定义规则）：内置 10 条规则（手机号／身份证／银行卡／统一社会信用代码／邮箱／IPv4／护照／车牌／邮编／固话），校验器含身份证 GB11643 与 USCC GB32100 模 31 校验、银行卡 Luhn。
- 【加解密】AES-256-GCM 与 SM4-GCM 双算法，密文格式 `ENC(base64([1B 算法标签][12B nonce][密文][16B tag]))`；密钥 32 字节存 `.dbdlp/key`（0600），失败即中止。SM4 改用 `emmansun/gmsm` 并以国标 GB/T 32907 向量校验。
- 【多数据源】Oracle（`sijms/go-ora/v2`）、达梦（`gitee.com/chunanyong/dm`）只读适配器，统一元数据 SQL 与主键范围分页（无 OFFSET）；另附测试用 SQLite 目标适配器。
- 【持久化】SQLite（`modernc.org/sqlite`，纯 Go 无 CGO）落 14 张表 DDL；数据源凭据以 `ENC()` 密文入库，报告与台账只存脱敏样例。
- 【报告】JSON／HTML／Excel 三格式导出（`excelize/v2`），风险分布与敏感字段台账同源，**报告只含掩码样例，不含明文原值**。
- 【CLI】`dbdlp init`（生成密钥）、`datasource add|test|import`（连接测试前置门禁、批量导入部分成功）、`scan`（元数据 → 采样 → 识别 → 聚合 → 台账 → 报告）、`report`（基于台账重导出）。

### 🐛 Bug Fixes  问题修复

- 【只读守卫】`scanToStrings` 原以 `maxTextLen=0` 表示元数据查询，导致所有元数据字符串被截断为空串，表名退化为 `""` 后 `PRAGMA table_info('')` 无返回，元数据链路整体失效。改为 `maxTextLen<=0` 表示不截断。
- 【只读守卫】允许 `PRAGMA` 前缀（仅 SQLite 元数据用；写/DDL 关键字与分号多语句拦截不变）。
- 【台账语义】扫描编排改为仅对「检出敏感数据」的字段记入台账，无命中列不再产生空台账项。
- 【幂等性】`SaveRuleSnapshot` 改为 `ON CONFLICT(rule_version,rule_id)` upsert，同一规则版本可被多次任务快照；`regex_digest` 改存 sha256 摘要，对齐 DDL 列语义。

### 📈 Improvements 性能/体验优化

- 【安全】连接测试前置门禁：连通性校验未通过的数据源一律不落库（已由用例覆盖）；CLI 报告与台账经实测确认不含明文手机号原值。
- 【可验证性】新增端到端引擎用例（SQLite 目标库 → 采样 → 识别 → 聚合 → 台账 → Excel/JSON 报告），断言命中率、风险等级、置信度与脱敏一致性。
- 【P0 门禁整改】CLI 批量导入结果标记由 `✓/✗` 改为 `[OK]/[FAIL]`（规避中文 Windows GBK 终端乱码）；HTML 报告色值由散落字面量收敛为 `colText`／`colMuted`／`colBorder`／`colHeadBG`／`colHighRisk`／`colPillBG` 命名常量，便于后续与 `docs/04` 设计令牌对齐。全量 Go 源码 emoji 扫描结果为 0。
- 【代码规范】全量 `gofmt -w` 归一，`gofmt -l` 输出为空。

### 🔧 Dependencies 依赖更新

- 引入：`spf13/cobra`、`pelletier/go-toml/v2`、`modernc.org/sqlite`、`xuri/excelize/v2`、`emmansun/gmsm`、`sijms/go-ora/v2`、`gitee.com/chunanyong/dm`。
- 工具链：Go 1.26.0（`go.mod` 声明 `go 1.26.0`）。

### 📚 Docs 文档更新

- 文档体系不变；本条记录文档契约向代码的首批落地范围与偏差（仅规则快照列语义、SQLite 目标适配器两处）。

---

## [ 2026.11.13.5 ] - 0.1.5

### 📚 Docs 文档更新

- 【文档二次去冗余】在 2026.11.13.4 整体精简基础上，进一步清理三类残留：① 纠偏叙事（01 §0.2「相对 v1.0 关键纠正」折叠为结论、§12.1 压为精简表；02/06 文件头纠偏长尾收为版本基线一句；ADR-001/003 后果中回扣「v1.0 偏差」句删除——ADR Background 决策理由保留）；② 反复交叉引用复述（07 §10.1/§10.2 章节枚举大表压为 2-3 句上游依据；04 §0 配套文档列章指针压缩；06 §0.2/§8.6、02 §23 参考依据长表压缩；OPEN-DECISIONS 溯源标注精简为短注）；③ 自查清单重复（06 §8.4 施工自检清单改为指向 02 §21/07 §10.3 的短指针；09 §0.4 团队 P0 自检删表改为引用 04 §0.5、前端离线自查 3 处合并为 1 处）。
- 净减约 41 行；强校验基线四项、U-03/U-01 关闭口径、端点 32/33 争议、所有锁定数字/版本/路径/DDL/接口签名/测试编号均原样保留，未触碰 AGENTS.md/references/。

## [ 2026.11.13.4 ] - 0.1.4

### 📚 Docs 文档更新

- 【文档集精简】以 `AGENTS.md` 为唯一基准，对 `docs/` 下 12 份文档做"保留施工级实质、删除冗余"的精简：删 v1.0/v2.0 纠偏长叙事、重复交叉引用复述、冗长自检/待验证清单与命令注释；保留全部锁定表格（驱动/方言差异、dbdlp.toml 字段表、14 张表 DDL、Go 接口签名、端点清单、测试编号 UT/IT/ST/EP/AC/RS）、代码块、SQL、已验证结论（U-03/U-01 已关闭）与四项强校验基线（默认 INFO、`项目根目录/logs/`、格式串、保留 32 天）。
- 影响文档 01/02/03/04/05/06/07/08/09 + ADR-001/002/003 + OPEN-DECISIONS；总行数约 9060 → 7090（整体精简约 22%）。端点 32/33 争议、DIFF-01 批量导入语义、跨文档强校验基线一致性均原样保留，未为凑篇幅牺牲施工级内容（锁定表格构成篇幅硬下限）。

## [ 2026.11.13.3 ] - 0.1.3

### 📚 Docs 文档更新

- 【U-03 真机验证关闭】在 `root@192.168.10.180` 数据库容器上以只读方式完成 U-03 三方方言元数据字典验证，U-03 正式关闭（此前 Oracle／达梦已于同环境验证，本期补齐 openGauss）：
  - Oracle（`e2e-oracle` 11.2.0.4-ee）：`ALL_TABLES`／`ALL_TAB_COLUMNS`／`ALL_COL_COMMENTS`／`ALL_TAB_COMMENTS` 全部可查，`ALL_TAB_COLUMNS` 实测 91138 行。
  - 达梦（`e2e-dm8` DM8）：系统模式实测仅 `SYS`／`SYSDBA`／`SYSAUDITOR`／`SYSSSO` 四项为用户级（`DMHR`／`CTISYS` 未创建为数据库用户，属可选示例／特定版模式）；`SYS.SYSCOLCYT`／`SYSCOLUMNCOMMENTS`／`SYSCOLUMNS` 均存在。
  - openGauss（`e2e-gauss` openGauss 6.0.0）：复用 PostgreSQL 兼容 `information_schema.columns`（实测 5911 行）／`information_schema.tables`（494 行）／`pg_catalog.pg_class`（133 行），`LIMIT n` 分页验证通过；系统模式实测 16 个（`public` 为用户默认模式不排查，其余 15 个须排除：`pg_catalog`／`information_schema`／`pg_toast`／`blockchain`／`coverage`／`cstore`／`db4ai`／`dbe_perf`／`dbe_pldebugger`／`dbe_pldeveloper`／`dbe_sql_util`／`gaussdb`／`pkg_service`／`snapshot`／`sqladvisor`）。
- 影响文档 02／05／06／07／08／09／ADR-001／ADR-002：U-03 表述由「已降级建议项」统一更新为「已关闭（Oracle／达梦／openGauss 真机验证通过）」；02 第 5.3 节方言差异表与 §5.3.1 元数据 SQL 基线据实测增补 openGauss 字段字典、系统模式排除清单与分页语法。
- 注：原 2026.11.13.3～.9 的细粒度迭代条目在文档体系收敛时并入里程碑视图，U-03 相关结论由本条覆盖；细节见各文档正文与 `.workbuddy/memory/2026-11-13.md`。

## [ 2026.11.13.2 ] - 0.1.2

### 📚 Docs 文档更新

- 【配置契约收敛】经用户裁定对齐 `AGENTS.md` 项目总纲：配置目录与密钥文件命名统一为 `.dbdlp`（密钥文件 `.dbdlp/key`，保留 `0600` 权限与 `--config-dir` ＞ 项目根 `./.dbdlp` ＞ 用户主目录 `~/.dbdlp` 查找顺序）；运行配置文件由 `dbcli.toml` 更名为 `dbdlp.toml`（仅文件名变更，`config_dir` 键与 `--config-dir` 通用名保留不动）。
- 【数据源配置移除文件化】数据源配置文件移除，资产数据源统一持久化到后端存储库 `data_source` 表（支持增删改查与 Excel／YAML 批量导入），命令行／TUI 通过控制台 API 或 CLI 子命令写入该表；密钥文件 `.dbdlp/key` 仍外置保留（凭据以 `ENC()` 密文入库，密钥与密文不同库）。
- 【文档同步】影响文档 `02`／`03`／`06`／`07`／`09` 与 `OPEN-DECISIONS.md`（路径命名分歧未决项翻 `RESOLVED`）；`configs/rules.yaml` 路径保持不变。



-------

## [ 2026.11.13.1 ] - 0.1.1

### ⚠️ Breaking Changes  破坏性变更

- 【文档体系】废弃旧「A 路线 —— 达梦单库 ＋ 内嵌 GoDLP 软件开发工具包」方案：`docs/01-项目产品说明书PRD.md` 与 `docs/02-架构设计文档.md` 全文重写为 v2.0 已确认基线，改以 `AGENTS.md` 项目总纲为唯一基准。
- 【技术路线】识别引擎改为自研四层流水线（字节扫描器 → 正则规则 → 校验规则 → 自定义 YAML 规则），godlp 降级为设计参考、零依赖；数据源改为统一抽象，Oracle 与达梦为首批实现。
- 【运行形态】本期纳入网页控制台（按总纲锁定的前端技术栈），资产数据源配置写入后端存储数据库并支持增删改查与 excel/yaml 批量导入，不再停留在文件配置。

### ✨ New Features 新增功能

- 【文档集】新增 `03-规格契约Spec.md`、`05-敏感识别规则与算法规格.md`、`06-数据源接入与配置管理手册.md`、`07-测试设计与验收规格.md`、`08-开发计划与任务拆解.md`、`09-部署运维与安全手册.md`，形成从需求到开工的闭环。
- 【前端】新增 `04-前端控制台与报告设计规格.md`（13 个页面）、`design-tokens.json` 与 `design-tokens.css`（机器可读设计令牌，浅色主题）。
- 【决策留痕】新增 `docs/decisions/ADR-001～003`（MADR 格式）与 `docs/decisions/OPEN-DECISIONS.md`（未决登记册，9 未决 + 1 已决）。
- 【记忆通道】新增 `.workbuddy/memory/pitfalls.jsonl`，落 1 条已验证的契约类踩坑记录，并在规格契约第 11 章内嵌已知坑。

### 🐛 Bug Fixes  问题修复

- 【文档】修复旧架构文档第 5 行失效互引（《达梦敏感扫描平台-PRD.md》与实际文件名不符）。
- 【文档】关闭旧版两份文档共 10 条悬空的「待确认事项」，能按总纲定调的给出结论，其余转入未决登记册并写明阻塞与关闭条件。
- 【配置契约】修正产品需求文档日志默认级别 `INFO → ERROR`、日志目录由 `data/` 移出至 `项目根目录/logs/`，补齐日志行格式串，消除三处「待锁定格式」占位，并同步 U8 状态为已关闭。
- 【第三方接口】证伪旧架构文档第 1.4 节 godlp `Close() error` 与真实签名不符；全部第三方库签名改为联网核实后写入「第三方库核实结论表」，未核实的标注「待验证」。
- 【图标版本】裁定 `lucide-react` 由 1.69.0 统一到 **1.72.0**（npm 实时版本＋包内类型声明核验），并锁定 v1 canonical 图标名，避免 0.x 旧别名失效。

### 📈 Improvements 性能/体验优化

- 【可施工性】全部文档提升到可施工级：含接口签名、14 张表完整 DDL、32 条端点清单、配置字段表、规则定义、EARS 验收标准与端到端验证命令。
- 【一致性】建立跨文档强校验基线（默认 ERROR、`项目根目录/logs/`、日志格式串、保留 32 天），在 01／02／03／09 文档锁定，并由 07 文档落持久回归用例。
- 【合规】明确全程只读、每列默认 20 行抽样、主键分页、分批限速低峰执行、报告只放掩码样例、IPv4 特殊脱敏规则与日志脱敏口径。
- 【质量门禁】建立 P0 规则扫描（禁 emoji 功能图标、禁紫粉渐变、禁硬编码色值）、代码组织门禁与 RoleVerdict 结构化裁决流程。

### 📚 Docs 文档更新

- 重写 `01-项目产品说明书PRD.md`（706 行）与 `02-架构设计文档.md`（1754 行，23 章），去掉「草案／待评审」与「部分内容由人工智能生成」注脚。
- 新增 `08-开发计划与任务拆解.md`：3 个里程碑、16 项工作分解、依赖关键路径、质量门禁、未决项影响与完成定义。
- 补齐文档集索引与互引，统一 `docs/` 目录顺序标号中文命名。

### 🔧 Dependencies 依赖更新

- 锁定并记录：Go 1.25.10+、Oracle `sijms/go-ora/v2`、达梦 `gitee.com/chunanyong/dm`、`modernc.org/sqlite`、`BurntSushi/toml`、TypeScript 7.0+、Next.js 16.2+ App Router、React 19.2+、ECharts 6.0.0、`@tanstack/react-table` 8.21.3、`lucide-react@1.72.0`。
- 标注待验证：`openGauss` 驱动、国密 GCM-SIV 导出签名、excelize 与 Go 最低版本 tag、Zustand 精确版本；Go 工具链不因依赖提升到 1.26。

### 🗑 Deprecated 废弃功能

- 废弃「内嵌 GoDLP 引擎」与「仅达梦 DM8 单库」的旧路线描述。
- 废弃旧版三份 YAML 配置（`configs/instances.yaml`／`scan.yaml`／`rules.yaml`）作为统一入口的做法，改为 `dbdlp.toml`（命令行 ＞ 配置文件 ＞ 默认值）。

---

### 待用户裁定

1. 架构文档第 13.2 节端点表格按行数统计为 32 条，架构师回传口径为 33 条，已列入规格契约门禁核对项。

---

