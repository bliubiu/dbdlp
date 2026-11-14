package scan

import (
	"context"
	"fmt"

	"dbdlp/internal/domain/port"
)

// 本文件实现「断点续扫」用例：从断点继续未完成的表。

// Resume 断点续扫：重扫未完成的表。
//
// 语义：
//   - 断点为 done 的表不重扫（其台账已按表增量落库，不会重复入库）；
//   - 断点为 pending 的表表示上次扫描未完成（进程中断），本轮整表重扫；
//   - 断点为 failed 的表重扫成功即置 done，仍失败则保持 failed；
//   - 本轮会重新枚举目标库，覆盖上次扫描尚未触及的表（无断点记录的表同样会被扫到）；
//   - 全部表续扫成功后任务状态回到 done，否则保持 partial。
func (s *Service) Resume(ctx context.Context, taskID, reportFormat, outDir string) (RetryResult, error) {
	res, err := s.resumeImpl(ctx, taskID, reportFormat, outDir)
	return s.runRetryResult(ctx, "task.resume", taskID, res, err)
}

func (s *Service) resumeImpl(ctx context.Context, taskID, reportFormat, outDir string) (RetryResult, error) {
	res := RetryResult{TaskID: taskID}
	task, err := s.store.GetTask(ctx, taskID)
	if err != nil {
		return res, fmt.Errorf("读取任务失败：%w", err)
	}
	if task.Status == port.TaskStatusDone {
		return res, fmt.Errorf("%w：任务已完成，没有未完成的表可续扫", ErrTaskNotRetryable)
	}
	doneSet, err := s.doneTableKeys(ctx, taskID)
	if err != nil {
		return res, err
	}
	ds, err := s.store.GetDataSource(ctx, task.DataSourceID)
	if err != nil {
		return res, fmt.Errorf("读取数据源失败：%w", err)
	}
	scope := port.ScanScope{
		IncludeSchemas: ds.IncludeSchemas,
		ExcludeTables:  ds.ExcludeTables,
		ExcludeColumns: ds.ExcludeColumns,
	}
	env, err := s.newScanEnv(ctx, task)
	if err != nil {
		return res, err
	}
	defer env.Close()

	s.log(ctx, port.LevelINFO, "开始断点续扫",
		port.Field{Key: "任务", Value: taskID},
		port.Field{Key: "已完成表数", Value: fmt.Sprintf("%d", len(doneSet))})

	schemas, err := env.driver.ListSchemas(ctx, env.conn)
	if err != nil {
		return res, fmt.Errorf("列举模式失败：%v", err)
	}
	kept := filterSchemas(schemas, scope)
	out := s.scanTables(ctx, env.driver, env.conn, env.det, taskID, env.cfg.ID, kept, scope, doneSet)

	failed, err := s.tableCountByState(ctx, taskID, checkpointStateFailed)
	if err != nil {
		return res, err
	}
	pending, err := s.tableCountByState(ctx, taskID, checkpointStatePending)
	if err != nil {
		return res, err
	}
	res.Retried = out.Scanned
	res.NewFindings = len(out.Findings)
	res.Remaining = failed + pending
	status := port.TaskStatusDone
	if res.Remaining > 0 {
		status = port.TaskStatusPartial
	}
	res.Status = status

	summary := taskSummaryOf(task)
	summary.SchemaCount = len(kept)
	summary.TableCount = len(out.Tables)
	summary.ColumnCount = len(out.Columns)
	summary.FailedCount = failed
	summary.Message = fmt.Sprintf("续扫完成，新增 %d 条台账，未完成表 %d 张", res.NewFindings, res.Remaining)
	if page, ferr := s.store.ListFindings(ctx, port.FindingFilter{TaskID: taskID}); ferr == nil {
		summary.HitColumnCount = int(page.Total)
	}
	if err := s.store.UpdateTaskStatus(ctx, taskID, status, summary); err != nil {
		return res, fmt.Errorf("更新任务状态失败：%w", err)
	}
	s.log(ctx, port.LevelINFO, "断点续扫完成",
		port.Field{Key: "任务", Value: taskID},
		port.Field{Key: "状态", Value: string(status)},
		port.Field{Key: "续扫表数", Value: fmt.Sprintf("%d", res.Retried)},
		port.Field{Key: "新增台账", Value: fmt.Sprintf("%d", res.NewFindings)},
		port.Field{Key: "未完成表", Value: fmt.Sprintf("%d", res.Remaining)})

	if reportFormat != "" && outDir != "" {
		page, ferr := s.store.ListFindings(ctx, port.FindingFilter{TaskID: taskID, Page: 1, Limit: 1000})
		if ferr == nil {
			if err := s.exportReport(ctx, taskID, env.cfg.ID, env.ver, page.Items, env.rules, reportFormat, outDir); err != nil {
				return res, err
			}
		}
	}
	return res, nil
}

// doneTableKeys 读取任务已完成的表（断点为 done）。
func (s *Service) doneTableKeys(ctx context.Context, taskID string) (map[string]bool, error) {
	cps, err := s.store.ListCheckpoints(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("读取断点失败：%w", err)
	}
	out := map[string]bool{}
	for _, cp := range cps {
		if cp.State == checkpointStateDone && cp.TableMetaID != "" {
			out[cp.TableMetaID] = true
		}
	}
	return out, nil
}
