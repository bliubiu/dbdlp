# dbdlp 敏感数据识别功能测试集

用于在 **Oracle** 与 **达梦（DM8）** 测试库中构建敏感数据测试表，验证 dbdlp 扫描识别能力。全部数据为构造数据，不含真实敏感信息。

## 目录结构

```
testdata/
├── README.md                          本说明
├── oracle/
│   ├── dbdlp_testdata_oracle.sql      Oracle 版自研测试集
│   └── dbdlp_testdata_opendlp_oracle.sql      Oracle 版 openDLP 来源测试集
├── dameng/
│   ├── dbdlp_testdata_dameng.sql      达梦版自研测试集（与 Oracle 版主体一致）
│   └── dbdlp_testdata_opendlp_dameng.sql      达梦版 openDLP 来源测试集
└── tools/
    └── opendlp_to_sql.py              openDLP 样本→SQL 转换器（可全量再生）
```

## 执行方式

### Oracle（11g+，字符集建议 AL32UTF8）

```sh
NLS_LANG=.AL32UTF8 sqlplus 用户名/密码@主机:1521/服务名 @testdata/oracle/dbdlp_testdata_oracle.sql
```

### 达梦（DM8+，实例字符集 UTF-8 或 GB18030 均可）

**达梦版脚本文件编码为 GB18030**（`testdata/dameng/*.sql`）：disql 读脚本文件固定按 GB18030 解码后再转服务端字符集发送，UTF-8 编码文件会导致中文值双重编码乱码（症状为大量 `[-2007]` 语法错误与 `[-70037]` 字符串不完整）。若自行用工具导出 UTF-8 版本，执行前须转换：

```sh
iconv -f UTF-8 -t GB18030 输入.sql > 输出.sql
disql 用户名/密码@主机:5236 `testdata/dameng/dbdlp_testdata_dameng.sql`
```

- 脚本幂等，可重复执行（开头自动清理同名旧表）。
- 执行末尾输出各表行数，应与下表一致。
- 脚本需 UTF-8 编码执行；若终端中文乱码，请确认客户端字符集（Oracle：`NLS_LANG=SIMPLIFIED CHINESE_CHINA.AL32UTF8`）。

## 表与数据总览

| 表名 | 行数 | 覆盖类型 | 设计要点 |
|---|---|---|---|
| T_DBDLP_TEST_CUSTOMER | 22 | 人名/身份证/手机号/邮箱/银行卡/地址 | 混合列，行5/10/15/20为负样本行，行21~22超出默认20行抽样窗口 |
| T_DBDLP_TEST_PERSON | 22 | 人名/固话/邮编/地址 | 语义类+中低置信类，负样本行同上 |
| T_DBDLP_TEST_ENTERPRISE | 22 | 公司名/统一社会信用代码/域名/邮箱 | 负样本行同上 |
| T_DBDLP_TEST_NETWORK | 22 | IPv4/IPv6/MAC/域名 | 行8含127.0.0.1（脱敏保留场景）；IPv6/MAC为预留数据 |
| T_DBDLP_TEST_VEHICLE | 22 | 车牌（含新能源）/护照/车主姓名 | 负样本行同上 |
| T_DBDLP_TEST_TEXT_LOB | 6 | JSON内嵌/CLOB超长/中文上下文/空值族/已掩码值 | 行2超长文本由匿名块生成（约4000字节） |
| T_DBDLP_TEST_NEGATIVE | 20 | 通用负样本池 NG-01~NG-06 | 逐行标注类别，验收口径：被判高风险数必须为 0 |
| T_DBDLP_TEST_BIGTABLE | 1000 | 手机号/邮箱/IPv4 + 非敏感对照 | SQL 表达式生成，用于主键分页/断点续扫/流式读取验证 |

> 各小表行 5/10/15/20 为负样本行、其余为正样本行，默认 20 行抽样窗口（主键升序取前 20 行）即可同时覆盖正负样本；行 21~22 用于验证抽样截断与主键分页。

## 期望识别结果对照表

正样本期望「正则命中＋校验通过」，负样本期望「不命中或仅格式命中进待复核」，不得判高风险（对应 docs/07 §3）。

### T_DBDLP_TEST_CUSTOMER

