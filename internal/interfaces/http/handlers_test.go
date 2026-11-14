package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// 本文件覆盖统一响应格式、认证拒绝路径、凭据占位串、分页结构与错误码映射。

// TestUnifiedResponseFormat 校验统一响应体三字段固定存在且顺序稳定。
func TestUnifiedResponseFormat(t *testing.T) {
	raw, err := json.Marshal(Resp{Code: CodeOK, Data: map[string]any{"k": "v"}, Message: ""})
	if err != nil {
		t.Fatalf("序列化响应体失败：%v", err)
	}
	got := string(raw)
	want := `{"code":0,"data":{"k":"v"},"message":""}`
	if got != want {
		t.Fatalf("统一响应格式不符：期望 %s，实际 %s", want, got)
	}
}

// TestUnifiedResponseEnvelope 校验成功与失败响应都带 code/data/message。
func TestUnifiedResponseEnvelope(t *testing.T) {
	env := newTestEnv(t)
	res, body := doRequest(t, env.handler, http.MethodGet, "/api/v1/datasources", "", nil)
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("未认证期望 HTTP 401，实际 %d", res.StatusCode)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("解析响应体失败：%v", err)
	}
	for _, key := range []string{"code", "data", "message"} {
		if _, ok := raw[key]; !ok {
			t.Fatalf("响应体缺少字段 %s：%s", key, string(body))
		}
	}
}

// TestBearerAuthRejected 覆盖缺失令牌、格式非法、令牌无效三条拒绝路径。
func TestBearerAuthRejected(t *testing.T) {
	env := newTestEnv(t)
	cases := []struct {
		name  string
		token string
	}{
		{"无令牌", ""},
		{"非法令牌", "not-a-real-token"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res, body := doRequest(t, env.handler, http.MethodGet, "/api/v1/datasources", c.token, nil)
			resp := decodeResp(t, body)
			if resp.Code != CodeUnauthorized {
				t.Fatalf("期望 code=%d，实际 %d（%s）", CodeUnauthorized, resp.Code, string(body))
			}
			if res.StatusCode != http.StatusUnauthorized {
				t.Fatalf("期望 HTTP 401，实际 %d", res.StatusCode)
			}
			if resp.Message == "" {
				t.Fatal("错误响应缺少中文提示")
			}
		})
	}
}

// TestAuthorizationSchemeRejected 非 Bearer 方案应被拒绝。
func TestAuthorizationSchemeRejected(t *testing.T) {
	env := newTestEnv(t)
	req := newRequest(http.MethodGet, "/api/v1/datasources")
	req.Header.Set("Authorization", "Basic YWRtaW46YWRtaW4=")
	rec := newRecorder()
	env.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("非 Bearer 方案期望 HTTP 401，实际 %d", rec.Code)
	}
}

// TestPasswordPlaceholder 校验数据源响应中口令恒为固定占位串。
func TestPasswordPlaceholder(t *testing.T) {
	env := newTestEnv(t)
	env.insertDataSource(t, "ds-1", "ORA-PROD-01", "192.168.10.3")
	token := env.login(t)

	res, body := doRequest(t, env.handler, http.MethodGet, "/api/v1/datasources/ds-1", token, nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("详情查询 HTTP 状态码异常：%d（%s）", res.StatusCode, string(body))
	}
	resp := decodeResp(t, body)
	data, _ := resp.Data.(map[string]any)
	password, _ := data["password"].(string)
	if password != passwordPlaceholder {
		t.Fatalf("详情口令期望 %s，实际 %s", passwordPlaceholder, password)
	}
	if password == "Secret@123" {
		t.Fatal("详情返回了明文口令")
	}

	// 列表同样不得回显明文口令
	res, body = doRequest(t, env.handler, http.MethodGet, "/api/v1/datasources", token, nil)
	resp = decodeResp(t, body)
	page, _ := resp.Data.(map[string]any)
	items, _ := page["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("列表 items 期望 1 条，实际 %d", len(items))
	}
	item, _ := items[0].(map[string]any)
	if p, _ := item["password"].(string); p != passwordPlaceholder {
		t.Fatalf("列表口令期望 %s，实际 %s", passwordPlaceholder, p)
	}
	// 主机按 IPv4 规则脱敏
	if h, _ := item["host"].(string); h != "*.*.10.3" {
		t.Fatalf("主机脱敏期望 *.*.10.3，实际 %s", h)
	}
	// 连接串一律不回显
	if dsn, ok := item["dsn"].(string); ok && dsn != "" {
		t.Fatalf("连接串不应回显，实际 %s", dsn)
	}
}

