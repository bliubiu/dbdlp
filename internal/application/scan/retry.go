package scan

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"dbdlp/internal/domain/port"
)

// 本文件实现「失败表重试」用例：按表重扫失败对象，已成功的表不重复扫描、不重复入库。

const (
	// checkpointStatePending 断点状态：表已登记但尚未扫描完成（可续扫）。
	checkpointStatePending = "pending"
	// checkpointStateFailed 断点状态：表级扫描失败，等待重试。
	checkpointStateFailed = "failed"
	// checkpointStateDone 断点状态：该表已扫描完成，不再重扫。
	checkpointStateDone = "done"
)

// 哨兵错误：供接口层区分「业务可判定失败」与「内部错误」。
var (
	// ErrTaskNotRetryable 任务当前状态不允许重试。
	ErrTaskNotRetryable = errors.New("任务状态不允许重试")
	// ErrNoFailedTable 任务没有失败表可重试。
	ErrNoFailedTable = errors.New("该任务没有失败表可重试")
	// ErrNothingToResume 任务没有未完成的表可续扫。
	ErrNothingToResume = errors.New("该任务没有未完成的表可续扫")
	// ErrTargetNotFailed 指定的重试对象不在失败清单中。
	ErrTargetNotFailed = errors.New("指定的重试对象不在该任务的失败清单中")
)

// RetryResult 重试／续扫结果。
type RetryResult struct {
	TaskID      string          // 任务 ID
	Retried     int             // 本次重扫的表数
	NewFindings int             // 本次新增的台账条数
	Remaining   int             // 重扫后仍失败的表数
	Status      port.TaskStatus // 重扫后的任务状态
}

// RetryFailed 重试任务中全部扫描失败的表。
func (s *Service) RetryFailed(ctx context.Context, taskID, reportFormat, outDir string) (RetryResult, error) {
	return s.RetryFailedTables(ctx, taskID, nil, reportFormat, outDir)
}

// RetryFailedTables 重试指定失败表；tableKeys 为空表示重试全部失败表。
//
// 语义：
//   - 仅重扫存在「failed」断点的表，已成功的表不重复扫描、不重复入库；
//   - tableKeys 与失败清单无交集时返回 ErrTargetNotFailed，避免调用方误以为已重试；
//   - 重扫成功的表断点置为 done；仍失败的表断点保持 failed 并刷新失败原因；
//   - 全部失败表重扫成功后任务状态回到 done，否则保持 partial。
func (s *Service) RetryFailedTables(ctx context.Context, taskID string, tableKeys []string, reportFormat, outDir string) (RetryResult, error) {
	res, err := s.retryFailedTablesImpl(ctx, taskID, tableKeys, reportFormat, outDir)
	return s.runRetryResult(ctx, "task.retry", taskID, res, err)
}

func (s *Service) retryFailedTablesImpl(ctx context.Context, taskID string, tableKeys []string, reportFormat, outDir string) (RetryResult, error) {
	res := RetryResult{TaskID: taskID}
	task, err := s.store.GetTask(ctx, taskID)
	if err != nil {
		return res, fmt.Errorf("读取任务失败：%w", err)
	}
	if task.Status != port.TaskStatusPartial && task.Status != port.TaskStatusFailed {
		return res, fmt.Errorf("%w：任务当前状态为 %s，只有失败或部分失败的任务可以重试", ErrTaskNotRetryable, task.Status)
	}
	keys, err := s.tableKeysByStates(ctx, taskID, []string{checkpointStateFailed})
	if err != nil {
		return res, err
	}
	if len(keys) == 0 {
		return res, ErrNoFailedTable
	}
	if len(tableKeys) > 0 {
		picked := pickTables(keys, tableKeys)
		if len(picked) == 0 {
			return res, ErrTargetNotFailed
		}
		keys = picked
	}
	return s.rescanTables(ctx, task, keys, reportFormat, outDir)
}

