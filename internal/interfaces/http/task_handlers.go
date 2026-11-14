package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"dbdlp/internal/application/scan"
	"dbdlp/internal/domain/port"
)

// 本文件实现扫描任务端点：列表、创建、详情、进度、暂停、恢复、重试。

// createTaskRequest 新建扫描任务请求体。
type createTaskRequest struct {
	DataSourceID  string          `json:"dataSourceId"`
	Scope         *port.ScanScope `json:"scope"`
	SampleRows    int             `json:"sampleRows"`
	BatchSize     int             `json:"batchSize"`
	Concurrency   int             `json:"concurrency"`
	LowPeakWindow string          `json:"lowPeakWindow"`
}

// handleListTasks 任务列表（服务端分页）。
func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	page, limit, err := parsePageLimit(r)
	if err != nil {
		writeError(w, err)
		return
	}
	q := r.URL.Query()
	f := port.TaskFilter{
		DataSourceID: strings.TrimSpace(q.Get("dataSourceId")),
		Status:       port.TaskStatus(strings.TrimSpace(q.Get("status"))),
		Page:         page, Limit: limit,
	}
	res, serr := s.store.ListTasks(r.Context(), f)
	if serr != nil {
		writeAPIError(w, errInternal("查询任务列表失败"))
		return
	}
	writeData(w, pageDataOf(newTaskViews(res.Items), res.Total, res.Page, res.Limit, res.HasMore))
}

// handleCreateTask 新建扫描任务：同一数据源存在运行中任务时拒绝（3001）。
func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	if s.scanSvc == nil {
		writeAPIError(w, errInternal("扫描服务未初始化"))
		return
	}
	var req createTaskRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, err)
		return
	}
	dsID := strings.TrimSpace(req.DataSourceID)
	if dsID == "" {
		writeAPIError(w, errInvalid("数据源标识不能为空"))
		return
	}
	if _, err := s.store.GetDataSource(r.Context(), dsID); err != nil {
		writeAPIError(w, errNotFound("指定的数据源不存在或已删除"))
		return
	}
	if req.SampleRows != 0 && (req.SampleRows < 1 || req.SampleRows > 1000) {
		writeAPIError(w, errInvalid("采样行数必须为 1 到 1000 之间的整数"))
		return
	}
	if req.BatchSize != 0 && (req.BatchSize < 1 || req.BatchSize > 500) {
		writeAPIError(w, errInvalid("批大小必须为 1 到 500 之间的整数"))
		return
	}
	if req.Concurrency != 0 && (req.Concurrency < 1 || req.Concurrency > 4) {
		writeAPIError(w, errInvalid("并发度必须为 1 到 4 之间的整数"))
		return
	}
	running, cerr := s.store.CountRunningTasksBySource(r.Context(), dsID)
	if cerr != nil {
		writeAPIError(w, errInternal("校验运行中任务失败"))
		return
	}
	if running > 0 {
		s.audit(r.Context(), currentUser(r.Context()).Username, actionTaskStart, "scan_task", dsID,
			"启动被拒绝：该数据源存在运行中的任务", auditResultFailed, "", clientIP(r))
		writeAPIError(w, errTaskConflict("该数据源已有运行中的任务，不能重复启动"))
		return
	}
	actor := currentUser(r.Context()).Username
	taskID, err := s.scanSvc.Run(r.Context(), dsID, "", "")
	if err != nil {
		s.audit(r.Context(), actor, actionTaskStart, "scan_task", dsID, "启动扫描任务失败", auditResultFailed, err.Error(), clientIP(r))
		s.log(r.Context(), port.LevelERROR, "启动扫描任务失败",
			port.Field{Key: "数据源ID", Value: dsID},
			port.Field{Key: "原因", Value: err.Error()})
		writeAPIError(w, errInternal("启动扫描任务失败："+err.Error()))
		return
	}
	detail := "启动扫描任务 " + taskID
	if req.SampleRows != 0 || req.BatchSize != 0 || req.Concurrency != 0 {
		detail += "（请求携带的采样参数本次未生效，实际以配置文件为准）"
	}
	s.audit(r.Context(), actor, actionTaskStart, "scan_task", taskID, detail, auditResultOK, "", clientIP(r))
	s.log(r.Context(), port.LevelINFO, "启动扫描任务",
		port.Field{Key: "任务", Value: taskID},
		port.Field{Key: "数据源ID", Value: dsID},
		port.Field{Key: "操作者", Value: actor})
	writeData(w, map[string]any{"taskId": taskID})
}

