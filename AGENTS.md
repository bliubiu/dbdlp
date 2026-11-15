## DBDLP 项目

dbdlp项目是基于go技术栈实现只读模式扫描识别数据库敏感信息一套CLI/Web项目工具。

支持构建一个二进制文件即可启动两种模式服务：

- CLI：默认行为，单次运行
- Web:  通过 `serve` 命令选项启动 Web 控制台与 HTTP API
- `version`： 无需启动任何服务即可输出版本信息



### 项目目标

dbdlp项目主要在不侵入业务库的前提下，构建一套只读的数据库敏感数据扫描系统。



### 项目功能

- 统一数据源抽象，支持Oracle、达梦、MySQL、postgresql、opengauss 等类型数据库。优先落地实现Oracle和达梦
  - 数据库连接：只读连接、连接池
- 统一数据采集层
  - 元数据读取
  - 表 / 列扫描调度
- 敏感识别
  - 内置 身份证、护照、银行卡、车牌、邮箱、邮编、手机号、固话、地址、公司名、统一社会信用代码、人名、域名、IPv4/IPv6、MAC 等规则。 可扩展企业内部证件号、工号、合同号、客户编号等。适配国内证件
  - 敏感识别引擎，正则 + 校验器 参考godlp项目实现
  - 置信度/分级
  - 风险分级
  - 校验机制

- 资产数据源支持excel/yaml文件批量导入，支持CURD更新
- 统一任务调度
  - 分页限流
  - 分批
  - 限速
  - 低峰执行
  - 抽样：每列抽样 N （默认20）行，主键分页；后续增强为自适应采样
- 统一数据展示层，输出指定格式报告文件
  - 输出 Excel/HTML/JSON 格式报告
  - 内容可包含实例、模式、表、字段、样本、命中数、风险等级
  - 示例掩码样本：报告中只放脱敏样例，不放完整明文
  - 报告目录：统一「项目根 reports/」
- 规则分层
  - 字节扫描器：
  - 正则规则：
  - 校验规则：
  - 自定义yaml规则：
  - 身份证校验位、银行卡 Luhn、邮箱格式校验



### 技术架构