// rescanTables 重扫指定的若干张表并落库台账、更新断点与任务状态。
func (s *Service) rescanTables(ctx context.Context, task port.ScanTask, keys []string, reportFormat, outDir string) (RetryResult, error) {
	res := RetryResult{TaskID: task.ID, Retried: len(keys)}
	env, err := s.newScanEnv(ctx, task)
	if err != nil {
		return res, err
	}
	defer env.Close()
	cfg, rules, version := env.cfg, env.rules, env.ver
	driver, conn, det := env.driver, env.conn, env.det

	s.log(ctx, port.LevelINFO, "开始重扫表",
		port.Field{Key: "任务", Value: task.ID},
		port.Field{Key: "表数", Value: fmt.Sprintf("%d", len(keys))})

	// 按模式分组重扫，避免同一模式重复列举字段。
	bySchema := map[string][]string{}
	var schemaOrder []string
	for _, k := range keys {
		schemaName, _ := splitTableKey(k)
		if _, ok := bySchema[schemaName]; !ok {
			schemaOrder = append(schemaOrder, schemaName)
		}
		bySchema[schemaName] = append(bySchema[schemaName], k)
	}

	var findings []port.ColumnFinding
	var metaTables []port.TableMeta
	var metaCols []port.ColumnMeta
	metaSchemas := map[string]port.Schema{}
	for _, schemaName := range schemaOrder {
		cols, cerr := driver.ListColumns(ctx, conn, schemaName)
		if cerr != nil {
			for _, k := range bySchema[schemaName] {
				s.markCheckpoint(ctx, task.ID, k, checkpointStateFailed, fmt.Sprintf("列举字段失败：%v", cerr))
				s.log(ctx, port.LevelWARN, "重扫时列举字段失败",
					port.Field{Key: "任务", Value: task.ID},
					port.Field{Key: "表", Value: k},
					port.Field{Key: "原因", Value: cerr.Error()})
			}
			continue
		}
		for _, k := range bySchema[schemaName] {
			_, tableName := splitTableKey(k)
			tableCols := filterColumnsByTable(cols, tableName)
			if len(tableCols) == 0 {
				s.markCheckpoint(ctx, task.ID, k, checkpointStateFailed, "目标库中已不存在该表")
				s.log(ctx, port.LevelWARN, "重扫目标表已不存在",
					port.Field{Key: "任务", Value: task.ID},
					port.Field{Key: "表", Value: k})
				continue
			}
			tbl, scannedCols, tableFindings, reason := s.scanTable(ctx, driver, conn, det, task.ID, cfg.ID, tableCols)
			metaTables = append(metaTables, tbl)
			metaCols = append(metaCols, scannedCols...)
			metaSchemas[tbl.SchemaName] = port.Schema{Name: tbl.SchemaName}
			if reason != "" {
				// 重扫仍失败：断点保持 failed 并刷新原因，等待下一轮重试或人工介入。
				s.markCheckpoint(ctx, task.ID, k, checkpointStateFailed, reason)
				s.log(ctx, port.LevelWARN, "重扫后该表仍失败",
					port.Field{Key: "任务", Value: task.ID},
					port.Field{Key: "表", Value: k},
					port.Field{Key: "原因", Value: reason})
				continue
			}
			s.markCheckpoint(ctx, task.ID, k, checkpointStateDone, "")
			if len(tableFindings) > 0 {
				if err := s.store.SaveFindings(ctx, tableFindings); err != nil {
					return res, fmt.Errorf("保存重扫台账失败：%w", err)
				}
				findings = append(findings, tableFindings...)
			}
		}
	}
	res.NewFindings = len(findings)
	// 元数据幂等补齐：原任务若在元数据落库前中断，续扫时补齐（不会产生重复行）。
	if len(metaTables) > 0 {
		var schemas []port.Schema
		for _, sc := range metaSchemas {
			schemas = append(schemas, sc)
		}
		if err := s.store.SaveMetadata(ctx, task.ID, schemas, metaTables, metaCols); err != nil {
			s.log(ctx, port.LevelWARN, "重扫元数据补齐失败",
				port.Field{Key: "任务", Value: task.ID},
				port.Field{Key: "原因", Value: err.Error()})
		}
	}

	// 以存储中的断点为准统计剩余失败表（可能包含本轮未重扫的其他失败表）。
	remaining, err := s.tableCountByState(ctx, task.ID, checkpointStateFailed)
	if err != nil {
		return res, err
	}
	res.Remaining = remaining
	status := port.TaskStatusDone
	if remaining > 0 {
		status = port.TaskStatusPartial
	}
	res.Status = status

	summary := taskSummaryOf(task)
	summary.FailedCount = remaining
	summary.Message = fmt.Sprintf("重扫完成，新增 %d 条台账，剩余失败表 %d 张", res.NewFindings, remaining)
	if page, ferr := s.store.ListFindings(ctx, port.FindingFilter{TaskID: task.ID}); ferr == nil {
		summary.HitColumnCount = int(page.Total)
	}
	if err := s.store.UpdateTaskStatus(ctx, task.ID, status, summary); err != nil {
		return res, fmt.Errorf("更新任务状态失败：%w", err)
	}
	s.log(ctx, port.LevelINFO, "重扫完成",
		port.Field{Key: "任务", Value: task.ID},
		port.Field{Key: "状态", Value: string(status)},
		port.Field{Key: "重扫表数", Value: fmt.Sprintf("%d", res.Retried)},
		port.Field{Key: "新增台账", Value: fmt.Sprintf("%d", res.NewFindings)},
		port.Field{Key: "剩余失败表", Value: fmt.Sprintf("%d", remaining)})

	if reportFormat != "" && outDir != "" {
		page, ferr := s.store.ListFindings(ctx, port.FindingFilter{TaskID: task.ID, Page: 1, Limit: 1000})
		if ferr == nil {
			if err := s.exportReport(ctx, task.ID, cfg.ID, version, page.Items, rules, reportFormat, outDir); err != nil {
				return res, err
			}
		}
	}
	return res, nil
}

