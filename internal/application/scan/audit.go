// audit.go CLI 形态的任务审计（docs/06：CLI 操作与控制台同构留痕）。
package scan

import (
	"context"
	"fmt"
	"strings"

	"dbdlp/internal/domain/port"
)

// WithAuditActor 设置用例层审计主体。非空（CLI 形态传 "cli"）时由用例层写任务审计；
// HTTP 形态保持空串，由控制台 handler 统一留痕，避免双写。
func (s *Service) WithAuditActor(actor string) *Service {
	if a := strings.TrimSpace(actor); a != "" {
		s.auditActor = a
	}
	return s
}

// auditTask 写任务审计；未设置审计主体时跳过（HTTP 形态由 handler 留痕）。
// 审计写入失败只记 WARN，不影响主流程。
func (s *Service) auditTask(ctx context.Context, action, taskID, detail, result, errMsg string) {
	if s.auditActor == "" || s.store == nil {
		return
	}
	entry := port.AuditLog{
		Actor: s.auditActor, Action: action, TargetType: "scan_task", TargetID: taskID,
		Detail: detail, Result: result, ErrorMessage: errMsg,
	}
	if err := s.store.AppendAudit(ctx, entry); err != nil && s.logger != nil {
		s.log(ctx, port.LevelWARN, "审计日志写入失败",
			port.Field{Key: "动作", Value: action},
			port.Field{Key: "原因", Value: err.Error()})
	}
}

// auditResult 审计结果与错误信息归一。
func auditResult(err error) (string, string) {
	if err != nil {
		return "failed", err.Error()
	}
	return "success", ""
}

// runRetryResult 重试/续扫的统一审计包装：按结果记 task.retry / task.resume。
func (s *Service) runRetryResult(ctx context.Context, action, taskID string, res RetryResult, err error) (RetryResult, error) {
	result, errMsg := auditResult(err)
	detail := fmt.Sprintf("重扫表=%d，新增命中=%d，剩余失败=%d，状态=%s",
		res.Retried, res.NewFindings, res.Remaining, res.Status)
	s.auditTask(ctx, action, taskID, detail, result, errMsg)
	return res, err
}
