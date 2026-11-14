#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""openDLP 样本集 → dbdlp SQL 测试表转换器。

数据来源: https://github.com/hitsz-ids/openDLP (Apache-2.0)
  - tests/data/regex_generation/test-data/<TYPE>.csv  每类 5000 条正负样本对
  - tests/data/dataset-test.csv                       17 列综合基准表 10000 行

输出两张表（Oracle / 达梦 语法同构）:
  T_DBDLP_TEST_OPENDLP_PAIRS  长表: 逐条正负样本断言用（按类轮转交错排列）
  T_DBDLP_TEST_OPENDLP_WIDE   宽表: 字段级扫描基准用（17 列全类型）

用法:
  python opendlp_to_sql.py --source-dir <csv目录> \
      --out-oracle ../oracle/dbdlp_testdata_opendlp_oracle.sql \
      --out-dameng ../dameng/dbdlp_testdata_opendlp_dameng.sql \
      [--pairs-per-type 500] [--wide-rows 1000] [--batch 50]
"""
import argparse
import csv
import io
import os

# openDLP 类型 → (dbdlp RuleID, 是否当前内置规则)
TYPE_MAP = [
    ("ID_CARD",             "ID_CARD",             True),
    ("BANK_CARD",           "BANK_CARD",           True),
    ("MOBILE_PHONE",        "MOBILE_PHONE",        True),
    ("TELEPHONE",           "FIXED_PHONE",         True),
    ("EMAIL",               "EMAIL",               True),
    ("SOCIAL_CREDIT_CODE",  "SOCIAL_CREDIT_CODE",  True),
    ("PASSPORT",            "PASSPORT",            True),
    ("LICENSE_PLATE",       "PLATE",               True),
    ("POSTCODE",            "POSTCODE",            True),
    ("IPV4",                "IPV4",                True),
    ("IPV6",                "IPV6",                False),
    ("MAC",                 "MAC",                 False),
    ("DOMAIN_NAME",         "DOMAIN",              False),
    ("DATE",                "DATE",                False),
]

WIDE_COLS = [
    "PERSON", "ID_CARD", "TELEPHONE", "MOBILE_PHONE", "EMAIL",
    "LICENSE_PLATE", "BANK_CARD", "PASSPORT", "COMPANY_NAME",
    "SOCIAL_CREDIT_CODE", "IPV4", "IPV6", "MAC", "DOMAIN_NAME",
    "LOCATION", "POSTCODE", "DATE",
]
# DATE 为 Oracle 保留字，不能作列名，宽表列名改用 SAMPLE_DATE（源列仍为 DATE）
WIDE_SQL_NAMES = {
    "PERSON": "PERSON", "ID_CARD": "ID_CARD", "TELEPHONE": "TELEPHONE",
    "MOBILE_PHONE": "MOBILE_PHONE", "EMAIL": "EMAIL", "LICENSE_PLATE": "LICENSE_PLATE",
    "BANK_CARD": "BANK_CARD", "PASSPORT": "PASSPORT", "COMPANY_NAME": "COMPANY_NAME",
    "SOCIAL_CREDIT_CODE": "SOCIAL_CREDIT_CODE", "IPV4": "IPV4", "IPV6": "IPV6",
    "MAC": "MAC", "DOMAIN_NAME": "DOMAIN_NAME", "LOCATION": "LOCATION",
    "POSTCODE": "POSTCODE", "DATE": "SAMPLE_DATE",
}
WIDE_COMMENTS = {
    "PERSON": "姓名", "ID_CARD": "身份证号码", "TELEPHONE": "固定电话",
    "MOBILE_PHONE": "手机号", "EMAIL": "电子邮箱", "LICENSE_PLATE": "车牌号码",
    "BANK_CARD": "银行卡号", "PASSPORT": "护照号码", "COMPANY_NAME": "公司名称",
    "SOCIAL_CREDIT_CODE": "统一社会信用代码", "IPV4": "IPv4地址", "IPV6": "IPv6地址",
    "MAC": "MAC地址", "DOMAIN_NAME": "域名", "LOCATION": "地址",
    "POSTCODE": "邮政编码", "DATE": "日期",
}


def esc(v):
    """SQL 字面量转义：单引号翻倍；空值返回 NULL。"""
    if v is None:
        return "NULL"
    if isinstance(v, int):
        return str(v)
    v = v.strip()
    if v == "":
        return "NULL"
    return "'" + v.replace("'", "''") + "'"


def read_pairs(path, limit):
    """读取正负样本对 CSV（positive,negative 表头），返回 [(positive, negative)]。"""
    pairs = []
    with io.open(path, encoding="utf-8", newline="") as f:
        reader = csv.reader(f)
        next(reader)  # 表头 positive,negative
        for row in reader:
            if not row or not any(c.strip() for c in row):
                continue
            pos = row[0].strip() if len(row) > 0 else ""
            neg = row[1].strip() if len(row) > 1 else ""
            pairs.append((pos, neg))
            if len(pairs) >= limit:
                break
    return pairs


def read_wide(path, limit):
    """读取综合基准表，返回 [ {列名: 值} ]。"""
    rows = []
    with io.open(path, encoding="utf-8", newline="") as f:
        reader = csv.DictReader(f)
        for row in reader:
            rows.append({k: (v or "").strip() for k, v in row.items()})
            if len(rows) >= limit:
                break
    return rows


def batched_inserts(table, cols, rows, batch):
    """生成 UNION ALL SELECT FROM DUAL 风格的批量 INSERT（Oracle/达梦通用）。"""
    out = []
    col_list = ", ".join(cols)
    for i in range(0, len(rows), batch):
        chunk = rows[i:i + batch]
        parts = []
        for j, r in enumerate(chunk):
            sel = ", ".join(esc(v) for v in r)
            kw = "SELECT" if j == 0 else "UNION ALL SELECT"
            parts.append(f"{kw} {sel} FROM DUAL")
        out.append(f"INSERT INTO {table} ({col_list})\n" + "\n".join(parts) + ";\n")
    return out


def gen_header(title, source_dir, pairs_n, wide_n):
    src = os.path.abspath(source_dir)
    return f"""-- ============================================================================
