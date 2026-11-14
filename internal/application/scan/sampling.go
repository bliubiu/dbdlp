// Package scan 扫描用例：元数据→采样→识别→聚合→台账→报告 的端到端编排。
package scan

import (
	"context"
	"strings"

	"dbdlp/internal/domain/port"
)

// 本文件实现按字段采样与识别调用。

func (s *Service) sampleAndDetect(ctx context.Context, driver port.Driver, conn port.ReadOnlyConn, col port.ColumnMeta, pk string, det port.Detector) (int, []port.Hit, error) {
	var total int
	var hits []port.Hit
	lastKey := ""
	for total < s.sampleRows {
		remaining := s.sampleRows - total
		batch, err := driver.SampleRows(ctx, conn, port.SampleQuery{
			Schema: col.SchemaName, Table: col.TableName, Columns: []string{col.ColumnName},
			PrimaryKey: pk, LastKey: lastKey, BatchSize: minInt(remaining, 200), MaxTextLen: s.maxTextLen,
		})
		if err != nil {
			return total, hits, err
		}
		if len(batch.Values) == 0 {
			break
		}
		for _, row := range batch.Values {
			if len(row) == 0 {
				continue
			}
			v := row[0]
			total++
			if strings.TrimSpace(v) == "" {
				continue
			}
			cellHits, derr := det.DetectValue(ctx, v)
			if derr != nil {
				continue
			}
			hits = append(hits, cellHits...)
		}
		if !batch.HasMore {
			break
		}
		lastKey = batch.LastKey
		// 批次间歇限速（docs/02 §8.6 分批限流）
		if err := s.paceBatch(ctx); err != nil {
			return total, hits, err
		}
	}
	return total, hits, nil
}

// exportReport 按逗号分隔的格式列表逐个导出报告（excel,html,json）。
// 任一格式失败即返回错误并记 ERROR 日志——不允许静默吞掉（否则扫描「成功」但无产物）。

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
