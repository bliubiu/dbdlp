package httpapi

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
	"gopkg.in/yaml.v3"
)

// 本文件实现导入模板生成与 Excel／YAML 文件解析。

// buildExcelTemplate 生成 Excel 表头模板（含一行示例，凭据列留空）。
func buildExcelTemplate() ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()
	const sheet = "数据源"
	idx, err := f.NewSheet(sheet)
	if err != nil {
		return nil, err
	}
	f.SetActiveSheet(idx)
	f.DeleteSheet("Sheet1")
	for i, col := range templateColumns {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue(sheet, cell, col); err != nil {
			return nil, err
		}
	}
	example := []any{"ORA-PROD-01", "oracle", "db01.intra", 1521, "ORCLPDB1", "ro_scan", "", "", "HR;SALES", "", "", "生产库只读扫描"}
	for i, v := range example {
		cell, _ := excelize.CoordinatesToCellName(i+1, 2)
		if err := f.SetCellValue(sheet, cell, v); err != nil {
			return nil, err
		}
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// yamlTemplateContent 生成 YAML 模板文本。
func yamlTemplateContent() string {
	var sb strings.Builder
	sb.WriteString("# dbdlp 数据源批量导入模板\n")
	sb.WriteString("# 凭据列（password）留空即可，导入时按填写值加密落库；导出的模板与错误清单均不含凭据值\n")
	sb.WriteString("datasources:\n")
	sb.WriteString("  - 实例标识: ORA-PROD-01\n")
	sb.WriteString("    数据库类型: oracle\n")
	sb.WriteString("    主机: db01.intra\n")
	sb.WriteString("    端口: 1521\n")
	sb.WriteString("    服务名/库名: ORCLPDB1\n")
	sb.WriteString("    只读账号: ro_scan\n")
	sb.WriteString("    凭据: \"\"\n")
	sb.WriteString("    完整连接串: \"\"\n")
	sb.WriteString("    纳入模式: HR;SALES\n")
	sb.WriteString("    排除表: \"\"\n")
	sb.WriteString("    排除字段: \"\"\n")
	sb.WriteString("    备注: 生产库只读扫描\n")
	return sb.String()
}

// parseExcelRows 解析 Excel 行（首行为表头）。
func parseExcelRows(file io.Reader) ([]importRow, error) {
	f, err := excelize.OpenReader(file)
	if err != nil {
		return nil, errInvalid("Excel 文件无法解析，请确认文件未损坏")
	}
	defer f.Close()
	sheet := f.GetSheetName(0)
	if sheet == "" {
		return nil, errInvalid("Excel 文件缺少工作表")
	}
	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, errInvalid("Excel 文件读取失败")
	}
	if len(rows) == 0 {
		return nil, errInvalid("Excel 文件为空")
	}
	header := map[string]int{}
	for i, h := range rows[0] {
		key := normalizeHeader(h)
		if canon, ok := columnAliases[key]; ok {
			header[canon] = i
		}
	}
	if _, ok := header["实例标识"]; !ok {
		return nil, errInvalid("Excel 表头缺少「实例标识」列，请使用模板文件")
	}
	var out []importRow
	for i := 1; i < len(rows); i++ {
		cells := rows[i]
		if isEmptyRow(cells) {
			continue
		}
		out = append(out, buildImportRow(i+1, func(name string) string {
			idx, ok := header[name]
			if !ok || idx >= len(cells) {
				return ""
			}
			return strings.TrimSpace(cells[idx])
		}))
	}
	return out, nil
}

// parseYAMLRows 解析 YAML 导入文件。
func parseYAMLRows(file io.Reader) ([]importRow, error) {
	var doc struct {
		DataSources []map[string]any `yaml:"datasources"`
	}
	dec := yaml.NewDecoder(file)
	if err := dec.Decode(&doc); err != nil {
		return nil, errInvalid("YAML 文件语法错误，请确认缩进与结构正确")
	}
	if len(doc.DataSources) == 0 {
		return nil, errInvalid("YAML 文件缺少 datasources 列表")
	}
	var out []importRow
	for i, item := range doc.DataSources {
		get := func(name string) string {
			for k, v := range item {
				if canon, ok := columnAliases[normalizeHeader(k)]; ok && canon == name {
					return fmt.Sprintf("%v", v)
				}
			}
			return ""
		}
		out = append(out, buildImportRow(i+1, get))
	}
	return out, nil
}

// buildImportRow 按列映射构造导入行。
func buildImportRow(rowNo int, get func(string) string) importRow {
	row := importRow{
		Row:            rowNo,
		Name:           get("实例标识"),
		Dialect:        get("数据库类型"),
		Host:           get("主机"),
		ServiceName:    get("服务名/库名"),
		Username:       get("只读账号"),
		Password:       get("凭据"),
		DSN:            get("完整连接串"),
		IncludeSchemas: get("纳入模式"),
		ExcludeTables:  get("排除表"),
		ExcludeColumns: get("排除字段"),
	}
	if p := get("端口"); p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			row.Port = n
		}
	}
	return row
}

// normalizeHeader 规整表头文本，便于别名匹配。
func normalizeHeader(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "_", "")
	return s
}

// isEmptyRow 判断整行是否为空。
func isEmptyRow(cells []string) bool {
	for _, c := range cells {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}
