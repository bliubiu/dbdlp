package reporter

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"dbdlp/internal/domain/port"
	"github.com/xuri/excelize/v2"
)

func sampleBundle() port.ReportBundle {
	return port.ReportBundle{
		Task: port.ScanTask{ID: "task-1", DataSourceID: "ds-1"},
		Findings: []port.ColumnFinding{{
			TaskID: "task-1", DataSourceID: "ds-1", SchemaName: "SCOTT", TableName: "USERS",
			ColumnName: "PHONE", DataType: "VARCHAR2", SampleTotal: 10, SampleHit: 10, HitRatio: 1.0,
			SensitiveTypes: []string{"MOBILE_PHONE"}, VerifyPassed: true, Confidence: "high",
			StorageForm: "plaintext", RiskLevel: "high", SampleMasked: "138****8000",
			Hits: []port.Hit{{RuleID: "MOBILE_PHONE", SensitiveType: "MOBILE_PHONE", Confidence: "high", MaskedValue: "138****8000", SourceForm: "plaintext", VerifyPassed: true}},
		}},
	}
}

func TestReporters(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	bundle := sampleBundle()

	for _, fmtName := range []port.ReportFormat{port.FormatJSON, port.FormatHTML, port.FormatExcel} {
		r, err := New(fmtName)
		if err != nil {
			t.Fatalf("New(%s): %v", fmtName, err)
		}
		path, err := r.Export(ctx, bundle, dir)
		if err != nil {
			t.Fatalf("Export(%s): %v", fmtName, err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("读取报告 %s: %v", path, err)
		}
		if fmtName == port.FormatExcel {
			// xlsx 为压缩包，需用 excelize 读取单元格验证
			ef, err := excelize.OpenFile(path)
			if err != nil {
				t.Fatalf("打开 xlsx %s: %v", path, err)
			}
			cell, _ := ef.GetCellValue("字段台账", "C2") // PHONE 字段名
			if cell != "PHONE" {
				t.Fatalf("xlsx 字段台账 C2 期望 PHONE，实际 %q", cell)
			}
			masked, _ := ef.GetCellValue("字段台账", "M2") // 脱敏样例
			if masked != "138****8000" {
				t.Fatalf("xlsx 脱敏样例 M2 期望 138****8000，实际 %q", masked)
			}
			// 安全断言：所有单元格不得出现明文原值
			for _, sheet := range ef.GetSheetList() {
				rows, _ := ef.GetRows(sheet)
				for _, row := range rows {
					for _, c := range row {
						if strings.Contains(c, "13800008000") {
							t.Fatalf("xlsx 泄露明文原值于 %s", sheet)
						}
					}
				}
			}
		} else {
			content := string(data)
			// 安全断言：报告中不得出现明文手机号原值
			if strings.Contains(content, "13800008000") {
				t.Fatalf("报告 %s 泄露明文原值", filepath.Base(path))
			}
			if !strings.Contains(content, "138****8000") {
				t.Fatalf("报告 %s 未包含脱敏样例", filepath.Base(path))
			}
		}
		t.Logf("格式=%s 产物=%s 大小=%d", fmtName, path, len(data))
	}
}

// TestReportersInvalidUTF8 验证报告出口对脏数据的防御：即使动态字段含非法
// UTF-8 字节（模拟数据库驱动字符集协商异常引入的 GBK 残留字节），HTML/Excel
// 产物也必须保证浏览器/Excel 能正常解析（HTML 整体合法 UTF-8；xlsx 单元格内容合法 UTF-8）。
func TestReportersInvalidUTF8(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	bundle := sampleBundle()
	// 在脱敏样例与字段名中注入孤立多字节首字节（非法 UTF-8）。
	bundle.Findings[0].SampleMasked = string([]byte{0xe4, 0x41, 0x42}) + "*****5"
	bundle.Findings[0].ColumnName = string([]byte{0xe6, 0xe6})

	for _, fmtName := range []port.ReportFormat{port.FormatHTML, port.FormatExcel} {
		r, err := New(fmtName)
		if err != nil {
			t.Fatalf("New(%s): %v", fmtName, err)
		}
		path, err := r.Export(ctx, bundle, dir)
		if err != nil {
			t.Fatalf("Export(%s): %v", fmtName, err)
		}
		if fmtName == port.FormatExcel {
			// xlsx 是 ZIP 二进制容器，不可对整文件做 utf8.Valid；
			// 改为用 excelize 打开并校验「单元格内容」为合法 UTF-8（toValidUTF8 已兜底）。
			ef, err := excelize.OpenFile(path)
			if err != nil {
				t.Fatalf("打开 xlsx %s: %v", path, err)
			}
			for _, sheet := range ef.GetSheetList() {
				rows, _ := ef.GetRows(sheet)
				for _, row := range rows {
					for _, c := range row {
						if !utf8.ValidString(c) {
							t.Fatalf("xlsx 单元格含非法 UTF-8：%q", c)
						}
					}
				}
			}
		} else {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("读取 %s: %v", path, err)
			}
			if !utf8.Valid(raw) {
				head := raw
				if len(head) > 32 {
					head = head[:32]
				}
				t.Fatalf("格式=%s 产物含非法 UTF-8 字节：% x", fmtName, head)
			}
		}
	}
}