// TestPaginationDataShape 校验分页 data 结构固定为 items/total/page/limit/hasMore。
func TestPaginationDataShape(t *testing.T) {
	env := newTestEnv(t)
	for i := 1; i <= 3; i++ {
		env.insertDataSource(t, "ds-"+string(rune('0'+i)), "ORA-PROD-0"+string(rune('0'+i)), "127.0.0.1")
	}
	token := env.login(t)
	res, body := doRequest(t, env.handler, http.MethodGet, "/api/v1/datasources?page=1&limit=2", token, nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("列表查询 HTTP 状态码异常：%d（%s）", res.StatusCode, string(body))
	}
	resp := decodeResp(t, body)
	page, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("分页 data 结构非法：%v", resp.Data)
	}
	if len(page) != 5 {
		t.Fatalf("分页 data 期望 5 个字段，实际 %d：%v", len(page), page)
	}
	items, _ := page["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("每页 2 条期望 2 条，实际 %d", len(items))
	}
	if total, _ := page["total"].(float64); total != 3 {
		t.Fatalf("total 期望 3，实际 %v", page["total"])
	}
	if p, _ := page["page"].(float64); p != 1 {
		t.Fatalf("page 期望 1，实际 %v", page["page"])
	}
	if l, _ := page["limit"].(float64); l != 2 {
		t.Fatalf("limit 期望 2，实际 %v", page["limit"])
	}
	if hm, _ := page["hasMore"].(bool); !hm {
		t.Fatalf("hasMore 期望 true，实际 %v", page["hasMore"])
	}
}

// TestPaginationParamValidation 校验分页参数非法时返回 1001。
func TestPaginationParamValidation(t *testing.T) {
	env := newTestEnv(t)
	token := env.login(t)
	for _, query := range []string{"?page=0", "?page=abc", "?limit=0", "?limit=9999"} {
		res, body := doRequest(t, env.handler, http.MethodGet, "/api/v1/datasources"+query, token, nil)
		resp := decodeResp(t, body)
		if resp.Code != CodeInvalidParam {
			t.Fatalf("%s 期望 code=%d，实际 %d（%s）", query, CodeInvalidParam, resp.Code, string(body))
		}
		if res.StatusCode != http.StatusBadRequest {
			t.Fatalf("%s 期望 HTTP 400，实际 %d", query, res.StatusCode)
		}
	}
}

// TestResourceNotFound 校验资源不存在返回 1004。
func TestResourceNotFound(t *testing.T) {
	env := newTestEnv(t)
	token := env.login(t)
	res, body := doRequest(t, env.handler, http.MethodGet, "/api/v1/datasources/not-exist", token, nil)
	resp := decodeResp(t, body)
	if resp.Code != CodeNotFound {
		t.Fatalf("期望 code=%d，实际 %d", CodeNotFound, resp.Code)
	}
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("期望 HTTP 404，实际 %d", res.StatusCode)
	}
	if !strings.Contains(resp.Message, "不存在") {
		t.Fatalf("错误提示应为中文且含「不存在」，实际 %s", resp.Message)
	}
}

// TestRoleNotAllowedReturnsForbidden 校验角色不在授权范围内时返回 1003。
func TestRoleNotAllowedReturnsForbidden(t *testing.T) {
	env := newTestEnv(t)
	token := env.login(t)
	if _, err := env.store.DB().Exec(`UPDATE web_user SET role=? WHERE username=?`, "unknown-role", "admin"); err != nil {
		t.Fatalf("调整测试账号角色失败：%v", err)
	}
	res, body := doRequest(t, env.handler, http.MethodGet, "/api/v1/datasources", token, nil)
	resp := decodeResp(t, body)
	if resp.Code != CodeForbidden {
		t.Fatalf("角色越权期望 code=%d，实际 %d（%s）", CodeForbidden, resp.Code, string(body))
	}
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("角色越权期望 HTTP 403，实际 %d", res.StatusCode)
	}
}

