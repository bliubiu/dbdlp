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

func (r *htmlReporter) Export(_ context.Context, bundle port.ReportBundle, outDir string) (string, error) {
	path, err := outPath(outDir, bundle.Task.ID, "html")
	if err != nil {
		return "", err
	}
	riskCount := map[string]int{}
	for _, f := range bundle.Findings {
		riskCount[f.RiskLevel]++
	}
	var b strings.Builder
	b.WriteString("<!doctype html><html lang=\"zh-CN\"><head><meta charset=\"utf-8\">")
	b.WriteString("<title>dbdlp 敏感数据发现报告</title>")
	b.WriteString("<style>body{font-family:-apple-system,Segoe UI,Roboto,'Microsoft YaHei',sans-serif;margin:24px;color:" + colText + "}")
	b.WriteString("table{border-collapse:collapse;width:100%;font-size:13px}th,td{border:1px solid " + colBorder + ";padding:6px 8px;text-align:left}")
	b.WriteString("th{background:" + colHeadBG + "}.high{color:" + colHighRisk + "}.pill{display:inline-block;padding:1px 6px;border-radius:8px;background:" + colPillBG + ";margin:1px}")
	b.WriteString("h1{font-size:20px}h2{font-size:16px;margin-top:24px}.meta{color:" + colMuted + ";font-size:12px}</style></head><body>")
	b.WriteString("<h1>数据库敏感数据发现报告</h1>")
	b.WriteString(fmt.Sprintf("<div class=\"meta\">任务：%s ｜ 数据源：%s ｜ 生成时间：%s</div>",
		bundle.Task.ID, bundle.Task.DataSourceID, time.Now().Format("2006-01-02 15:04:05")))
	b.WriteString("<h2>风险分布</h2><p>")
	for _, lvl := range []string{"high", "medium_high", "medium", "medium_low", "low", "pending_review"} {
		if c, ok := riskCount[lvl]; ok {
			b.WriteString(fmt.Sprintf("<span class=\"pill\">%s：%d</span> ", lvl, c))
		}
	}
	b.WriteString(fmt.Sprintf("｜ 合计命中字段：%d</p>", len(bundle.Findings)))
	b.WriteString("<h2>敏感字段台账</h2>")
	b.WriteString("<table><thead><tr><th>库</th><th>表</th><th>字段</th><th>类型</th><th>敏感类型</th>")
	b.WriteString("<th>命中/采样</th><th>命中率</th><th>验证</th><th>置信度</th><th>风险</th><th>脱敏样例</th></tr></thead><tbody>")
	for _, f := range bundle.Findings {
		riskClass := ""
		if f.RiskLevel == "high" || f.RiskLevel == "medium_high" {
			riskClass = " class=\"high\""
		}
		types := strings.Join(f.SensitiveTypes, "、")
		b.WriteString(fmt.Sprintf("<tr%s><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td>",
			riskClass, esc(f.SchemaName), esc(f.TableName), esc(f.ColumnName), esc(f.DataType), esc(types)))
		b.WriteString(fmt.Sprintf("<td>%d/%d</td><td>%.2f</td><td>%v</td><td>%s</td><td>%s</td><td>%s</td></tr>",
			f.SampleHit, f.SampleTotal, f.HitRatio, f.VerifyPassed, esc(f.Confidence), esc(f.RiskLevel), esc(f.SampleMasked)))
	}
	b.WriteString("</tbody></table>")
	b.WriteString("<p class=\"meta\">说明：样例均为脱敏形态，不含完整明文；本报告仅用于内部安全合规。</p>")
	b.WriteString("</body></html>")
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		return "", fmt.Errorf("写入报告失败：%w", err)
	}
	return path, nil
}

func esc(s string) string {
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