-- {title}
-- ----------------------------------------------------------------------------
-- 数据来源 : hitsz-ids/openDLP 测试数据集（Apache-2.0）
--            https://github.com/hitsz-ids/openDLP  tests/data/
-- 本地源   : {src}
-- 转换工具 : testdata/tools/opendlp_to_sql.py（变更源数据后可全量再生）
-- 规模     : 长表每类 {pairs_n} 对正负样本（14 类共 {pairs_n * len(TYPE_MAP) * 2} 行）；
--            宽表取综合基准表前 {wide_n} 行（17 列全类型）
-- 类型映射 : TELEPHONE→FIXED_PHONE；LICENSE_PLATE→PLATE；DOMAIN_NAME→DOMAIN（预留）；
--            IPV6/MAC/DATE 为 dbdlp 规则预留数据，未实现前不应误判
-- 用法     : 长表配合程序化断言统计召回/误报；宽表直接用 dbdlp 扫描出报告基准
-- ============================================================================

"""


def gen_ddl():
    wide_col_defs = ",\n".join(f"  {WIDE_SQL_NAMES[c]} VARCHAR2(128)" for c in WIDE_COLS)
    return """-- ============================================================================
-- 0. 清理旧表
-- ============================================================================
BEGIN
  EXECUTE IMMEDIATE 'DROP TABLE T_DBDLP_TEST_OPENDLP_PAIRS CASCADE CONSTRAINTS';
EXCEPTION WHEN OTHERS THEN NULL;
END;
/
BEGIN
  EXECUTE IMMEDIATE 'DROP TABLE T_DBDLP_TEST_OPENDLP_WIDE CASCADE CONSTRAINTS';
EXCEPTION WHEN OTHERS THEN NULL;
END;
/

-- ============================================================================
-- 1. 长表：逐条正负样本断言（按类轮转交错排列，前28行即覆盖全部14类正负样本）
-- ============================================================================
CREATE TABLE T_DBDLP_TEST_OPENDLP_PAIRS (
  PAIR_ID      NUMBER        PRIMARY KEY,
  SAMPLE_TYPE  VARCHAR2(32),
  IS_POSITIVE  NUMBER(1),
  SAMPLE_VALUE VARCHAR2(128)
);

COMMENT ON TABLE  T_DBDLP_TEST_OPENDLP_PAIRS IS 'openDLP来源-逐条样本断言表';
COMMENT ON COLUMN T_DBDLP_TEST_OPENDLP_PAIRS.SAMPLE_TYPE  IS '样本类型编号';
COMMENT ON COLUMN T_DBDLP_TEST_OPENDLP_PAIRS.IS_POSITIVE  IS '是否正样本';
COMMENT ON COLUMN T_DBDLP_TEST_OPENDLP_PAIRS.SAMPLE_VALUE IS '样本值';

-- ============================================================================
-- 2. 宽表：字段级扫描基准（17 列全类型综合数据）
-- ============================================================================
CREATE TABLE T_DBDLP_TEST_OPENDLP_WIDE (
  WIDE_ID NUMBER PRIMARY KEY,
""" + wide_col_defs + """
);