- **四层DDD (Domain-Driven Design，领域驱动设计) **建模方法论
- 整洁架构 (Clean Architecture) 
- **TDD（Test-Driven Development，测试驱动开发）**设计思想
- Go语言： 1.25.10+
- 前端：TypeScript 7.0+， Next.js 16.2+（App Router），React 19.2+, Tailwind CSS , ECharts , Zustand, shadcn/ui,  TanStack Table
- 优先选择最新稳定版本框架/库
- 加密算法: AES-256-GCM（主选，算法标签 0x00）+ SM4-GCM-SIV（国密候选，emmansun/gmsm，0x01），按算法标签自动路由
- [按需可选]统一入口运行配置文件：`项目根目录/dbdlp.toml`. 参数优先级: 命令行参数 > toml 配置文件 > 默认值
- 版本号：**CalVer（日历版本）**/ **SemVer（语义化版本）**
- 数据库驱动：
  - Oracle: [go-ora](https://github.com/sijms/go-ora)
  - MySQL: [go-sql-driver/mysql](https://github.com/go-sql-driver/mysql)
  - PostgreSQL: [pgx v5](github.com/jackc/pgx)
  - 达梦: [dm](https://gitee.com/chunanyong/dm)
  - openGauss: [openGauss-connector-go-pq](https://gitee.com/opengauss/openGauss-connector-go-pq)
- 后端存储
  - sqlite(wal)： `modernc.org/sqlite` ：纯 Go 实现，无 CGO
  - postgresql 15+



### 文档和注释

- 项目文档统一存储docs目录，按顺序标号，中文命名的markdown文档
- 日志内容统一使用中文，使用utf8编码
- 统一错误处理，所有错误信息都必须使用中文，禁止打印明文密码等敏感信息文档
- 更新机制：功能迭代后同步更新对应文档，避免文档与代码脱节
- SSH 推送github仓库（git@github.com:bliubiu/dbdlp.git）



### 数据持久化

- 默认使用sqlite数据库存储，支持选用postgresql数据库作为后端数据存储

- sqlite(wal模式) 持久化数据，存放位置：`项目根目录/data/`

- 任务重试状态数据持久化 checkpoint 表



### 日志系统

支持多日志级别（TRACE、DEBUG、INFO、WARN、ERROR），并提供文件输出和日志轮转功能。 

- 日志级别：支持 TRACE、DEBUG、INFO、WARN、ERROR 五个级别，可通过配置文件指定日志级别，默认 INFO 级别
- 时间戳格式：YYYY-MM-DD HH:MM:SS.SSS  本地时间，不含时区
- 日志格式：`YYYY-MM-DD HH:MM:SS.SSS  日志级别  trace_id  线程ID或进程ID  模块:行号 - 日志内容`、附加字段（实例、schema、表名、字段名、耗时、错误栈等
- 日志输出：支持文件输出和终端输出，文件输出路径默认 `项目根目录/logs/`，日志文件按日期轮转（每日一个文件），保留 32 天日志，自动清理过期日志
- 日志内容：统一使用UTF-8编码，中文语言，清晰记录操作行为、执行结果、错误信息，便于问题排查和审计
- 日志内容不能记录敏感信息，必须严格遵循脱敏策略，禁止记录：密码、完整身份证号、手机号等敏感数据或隐私数据
- 文件命名格式：`项目名-YYYYMMDD.log`，例如 `dbdlp-20231225.log`
- 日志内容：统一使用中文，清晰记录操作行为、执行结果、错误信息，便于问题排查和审计



#### 日志级别

级别顺序：`TRACE > DEBUG > INFO > WARN > ERROR`

| 级别  | 用途                                                       |
| ----- | ---------------------------------------------------------- |
| TRACE | 微步骤追踪，极底层，**一般只给开发深度排障**               |
| DEBUG | 模块级调试细节，流程逻辑                                   |
| INFO  | 关键状态变化信息                                           |
| WARN  | 异常但已降级处理。 存在风险 / 异常点，需要关注，不阻断任务 |
| ERROR | 功能失败，局部失败，隔离失败对象                           |

> 内部保留 `FATAL/PANIC`，不在配置项对外开放
>
> - FATAL：启动致命错误（配置损坏、驱动加载失败），打印日志后进程退出
> - PANIC：代码非预期异常，打印堆栈，仅开发调试



### 版本控制

#### CHANGELOG 规范

> 版本号类型说明：**CalVer（日历版本）**/**SemVer（语义化版本）**
>
> - CalVer 版本号格式：`YYYY.MM.DD.MICRO`（完整年份，如 `2026.09.11.3`）
> - SemVer 格式：`主版本号Major.次版本号Minor.补丁号Patch[-可选标签 alpha|beta|rc]`

- 使用CalVer（日历版本）版本号
- **统一版本规范格式**：`YYYY.MM.DD.MICRO`（年.月.日.迭代号）
- 项目根目建立 CHANGELOG.md 文档，单个CHANGELOG.md 文档随时间线推移逐渐变得臃肿，定期归档到docs\changelogs目录
- 根据迭代内容，严格按照 SemVer/CalVer 规则确定新版本号
- 在稳定分支打 Tag，Tag 名称与版本号一致
- Tag 仅允许正式发布时创建，禁止随意打测试 Tag


##### 格式模板

```
## [ 2026.09.11.3 ] - 0.1.0
### ⚠️ Breaking Changes  破坏性变更
- 【用户模块】废弃旧版登录接口 /api/v1/login，请迁移至 /api/v2/login

### ✨ New Features 新增功能
- 【权限模块】新增角色批量授权功能 #123

### 🐛 Bug Fixes  问题修复
- 【订单模块】修复极端场景下订单状态同步异常问题 #121

### 📈 Improvements 性能/体验优化
- 【接口服务】优化列表查询性能，响应速度提升40%

### 📚 Docs 文档更新
- 更新权限模块使用文档与接口说明

### 🔧 Dependencies 依赖更新
- 

### 🗑 Deprecated 废弃功能
- 
```



#### 构建产物

1. 产物命名统一格式： `{产品名称}-{平台}-{架构}.{可选后缀名称}`

   - 产品名称，如：dbdlp

   - 平台： window|linux|macos

   - 架构： amd64|arm64

   - windows平台建议使用文件后缀名，linux默认不含文件后缀名

   - 示例：

     ```
     dbdlp-linux-amd64
     dbdlp-linux-amd64.sha256
     ```

     

2. 正式发布时生成对应产物文件的SHA256 checksum 校验文件

3. 正式发布时同时推送远端仓库，tag, release





### 安全与约束

#### 数据加密

- 数据库敏感数据（如密码,IP地址）以 `ENC(base64)` 格式加密存储，加载时自动解密、保存时自动加密，无需人工干预

- 加密算法：AES-256-GCM（主选），候选 SM4-GCM-SIV 

  - 解密时自动读取载荷首字节算法标签，路由到对应算法解密

  - **密文格式**：`ENC(base64密文)`，其中 base64 解码后的二进制载荷布局为：

    ```sh
    [算法标签 1字节] + [nonce 12字节] + [ciphertext 密文] + [tag 认证标签 16字节]
    ```

  - 算法标签 (1字节) 选择加密算法

    | 算法标签 | 加密算法    | 说明     |
    | -------- | ----------- | -------- |
    | `0x00`   | AES-256-GCM | 主选算法 |
    | `0x01`   | SM4-GCM-SIV | 候选算法 |

  - **密钥管理**：首次运行自动生成 32 字节随机密钥并保存到 `项目根目/.dbdlp/key`二进制文件（Unix 权限 0600），禁止手工编辑。AES-256-GCM 使用完整 32 字节，SM4-GCM-SIV 使用前 16 字节。

- 加密串格式： ENC(base64密文)， 避免循环加密，自动识别加密标识符`ENC()`

- **安全策略**：密钥获取失败（文件损坏、长度错误、环境变量无效）直接报错终止，绝不回退到默认密钥或全零密钥。

- 统一数据库连接信息配置文件（被监控数据源），敏感信息使用加密算法

  - 当且仅当被监控数据源连接测试验证成功时保存，失败则抛出错误信息

- 流式处理大结果集：当 SQL 执行结果集过大时，采用流式读取，避免内存溢出，同时支持分页展示和导出。


#### 脱敏策略

- 密码、IP、身份证、手机号、邮箱等敏感信息实现脱敏
  - IPv4：`192.168.10.3`脱敏为 `*.*.10.3`。127.0.0.1,localhost 可以保留完整
  - 其他类型敏感数据使用`*`号替换
- 识别敏感字段可使用字节扫描，避免使用低效的正则匹配规则
- SQL输出字面量结果需脱敏
- 运行日志/报告文件(html/excel等格式报告文件)需脱敏，避免敏感数据泄露



#### 约束

- 禁止打印明文敏感数据



## 附录

### 参考文档

- github.com/bytedance/godlp
- https://github.com/hitsz-ids/openDLP
- DBMasker（Apache 2.0，Java SDK）



# AI Assistant Guidelines (Karpathy Standard)

## Core Principles

1. **Clarify first, don't assume**
   If the request is ambiguous, unclear, or underspecified, ask brief clarifying questions before writing code or proceeding. Do not guess intent.

2. **Simplicity over cleverness**
   Prefer simple, readable, minimal code. Avoid over-engineering, unnecessary abstractions, fancy patterns, or premature optimization. Keep solutions straightforward.

3. **Minimal changes only**
   Make only the smallest necessary edits to solve the problem. Do not refactor unrelated code, do not reformat entire files, do not change coding style arbitrarily. Stay within the scope of the request.

4. **Goal-driven work**
   Stay focused on the immediate task. Do not add extra features, improvements, or "helpful" additions that were not requested. Solve exactly what is asked.

## Code Style

- Write clean, standard, idiomatic code for the language.
- No overly verbose comments; code should speak for itself.
- Keep functions and logic concise.



## 语言强制规则（必须严格遵守）

1. 所有回复、解释、对话、说明必须**全程使用中文**，禁止出现任何英文。
2. 不允许切换语言，全程保持纯中文交互。
3. 解释、说明、思考过程、总结、步骤描述，全部使用中文。