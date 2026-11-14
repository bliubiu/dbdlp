package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"dbdlp/internal/domain/port"
	"golang.org/x/crypto/bcrypt"
)

// 审计动作名（与 docs/02 §15 审计留痕要求对齐）。
const (
	actionLogin       = "auth.login"
	actionLogout      = "auth.logout"
	actionLockout     = "auth.lockout"
	actionDsCreate    = "datasource.create"
	actionDsUpdate    = "datasource.update"
	actionDsDelete    = "datasource.delete"
	actionDsTest      = "datasource.test"
	actionDsImport    = "datasource.import"
	actionTaskStart   = "task.start"
	actionTaskPause   = "task.pause"
	actionTaskResume  = "task.resume"
	actionTaskRetry   = "task.retry"
	actionReview      = "review.submit"
	actionReportExp   = "report.export"
	actionReportDown  = "report.download"
	auditResultOK     = "success"
	auditResultFailed = "failed"
)

// authed 包装需要 Bearer 认证的处理函数：校验令牌哈希、查会话与账号状态。
func (s *Server) authed(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, err := bearerToken(r)
		if err != nil {
			s.log(r.Context(), port.LevelWARN, "请求缺少访问令牌",
				port.Field{Key: "路径", Value: r.URL.Path},
				port.Field{Key: "方法", Value: r.Method})
			writeAPIError(w, errUnauthorized("未提供访问令牌，请先登录"))
			return
		}
		sess, serr := s.store.GetValidSession(r.Context(), tokenHash(token), time.Now())
		if serr != nil {
			s.log(r.Context(), port.LevelWARN, "会话无效或已过期",
				port.Field{Key: "路径", Value: r.URL.Path})
			writeAPIError(w, errUnauthorized("登录状态已失效，请重新登录"))
			return
		}
		user, uerr := s.store.GetUserByID(r.Context(), sess.UserID)
		if uerr != nil || !user.Enabled {
			writeAPIError(w, errUnauthorized("登录状态已失效，请重新登录"))
			return
		}
		if !roleAllowed(user.Role) {
			s.log(r.Context(), port.LevelWARN, "账号角色不在控制台授权范围内",
				port.Field{Key: "账号", Value: user.Username})
			writeAPIError(w, errForbidden("当前账号无权执行该操作"))
			return
		}
		ctx := context.WithValue(r.Context(), ctxUserKey{}, user)
		ctx = context.WithValue(ctx, ctxSessionKey{}, sess)
		next(w, r.WithContext(ctx))
	}
}

// bearerToken 从 Authorization 头提取 Bearer 令牌。
func bearerToken(r *http.Request) (string, error) {
	h := strings.TrimSpace(r.Header.Get("Authorization"))
	if h == "" {
		return "", fmt.Errorf("缺少 Authorization 头")
	}
	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", fmt.Errorf("Authorization 头格式非法")
	}
	tok := strings.TrimSpace(parts[1])
	if tok == "" {
		return "", fmt.Errorf("Bearer 令牌为空")
	}
	return tok, nil
}

// tokenHash 计算令牌哈希；库内只存哈希，明文令牌仅在登录响应中返回一次。
func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// roleAllowed 判断控制台角色是否在授权范围内。
// 本期为单管理员＋会话范围（docs/03 第 7 章 P13），仅放行既定控制台角色；
// 角色为空或非法视为无权访问，返回 1003。
func roleAllowed(role string) bool {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "admin", "operator", "auditor", "viewer":
		return true
	}
	return false
}

// randomToken 生成 32 字节随机令牌。
func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("生成会话令牌失败：%w", err)
	}
	return hex.EncodeToString(b), nil
}