COMMENT ON TABLE T_DBDLP_TEST_OPENDLP_WIDE IS 'openDLP来源-字段级扫描基准表';
"""


def gen_wide_comments():
    return "\n".join(
        f"COMMENT ON COLUMN T_DBDLP_TEST_OPENDLP_WIDE.{WIDE_SQL_NAMES[c]} IS '{WIDE_COMMENTS[c]}';"
        for c in WIDE_COLS
    )


def main():
    ap = argparse.ArgumentParser(description="openDLP 样本转 dbdlp SQL 测试表")
    ap.add_argument("--source-dir", required=True, help="openDLP CSV 文件目录")
    ap.add_argument("--out-oracle", required=True)
    ap.add_argument("--out-dameng", required=True)
    ap.add_argument("--pairs-per-type", type=int, default=500)
    ap.add_argument("--wide-rows", type=int, default=1000)
    ap.add_argument("--batch", type=int, default=50)
    args = ap.parse_args()

    # 长表数据：每类取前 N 对，按类轮转交错（pair_k: 类1正,类1负,类2正,类2负,...）
    all_pairs = {}
    for od_type, _, _ in TYPE_MAP:
        path = os.path.join(args.source_dir, od_type + ".csv")
        all_pairs[od_type] = read_pairs(path, args.pairs_per_type)
        if len(all_pairs[od_type]) < args.pairs_per_type:
            raise SystemExit(f"样本不足: {path} 仅 {len(all_pairs[od_type])} 对")

    pair_rows = []
    pair_id = 0
    for k in range(args.pairs_per_type):
        for od_type, _, _ in TYPE_MAP:
            pos, neg = all_pairs[od_type][k]
            for flag, val in ((1, pos), (0, neg)):
                pair_id += 1
                pair_rows.append((pair_id, od_type, flag, val))

    # 宽表数据
    wide = read_wide(os.path.join(args.source_dir, "dataset-test.csv"), args.wide_rows)
    wide_rows = [
        tuple([idx + 1] + [r.get(c, "") for c in WIDE_COLS])
        for idx, r in enumerate(wide)
    ]

    # 组装 SQL
    header_o = gen_header("dbdlp openDLP 来源测试集（Oracle 版）", args.source_dir,
                          args.pairs_per_type, args.wide_rows)
    header_d = header_o.replace("（Oracle 版）", "（达梦版）")
    parts = [gen_ddl(), gen_wide_comments(), "\n\n-- ============================================================================\n-- 3. 长表数据装载\n-- ============================================================================\n"]
    parts += batched_inserts("T_DBDLP_TEST_OPENDLP_PAIRS",
                             ["PAIR_ID", "SAMPLE_TYPE", "IS_POSITIVE", "SAMPLE_VALUE"],
                             pair_rows, args.batch)
    parts.append("\n-- ============================================================================\n-- 4. 宽表数据装载\n-- ============================================================================\n")
    parts += batched_inserts("T_DBDLP_TEST_OPENDLP_WIDE",
                             ["WIDE_ID"] + [WIDE_SQL_NAMES[c] for c in WIDE_COLS],
                             wide_rows, args.batch)
    parts.append("""
COMMIT;

-- ============================================================================
-- 5. 装载校验（按类型×正负统计，应与转换参数一致）
-- ============================================================================
SELECT SAMPLE_TYPE,
       SUM(CASE WHEN IS_POSITIVE = 1 THEN 1 ELSE 0 END) AS POSITIVE_CNT,
       SUM(CASE WHEN IS_POSITIVE = 0 THEN 1 ELSE 0 END) AS NEGATIVE_CNT
FROM T_DBDLP_TEST_OPENDLP_PAIRS
GROUP BY SAMPLE_TYPE
ORDER BY SAMPLE_TYPE;

SELECT COUNT(*) AS WIDE_ROWS FROM T_DBDLP_TEST_OPENDLP_WIDE;

EXIT;
""")

    # 达梦 disql 读脚本文件固定按 GB18030 解码后再转服务端字符集发送，
    # 故达梦版脚本必须以 GB18030 编码落盘，否则中文值会双重编码乱码。
    for header, out_path, enc in ((header_o, args.out_oracle, "utf-8"),
                                  (header_d, args.out_dameng, "gb18030")):
        os.makedirs(os.path.dirname(os.path.abspath(out_path)), exist_ok=True)
        with io.open(out_path, "w", encoding=enc, newline="\n") as f:
            f.write(header)
            for p in parts:
                f.write(p)
        print("已生成:", out_path)

    print(f"长表 {len(pair_rows)} 行, 宽表 {len(wide_rows)} 行, 批量语句 {args.batch} 行/条")


if __name__ == "__main__":
    main()