// handleGetTask 任务详情（含解析后的汇总）。
func (s *Server) handleGetTask(w http.ResponseWriter, r *http.Request) {
	t, err := s.mustTask(r)
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, newTaskView(t))
}

// handleTaskProgress 任务进度。
func (s *Server) handleTaskProgress(w http.ResponseWriter, r *http.Request) {
	t, err := s.mustTask(r)
	if err != nil {
		writeError(w, err)
		return
	}
	view := newTaskView(t)
	stats, serr := s.store.CountTaskProgress(r.Context(), t.ID)
	if serr != nil {
		writeAPIError(w, errInternal("统计任务进度失败"))
		return
	}
	totalTables := view.Summary.TableCount
	if totalTables <= 0 {
		totalTables = stats.Tables
	}
	totalColumns := view.Summary.ColumnCount
	if totalColumns <= 0 {
		totalColumns = stats.Columns
	}
	writeData(w, map[string]any{
		"status":       view.Status,
		"doneTables":   minInt(stats.Tables, totalTables),
		"totalTables":  totalTables,
		"doneColumns":  minInt(stats.Columns, totalColumns),
		"totalColumns": totalColumns,
		"failedCount":  view.Summary.FailedCount,
	})
}

// handlePauseTask 暂停任务：仅运行中任务可暂停。
func (s *Server) handlePauseTask(w http.ResponseWriter, r *http.Request) {
	t, err := s.mustTask(r)
	if err != nil {
		writeError(w, err)
		return
	}
	if t.Status != port.TaskStatusRunning {
		s.audit(r.Context(), currentUser(r.Context()).Username, actionTaskPause, "scan_task", t.ID,
			"暂停被拒绝：任务状态不是运行中", auditResultFailed, "", clientIP(r))
		writeAPIError(w, errTaskConflict(fmt.Sprintf("任务当前状态为 %s，只有运行中的任务可以暂停", t.Status)))
		return
	}
	if uerr := s.store.UpdateTaskState(r.Context(), t.ID, port.TaskStatusPaused, ""); uerr != nil {
		writeAPIError(w, errInternal("更新任务状态失败"))
		return
	}
	s.audit(r.Context(), currentUser(r.Context()).Username, actionTaskPause, "scan_task", t.ID,
		"暂停扫描任务", auditResultOK, "", clientIP(r))
	writeDataMessage(w, map[string]any{"status": string(port.TaskStatusPaused)},
		"任务已置为暂停状态；当前扫描执行体未提供中断钩子，暂停标记将在本次扫描自然结束时被最终状态覆盖")
}

// handleResumeTask 恢复任务：暂停或失败的任务可恢复。
func (s *Server) handleResumeTask(w http.ResponseWriter, r *http.Request) {
	t, err := s.mustTask(r)
	if err != nil {
		writeError(w, err)
		return
	}
	if t.Status != port.TaskStatusPaused && t.Status != port.TaskStatusFailed && t.Status != port.TaskStatusPartial {
		s.audit(r.Context(), currentUser(r.Context()).Username, actionTaskResume, "scan_task", t.ID,
			"恢复被拒绝：任务状态不允许恢复", auditResultFailed, "", clientIP(r))
		writeAPIError(w, errTaskConflict(fmt.Sprintf("任务当前状态为 %s，只有暂停或失败的任务可以恢复", t.Status)))
		return
	}
	if uerr := s.store.UpdateTaskState(r.Context(), t.ID, port.TaskStatusRunning, ""); uerr != nil {
		writeAPIError(w, errInternal("更新任务状态失败"))
		return
	}
	s.audit(r.Context(), currentUser(r.Context()).Username, actionTaskResume, "scan_task", t.ID,
		"恢复扫描任务", auditResultOK, "", clientIP(r))
	writeDataMessage(w, map[string]any{"status": string(port.TaskStatusRunning)},
		"任务已置为运行中状态；当前扫描执行体未提供断点续扫钩子，实际重扫需重新发起扫描任务")
}

