# 悬而未决登记册

> 项目：dbdlp 数据库敏感数据扫描发现平台
> 建立日期：2026-11-13 ｜ 维护者：项目总监（编排层）
> 纪律：**只追加、就地关闭**。条目解决时把 `OPEN` 翻成 `RESOLVED` 并补 `Resolution`，绝不删除历史痕迹。
> 复现要求：每次阶段开工时，把仍为 `OPEN` 的条目复现到上下文最前，带「N 未决 + M 已决」汇总，逐条判断能否关闭。

当前汇总：**7 未决 + 4 已决**

## 来源说明

本册首建于 Phase 1 文档审查。旧版 PRD 第 10 章与旧架构文档第 17 章共留 10 条「待确认事项」，可由 `AGENTS.md` 总纲直接定调的已在修订中就地关闭；仍受外部条件或后续任务约束的落于下方，避免随上下文滚动丢失。

---

## RESOLVED — waiting-on-external-condition — U-01 openGauss 驱动签名 + U-03 目标库字典视图真机验证

- Date: 2026-11-13
- Source: Phase 0/1 文档审查；旧 PRD 第 10 章第 1 条、旧架构文档第 17 章第 1 条
- Open item（历史）: 首批落地环境 Oracle／达梦／PG／openGauss 具体小版本、可用驱动形态与字典视图尚未实测。
- Related constraints: 总纲锁定 Oracle 用 sijms/go-ora、达梦用 chunanyong/dm；字典视图、加密列字典、分页语法随版本变化；识别须在客户端完成。
- Resolution: 2026.11.13.3 真机验证通过——Oracle（e2e-oracle 11.2.0.4）、达梦（e2e-dm8）、openGauss（e2e-gauss 6.0.0）字典视图、系统模式与分页语法均确认可用，未验证视图不得写进生产代码（**U-03 已关闭**）；openGauss 驱动签名已取得（**U-01 已关闭**），但实现仍按总纲本期暂缓排期。关联 ADR-002。

## OPEN — waiting-on-external-condition — 首批纳入扫描的实例清单与业务归属

- Date: 2026-11-13
- Source: 旧 PRD 第 10 章
- Open item: 首批要扫描的实例地址、模式范围、业务归属、排除清单未确定。
- Related constraints: 只读账号、连接池上限、低峰窗口、限速阈值均依赖实例规模；资产数据源须落后端存储库并支持增删改查与批量导入。
- Current leaning: 以控制台录入为主，提供 excel/yaml 批量导入模板，连接测试通过才允许保存。
- Blocked by: 安全合规人员与 DBA 给出资产台账。
- Resolves when: 首批实例清单录入并通过连接测试，导入模板定稿。

## OPEN — waiting-on-external-condition — U-04 企业内部分类分级标准是否对接

- Date: 2026-11-13
- Source: 旧 PRD 第 10 章、旧架构文档第 17 章
- Open item: 风险等级映射是否对齐企业内部数据分类分级规范，尚未拿到规范文本。
- Related constraints: 总纲要求置信度／分级／风险分级可配置；硬编码会无法适配不同企业。
- Current leaning: 高／中／低／待复核映射表配置化，默认「明文高敏感＝高、已掩码＝低、仅格式命中＝待复核」。
- Blocked by: 用户提供企业规范或确认用默认策略。
- Resolves when: 用户确认默认映射表，或提供规范文本并完成映射配置。

## OPEN — waiting-on-external-condition — U-02 国密 SM4-GCM-SIV 切换时机

- Date: 2026-11-13
- Source: `AGENTS.md` 技术架构与安全约束
- Open item: 总纲标注 SM4-GCM-SIV「待 gmsm 库支持后切换」，当前 0x01 落 SM4-GCM 还是暂缓，需核实库能力后定。
- Related constraints: 密文布局固定「1B 标签＋12B nonce＋密文＋16B tag」；解密按首字节路由；密钥 32 字节，国密取前 16 字节。
- Current leaning: 0x00（AES-256-GCM）主选且须完整可用；0x01 先按 SM4-GCM 实现并保留标签位，SIV 待库支持后同标签升级。
- Blocked by: emmansun/gmsm 当前版本是否提供 SM4-GCM-SIV 的实测结论。
- Resolves when: 架构师联网核实库能力并写入决策记录；若不支持则保持 OPEN。

## OPEN — design-decision-to-evaluate — 人名／地址／公司名的本期识别实现方式

- Date: 2026-11-13
- Source: `AGENTS.md` 敏感识别清单；旧 PRD 推到后续 NER 阶段
- Open item: 本期用「字典＋正则＋字段名辅助」还是引入语义模型，尚未评估。
- Related constraints: 离线单二进制、不依赖外网模型；域名／IPv6／MAC 等同需明确层级。
- Current leaning: 以自研规则层（字节扫描＋正则＋校验器＋中文分词词典辅助）覆盖，语义模型列后续，避免破坏离线部署。
- Blocked by: 需要真实样本做召回／误报基线评测。
- Resolves when: 规则与算法规格给出方案与正负样本集，经基线评测确认可用。