// markCheckpoint 写入断点状态；失败仅记 WARN，不阻断重试主流程。
func (s *Service) markCheckpoint(ctx context.Context, taskID, tableKey, state, reason string) {
	cp := port.ScanCheckpoint{TaskID: taskID, TableMetaID: tableKey, ColumnName: "", State: state}
	if state == checkpointStateFailed {
		cp.ErrorMessage = reason
	}
	if err := s.store.SaveCheckpoint(ctx, cp); err != nil {
		s.log(ctx, port.LevelWARN, "断点状态更新失败",
			port.Field{Key: "任务", Value: taskID},
			port.Field{Key: "表", Value: tableKey},
			port.Field{Key: "状态", Value: state},
			port.Field{Key: "原因", Value: err.Error()})
	}
}

// tableKeysByStates 读取任务处于指定断点状态的表。
func (s *Service) tableKeysByStates(ctx context.Context, taskID string, states []string) ([]string, error) {
	cps, err := s.store.ListCheckpoints(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("读取断点失败：%w", err)
	}
	want := map[string]bool{}
	for _, st := range states {
		want[st] = true
	}
	var keys []string
	seen := map[string]bool{}
	for _, cp := range cps {
		if !want[cp.State] || cp.TableMetaID == "" || seen[cp.TableMetaID] {
			continue
		}
		seen[cp.TableMetaID] = true
		keys = append(keys, cp.TableMetaID)
	}
	return keys, nil
}

// tableCountByState 统计任务处于指定断点状态的表数。
func (s *Service) tableCountByState(ctx context.Context, taskID, state string) (int, error) {
	keys, err := s.tableKeysByStates(ctx, taskID, []string{state})
	if err != nil {
		return 0, err
	}
	return len(keys), nil
}

// pickTables 在失败清单中挑选指定表（保持失败清单的顺序）。
func pickTables(failedKeys, targets []string) []string {
	want := map[string]bool{}
	for _, t := range targets {
		t = strings.TrimSpace(t)
		if t != "" {
			want[t] = true
		}
	}
	var picked []string
	for _, k := range failedKeys {
		if want[k] {
			picked = append(picked, k)
		}
	}
	return picked
}

// taskSummaryOf 解析任务落库的汇总；解析失败返回空汇总（不阻断重扫）。
func taskSummaryOf(task port.ScanTask) port.TaskSummary {
	var s port.TaskSummary
	if task.Summary == "" {
		return s
	}
	if err := json.Unmarshal([]byte(task.Summary), &s); err != nil {
		return port.TaskSummary{}
	}
	return s
}
