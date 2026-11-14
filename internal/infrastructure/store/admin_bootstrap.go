package store

import (
	"context"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// EnsureAdminUser 确保存在指定控制台管理员账号。
//
// 语义：
//   - 账号已存在 → 不覆盖其口令（返回 created=false），避免每次启动重置管理员口令；
//   - 账号不存在 → 以 bcrypt 散列写入，返回 created=true；
//   - 口令散列永不出现在返回值与日志中。
//
// 该方法刻意不进入 port.Store 接口：控制台引导属于基础设施实现细节，
// 不应扩大领域端口的职责范围。
func (s *SQLiteStore) EnsureAdminUser(ctx context.Context, username, password, displayName string) (created bool, err error) {
	if username == "" {
		return false, fmt.Errorf("管理员账号不能为空")
	}
	if password == "" {
		return false, fmt.Errorf("管理员口令不能为空")
	}
	var exists int
	if err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM web_user WHERE username=? AND deleted_at IS NULL", username).Scan(&exists); err != nil {
		return false, fmt.Errorf("查询管理员账号失败：%w", err)
	}
	if exists > 0 {
		return false, nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return false, fmt.Errorf("生成口令散列失败：%w", err)
	}
	if displayName == "" {
		displayName = "系统管理员"
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO web_user
		(id,username,password_hash,display_name,role,enabled,created_at,updated_at)
		VALUES (?,?,?,?,'admin',1,?,?)`,
		newID(), username, string(hash), displayName, nowText(), nowText())
	if err != nil {
		return false, fmt.Errorf("写入管理员账号失败：%w", err)
	}
	return true, nil
}
