// Package scan 扫描用例：元数据→采样→识别→聚合→台账→报告 的端到端编排。
package scan

import (
	"context"
	"fmt"
	"strings"

	"dbdlp/internal/application"
	"dbdlp/internal/domain/aggregator"
	"dbdlp/internal/domain/port"
)

// 本文件实现按「模式→表」的扫描编排：
// 单表失败不阻断整体，失败表留存断点（failed），未完成表留存断点（pending），两者均可重试／续扫。

// scanOutcome 一次扫描的产出。
type scanOutcome struct {
	Tables   []port.TableMeta      // 表元数据（含失败表，保证 column_meta 可关联）
	Columns  []port.ColumnMeta     // 字段元数据
	Findings []port.ColumnFinding  // 字段台账（仅含整表扫描成功的表）
	Failed   []port.ScanCheckpoint // 失败表断点
	Scanned  int                   // 整表扫描成功的表数
}

// scanTables 逐模式逐表扫描。
//
// doneSet 非空时跳过其中已完成的表（续扫场景）：已完成的表其台账已按表增量落库，
// 不得重复扫描、重复入库。
//
// 断点写入策略：扫描前落 pending（进程中断后可续扫识别出未完成的表），
// 扫描成功后置 done，失败则置 failed 并记录原因（可单独重试）。
//
// 台账写入策略：按表增量落库。原因是台账一旦在任务末尾统一提交，
// 进程中途退出时已完成表的台账会一并丢失；按表提交可保证「已提交即已落库」，
// 续扫时不会重复入库。
func (s *Service) scanTables(ctx context.Context, driver port.Driver, conn port.ReadOnlyConn,
	det port.Detector, taskID, dataSourceID string, kept []port.Schema, scope port.ScanScope, doneSet map[string]bool) scanOutcome {
	var out scanOutcome
	for _, sc := range kept {
		cols, err := driver.ListColumns(ctx, conn, sc.Name)
		if err != nil {
			// 模式级失败：无法列举字段，跳过该模式（不阻断其余模式）。
			s.log(ctx, port.LevelWARN, "列举字段失败，已跳过该模式",
				port.Field{Key: "任务", Value: taskID},
				port.Field{Key: "模式", Value: sc.Name},
				port.Field{Key: "原因", Value: err.Error()})
			continue
		}
		byTable, order := groupColumns(cols, scope)
		var schemaTables []port.TableMeta
		var schemaCols []port.ColumnMeta
		for _, tkey := range order {
			if _, done := doneSet[tkey]; done {
				// 已完成的表不重扫，仅补齐元数据（已存在的元数据行不会被覆盖）。
				schemaName, tableName := splitTableKey(tkey)
				schemaTables = append(schemaTables, port.TableMeta{SchemaName: schemaName, TableName: tableName})
				schemaCols = append(schemaCols, byTable[tkey]...)
				out.Tables = append(out.Tables, port.TableMeta{SchemaName: schemaName, TableName: tableName})
				out.Columns = append(out.Columns, byTable[tkey]...)
				out.Scanned++
				continue
			}
			// 先落 pending 断点：进程中断时该表可被续扫识别出来。
			s.markCheckpoint(ctx, taskID, tkey, checkpointStatePending, "")
			tbl, tableCols, findings, reason := s.scanTable(ctx, driver, conn, det, taskID, dataSourceID, byTable[tkey])
			out.Tables = append(out.Tables, tbl)
			out.Columns = append(out.Columns, tableCols...)
			schemaTables = append(schemaTables, tbl)
			schemaCols = append(schemaCols, tableCols...)
			if reason != "" {
				out.Failed = append(out.Failed, port.ScanCheckpoint{
					TaskID: taskID, TableMetaID: tkey, ColumnName: "",
					State: checkpointStateFailed, ErrorMessage: reason,
				})
				s.markCheckpoint(ctx, taskID, tkey, checkpointStateFailed, reason)
				s.log(ctx, port.LevelWARN, "表扫描失败，已跳过并留存断点",
					port.Field{Key: "任务", Value: taskID},
					port.Field{Key: "表", Value: tkey},
					port.Field{Key: "原因", Value: reason})
				continue
			}
			s.markCheckpoint(ctx, taskID, tkey, checkpointStateDone, "")
			out.Scanned++
			if len(findings) > 0 {
				if err := s.store.SaveFindings(ctx, findings); err != nil {
					s.log(ctx, port.LevelERROR, "台账写入失败",
						port.Field{Key: "任务", Value: taskID},
						port.Field{Key: "表", Value: tkey},
						port.Field{Key: "原因", Value: err.Error()})
				}
			}
			out.Findings = append(out.Findings, findings...)
		}
		// 模式级元数据落库（幂等）：进程中断后续扫可据此补齐缺失的元数据。
		if len(schemaTables) > 0 {
			if err := s.store.SaveMetadata(ctx, taskID, []port.Schema{sc}, schemaTables, schemaCols); err != nil {
				s.log(ctx, port.LevelWARN, "元数据保存失败",
					port.Field{Key: "任务", Value: taskID},
					port.Field{Key: "模式", Value: sc.Name},
					port.Field{Key: "原因", Value: err.Error()})
			}
		}
	}
	return out
}

