package httpapi

import (
	"net/http"
	"strings"

	"dbdlp/internal/domain/port"
)

// 本文件实现仪表盘汇总、审计日志查询与配置只读回显。

// handleDashboardSummary 仪表盘汇总。
func (s *Server) handleDashboardSummary(w http.ResponseWriter, r *http.Request) {
	taskID := strings.TrimSpace(r.URL.Query().Get("taskId"))
	if taskID == "" {
		tasks, terr := s.store.ListTasks(r.Context(), port.TaskFilter{Page: 1, Limit: 1})
		if terr != nil {
			writeAPIError(w, errInternal("查询任务失败"))
			return
		}
		if len(tasks.Items) > 0 {
			taskID = tasks.Items[0].ID
		}
	}
	riskCounts, err := s.store.CountFindingsByRisk(r.Context(), taskID)
	if err != nil {
		writeAPIError(w, errInternal("统计风险分布失败"))
		return
	}
	typeCounts, err := s.store.CountFindingsByType(r.Context(), taskID)
	if err != nil {
		writeAPIError(w, errInternal("统计敏感类型分布失败"))
		return
	}
	schemaCounts, err := s.store.CountFindingsBySchema(r.Context(), taskID)
	if err != nil {
		writeAPIError(w, errInternal("统计模式分布失败"))
		return
	}
	tableCount, columnCount, hitColumnCount := 0, 0, 0
	if taskID != "" {
		if t, terr := s.store.GetTask(r.Context(), taskID); terr == nil {
			view := newTaskView(t)
			tableCount = view.Summary.TableCount
			columnCount = view.Summary.ColumnCount
			hitColumnCount = view.Summary.HitColumnCount
		}
	}
	writeData(w, map[string]any{
		"riskCounts":     riskCounts,
		"typeCounts":     typeCounts,
		"schemaCounts":   schemaCounts,
		"tableCount":     tableCount,
		"columnCount":    columnCount,
		"hitColumnCount": hitColumnCount,
	})
}

// handleListAuditLogs 审计日志列表（已脱敏）。
func (s *Server) handleListAuditLogs(w http.ResponseWriter, r *http.Request) {
	page, limit, err := parsePageLimit(r)
	if err != nil {
		writeError(w, err)
		return
	}
	q := r.URL.Query()
	f := port.AuditFilter{
		Actor:  strings.TrimSpace(q.Get("actor")),
		Action: strings.TrimSpace(q.Get("action")),
		Page:   page, Limit: limit,
	}
	res, serr := s.store.ListAuditLogs(r.Context(), f)
	if serr != nil {
		writeAPIError(w, errInternal("查询审计日志失败"))
		return
	}
	writeData(w, pageDataOf(newAuditLogViews(res.Items), res.Total, res.Page, res.Limit, res.HasMore))
}

// handleGetConfig 配置只读回显（不含密钥与 DSN）。
func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	writeData(w, s.configEcho)
}
