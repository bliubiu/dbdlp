package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"time"

	"dbdlp/internal/domain/port"
)

// 本文件实现规则快照、复核、审计、报告产物与会话的持久化。

func (s *SQLiteStore) SaveRuleSnapshot(ctx context.Context, version string, rules []port.Rule) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	for _, r := range rules {
		// regex_digest 只存摘要，不落原始正则（对齐 DDL 列语义）
		sum := sha256.Sum256([]byte(r.Regex))
		digest := hex.EncodeToString(sum[:])
		_, err := tx.ExecContext(ctx, `INSERT INTO rule_snapshot
			(id,rule_version,rule_id,sensitive_type,enabled,custom,regex_digest,verifier,threshold,created_at,updated_at)
			VALUES (?,?,?,?,?,?,?,?,?,?,?)
			ON CONFLICT(rule_version,rule_id) DO UPDATE SET
				sensitive_type=excluded.sensitive_type,enabled=excluded.enabled,custom=excluded.custom,
				regex_digest=excluded.regex_digest,verifier=excluded.verifier,threshold=excluded.threshold,updated_at=excluded.updated_at`,
			newID(), version, r.RuleID, r.SensitiveType, boolToInt(r.Enabled), boolToInt(r.Custom), digest, r.Verifier, r.Threshold, nowText(), nowText())
		if err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (s *SQLiteStore) SaveReview(ctx context.Context, r port.ReviewRecord) error {
	if r.ID == "" {
		r.ID = newID()
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO review_record
		(id,finding_id,reviewer,conclusion,comment,reviewed_at,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?)`,
		r.ID, r.FindingID, r.Reviewer, r.Conclusion, r.Comment, r.ReviewedAt, nowText(), nowText())
	return err
}

func (s *SQLiteStore) AppendAudit(ctx context.Context, a port.AuditLog) error {
	if a.ID == "" {
		a.ID = newID()
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO audit_log
		(id,actor,action,target_type,target_id,detail,result,error_message,ip_masked,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		a.ID, a.Actor, a.Action, a.TargetType, a.TargetID, a.Detail, a.Result, a.ErrorMessage, a.IPMasked, nowText(), nowText())
	return err
}

func (s *SQLiteStore) SaveReportArtifact(ctx context.Context, r port.ReportArtifact) error {
	if r.ID == "" {
		r.ID = newID()
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO report_artifact
		(id,task_id,format,file_path,file_size,generated_by,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?)`,
		r.ID, r.TaskID, r.Format, r.FilePath, r.FileSize, r.GeneratedBy, nowText(), nowText())
	return err
}

func (s *SQLiteStore) GetUserByName(ctx context.Context, username string) (port.WebUser, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id,username,password_hash,display_name,role,enabled,last_login_at,created_at,updated_at,deleted_at
		FROM web_user WHERE username=? AND deleted_at IS NULL`, username)
	var u port.WebUser
	var deleted, ll sql.NullString
	if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.DisplayName, &u.Role, &u.Enabled, &ll, &u.CreatedAt, &u.UpdatedAt, &deleted); err != nil {
		return port.WebUser{}, err
	}
	u.LastLoginAt = ll.String
	return u, nil
}

func (s *SQLiteStore) CreateSession(ctx context.Context, sess port.WebSession) error {
	if sess.ID == "" {
		sess.ID = newID()
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO web_session (id,user_id,token_hash,expires_at,created_at,updated_at)
		VALUES (?,?,?,?,?,?)`, sess.ID, sess.UserID, sess.TokenHash, sess.ExpiresAt, nowText(), nowText())
	return err
}

func (s *SQLiteStore) GetValidSession(ctx context.Context, tokenHash string, now time.Time) (port.WebSession, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id,user_id,token_hash,expires_at,created_at,updated_at FROM web_session WHERE token_hash=? AND expires_at>?`, tokenHash, now.Format("2006-01-02 15:04:05.000"))
	var s2 port.WebSession
	if err := row.Scan(&s2.ID, &s2.UserID, &s2.TokenHash, &s2.ExpiresAt, &s2.CreatedAt, &s2.UpdatedAt); err != nil {
		return port.WebSession{}, err
	}
	return s2, nil
}

func (s *SQLiteStore) DeleteSession(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM web_session WHERE id=?`, id)
	return err
}

func pageLimitOffset(page, limit int) (int, int64) {
	if limit <= 0 {
		limit = 20
	}
	if page <= 0 {
		page = 1
	}
	return limit, int64((page - 1) * limit)
}