// scanTable 扫描单表：返回表元数据、字段元数据、台账条目与失败原因（reason 为空表示成功）。
//
// 失败时丢弃该表已采集到的部分结果——台账只收录整表扫描成功的字段，
// 避免重试／续扫后与既有台账产生重复行（docs/07 EP-18「台账无重复行」）。
func (s *Service) scanTable(ctx context.Context, driver port.Driver, conn port.ReadOnlyConn,
	det port.Detector, taskID, dataSourceID string, cols []port.ColumnMeta) (port.TableMeta, []port.ColumnMeta, []port.ColumnFinding, string) {
	schemaName, tableName := cols[0].SchemaName, cols[0].TableName
	tbl := port.TableMeta{SchemaName: schemaName, TableName: tableName}
	stats, serr := driver.TableStats(ctx, conn, schemaName, tableName)
	if serr != nil {
		return tbl, cols, nil, fmt.Sprintf("统计表失败：%v", serr)
	}
	tbl.EstRows = stats.EstRows
	tbl.PrimaryKey = stats.PrimaryKey
	tbl.IsLarge = stats.IsLarge
	tbl.IsHuge = stats.IsHuge
	var findings []port.ColumnFinding
	for _, c := range cols {
		total, hits, derr := s.sampleAndDetect(ctx, driver, conn, c, stats.PrimaryKey, det)
		if derr != nil {
			return tbl, cols, nil, fmt.Sprintf("采样识别失败（%s.%s）：%v", c.TableName, c.ColumnName, derr)
		}
		if total == 0 || len(hits) == 0 {
			continue
		}
		f, aerr := aggregator.New().Aggregate(ctx, c, total, hits)
		if aerr != nil {
			return tbl, cols, nil, fmt.Sprintf("聚合失败：%v", aerr)
		}
		f.TaskID = taskID
		f.DataSourceID = dataSourceID
		findings = append(findings, f)
		s.log(ctx, port.LevelDEBUG, "字段检出敏感数据",
			port.Field{Key: "任务", Value: taskID},
			port.Field{Key: "模式", Value: c.SchemaName},
			port.Field{Key: "表", Value: c.TableName},
			port.Field{Key: "字段", Value: c.ColumnName},
			port.Field{Key: "敏感类型", Value: strings.Join(f.SensitiveTypes, ",")},
			port.Field{Key: "风险等级", Value: f.RiskLevel},
			port.Field{Key: "命中率", Value: fmt.Sprintf("%.2f", f.HitRatio)})
	}
	return tbl, cols, findings, ""
}

// filterColumnsByTable 取出属于指定表的字段。
func filterColumnsByTable(cols []port.ColumnMeta, tableName string) []port.ColumnMeta {
	var out []port.ColumnMeta
	for _, c := range cols {
		if c.TableName == tableName {
			out = append(out, c)
		}
	}
	return out
}

// groupColumns 按表分组字段，返回表键顺序与分组结果（已剔除排除项）。
func groupColumns(cols []port.ColumnMeta, scope port.ScanScope) (map[string][]port.ColumnMeta, []string) {
	byTable := map[string][]port.ColumnMeta{}
	var order []string
	for _, c := range cols {
		if application.ContainsToken(scope.ExcludeTables, c.TableName) {
			continue
		}
		if application.ContainsToken(scope.ExcludeColumns, c.ColumnName) {
			continue
		}
		tkey := c.SchemaName + "/" + c.TableName
		if _, ok := byTable[tkey]; !ok {
			order = append(order, tkey)
		}
		byTable[tkey] = append(byTable[tkey], c)
	}
	return byTable, order
}

// filterSchemas 过滤系统模式与不在扫描范围内的模式。
func filterSchemas(schemas []port.Schema, scope port.ScanScope) []port.Schema {
	var kept []port.Schema
	for _, sc := range schemas {
		if sc.IsSystem {
			continue
		}
		if scope.IncludeSchemas != "" && !application.ContainsToken(scope.IncludeSchemas, sc.Name) {
			continue
		}
		kept = append(kept, sc)
	}
	return kept
}
