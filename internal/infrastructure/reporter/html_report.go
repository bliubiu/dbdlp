// html_report.go HTML 审计报告八章节渲染（docs/02 §14.1、docs/04 §4 设计原型对齐）。
package reporter

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"dbdlp/internal/domain/port"
)

// 风险四级色彩令牌（docs/04 §4.4：高红/中橙/低绿/待复核灰蓝；色值收敛为命名常量）。
const (
	htmlRiskHighFg   = "#B42318"
	htmlRiskHighBg   = "#FEF3F2"
	htmlRiskHighBd   = "#FECDCA"
	htmlRiskHighFill = "#D92D20"
	htmlRiskMedFg    = "#B54708"
	htmlRiskMedBg    = "#FFFAEB"
	htmlRiskMedBd    = "#FEDF89"
	htmlRiskMedFill  = "#FDB022"
	htmlRiskLowFg    = "#067647"
	htmlRiskLowBg    = "#ECFDF3"
	htmlRiskLowBd    = "#ABEFC6"
	htmlRiskLowFill  = "#12B76A"
	htmlRiskPendFg   = "#475467"
	htmlRiskPendBg   = "#F2F4F7"
	htmlRiskPendBd   = "#D0D5DD"
	htmlRiskPendFill = "#98A2B3"
	htmlSampleBg     = "#F7F9FC"
	htmlSampleBd     = "#EAECF0"
	htmlAccent       = "#1D4ED8"
	htmlChart2       = "#0E7490"
	htmlFg           = "#1D2939"
	htmlFg2          = "#475467"
	htmlMuted        = "#667085"
	htmlBorder       = "#EAECF0"
)

// uiRisk4 后端六值收敛四级展示（docs/04 §4.4）。
func uiRisk4(level string) string {
	switch level {
	case "high":
		return "高"
	case "medium_high", "medium", "medium_low":
		return "中"
	case "low":
		return "低"
	case "pending_review":
		return "待复核"
	}
	return "低"
}

// htmlRiskIcon 风险徽章图标 path（CircleExclamationPoint/CircleDot/CircleCheck/CircleQuestionMark）。
func htmlRiskIcon(level string) string {
	switch level {
	case "高":
		return "M12 2a10 10 0 1 0 0 20 10 10 0 0 0 0-20zm-1 5h2v7h-2V7zm0 9h2v2h-2v-2z"
	case "中":
		return "M12 2a10 10 0 1 0 0 20 10 10 0 0 0 0-20zm0 4a6 6 0 1 1 0 12 6 6 0 0 1 0-12z"
	case "低":
		return "M12 2a10 10 0 1 0 0 20 10 10 0 0 0 0-20zm-1 13.6-3.3-3.3 1.4-1.4L11 12.8l4.9-4.9 1.4 1.4L11 15.6z"
	}
	return "M12 2a10 10 0 1 0 0 20 10 10 0 0 0 0-20zm0 15a1.3 1.3 0 1 1 0 2.6 1.3 1.3 0 0 1 0-2.6zM11 6h2v8h-2V6z"
}

// htmlRiskBadge 风险徽章三重编码：色块 + 图标 + 中文文字（docs/04 §4.4）。
func htmlRiskBadge(level string) string {
	var fg, bg, bd string
	switch level {
	case "高":
		fg, bg, bd = htmlRiskHighFg, htmlRiskHighBg, htmlRiskHighBd
	case "中":
		fg, bg, bd = htmlRiskMedFg, htmlRiskMedBg, htmlRiskMedBd
	case "低":
		fg, bg, bd = htmlRiskLowFg, htmlRiskLowBg, htmlRiskLowBd
	default:
		fg, bg, bd = htmlRiskPendFg, htmlRiskPendBg, htmlRiskPendBd
	}
	return fmt.Sprintf(`<span style="display:inline-flex;align-items:center;gap:4px;border-radius:999px;padding:1px 8px;font-size:12px;font-weight:600;color:%s;background:%s;border:1px solid %s">`+
		`<svg viewBox="0 0 24 24" width="11" height="11" fill="currentColor"><path d="%s"/></svg>%s</span>`,
		fg, bg, bd, htmlRiskIcon(level), esc(level))
}