## OPEN — design-decision-to-evaluate — U-08 网页控制台的认证与权限方案

- Date: 2026-11-13
- Source: Phase 0 用户决策「本期纳入控制台，按总纲前端栈实现」；总纲未规定认证方案
- Open item: 控制台是否需登录、角色划分与报告导出权限控制，方案未定。
- Related constraints: 总纲要求报告导出需权限控制与审计、脱敏、日志留痕；存储库为 SQLite（可选 PG）。
- Current leaning: 轻量会话认证＋角色区分（安全合规／DBA／审计），明细导出单独授权并记审计；不做复杂 SSO 集成。
- Blocked by: 用户是否已有统一身份源；控制台部署边界。
- Resolves when: 用户确认部署边界与身份来源，架构文档锁定认证方案并写入建表语句。

## OPEN — existing-design-boundary — 自适应采样与抽样代表性

- Date: 2026-11-13
- Source: `AGENTS.md` 任务调度「每列抽样 N（默认 20）行，主键分页；后续增强为自适应采样」
- Open item: 本期固定每列 20 行，抽样代表性不足时可能漏少量敏感行，尚未约定补偿机制。
- Related constraints: 低侵入、不影响生产；大表降级；报告须标注样本量与命中率。
- Current leaning: 报告显式标注样本量、命中率与「未覆盖」提示，提供高风险字段追加扫描入口；自适应采样列后续。
- Blocked by: 需要真实库表分布数据设计自适应策略。
- Resolves when: 有线上扫描数据支撑自适应阈值，或用户接受本期边界并签字确认。

## OPEN — existing-design-boundary — 主备／同步库／数仓血缘与备份集覆盖

- Date: 2026-11-13
- Source: 旧 PRD 第 1.4/6 章、旧架构文档第 6 节
- Open item: 本期只按手动配置实例清单逐个扫描，不自动发现主备关系、同步链路与数仓血缘，也不扫导出备份集。
- Related constraints: 多数据源抽象要求「统一数据源」，但自动血缘需额外元数据与权限；离线只读定位。
- Current leaning: 写入「不做」清单并注明原因，由实例清单手动纳入备库与同步库，避免「主库脱敏、备库明文」遗漏。
- Blocked by: 用户确认本期是否接受该覆盖边界。
- Resolves when: 用户确认边界，或后续立项自动血缘发现时关闭本条并升级为 ADR。

## RESOLVED — 总纲条款摘录中转与 U8 关闭

- Date: 2026-11-13
- Source: Phase 1 产品经理回传 U8；AGENTS.md 读取受内容安全审批拦截
- Open item（历史）: 产品／设计需总纲「前端技术栈锁定值」与「日志指定格式」原文条款方可落施工级。
- Resolution: 2026-11-13 已关闭。项目总监转出前端技术栈锁定值（TS 7.0+／Next.js 16.2+ App Router／React 19.2+／Tailwind／ECharts／Zustand／shadcn／TanStack Table）与日志条款（默认 ERROR、时间戳 YYYY-MM-DD HH:MM:SS.SSS、含模块:行号、logs/、dbdlp-YYYYMMDD.log、按日轮转保留 32 天、UTF-8 中文、禁记密码与完整身份证／手机号），产品经理据此完成 01 文档订正。后续用户裁定废除二手摘录，全库改以磁盘 AGENTS.md 原文为唯一基准，摘录通道废止。

## RESOLVED — 总纲配置目录命名与文档集不一致（U-06）

- Date: 2026-11-13
- Source: Phase 1 架构师回传；项目总监独立核实磁盘 AGENTS.md
- Open item（历史）: 会话快照写 .dbcli/key、.dbcli/ds、dbcli.toml；磁盘总纲第 169 行写 .dbdlp/key（dbcli 计数 0），命名并存会导致凭据找不到。
- Resolution: 2026-11-13 用户裁定对齐总纲，配置目录与密钥统一为 `.dbdlp/key`，`.dbcli` 作废；`.dbdlp/ds` 移除，数据源仅持久化到 data_source 表。02/03/06/07/09/CHANGELOG 已同步收敛。（行号复核：原「第 169 行」应为第 223 行，语义不变；总纲「项目根目」为笔误，不回写。）

## RESOLVED — 磁盘总纲原文与文档基线取值偏差（基准切换暴露）

- Date: 2026-11-13
- Source: 用户裁定「以 AGENTS.md 为纲、废除二手摘录」；磁盘总纲原文（291 行）逐条核对全库
- Open item（历史）: ①日志级别集合；②日志默认级别；③任务重试状态载体（登记笔误，实为一致）。
- Resolution: 2026-11-13 用户裁定：①日志支持五级（TRACE/DEBUG/INFO/WARN/ERROR）可配置，默认 INFO，FATAL/PANIC 内部保留；②同①默认 INFO；③重试状态维持 `scan_checkpoint` 表、不用 json 文件。全库 01/02/03/05/06/07/08/09 已改为五级默认 INFO 同步改造。