| 列 | 期望敏感类型 | 风险 | 负样本行预期 |
|---|---|---|---|
| CUST_NAME | 人名（语义） | 中（需复核） | — |
| ID_CARD_NO | 身份证 | 高 | 行5 占位值110101199001011234（校验位错）；行10 末位错；行15 2月30日非法日期；行20 15位旧号 |
| MOBILE_NO | 手机号 | 高 | 行5 全同号11111111111；行10 非法号段12345678901；行15 12位超长；行20 7位过短 |
| EMAIL | 邮箱 | 中 | 行5 缺@；行10 双@；行15 无@；行20 双@变体 |
| BANK_CARD_NO | 银行卡 | 高 | 行5 18位订单号（不通过Luhn）；行10 末位错；行15 20位超长；行20 含字母 |
| REG_ADDRESS | 地址（语义） | 中（需复核） | — |
| MEMO | 不命中 | 无 | 全列普通文本对照 |

### T_DBDLP_TEST_PERSON

| 列 | 期望敏感类型 | 风险 | 负样本行预期 |
|---|---|---|---|
| REAL_NAME | 人名（语义） | 中（需复核） | 行5 地名；行10 商品名；行15 泛指词；行20 通用词 |
| FIXED_PHONE | 固话 | 中 | 行5 无0开头；行10 过短；行15 带国家码前缀；行20 带分机号 |
| POST_CODE | 邮编 | 中 | 行5 7位；行10 含字母；行15 5位；行20 含连字符 |
| HOME_ADDRESS | 地址（语义） | 中（需复核） | 负样本行为普通文本 |
| REMARK | 不命中 | 无 | — |

### T_DBDLP_TEST_ENTERPRISE

| 列 | 期望敏感类型 | 风险 | 负样本行预期 |
|---|---|---|---|
| COMPANY_NAME | 公司名（语义） | 中（需复核） | 行5/10/15/20 泛指句/通用词 |
| SOCIAL_CREDIT_CODE | 统一社会信用代码 | 高 | 行5 末位错；行10 17位；行15 19位；行20 含非法字符I |
| DOMAIN_NAME | 域名 | 中 | 行5 无顶级域；行10 邮箱本地误当域名；行15 标签以连字符开头；行20 双点 |
| CONTACT_EMAIL | 邮箱 | 中 | 负样本行对应邮箱负样本 |
| BUSINESS_SCOPE | 不命中 | 无 | — |

### T_DBDLP_TEST_NETWORK

| 列 | 期望敏感类型 | 风险 | 负样本行预期 |
|---|---|---|---|
| IPV4_ADDR | IPv4 | 中 | 行5 段超255；行10 段数不足；行15 段数过多；行20 非数字 |
| IPV6_ADDR | IPv6（预留） | — | 行5 双冒号；行10 共9组；行15 非法十六进制；行20 全压缩空地址 |
| MAC_ADDR | MAC（预留） | — | 行5 仅5组；行10 含字母G；行15 空段 |
| HOST_DOMAIN | 域名 | 中 | 负样本行对应域名负样本 |
| SERVER_DESC | 不命中 | 无 | — |

> 特例：行8 IPV4_ADDR = `127.0.0.1`，按脱敏策略应保留完整不脱敏（`192.168.10.3` 等应脱敏为 `*.*.10.3`）。
> IPv6/MAC 当前内置规则未实现，测试集数据为规则上线预留，未实现前不应产生误判。

### T_DBDLP_TEST_VEHICLE

| 列 | 期望敏感类型 | 风险 | 负样本行预期 |
|---|---|---|---|
| PLATE_NO | 车牌 | 中 | 行5 省份「森」非法；行10 第二位非字母；行15 缺省份；行20 总长不足 |
| PASSPORT_NO | 护照 | 高 | 行5 数字仅7位；行10 数字9位；行15 纯数字；行20 尾部为字母 |
| OWNER_NAME | 人名（语义） | 中（需复核） | 负样本行对应人名负样本 |
| TRIP_NOTE | 不命中 | 无 | — |

### T_DBDLP_TEST_TEXT_LOB（边界样本）