// htmlDonut C1 风险等级分布环形图（SVG，固定顺序 高→中→低→待复核）。
func htmlDonut(counts map[string]int) string {
	order := []string{"高", "中", "低", "待复核"}
	fills := map[string]string{"高": htmlRiskHighFill, "中": htmlRiskMedFill, "低": htmlRiskLowFill, "待复核": htmlRiskPendFill}
	total := 0
	for _, k := range order {
		total += counts[k]
	}
	if total == 0 {
		total = 1
	}
	var segs strings.Builder
	off := 25.0
	const c = 2 * 3.14159 * 40
	for _, k := range order {
		v := counts[k]
		if v == 0 {
			continue
		}
		segs.WriteString(fmt.Sprintf(`<circle r="40" cx="60" cy="60" fill="none" stroke="%s" stroke-width="18" stroke-dasharray="%.1f %.1f" stroke-dashoffset="%.1f" transform="rotate(-90 60 60)"/>`,
			fills[k], float64(v)/float64(total)*c, c, -off))
		off += float64(v) / float64(total) * c
	}
	return fmt.Sprintf(`<svg viewBox="0 0 120 120" width="170" height="170">%s<text x="60" y="57" text-anchor="middle" font-size="20" font-weight="700" fill="#344054">%d</text><text x="60" y="72" text-anchor="middle" font-size="8" fill="#667085">敏感字段总数</text></svg>`,
		segs.String(), total)
}

// htmlHbar 水平条形图（C2 识别率 / C3 类型 TOP）。
func htmlHbar(items [][2]interface{}, color string) string {
	max := 0
	for _, it := range items {
		if v, ok := it[1].(int); ok && v > max {
			max = v
		}
	}
	if max == 0 {
		max = 1
	}
	var b strings.Builder
	for _, it := range items {
		k, _ := it[0].(string)
		v, _ := it[1].(int)
		b.WriteString(fmt.Sprintf(`<div style="display:grid;grid-template-columns:130px 1fr 36px;gap:8px;align-items:center;margin:6px 0"><span title="%s" style="font-size:12px;color:%s;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">%s</span><div style="background:#F7F9FC;border-radius:4px;height:12px;overflow:hidden"><div style="height:100%%;border-radius:4px;width:%.0f%%;background:%s"></div></div><span style="font-size:12px;text-align:right;font-variant-numeric:tabular-nums">%d</span></div>`,
			esc(k), htmlFg2, esc(k), float64(v)/float64(max)*100, color, v))
	}
	return b.String()
}

// htmlTable 通用表格。
func htmlTable(headers []string, rows [][]string) string {
	var b strings.Builder
	b.WriteString("<table style=\"border-collapse:collapse;width:100%;font-size:12.5px\">")
	b.WriteString("<thead><tr>")
	for _, h := range headers {
		b.WriteString(fmt.Sprintf("<th style=\"background:#F7F9FC;text-align:left;font-weight:600;color:%s;padding:7px 9px;border:1px solid %s;white-space:nowrap\">%s</th>", htmlFg2, htmlBorder, h))
	}
	b.WriteString("</tr></thead><tbody>")
	for _, row := range rows {
		b.WriteString("<tr>")
		for _, c := range row {
			b.WriteString(fmt.Sprintf("<td style=\"padding:7px 9px;border:1px solid %s;vertical-align:top\">%s</td>", htmlBorder, c))
		}
		b.WriteString("</tr>")
	}
	b.WriteString("</tbody></table>")
	return b.String()
}

// htmlNote 口径/说明条。
func htmlNote(s string) string {
	return fmt.Sprintf(`<p style="font-size:12px;color:%s;background:#F7F9FC;border-radius:8px;padding:8px 12px">%s</p>`, htmlMuted, s)
}

// htmlSampleCell 掩码样例单元格（--sample-* 形态，仅掩码值）。
func htmlSampleCell(s string) string {
	return fmt.Sprintf(`<code title="样例已脱敏，平台不展示完整明文" style="font-family:ui-monospace,monospace;font-size:11.5px;background:%s;color:#475467;border:1px solid %s;border-radius:6px;padding:1px 6px">%s</code>`,
		htmlSampleBg, htmlSampleBd, esc(truncateStr(s, 80)))
}

func truncateStr(s string, n int) string {
	rs := []rune(s)
	if len(rs) <= n {
		return s
	}
	return string(rs[:n]) + "…"
}

// htmlSection 章节包装。
func htmlSection(no int, title, body string) string {
	return fmt.Sprintf(`<h2 style="font-size:16px;margin:26px 0 10px;padding-bottom:6px;border-bottom:1px solid %s"><span style="color:%s;font-weight:700;margin-right:8px">%d</span>%s</h2>%s`,
		htmlBorder, htmlAccent, no, esc(title), body)
}