// handleRetryTask 重试失败对象：仅重扫失败表，已成功的表不重复扫描、不重复入库。
//
// 请求体可选 targetIds：指定要重试的表（形如「模式/表」）；为空表示重试全部失败表。
func (s *Server) handleRetryTask(w http.ResponseWriter, r *http.Request) {
	t, err := s.mustTask(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var req struct {
		TargetIDs []string `json:"targetIds"`
	}
	if r.ContentLength > 0 {
		if derr := decodeJSON(w, r, &req); derr != nil {
			writeError(w, derr)
			return
		}
	}
	if s.scanSvc == nil {
		writeAPIError(w, errInternal("扫描服务未初始化"))
		return
	}
	if t.Status != port.TaskStatusFailed && t.Status != port.TaskStatusPartial {
		s.audit(r.Context(), currentUser(r.Context()).Username, actionTaskRetry, "scan_task", t.ID,
			"重试被拒绝：任务状态不允许重试", auditResultFailed, "", clientIP(r))
		writeAPIError(w, errTaskConflict(fmt.Sprintf("任务当前状态为 %s，只有失败或部分失败的任务可以重试", t.Status)))
		return
	}
	actor := currentUser(r.Context()).Username
	res, rerr := s.scanSvc.RetryFailedTables(r.Context(), t.ID, req.TargetIDs, "", s.reportOutDir)
	if rerr != nil {
		s.audit(r.Context(), actor, actionTaskRetry, "scan_task", t.ID, "重试失败对象失败", auditResultFailed, rerr.Error(), clientIP(r))
		s.log(r.Context(), port.LevelERROR, "重试失败对象失败",
			port.Field{Key: "任务", Value: t.ID},
			port.Field{Key: "操作者", Value: actor},
			port.Field{Key: "原因", Value: rerr.Error()})
		writeAPIError(w, retryErrorOf(rerr))
		return
	}
	s.audit(r.Context(), actor, actionTaskRetry, "scan_task", t.ID,
		fmt.Sprintf("重试失败对象 %d 个，新增台账 %d 条，剩余失败 %d 张", res.Retried, res.NewFindings, res.Remaining),
		auditResultOK, "", clientIP(r))
	s.log(r.Context(), port.LevelINFO, "重试失败对象",
		port.Field{Key: "任务", Value: t.ID},
		port.Field{Key: "操作者", Value: actor},
		port.Field{Key: "重试表数", Value: fmt.Sprintf("%d", res.Retried)},
		port.Field{Key: "剩余失败表", Value: fmt.Sprintf("%d", res.Remaining)})
	writeData(w, map[string]any{
		"retried":     res.Retried,
		"newFindings": res.NewFindings,
		"remaining":   res.Remaining,
		"status":      string(res.Status),
	})
}

// retryErrorOf 把应用层错误映射为业务错误码：状态冲突 3001、无失败表/对象不存在 1004、其余 5000。
func retryErrorOf(err error) *apiError {
	switch {
	case errors.Is(err, scan.ErrTaskNotRetryable):
		return errTaskConflict(err.Error())
	case errors.Is(err, scan.ErrNoFailedTable), errors.Is(err, scan.ErrTargetNotFailed):
		return errNotFound(err.Error())
	default:
		return errInternal("重试失败：" + err.Error())
	}
}

// mustTask 按路径参数读取任务；不存在返回 1004。
func (s *Server) mustTask(r *http.Request) (port.ScanTask, error) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		return port.ScanTask{}, errInvalid("任务标识不能为空")
	}
	t, err := s.store.GetTask(r.Context(), id)
	if err != nil {
		return port.ScanTask{}, errNotFound("指定的扫描任务不存在")
	}
	return t, nil
}

// minInt 返回两数较小值。
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
