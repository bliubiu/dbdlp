package store

import (
	"context"
	"database/sql"
	"time"

	"dbdlp/internal/domain/port"
)

// 本文件实现审计日志与控制台账号相关的 Store 方法。

// ListAuditLogs 分页列出审计日志。
func (s *SQLiteStore) ListAuditLogs(ctx context.Context, f port.AuditFilter) (port.Page[port.AuditLog], error) {
	where := "WHERE 1=1"
	args := []any{}
	if f.Actor != "" {
		where += " AND actor=?"
		args = append(args, f.Actor)
	}
	if f.Action != "" {
		where += " AND action=?"
		args = append(args, f.Action)
	}
	var total int64
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM audit_log "+where, args...).Scan(&total); err != nil {
		return port.Page[port.AuditLog]{}, err
	}
	limit, offset := pageLimitOffset(f.Page, f.Limit)
	args = append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx, `SELECT id,actor,action,target_type,target_id,detail,result,error_message,ip_masked,created_at,updated_at
		FROM audit_log `+where+` ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return port.Page[port.AuditLog]{}, err
	}
	defer rows.Close()
	var items []port.AuditLog
	for rows.Next() {
		var a port.AuditLog
		var targetType, targetID, detail, errMsg, ip sql.NullString
		if err := rows.Scan(&a.ID, &a.Actor, &a.Action, &targetType, &targetID, &detail, &a.Result, &errMsg, &ip,
			&a.CreatedAt, &a.UpdatedAt); err != nil {
			return port.Page[port.AuditLog]{}, err
		}
		a.TargetType = targetType.String
		a.TargetID = targetID.String
		a.Detail = detail.String
		a.ErrorMessage = errMsg.String
		a.IPMasked = ip.String
		items = append(items, a)
	}
	return port.Page[port.AuditLog]{Items: items, Total: total, Page: f.Page, Limit: limit, HasMore: offset+int64(len(items)) < total}, nil
}

// CountAuditEventsSinceSuccess 统计 actor 的指定动作失败次数（自最近一次成功事件之后）。
// 用于登录连续失败递增锁定：无成功事件时统计全部失败记录。
func (s *SQLiteStore) CountAuditEventsSinceSuccess(ctx context.Context, actor, action string) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_log
		WHERE actor=? AND action=? AND result<>'success'
		  AND created_at > COALESCE(
			(SELECT MAX(created_at) FROM audit_log WHERE actor=? AND action=? AND result='success'), '')`,
		actor, action, actor, action).Scan(&n)
	return n, err
}

// LastEventAt 返回 actor 的指定动作最近一次发生时间；无记录返回零值。
func (s *SQLiteStore) LastEventAt(ctx context.Context, actor, action string) (time.Time, error) {
	var ts sql.NullString
	if err := s.db.QueryRowContext(ctx, `SELECT MAX(created_at) FROM audit_log WHERE actor=? AND action=?`,
		actor, action).Scan(&ts); err != nil {
		return time.Time{}, err
	}
	if !ts.Valid || ts.String == "" {
		return time.Time{}, nil
	}
	t, err := time.ParseInLocation("2006-01-02 15:04:05.000", ts.String, time.Local)
	if err != nil {
		return time.Time{}, nil
	}
	return t, nil
}

// GetUserByID 按 ID 查控制台用户。
func (s *SQLiteStore) GetUserByID(ctx context.Context, id string) (port.WebUser, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id,username,password_hash,display_name,role,enabled,last_login_at,created_at,updated_at,deleted_at
		FROM web_user WHERE id=? AND deleted_at IS NULL`, id)
	var u port.WebUser
	var deleted, ll sql.NullString
	if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.DisplayName, &u.Role, &u.Enabled, &ll, &u.CreatedAt, &u.UpdatedAt, &deleted); err != nil {
		return port.WebUser{}, err
	}
	u.LastLoginAt = ll.String
	return u, nil
}

// UpdateUserLastLogin 更新用户最近登录时间。
func (s *SQLiteStore) UpdateUserLastLogin(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE web_user SET last_login_at=?,updated_at=? WHERE id=?`, nowText(), nowText(), id)
	return err
}