// Export 渲染 HTML 审计报告（八章节，docs/02 §14.1）。
func (r *htmlReporter) Export(_ context.Context, bundle port.ReportBundle, outDir string) (string, error) {
	path, err := outPath(outDir, bundle.Task.ID, "html")
	if err != nil {
		return "", err
	}
	findings := bundle.Findings
	// ── 聚合（docs/04 §4.1 口径）──
	risk4 := map[string]int{}
	typeCnt := map[string]int{}
	formCnt := map[string]int{}
	for _, f := range findings {
		risk4[uiRisk4(f.RiskLevel)]++
		formCnt[storageFormCN(f.StorageForm)]++
		for _, h := range f.Hits {
			typeCnt[h.SensitiveType]++
		}
	}
	var highF, medF, pendF []port.ColumnFinding
	for _, f := range findings {
		switch uiRisk4(f.RiskLevel) {
		case "高":
			highF = append(highF, f)
		case "中":
			medF = append(medF, f)
		case "待复核":
			pendF = append(pendF, f)
		}
	}
	// C3 类型 TOP10（同字段多类型分别计数）
	top := topTypes(typeCnt, 10)

	var b strings.Builder
	b.WriteString("<!doctype html><html lang=\"zh-CN\"><head><meta charset=\"utf-8\">")
	b.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">")
	b.WriteString("<title>数据库敏感数据发现报告</title>")
	b.WriteString(htmlStyle())
	b.WriteString("</head><body><div style=\"max-width:1080px;margin:0 auto;background:#fff;padding:32px 40px\">")

	// 封面
	b.WriteString(fmt.Sprintf(`<h1 style="font-size:22px;margin:0 0 6px">数据库敏感数据发现报告</h1>`+
		`<div style="color:%s;font-size:13px;line-height:1.7">任务 <b>%s</b>｜状态 <b>%s</b>｜规则版本 <b>%s</b>｜数据源 <b>%s</b>｜生成时间 <b>%s</b><br/>`+
		`说明：样例均为脱敏形态，不含完整明文；本报告仅用于内部安全合规。</div>`,
		htmlMuted, esc(bundle.Task.ID), esc(taskStatusCN(string(bundle.Task.Status))), esc(bundle.Task.RuleVersion),
		esc(bundle.Task.DataSourceID), time.Now().Format("2006-01-02 15:04:05")))

	// ① 扫描概况（口径：台账命中范围；总列数在任务进度接口，bundle 不含）
	tableSet := map[string]bool{}
	for _, f := range findings {
		tableSet[f.TableName] = true
	}
	s1 := fmt.Sprintf(`<div style="display:grid;grid-template-columns:repeat(4,1fr);gap:12px;margin-bottom:14px">%s%s%s%s</div>%s%s`,
		htmlKPI(fmt.Sprintf("%d", len(tableSet)), "涉及表数"),
		htmlKPI(fmt.Sprintf("%d", len(findings)), "命中敏感字段"),
		htmlKPI(fmt.Sprintf("%d", len(highF)), "高风险字段"),
		htmlKPI(fmt.Sprintf("%d", len(pendF)), "待复核字段"),
		htmlHbar([][2]interface{}{{"命中敏感字段", len(findings)}, {"高风险字段", len(highF)}, {"中风险字段", len(medF)}}, htmlAccent),
		htmlNote("口径：台账命中范围统计；全部扫描字段数见任务进度接口。"))
	b.WriteString(htmlSection(1, "扫描概况", s1))

	// ② 风险汇总
	total4 := len(findings)
	if total4 == 0 {
		total4 = 1
	}
	var legend strings.Builder
	for _, k := range []string{"高", "中", "低", "待复核"} {
		legend.WriteString(fmt.Sprintf(`<div><i style="display:inline-block;width:10px;height:10px;border-radius:3px;margin-right:6px;background:%s"></i>%s %d 个（%.0f%%）</div>`,
			riskFillCN(k), k, risk4[k], float64(risk4[k])/float64(total4)*100))
	}
	s2 := fmt.Sprintf(`<div style="display:grid;grid-template-columns:320px 1fr;gap:16px"><div>%s<div style="font-size:12px;color:%s;margin-top:8px">%s</div></div><div><h3 style="font-size:14px;margin:0 0 4px">C3 敏感类型 TOP %d</h3><p style="font-size:12px;color:%s;margin:0 0 10px">按类型展开计数，同一字段命中多类型分别计数（非去重字段数）</p>%s</div></div>`,
		htmlDonut(risk4), htmlMuted, legend.String(), len(top), htmlMuted, htmlHbar(top, htmlChart2))
	b.WriteString(htmlSection(2, "风险汇总", s2))

	// ③④⑤ 清单
	head := []string{"模式", "表", "字段", "敏感类型", "命中率", "命中/样本", "掩码样例", "置信度", "风险"}
	s3 := htmlTable(head, htmlFindingRows(highF)) + htmlNote("以上字段均为明文存储的高敏感数据（身份证/手机号/银行卡/护照/统一社会信用代码），建议优先整改。")
	s4 := htmlTable(head, htmlFindingRows(medF)) + htmlNote("中风险包含：校验通过但列名上下文不吻合（需人工确认）、明文低敏感类型等；原始枚举保留于字段详情。")
	var s5 string
	if len(pendF) == 0 {
		s5 = htmlNote("本期无待复核字段。命中级明细中 verify_passed=false 的记录可在控制台台账按复核状态过滤。")
	} else {
		s5 = htmlTable(head, htmlFindingRows(pendF))
	}
	b.WriteString(htmlSection(3, fmt.Sprintf("高风险字段（%d）", len(highF)), s3))
	b.WriteString(htmlSection(4, fmt.Sprintf("中风险字段（%d）", len(medF)), s4))
	b.WriteString(htmlSection(5, fmt.Sprintf("待复核字段（%d）", len(pendF)), s5))

	// ⑥ 加密脱敏现状
	var formRows [][]string
	for k, v := range formCnt {
		formRows = append(formRows, []string{esc(k), fmt.Sprintf("%d", v)})
	}
	maskRules := [][]string{
		{"IPv4", "前两段以 * 替换，保留后两段", "*.*.10.3"},
		{"本地回环", "127.0.0.1、localhost 保留完整", "127.0.0.1 原样"},
		{"手机号/身份证/银行卡/邮箱", "按类型掩码，不保留可还原片段", "138****1234 / 1101**********1234"},
		{"密码", "所有响应恒为固定占位串", "******"},
	}
	s6 := htmlTable([]string{"类型", "规则", "示例"}, maskRules) + "<h3 style=\"font-size:14px\">存储形态分布</h3>" +
		htmlTable([]string{"存储形态", "字段数"}, formRows) +
		htmlNote("脱敏适用三处缺一不可：① SQL 输出字面量；② 运行日志（全级别）；③ 报告文件（本报告样例列仅掩码值）。")
	b.WriteString(htmlSection(6, "加密脱敏现状", s6))

	// ⑦ 整改建议
	var sug strings.Builder
	sug.WriteString("<ul style=\"margin:8px 0;padding-left:20px\">")
	if len(highF) > 0 {
		sug.WriteString(fmt.Sprintf("<li style=\"margin:6px 0;font-size:13px;line-height:1.7\"><b>优先加密/脱敏高敏感明文字段（%d 个）：</b>身份证、银行卡、手机号等明文存储字段建议按分类分级要求改造为加密存储（AES-256-GCM）或展示侧掩码，并收敛访问权限。</li>", len(highF)))
		tables := map[string]bool{}
		var names []string
		for _, f := range highF {
			if !tables[f.TableName] {
				tables[f.TableName] = true
				names = append(names, esc(f.TableName))
			}
		}
		sug.WriteString(fmt.Sprintf("<li style=\"margin:6px 0;font-size:13px;line-height:1.7\"><b>重点表治理：</b>%s 为高敏感字段集中表，建议核对业务必要性并纳入脱敏改造排期。</li>", strings.Join(names, "、")))
	}
	if len(medF) > 0 {
		sug.WriteString(fmt.Sprintf("<li style=\"margin:6px 0;font-size:13px;line-height:1.7\"><b>复核中风险字段（%d 个）：</b>其中「校验通过但列名不吻合」的字段需人工确认是否为真实敏感数据（负样本池类），确认后在控制台标记复核结论。</li>", len(medF)))
	}
	sug.WriteString("<li style=\"margin:6px 0;font-size:13px;line-height:1.7\"><b>接入复核流程：</b>在控制台台账对字段逐条复核，复核结论写入台账并同步下游治理平台。</li></ul>")
	b.WriteString(htmlSection(7, "整改建议", sug.String()))

	// ⑧ 附录
	var ruleRows [][]string
	for _, r := range bundle.Rules {
		verifier := r.Verifier
		if verifier == "" {
			verifier = "—"
		}
		state := "禁用"
		if r.Enabled {
			state = "启用"
		}
		custom := "否"
		if r.Custom {
			custom = "是"
		}
		ruleRows = append(ruleRows, []string{esc(r.RuleID), esc(r.SensitiveType), esc(verifier),
			fmt.Sprintf("%.2f", r.Threshold), state, custom})
	}
	var taskRows [][]string
	taskRows = append(taskRows, []string{esc(bundle.Task.ID), esc(taskStatusCN(string(bundle.Task.Status))),
		fmt.Sprintf("%d", len(tableSet)), fmt.Sprintf("%d", len(findings)),
		fmt.Sprintf("%d", len(findings)), esc(bundle.Task.RuleVersion)})
	s8 := htmlTable([]string{"规则ID", "敏感类型", "校验器", "阈值", "状态", "自定义"}, ruleRows) +
		"<h3 style=\"font-size:14px\">任务记录</h3>" +
		htmlTable([]string{"任务", "状态", "表数", "字段数", "命中字段", "规则版本"}, taskRows) +
		htmlNote("本报告全部样例值为脱敏形态；报告生成/下载行为已写入审计日志。JSON 明细与 Excel 六 Sheet（概览/字段台账/命中明细/复核记录/规则清单/任务记录）内容口径与本报告一致。")
	b.WriteString(htmlSection(8, "附录：规则清单与任务记录", s8))

	b.WriteString("</div></body></html>")
	// 整段再做一次 UTF-8 规范化兜底，保证产物字节级合法 UTF-8。
	if err := os.WriteFile(path, []byte(toValidUTF8(b.String())), 0o600); err != nil {
		return "", fmt.Errorf("写入报告失败：%w", err)
	}
	return path, nil
}

