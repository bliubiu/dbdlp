// Package reporter 报告导出：JSON / HTML / Excel 三种格式共享 ReportBundle。
// 安全约束：报告只含脱敏样例（SampleMasked、Hit.MaskedValue），绝不含明文原值。
package reporter

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dbdlp/internal/domain/port"
)

// New 按格式构造报告导出器。
func New(format port.ReportFormat) (port.Reporter, error) {
	switch format {
	case port.FormatJSON:
		return &jsonReporter{}, nil
	case port.FormatHTML:
		return &htmlReporter{}, nil
	case port.FormatExcel:
		return &excelReporter{}, nil
	default:
		return nil, fmt.Errorf("不支持的报告格式：%s", format)
	}
}

func outPath(outDir, taskID string, ext string) (string, error) {
	if err := os.MkdirAll(outDir, 0o700); err != nil {
		return "", fmt.Errorf("创建输出目录失败：%w", err)
	}
	name := fmt.Sprintf("dbdlp_report_%s.%s", taskID, ext)
	return filepath.Join(outDir, name), nil
}

// ===== JSON =====

type jsonReporter struct{}

func (r *jsonReporter) Format() port.ReportFormat { return port.FormatJSON }

func (r *jsonReporter) Export(_ context.Context, bundle port.ReportBundle, outDir string) (string, error) {
	path, err := outPath(outDir, bundle.Task.ID, "json")
	if err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return "", fmt.Errorf("序列化报告失败：%w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", fmt.Errorf("写入报告失败：%w", err)
	}
	return path, nil
}

// ===== HTML =====

// 报告配色：HTML 报告为离线单文件产物，不依赖控制台设计令牌；
// 此处集中定义，避免色值散落为魔法字面量，便于后续与 docs/04 设计令牌对齐。
const (
	colText     = "#222"
	colMuted    = "#666"
	colBorder   = "#ddd"
	colHeadBG   = "#f5f7fa"
	colHighRisk = "#c00"
	colPillBG   = "#eee"
)

type htmlReporter struct{}

func (r *htmlReporter) Format() port.ReportFormat { return port.FormatHTML }

// toValidUTF8 把含非法 UTF-8 字节的字符串规范为合法 UTF-8：将任何非法字节序列
// 替换为 U+FFFD。该降级行为刻意与 encoding/json 对非 ASCII 字节的处理保持一致，
// 确保任何来源（如数据库驱动字符集协商异常引入的 GBK 残留字节）的脏数据都不会
// 让 HTML/Excel 报告产物成为非法编码文件，避免浏览器预览或 Excel 解析乱码、崩溃。
func toValidUTF8(s string) string {
	return strings.ToValidUTF8(s, "\uFFFD")
}

func esc(s string) string {
	s = toValidUTF8(s)
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;")
	return r.Replace(s)
}

// ===== Excel =====

type excelReporter struct{}

func (r *excelReporter) Format() port.ReportFormat { return port.FormatExcel }

func (r *excelReporter) Export(_ context.Context, bundle port.ReportBundle, outDir string) (string, error) {
	path, err := outPath(outDir, bundle.Task.ID, "xlsx")
	if err != nil {
		return "", err
	}
	f := newExcel()
	// 概览 sheet
	f.setCell("概览", "A1", "数据库敏感数据发现报告")
	f.setCell("概览", "A2", "任务ID")
	f.setCell("概览", "B2", bundle.Task.ID)
	f.setCell("概览", "A3", "数据源ID")
	f.setCell("概览", "B3", bundle.Task.DataSourceID)
	f.setCell("概览", "A4", "生成时间")
	f.setCell("概览", "B4", time.Now().Format("2006-01-02 15:04:05"))
	f.setCell("概览", "A5", "命中字段数")
	f.setCell("概览", "B5", fmt.Sprintf("%d", len(bundle.Findings)))

	// 字段台账 sheet
	headers := []string{"库", "表", "字段", "类型", "敏感类型", "命中数", "采样数", "命中率", "验证通过", "置信度", "风险等级", "存储形态", "脱敏样例"}
	for i, h := range headers {
		f.setCell("字段台账", colLetter(i)+"1", h)
	}
	for ri, fd := range bundle.Findings {
		row := ri + 2
		vals := []string{
			fd.SchemaName, fd.TableName, fd.ColumnName, fd.DataType,
			strings.Join(fd.SensitiveTypes, "、"),
			fmt.Sprintf("%d", fd.SampleHit), fmt.Sprintf("%d", fd.SampleTotal),
			fmt.Sprintf("%.2f", fd.HitRatio), fmt.Sprintf("%v", fd.VerifyPassed),
			fd.Confidence, fd.RiskLevel, fd.StorageForm, fd.SampleMasked,
		}
		for ci, v := range vals {
			f.setCell("字段台账", colLetter(ci)+fmt.Sprintf("%d", row), v)
		}
	}
	if err := f.save(path); err != nil {
		return "", err
	}
	return path, nil
}
