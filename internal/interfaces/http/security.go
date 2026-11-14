package httpapi

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"dbdlp/internal/domain/mask"
	"dbdlp/internal/domain/port"
)

// 本文件提供协议层通用工具：请求体解析、查询参数解析、分页装配、客户端 IP 脱敏、审计留痕。

// maxJSONBody 单个 JSON 请求体上限（1MB）。
const maxJSONBody = 1 << 20

// decodeJSON 解析 JSON 请求体；解析失败统一返回参数校验错误（1001）。
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	if r.Body == nil {
		return errInvalid("请求体不能为空")
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxJSONBody))
	if err := dec.Decode(dst); err != nil {
		return errInvalid("请求体不是合法的 JSON，请检查字段与格式")
	}
	return nil
}

// parsePageLimit 解析分页参数；缺省 page=1、limit=20，limit 上限 200。
func parsePageLimit(r *http.Request) (page int, limit int, err error) {
	page, limit = 1, 20
	q := r.URL.Query()
	if v := strings.TrimSpace(q.Get("page")); v != "" {
		n, cerr := strconv.Atoi(v)
		if cerr != nil || n < 1 {
			return 0, 0, errInvalid("页码必须为大于 0 的整数")
		}
		page = n
	}
	if v := strings.TrimSpace(q.Get("limit")); v != "" {
		n, cerr := strconv.Atoi(v)
		if cerr != nil || n < 1 || n > 200 {
			return 0, 0, errInvalid("每页条数必须为 1 到 200 之间的整数")
		}
		limit = n
	}
	return page, limit, nil
}

// pageDataOf 把分页结果装配为统一响应结构。
func pageDataOf[T any](items []T, total int64, page, limit int, hasMore bool) PageData {
	if items == nil {
		items = []T{}
	}
	return PageData{
		Items:   items,
		Total:   total,
		Page:    page,
		Limit:   limit,
		HasMore: hasMore,
	}
}

// nowMillis 返回当前毫秒时间戳（用于连接测试耗时统计）。
func nowMillis() int64 { return time.Now().UnixMilli() }

// clientIP 取客户端来源地址（仅用于审计脱敏展示，不作为信任判定依据）。
func clientIP(r *http.Request) string {
	if v := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); v != "" {
		if i := strings.Index(v, ","); i > 0 {
			v = v[:i]
		}
		if ip := strings.TrimSpace(v); ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// maskedIP 返回脱敏后的客户端地址。
func maskedIP(r *http.Request) string { return mask.MaskIPv4(clientIP(r)) }

// maskIPText 返回脱敏后的地址文本（供日志字段使用）。
func maskIPText(ip string) string { return mask.MaskIPv4(ip) }

// audit 写审计留痕；审计写入失败只记日志，不影响主流程返回。
func (s *Server) audit(ctx context.Context, actor, action, targetType, targetID, detail, result, errMsg string, ip string) {
	entry := port.AuditLog{
		Actor: actor, Action: action, TargetType: targetType, TargetID: targetID,
		Detail: detail, Result: result, ErrorMessage: errMsg, IPMasked: mask.MaskIPv4(ip),
	}
	if err := s.store.AppendAudit(ctx, entry); err != nil {
		s.log(ctx, port.LevelWARN, "审计日志写入失败",
			port.Field{Key: "动作", Value: action},
			port.Field{Key: "原因", Value: err.Error()})
	}
}

// ctxUserKey 会话用户在上下文中的键。
type ctxUserKey struct{}

// ctxSessionKey 会话记录在上下文中的键。
type ctxSessionKey struct{}

// currentUser 取当前登录用户；未登录返回零值。
func currentUser(ctx context.Context) port.WebUser {
	if u, ok := ctx.Value(ctxUserKey{}).(port.WebUser); ok {
		return u
	}
	return port.WebUser{}
}

// currentSession 取当前会话；未登录返回零值。
func currentSession(ctx context.Context) port.WebSession {
	if s, ok := ctx.Value(ctxSessionKey{}).(port.WebSession); ok {
		return s
	}
	return port.WebSession{}
}