// storageFormCN 存储形态中文。
func storageFormCN(v string) string {
	switch v {
	case "encrypted":
		return "透明加密"
	case "hash":
		return "无盐哈希"
	case "masked":
		return "已掩码"
	}
	return "明文"
}

// taskStatusCN 任务状态中文。
func taskStatusCN(v string) string {
	switch v {
	case "done":
		return "成功"
	case "partial":
		return "部分失败"
	case "failed":
		return "失败"
	case "paused":
		return "已暂停"
	case "running":
		return "运行中"
	}
	return "待执行"
}

// riskFillCN 四级图例填充色。
func riskFillCN(k string) string {
	switch k {
	case "高":
		return htmlRiskHighFill
	case "中":
		return htmlRiskMedFill
	case "低":
		return htmlRiskLowFill
	}
	return htmlRiskPendFill
}

// htmlKPI 概况指标卡。
func htmlKPI(v, label string) string {
	return fmt.Sprintf(`<div style="border:1px solid %s;border-radius:10px;padding:12px 14px"><div style="font-size:24px;font-weight:700;font-variant-numeric:tabular-nums">%s</div><div style="font-size:12px;color:%s;margin-top:2px">%s</div></div>`,
		htmlBorder, v, htmlMuted, label)
}

// htmlFindingRows 台账清单行。
func htmlFindingRows(fs []port.ColumnFinding) [][]string {
	confCN := map[string]string{"high": "高", "medium": "中", "low": "低"}
	var rows [][]string
	for _, f := range fs {
		types := strings.Join(f.SensitiveTypes, "、")
		rows = append(rows, []string{
			esc(f.SchemaName), esc(f.TableName), esc(f.ColumnName), esc(types),
			fmt.Sprintf("%.0f%%", f.HitRatio*100),
			fmt.Sprintf("%d/%d", f.SampleHit, f.SampleTotal),
			htmlSampleCell(f.SampleMasked),
			esc(confCN[f.Confidence]),
			htmlRiskBadge(uiRisk4(f.RiskLevel)),
		})
	}
	return rows
}

// topTypes 类型计数降序前 n 项（C3）。
func topTypes(cnt map[string]int, n int) [][2]interface{} {
	var keys []string
	for k := range cnt {
		keys = append(keys, k)
	}
	// 简单选择排序：条目少（≤类型数）
	for i := 0; i < len(keys) && i < n; i++ {
		mx := i
		for j := i + 1; j < len(keys); j++ {
			if cnt[keys[j]] > cnt[keys[mx]] {
				mx = j
			}
		}
		keys[i], keys[mx] = keys[mx], keys[i]
	}
	var out [][2]interface{}
	for i, k := range keys {
		if i >= n {
			break
		}
		out = append(out, [2]interface{}{k, cnt[k]})
	}
	return out
}

// htmlStyle 报告样式（打印分页友好）。
func htmlStyle() string {
	return `<style>
body{font-family:"Inter","Noto Sans SC",-apple-system,sans-serif;margin:0;padding:24px;color:` + htmlFg + `;background:#F7F9FC}
h3{font-size:14px;margin:12px 0 6px}
h1{font-size:22px;margin:0 0 6px}
@media print{body{padding:0;background:#fff}}
</style>`
}
