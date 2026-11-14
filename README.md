# dbdlp — 数据库敏感数据发现平台

只读模式的数据库敏感数据扫描与识别工具（CLI / Web 控制台），不侵入业务库，构建「采集 → 识别 → 台账 → 报告」的合规闭环。

[![Go](https://img.shields.io/badge/Go-1.26-00ADD8)](https://go.dev) [![Next.js](https://img.shields.io/badge/Next.js-16-black)](https://nextjs.org) [![License](https://img.shields.io/badge/license-TBD-blue)](#)

## 功能特性

- **多数据源只读接入**：Oracle（`sijms/go-ora`）、达梦 DM8（`chunanyong/dm`）真机验证；SQLite 作为测试目标方言。只读三防线：只读账号 + `ReadOnlyConn`（无 Exec）+ SQL 白名单守卫（占位符绑定，禁拼接）。
- **敏感识别引擎**（自研四层流水线）：字节扫描 → 正则 → 校验器 → 自定义 YAML 规则。内置 10 类规则（身份证/手机号/银行卡/统一社会信用代码/邮箱/IPv4/护照/车牌/邮编/固话）＋ 9 种校验器（加权模11、Luhn、GB32100 模31 等）；支持不改代码新增企业自定义规则并单独启停。
- **扫描调度**：每列默认 20 行主键范围分页抽样（无深偏移 `OFFSET`）、checkpoint 断点续扫、单表失败不阻断、失败重试、批次限速与低峰窗口。
- **风险分级与复核**：高风险须「校验通过＋列名吻合」；仅格式命中进 `pending_review` 人工复核；置信度三级（高/中/低）。
- **报告**：Excel 六 Sheet（概览/字段台账/命中明细/复核记录/规则清单/任务记录，`NewStreamWriter` 流式）/ HTML 审计报告 / JSON 明细；**三处脱敏缺一不可**——SQL 字面量、运行日志、报告文件；`127.0.0.1`/`localhost` 保留完整。
- **安全**：数据源凭据 `ENC(base64)` 加密落库（AES-256-GCM 主选 / SM4-GCM-SIV 国密，算法标签自动路由），密钥 32 字节存 `.dbdlp/key`（0600）；控制台 bcrypt 登录、连续失败锁定、操作审计留痕。
- **Web 控制台**：Next.js 16 App Router 静态导出，Go 进程同端口托管；14 个页面（总览/数据源/任务/台账/报告/规则/配置），ECharts ＋ TanStack Table ＋ Zustand，设计令牌体系（`docs/design-tokens.css`）。

## 架构

四层 DDD ＋ 整洁架构，依赖只向内：`interfaces（cli/http）→ application 用例 → domain 端口`，infrastructure 实现 domain 端口（依赖倒置注入）。

可探索架构图：[archify/architecture-dbdlp-20261114-112500/dbdlp-architecture.html](archify/architecture-dbdlp-20261114-112500/dbdlp-architecture.html)

## 快速开始

```sh
# 构建（Go 1.26+，CGO_ENABLED=0 可交叉编译）
go build -o dbdlp.exe ./cmd/dbdlp

# 初始化工作区（自动生成 32 字节密钥与本地存储）
./dbdlp.exe init

# 登记数据源（先连接测试，通过后才加密落库）
./dbdlp.exe datasource add --name ORA-PROD-01 --dialect oracle \
  --host <主机> --port 1521 --service <服务名> \
  --username <只读账号> --password <口令> --include-schemas <模式>

# 只读扫描并导出报告（excel/html/json 可多选）
./dbdlp.exe scan --datasource <数据源ID> --report excel,html,json

# 启动控制台（REST API + 前端静态资源同端口）
DBDLP_ADMIN_USER=admin DBDLP_ADMIN_PASSWORD=<口令> ./dbdlp.exe serve --listen 127.0.0.1:8080
```

运行配置文件 `dbdlp.toml`（命令行参数 ＞ 配置文件 ＞ 默认值）；自定义规则见 `configs/rules.yaml`。

## 测试

```sh
go test ./...
```

端到端测试数据集见 [testdata/README.md](testdata/README.md)：自研 8 表（Oracle/达梦双版本 SQL，正样本校验位按引擎校验器算法生成）＋ [openDLP](https://github.com/hitsz-ids/openDLP) 14 类正负样本集，已在 e2e 环境完成 Spec §12 全流程验证（见 [docs/10-端到端验证测试报告.md](docs/10-端到端验证测试报告.md)）。

## 文档索引

| 文档 | 内容 |
|---|---|
| [docs/01-项目产品说明书PRD.md](docs/01-项目产品说明书PRD.md) | 产品需求 |
| [docs/02-架构设计文档.md](docs/02-架构设计文档.md) | 架构与契约（23 章） |
| [docs/03-规格契约Spec.md](docs/03-规格契约Spec.md) | 规格契约与端到端验收流程 |
| [docs/04-前端控制台与报告设计规格.md](docs/04-前端控制台与报告设计规格.md) | 前端与报告可视化设计 |
| [docs/05-敏感识别规则与算法规格.md](docs/05-敏感识别规则与算法规格.md) | 规则与算法 |
| [docs/06-数据源接入与配置管理手册.md](docs/06-数据源接入与配置管理手册.md) | 数据源接入 |
| [docs/07-测试设计与验收规格.md](docs/07-测试设计与验收规格.md) | 测试设计与正负样本集 |
| [docs/08-开发计划与任务拆解.md](docs/08-开发计划与任务拆解.md) | 里程碑与任务 |
| [docs/09-部署运维与安全手册.md](docs/09-部署运维与安全手册.md) | 部署运维 |
| [docs/10-端到端验证测试报告.md](docs/10-端到端验证测试报告.md) | 最新端到端验证结论 |

版本历史见 [CHANGELOG.md](CHANGELOG.md)（CalVer：`YYYY.MM.DD.MICRO`）。

## 安全与合规约束

- 全程只读：被监控库零写入，写操作被拒并记中文 WARN。
- 禁止明文：日志/报告/SQL 字面量三处脱敏，禁止记录密码、完整身份证号、手机号。
- 密钥获取失败直接报错终止，绝不回退默认密钥或全零密钥。
- 所有错误信息与日志统一中文、UTF-8 编码。

## 参考项目

[bytedance/godlp](https://github.com/bytedance/godlp) · [hitsz-ids/openDLP](https://github.com/hitsz-ids/openDLP) · DBMasker（Apache 2.0）