// loginRequest 登录请求体。
type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// handleLogin 登录：bcrypt 校验口令，连续失败递增锁定并记审计。
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, err)
		return
	}
	username := strings.TrimSpace(req.Username)
	if username == "" || req.Password == "" {
		writeAPIError(w, errInvalid("用户名与口令不能为空"))
		return
	}
	ip := clientIP(r)

	// 锁定判定：存在锁定事件且未超过锁定时长时直接拒绝
	if lockAt, err := s.store.LastEventAt(r.Context(), username, actionLockout); err == nil && !lockAt.IsZero() {
		if remain := s.lockDuration - time.Since(lockAt); remain > 0 {
			s.log(r.Context(), port.LevelWARN, "账号处于登录锁定状态",
				port.Field{Key: "账号", Value: username},
				port.Field{Key: "剩余分钟", Value: fmt.Sprintf("%d", int(remain.Minutes())+1)})
			writeAPIError(w, errUnauthorized(fmt.Sprintf("账号已锁定，请在 %d 分钟后重试", int(remain.Minutes())+1)))
			return
		}
	}

	user, uerr := s.store.GetUserByName(r.Context(), username)
	if uerr != nil {
		s.failLogin(r, username, "", "账号或口令错误")
		writeAPIError(w, errUnauthorized("用户名或口令错误"))
		return
	}
	if !user.Enabled {
		s.failLogin(r, username, user.ID, "账号已禁用")
		writeAPIError(w, errUnauthorized("用户名或口令错误"))
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)) != nil {
		s.failLogin(r, username, user.ID, "口令校验失败")
		writeAPIError(w, errUnauthorized("用户名或口令错误"))
		return
	}

	token, terr := randomToken()
	if terr != nil {
		writeAPIError(w, errInternal("生成会话令牌失败"))
		return
	}
	expiresAt := time.Now().Add(s.sessionTTL).Format("2006-01-02 15:04:05.000")
	sess := port.WebSession{
		UserID: user.ID, TokenHash: tokenHash(token), ExpiresAt: expiresAt,
		CreatedAt: port.NowText(), UpdatedAt: port.NowText(),
	}
	if err := s.store.CreateSession(r.Context(), sess); err != nil {
		s.log(r.Context(), port.LevelERROR, "创建会话失败",
			port.Field{Key: "账号", Value: username},
			port.Field{Key: "原因", Value: err.Error()})
		writeAPIError(w, errInternal("创建会话失败，请稍后重试"))
		return
	}
	_ = s.store.UpdateUserLastLogin(r.Context(), user.ID)
	s.audit(r.Context(), username, actionLogin, "web_user", user.ID, "控制台登录成功", auditResultOK, "", ip)
	s.log(r.Context(), port.LevelINFO, "控制台登录成功",
		port.Field{Key: "账号", Value: username},
		port.Field{Key: "来源", Value: maskIPText(ip)})

	displayName := user.DisplayName
	if displayName == "" {
		displayName = user.Username
	}
	writeData(w, map[string]any{
		"token":       token,
		"expiresAt":   expiresAt,
		"displayName": displayName,
	})
}

// failLogin 记录一次登录失败并在达到阈值时写入锁定事件。
func (s *Server) failLogin(r *http.Request, username, userID, reason string) {
	failed, err := s.store.CountAuditEventsSinceSuccess(r.Context(), username, actionLogin)
	if err != nil {
		failed = 0
	}
	total := failed + 1
	ip := clientIP(r)
	if total >= s.lockThreshold {
		s.audit(r.Context(), username, actionLockout, "web_user", userID,
			fmt.Sprintf("连续登录失败 %d 次达到阈值，账号临时锁定", total), auditResultFailed, reason, ip)
		s.log(r.Context(), port.LevelWARN, "账号登录失败次数达到阈值，已临时锁定",
			port.Field{Key: "账号", Value: username},
			port.Field{Key: "次数", Value: fmt.Sprintf("%d", total)})
		return
	}
	s.audit(r.Context(), username, actionLogin, "web_user", userID, reason, auditResultFailed, reason, ip)
	s.log(r.Context(), port.LevelWARN, "登录失败",
		port.Field{Key: "账号", Value: username},
		port.Field{Key: "连续失败次数", Value: fmt.Sprintf("%d", total)})
}

// handleLogout 登出：删除当前会话。
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	sess := currentSession(r.Context())
	user := currentUser(r.Context())
	if sess.ID != "" {
		if err := s.store.DeleteSession(r.Context(), sess.ID); err != nil {
			writeAPIError(w, errInternal("注销会话失败"))
			return
		}
	}
	s.audit(r.Context(), user.Username, actionLogout, "web_user", user.ID, "控制台登出", auditResultOK, "", clientIP(r))
	writeData(w, map[string]any{})
}

// handleMe 返回当前登录态。
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	writeData(w, map[string]any{
		"userId":      user.ID,
		"username":    user.Username,
		"role":        user.Role,
		"displayName": user.DisplayName,
	})
}
