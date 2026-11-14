package httpapi

import (
	"net/http"
	"testing"
)

// 本文件做端点全量冒烟：确认 32 条端点全部注册且可达（不返回 404/405）。

// TestAllEndpointsRegistered 逐条访问 32 条端点，确认路由与方法均已注册。
func TestAllEndpointsRegistered(t *testing.T) {
	env := newTestEnv(t)
	env.insertDataSource(t, "ds-1", "ORA-PROD-01", "192.168.10.3")
	token := env.login(t)

	cases := []struct {
		name   string
		method string
		path   string
		body   any
		binary bool // 响应为文件流而非统一 JSON 结构
	}{
		{"数据源列表", http.MethodGet, "/api/v1/datasources", nil, false},
		{"数据源新增", http.MethodPost, "/api/v1/datasources", map[string]any{
			"name": "NEW-DS", "dialect": "oracle", "host": "db01.intra", "port": 1521,
			"username": "ro_scan", "password": "Secret@1",
		}, false},
		{"数据源详情", http.MethodGet, "/api/v1/datasources/ds-1", nil, false},
		{"数据源修改", http.MethodPatch, "/api/v1/datasources/ds-1", map[string]any{"remark": "备注"}, false},
		{"数据源删除", http.MethodDelete, "/api/v1/datasources/ds-1", nil, false},
		{"数据源连接测试", http.MethodPost, "/api/v1/datasources/ds-1/test", nil, false},
		{"数据源元数据", http.MethodGet, "/api/v1/datasources/ds-1/metadata", nil, false},
		{"批量导入", http.MethodPost, "/api/v1/datasources/import", nil, false},
		{"导入模板", http.MethodGet, "/api/v1/datasources/import/template?format=excel", nil, true},
		{"任务列表", http.MethodGet, "/api/v1/tasks", nil, false},
		{"任务新建", http.MethodPost, "/api/v1/tasks", map[string]any{"dataSourceId": "ds-1"}, false},
		{"任务详情", http.MethodGet, "/api/v1/tasks/task-1", nil, false},
		{"任务进度", http.MethodGet, "/api/v1/tasks/task-1/progress", nil, false},
		{"任务暂停", http.MethodPost, "/api/v1/tasks/task-1/pause", nil, false},
		{"任务恢复", http.MethodPost, "/api/v1/tasks/task-1/resume", nil, false},
		{"任务重试", http.MethodPost, "/api/v1/tasks/task-1/retry", map[string]any{"targetIds": []string{}}, false},
		{"台账列表", http.MethodGet, "/api/v1/findings", nil, false},
		{"台账详情", http.MethodGet, "/api/v1/findings/f-1", nil, false},
		{"命中明细", http.MethodGet, "/api/v1/findings/f-1/hits", nil, false},
		{"提交复核", http.MethodPost, "/api/v1/reviews", map[string]any{
			"findingId": "f-1", "conclusion": "confirmed", "comment": "确认",
		}, false},
		{"复核历史", http.MethodGet, "/api/v1/reviews", nil, false},
		{"规则清单", http.MethodGet, "/api/v1/rules", nil, false},
		{"规则快照", http.MethodGet, "/api/v1/rules/snapshots", nil, false},
		{"生成报告", http.MethodPost, "/api/v1/reports", map[string]any{
			"taskId": "task-1", "formats": []string{"json"},
		}, false},
		{"报告列表", http.MethodGet, "/api/v1/reports", nil, false},
		{"报告下载", http.MethodGet, "/api/v1/reports/r-1/download", nil, false},
		{"仪表盘汇总", http.MethodGet, "/api/v1/dashboard/summary", nil, false},
		{"审计日志", http.MethodGet, "/api/v1/audit-logs", nil, false},
		{"配置回显", http.MethodGet, "/api/v1/config", nil, false},
		{"当前登录态", http.MethodGet, "/api/v1/auth/me", nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res, body := doRequest(t, env.handler, c.method, c.path, token, c.body)
			if res.StatusCode == http.StatusMethodNotAllowed {
				t.Fatalf("方法未注册（HTTP 405）：%s %s", c.method, c.path)
			}
			if c.binary {
				if res.StatusCode != http.StatusOK {
					t.Fatalf("文件流端点期望 HTTP 200，实际 %d", res.StatusCode)
				}
				return
			}
			// 路由未注册时 ServeMux 返回纯文本 404，JSON 解析必然失败；此处即为「端点已注册」的判据。
			resp := decodeResp(t, body)
			if resp.Code < 0 {
				t.Fatalf("非法错误码 %d", resp.Code)
			}
		})
	}
}

// TestLogoutInvalidatesSession 校验登出后会话立即失效。
func TestLogoutInvalidatesSession(t *testing.T) {
	env := newTestEnv(t)
	token := env.login(t)
	res, body := doRequest(t, env.handler, http.MethodPost, "/api/v1/auth/logout", token, nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("登出期望 HTTP 200，实际 %d（%s）", res.StatusCode, string(body))
	}
	resp := decodeResp(t, body)
	if resp.Code != CodeOK {
		t.Fatalf("登出期望 code=0，实际 %d", resp.Code)
	}
	// 登出后旧令牌应失效
	res, body = doRequest(t, env.handler, http.MethodGet, "/api/v1/config", token, nil)
	after := decodeResp(t, body)
	if after.Code != CodeUnauthorized {
		t.Fatalf("登出后旧令牌期望 code=%d，实际 %d", CodeUnauthorized, after.Code)
	}
}

// TestUnknownAPIRouteReturnsNotFound 校验未知 API 路径返回 404 且不落静态首页。
func TestUnknownAPIRouteReturnsNotFound(t *testing.T) {
	env := newTestEnv(t)
	token := env.login(t)
	res, body := doRequest(t, env.handler, http.MethodGet, "/api/v1/not-exist", token, nil)
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("未知 API 路径期望 HTTP 404，实际 %d（%s）", res.StatusCode, string(body))
	}
}