| 行 | 场景 | 期望 |
|---|---|---|
| 1 | JSON 内嵌身份证/手机号/银行卡/邮箱（BD-01） | 提取叶子值识别，命中归正确类型，不把整个JSON当一值 |
| 2 | CLOB 约4000字节中文+中部埋点（BD-02） | 按 maxTextLen=2000 截断仍识别段内命中或记录截断决策，不崩溃不静默丢列 |
| 3 | 中文上下文号码（BD-03） | 正确切出候选过校验，无数字粘连误判 |
| 4 | JSON 含已掩码值（NG-04） | 掩码值不命中、不二次掩码、不还原 |
| 5 | 空值族 NULL/空串（BD-05） | hit_ratio=0 不除零，无异常 |
| 6 | 普通中文文本（NG-05） | 不命中 |

### T_DBDLP_TEST_NEGATIVE（通用负样本池，验收：被判高风险数=0）

| SAMPLE_TYPE | 行号 | 样本值 | 期望 |
|---|---|---|---|
| NG-01 订单号 | 1 | 202611140000123456（18位，不通过Luhn） | 不命中 |
| NG-01 订单号 | 2 | 2026010112345678（16位，**碰巧通过Luhn**） | 已知边界：用于验证结合字段上下文降误报，期望不判高；若引擎仅按校验器判定，此行即为暴露的误报点 |
| NG-01 订单号 | 3 | SO202600001234（带字母前缀） | 不命中 |
| NG-01 订单号 | 4 | 10000012345678901（17位） | 不命中或仅格式命中进待复核 |
| NG-01 订单号 | 5 | 98765432109876543210（20位超长） | 不命中 |
| NG-02 随机数字串 | 6~9 | 11/16/18/19位随机串（均不通过Luhn） | 不命中或仅格式命中进待复核 |
| NG-03 占位值 | 10 | 110101199001011234 | 不命中落负样本池 |
| NG-03 占位值 | 11 | 0000000000000000 | 已知边界：通过Luhn（校验器无全同号排除），期望结合规则降级，用于验证待复核分流 |
| NG-03 占位值 | 12~13 | 全1串 / 全9串 | 不命中或仅格式命中 |
| NG-04 已掩码值 | 14~17 | 138****1234、1101**********1234、6222**********1234、*.*.10.3 | 不命中、不二次掩码、不还原 |
| NG-05 普通中文 | 18~19 | 业务描述句 | 不命中 |
| NG-06 噪声 | 20 | UUID 550e8400-… | 不命中 DOMAIN/ID_CARD/BANK_CARD |

### T_DBDLP_TEST_BIGTABLE（分页大表）

| 列 | 期望敏感类型 | 说明 |
|---|---|---|
| ID | 主键 | 1~1000 连续，用于主键范围分页与断点续扫（WHERE pk > last_key） |
| MOBILE_NO | 手机号 | 13910000001~13910001000，1000 行全命中 |
| EMAIL | 邮箱 | user1@example.com~user1000@example.com，1000 行全命中 |
| IPV4_ADDR | IPv4 | 10.0.0.1~10.0.3.232，1000 行全命中 |
| ORDER_NO | 不命中 | SO2026+10位流水，带字母前缀，不误判为卡号 |
| CUST_LEVEL | 不命中 | 1~3 数值 |

## 验收检查清单

1. **报告内容**：实例、模式、表、字段、样本（脱敏）、命中数、风险等级齐全；三处（SQL字面量/日志/报告）无完整明文。
2. **召回口径**（EV-02）：身份证/银行卡/手机号/信用代码/邮箱/IPv4 ≥95%；车牌/邮编/固话/护照/域名 ≥85%；语义类（人名/地址/公司名）单列评估，最高置信「中」且进人工复核（SEM-01）。
3. **误报口径**（EV-03）：T_DBDLP_TEST_NEGATIVE 被判高风险数必须为 0。
4. **待复核分流**（EV-04）：仅格式命中/校验未过 100% 进 pending_review。
5. **分页与续扫**（IT-03）：大表主键分页无深偏移 OFFSET，中断后从 last_key 续扫不重复，单列 sample_total ≤ 抽样上限。
6. **脱敏特例**：127.0.0.1 保留完整，其余 IPv4 按 `*.*.10.3` 形态脱敏。

## 正样本数据合规性说明