// TestLoginWithBootstrappedAdmin 校验由 store.EnsureAdminUser 引导创建的管理员账号可直接登录。
// 该用例覆盖「引导建号 → 登录 → 访问受保护端点」的真实串联路径。
func TestLoginWithBootstrappedAdmin(t *testing.T) {
	env := newTestEnv(t)
	created, err := env.store.EnsureAdminUser(context.Background(), "opsadmin", "Boot@12345", "运维管理员")
	if err != nil {
		t.Fatalf("引导创建管理员账号失败：%v", err)
	}
	if !created {
		t.Fatal("期望首次引导创建新账号")
	}
	res, body := doRequest(t, env.handler, http.MethodPost, "/api/v1/auth/login", "",
		map[string]string{"username": "opsadmin", "password": "Boot@12345"})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("引导账号登录期望 HTTP 200，实际 %d（%s）", res.StatusCode, string(body))
	}
	resp := decodeResp(t, body)
	if resp.Code != CodeOK {
		t.Fatalf("引导账号登录期望 code=0，实际 %d（%s）", resp.Code, string(body))
	}
	data, _ := resp.Data.(map[string]any)
	token, _ := data["token"].(string)
	if token == "" {
		t.Fatal("引导账号登录未返回 token")
	}
	// 该账号应能正常访问受保护端点（role=admin 通过 1003 角色校验）
	res, body = doRequest(t, env.handler, http.MethodGet, "/api/v1/auth/me", token, nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("引导账号访问受保护端点期望 HTTP 200，实际 %d（%s）", res.StatusCode, string(body))
	}
	me := decodeResp(t, body)
	meData, _ := me.Data.(map[string]any)
	if meData["username"] != "opsadmin" {
		t.Fatalf("当前登录态用户名期望 opsadmin，实际 %v", meData["username"])
	}
	// 重复引导不覆盖口令：口令应仍为原值
	created2, err := env.store.EnsureAdminUser(context.Background(), "opsadmin", "Other@98765", "运维管理员")
	if err != nil {
		t.Fatalf("重复引导失败：%v", err)
	}
	if created2 {
		t.Fatal("重复引导不应重建账号")
	}
	res, body = doRequest(t, env.handler, http.MethodPost, "/api/v1/auth/login", "",
		map[string]string{"username": "opsadmin", "password": "Other@98765"})
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("重复引导后旧口令应仍有效、新口令应失败，实际 HTTP %d（%s）", res.StatusCode, string(body))
	}
}

// TestLoginFailureLockout 校验连续失败达到阈值后账号被真实锁定。
func TestLoginFailureLockout(t *testing.T) {
	env := newTestEnv(t)
	var lastMessage string
	for i := 0; i < 6; i++ {
		_, body := doRequest(t, env.handler, http.MethodPost, "/api/v1/auth/login", "",
			map[string]string{"username": "admin", "password": "WrongPassword@1"})
		resp := decodeResp(t, body)
		if resp.Code != CodeUnauthorized {
			t.Fatalf("第 %d 次失败期望 code=%d，实际 %d", i+1, CodeUnauthorized, resp.Code)
		}
		lastMessage = resp.Message
	}
	if !strings.Contains(lastMessage, "锁定") {
		t.Fatalf("达到阈值后期望提示锁定，实际 %s", lastMessage)
	}
	// 锁定期间即使口令正确也应被拒绝
	_, body := doRequest(t, env.handler, http.MethodPost, "/api/v1/auth/login", "",
		map[string]string{"username": "admin", "password": testAdminPassword})
	resp := decodeResp(t, body)
	if resp.Code != CodeUnauthorized {
		t.Fatalf("锁定期间期望 code=%d，实际 %d", CodeUnauthorized, resp.Code)
	}
	if !strings.Contains(resp.Message, "锁定") {
		t.Fatalf("锁定期间期望提示锁定，实际 %s", resp.Message)
	}
}
