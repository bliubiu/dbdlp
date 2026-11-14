// Package scan 扫描用例：元数据→采样→识别→聚合→台账→报告 的端到端编排。
package scan

import (
	"context"
	"fmt"
	"strings"
	"time"

	"dbdlp/internal/domain/port"
)

// 本文件实现扫描完成后的多格式报告导出。

func (s *Service) exportReport(ctx context.Context, taskID, dataSourceID, version string, findings []port.ColumnFinding, rules []port.Rule, format, outDir string) error {
	formats := splitReportFormats(format)
	if len(formats) == 0 {
		return nil
	}
	for _, f := range formats {
		r, err := s.reporterFactory(port.ReportFormat(f))
		if err != nil {
			s.log(ctx, port.LevelERROR, "报告器构建失败",
				port.Field{Key: "任务", Value: taskID},
				port.Field{Key: "格式", Value: f},
				port.Field{Key: "原因", Value: err.Error()})
			return fmt.Errorf("构建 %s 报告器失败：%w", f, err)
		}
		bundle := port.ReportBundle{
			Task:     port.ScanTask{ID: taskID, DataSourceID: dataSourceID, Status: port.TaskStatusDone, RuleVersion: version},
			Findings: findings,
			Rules:    rules,
		}
		path, err := r.Export(ctx, bundle, outDir)
		if err != nil {
			s.log(ctx, port.LevelERROR, "报告导出失败",
				port.Field{Key: "任务", Value: taskID},
				port.Field{Key: "格式", Value: f},
				port.Field{Key: "原因", Value: err.Error()})
			return fmt.Errorf("导出 %s 报告失败：%w", f, err)
		}
		if err := s.store.SaveReportArtifact(ctx, port.ReportArtifact{
			TaskID: taskID, Format: f, FilePath: path, GeneratedBy: "scan", CreatedAt: nowText(), UpdatedAt: nowText(),
		}); err != nil {
			// 产物已落盘，仅登记失败：记 WARN 不影响交付。
			s.log(ctx, port.LevelWARN, "报告产物登记失败",
				port.Field{Key: "任务", Value: taskID},
				port.Field{Key: "文件", Value: path},
				port.Field{Key: "原因", Value: err.Error()})
		}
		s.log(ctx, port.LevelINFO, "报告导出完成",
			port.Field{Key: "任务", Value: taskID},
			port.Field{Key: "格式", Value: f},
			port.Field{Key: "文件", Value: path})
	}
	return nil
}

// splitReportFormats 解析逗号分隔的报告格式列表（大小写不敏感，忽略空项）。

func splitReportFormats(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		p := strings.ToLower(strings.TrimSpace(part))
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func nowText() string { return time.Now().Format("2006-01-02 15:04:05.000") }