所有身份证、银行卡、统一社会信用代码正样本的校验位均按 `internal/domain/rule/verifier.go` 的算法（身份证加权模11＋出生日期、Luhn 模10、GB32100 信用代码模31）生成并复核通过，可直接用于「正则命中＋校验通过」链路验证。

---

## openDLP 来源测试集（dbdlp_testdata_opendlp_*.sql）

数据来源：[hitsz-ids/openDLP](https://github.com/hitsz-ids/openDLP)（Apache-2.0）`tests/data/`，经 `testdata/tools/opendlp_to_sql.py` 转换生成。所有数据为程序生成的模拟数据，无真实隐私风险；引入使用请遵守 Apache-2.0 许可。

### 表结构

| 表 | 行数 | 用途 |
|---|---|---|
| T_DBDLP_TEST_OPENDLP_PAIRS | 14000 | 长表：逐条正负样本断言。14 类 × 500 对 × 2（正/负各 1 行），按类轮转交错排列，前 28 行即覆盖全部 14 类正负样本 |
| T_DBDLP_TEST_OPENDLP_WIDE | 1000 | 宽表：字段级扫描基准。综合表 17 列全类型（PERSON/ID_CARD/…/POSTCODE/SAMPLE_DATE），直接用 dbdlp 扫描出报告基线 |

长表列：`PAIR_ID`（主键）、`SAMPLE_TYPE`（openDLP 原类型名）、`IS_POSITIVE`（1 正/0 负）、`SAMPLE_VALUE`。
宽表列名保留 openDLP 原名并附中文注释，仅一处例外：源列 `DATE` 因是 Oracle 保留字，SQL 列名改为 `SAMPLE_DATE`（转换器内 `WIDE_SQL_NAMES` 映射）。

### 类型映射（openDLP → dbdlp RuleID）

| openDLP | dbdlp | 当前内置 | 说明 |
|---|---|---|---|
| ID_CARD / BANK_CARD / MOBILE_PHONE / EMAIL / SOCIAL_CREDIT_CODE / PASSPORT / POSTCODE / IPV4 | 同名 | ✓ | 直接对应 |
| TELEPHONE | FIXED_PHONE | ✓ | 改名映射 |
| LICENSE_PLATE | PLATE | ✓ | 改名映射 |
| IPV6 / MAC / DOMAIN_NAME | IPV6 / MAC / DOMAIN | ✗ 预留 | 规则上线前不应误判 |
| DATE | — | ✗ 预留 | dbdlp 无日期规则 |

### 验收口径

1. **召回率断言**（长表，按类统计）：
   ```sql
   -- 期望：高置信校验类（ID_CARD/BANK_CARD/MOBILE_PHONE/EMAIL/SOCIAL_CREDIT_CODE/IPV4）正样本命中率 ≥95%
   --      中置信格式类（PASSPORT/LICENSE_PLATE/POSTCODE/TELEPHONE）≥85%（docs/07 EV-02）
   SELECT SAMPLE_TYPE,
          SUM(CASE WHEN IS_POSITIVE = 1 THEN 1 ELSE 0 END) AS POSITIVE_CNT,
          SUM(CASE WHEN IS_POSITIVE = 0 THEN 1 ELSE 0 END) AS NEGATIVE_CNT
   FROM T_DBDLP_TEST_OPENDLP_PAIRS GROUP BY SAMPLE_TYPE;
   ```
2. **误报口径**（EV-03）：负样本（IS_POSITIVE=0）被判高风险数必须为 0。
3. **变形样本**：openDLP 手机号正样本含 `+86 前缀`、`134-4888-1249`、空格分隔等变形（对应 RS-07/BD-03 边界）。当前内置规则为整串正则匹配，变形值不会命中——此为**如实暴露的引擎差距**，非数据问题；规则增强为「切分候选」后应逐步命中。
4. **宽表基准**：扫描宽表生成报告，逐列期望敏感类型见列注释；PERSON/COMPANY_NAME/LOCATION 列用于语义类（人名/公司名/地址）评估。

### 执行方式

与自研测试集相同（sqlplus / disql），脚本幂等可重复执行，尾部自带按类统计查询核对装载结果。达梦版文件为 GB18030 编码（见前文执行方式说明）。

### 实际执行验证记录（e2e 环境）

以下脚本已在 e2e 环境实际装载并完成 **dbdlp 端到端扫描验证**（Oracle 11.2.0.4 EE 容器 + 达梦 DM8 容器，均为 UTF-8 实例）：

| 脚本 | Oracle | 达梦 |
|---|---|---|
| 自研测试集 | 0 错误，8 表 22/22/22/22/22/6/20/1000 行 | 0 错误，行数一致，中文 HEX 校验为标准 UTF-8 |
| openDLP 测试集 | 0 错误，长表 14 类各 500/500，宽表 1000 行 | 0 错误，统计一致，中文抽查正确 |

### e2e 测试数据源（dbdlp 已登记）

| 项 | Oracle | 达梦 |
|---|---|---|
| 数据源名称 | ORA-E2E-TEST | DM-E2E-TEST |
| 方言 | oracle | dameng |
| 主机 | `*.*.10.180`（内网 e2e 测试机，完整地址见环境部署） | 同左 |
| 端口 | 1521 | 5236 |
| 服务名/模式 | 服务名 `XE` | 库名 `SYSDBA` |
| 扫描模式 | `DBDLP_TEST` | `DBDLP_TEST` |
| 账号 | system（口令见容器环境变量 `ORACLE_PWD`） | SYSDBA（口令见容器环境变量 `SYSDBA_PWD`） |
| 容器 | `e2e-oracle`（oracle/database:11.2.0.4-ee） | `e2e-dm8`（dm8:v250912-sec-kylin10sp3-2403） |

> 安全说明：两库口令仅存在于容器环境变量，不落任何文档/代码；数据源登记后口令以 `ENC(...)` 加密存于本地 `data/dbdlp.db`。测试表统一置于专用业务用户 `DBDLP_TEST`（扫描器按设计强制排除 SYSTEM/SYSDBA 等系统模式，`include-schemas` 无法覆盖）。

#### 端到端扫描核验结论（2026-11-14）

两库各扫描 10 表 / 66 字段 / 命中字段 28，**逐字段结果完全一致**（交叉验证）：

- **正样本召回符合预期**：手机/邮箱/IPv4/车牌/邮编/固话列在采样 20 行下命中 16/20（负样本行 5/10/15/20 精确不命中）；身份证/银行卡/信用代码/护照列命中并校验通过；BIGTABLE 千行表手机/邮箱/IPv4 全命中（20/20）。
- **脱敏合规**：报告样本全部掩码（`139****0001`、`110101********1006`、`u****@example.com`、`*.*.10.3`），`127.0.0.1` 按策略**原样保留**。
- **对照列零误报**：MEMO/REMARK/ORDER_NO/CUST_LEVEL/BUSINESS_SCOPE/TRIP_NOTE/SERVER_DESC 等非敏感列无任何命中。
- **暴露的引擎边界**（即测试集价值，待引擎迭代）：
  1. 仅格式命中（校验未过、置信 low）未进 `pending_review` 分流（EV-04 口径未达成），随列聚合入 high 的需按规则逐条降级；
  2. 纯数字串与信用代码字符集重叠：`202611140000123456`（订单号）碰巧通过模31校验被判 high——需结合字段名/上下文降误报（NG-01 预期行为）；
  3. openDLP 手机号变形样本（`+86` 前缀、分隔符）整串正则仅命中 4/20（RS-07/BD-03 预期差距，规则增强后应召回）。
- **过程中修复的驱动兼容缺陷**（`internal/infrastructure/datasource/oracle.go`）：① Oracle 占位符误用 `?`（go-ora 需 `:N`）；② 分页使用 12c 语法 `FETCH FIRST`（11g 不支持），已改为 ROWNUM 三明治分页并修正 `__RN` 标识符引号。

### 全量再生

默认转换规模：每类 500 对、宽表前 1000 行（SQL 约 1.4MB）。需要全量（每类 5000 对、宽表 10000 行）时：

```sh
# 1. 下载 openDLP 测试数据（tests/data 目录）到本地目录
# 2. 运行转换器
python testdata/tools/opendlp_to_sql.py \
  --source-dir <openDLP/tests/data 目录> \
  --out-oracle testdata/oracle/dbdlp_testdata_opendlp_oracle.sql \
  --out-dameng testdata/dameng/dbdlp_testdata_opendlp_dameng.sql \
  --pairs-per-type 5000 --wide-rows 10000
```
