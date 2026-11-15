package httpapi

import (
	"database/sql"
	"net/http"
	"path/filepath"
	"testing"

	"dbdlp/internal/application/scan"
	apptesting "dbdlp/internal/application/testing"
	"dbdlp/internal/domain/port"
	"dbdlp/internal/domain/rule"
	dsdriver "dbdlp/internal/infrastructure/datasource"
	"dbdlp/internal/infrastructure/reporter"

	_ "modernc.org/sqlite"
)

// 本文件覆盖 POST /api/v1/tasks/{id}/retry 的 HTTP 层行为（docs/07 IT-06 / EP-19）。

// setupRetryTargetDB 建两张表：USERS 正常，ORDERS 的列名取只读守卫关键字 DELETE，
// 采样 SQL 会被只读守卫拒绝 → 该表扫描失败，其余表正常产出。
func setupRetryTargetDB(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "target.db")
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatalf("打开目标库：%v", err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE USERS (ID INTEGER PRIMARY KEY, PHONE VARCHAR(20), NAME VARCHAR(20))`); err != nil {
		t.Fatalf("建表：%v", err)
	}
	if _, err := db.Exec(`CREATE TABLE ORDERS (ID INTEGER PRIMARY KEY, "DELETE" VARCHAR(20), AMOUNT INTEGER)`); err != nil {
		t.Fatalf("建表：%v", err)
	}
	for i := 0; i < 5; i++ {
		phone := "1380000800" + string(rune('0'+i))
		if _, err := db.Exec(`INSERT INTO USERS (ID,PHONE,NAME) VALUES (?,?,?)`, i+1, phone, "u"); err != nil {
			t.Fatalf("插数：%v", err)
		}
		if _, err := db.Exec(`INSERT INTO ORDERS (ID,"DELETE",AMOUNT) VALUES (?,?,?)`, i+1, phone, i*10); err != nil {
			t.Fatalf("插数：%v", err)
		}
	}
	return path
}

// insertSQLiteDataSource 写入一条指向 SQLite 目标库的数据源（凭据为真实密文）。
func insertSQLiteDataSource(t *testing.T, env *testEnv, id, target string) {
	t.Helper()
	dsnEnc, err := env.cipher.Encrypt(target, port.AlgAES256GCM)
	if err != nil {
		t.Fatalf("加密 DSN 失败：%v", err)
	}
	now := port.NowText()
	if _, ierr := env.store.DB().Exec(`INSERT INTO data_source
		(id,name,dialect,host_enc,port,service_name,username,password_enc,dsn_enc,include_schemas,exclude_tables,exclude_columns,
		 conn_max_open,conn_max_idle,conn_timeout_ms,query_timeout_ms,last_test_at,last_test_ok,enabled,remark,created_at,updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		id, "sqlite-target", "sqlite", "", 0, "", "ro_scan", "", dsnEnc,
		"", "", "", 1, 1, 10000, 30000, now, 1, 1, "", now, now); ierr != nil {
		t.Fatalf("写入测试数据源失败：%v", ierr)
	}
}

func TestRetryTaskEndpoint_部分失败可重试(t *testing.T) {
	env := newTestEnv(t)
	target := setupRetryTargetDB(t)
	insertSQLiteDataSource(t, env, "ds-retry", target)

	env.server.scanSvc = scan.NewService(
		env.store,
		func(d port.Dialect) (port.Driver, error) { return dsdriver.NewDriver(d) },
		func(f port.ReportFormat) (port.Reporter, error) { return reporter.New(f) },
		func() ([]port.Rule, string, error) { return rule.BuildRules("") },
		env.cipher.Decrypt,
		apptesting.NewNopLogger(), 20, 2000,
	)
	token := env.login(t)

	// 1) 发起扫描：ORDERS 表因只读守卫被拒而失败 → 任务为 partial。
	res, body := doRequest(t, env.handler, http.MethodPost, "/api/v1/tasks",
		token, map[string]any{"dataSourceId": "ds-retry"})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("发起扫描期望 HTTP 200，实际 %d（%s）", res.StatusCode, string(body))
	}
	resp := decodeResp(t, body)
	if resp.Code != CodeOK {
		t.Fatalf("发起扫描返回错误码 %d：%s", resp.Code, resp.Message)
	}
	data, _ := resp.Data.(map[string]any)
	taskID, _ := data["taskId"].(string)
	if taskID == "" {
		t.Fatalf("发起扫描未返回 taskId：%s", string(body))
	}

	res, body = doRequest(t, env.handler, http.MethodGet, "/api/v1/tasks/"+taskID, token, nil)
	resp = decodeResp(t, body)
	view, _ := resp.Data.(map[string]any)
	if view["status"] != "partial" {
		t.Fatalf("任务状态应为 partial，实际 %v（%s）", view["status"], string(body))
	}
	summary, _ := view["summary"].(map[string]any)
	if summary["failedCount"] != float64(1) {
		t.Fatalf("失败表数应为 1，实际 %v（%s）", summary["failedCount"], string(body))
	}
	// 控制台契约字段必须与后端原始计数同源，否则页面「命中列恒为 0」「重试按钮恒禁用」。
	if summary["totalTables"] != summary["tableCount"] || summary["totalColumns"] != summary["columnCount"] ||
		summary["hitColumns"] != summary["hitColumnCount"] || summary["failedTables"] != float64(1) {
		t.Fatalf("控制台汇总字段与后端计数不一致：%v（%s）", summary, string(body))
	}
	objects, _ := summary["failedObjects"].([]any)
	if len(objects) != 1 {
		t.Fatalf("详情应下发 1 个失败对象，实际 %v（%s）", objects, string(body))
	}
	failed, _ := objects[0].(map[string]any)
	failedID, _ := failed["id"].(string)
	if failedID == "" || failed["tableName"] != "ORDERS" || failed["reason"] == "" {
		t.Fatalf("失败对象缺少可重试标识或原因：%v（%s）", failed, string(body))
	}

	// 2) 台账只含成功表的字段。
	res, body = doRequest(t, env.handler, http.MethodGet, "/api/v1/findings?taskId="+taskID, token, nil)
	resp = decodeResp(t, body)
	page, _ := resp.Data.(map[string]any)
	items, _ := page["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("台账应仅含成功表 1 条，实际 %d 条（%s）", len(items), string(body))
	}

	// 3) 修复目标库后重试失败表。
	con, err := sql.Open("sqlite", "file:"+target)
	if err != nil {
		t.Fatalf("打开目标库：%v", err)
	}
	if _, err := con.Exec(`ALTER TABLE ORDERS RENAME COLUMN "DELETE" TO PHONE`); err != nil {
		t.Fatalf("重命名列失败：%v", err)
	}
	_ = con.Close()

	// 用控制台详情页下发的失败对象 id 定向重试，验证 targetIds 契约可直接闭环。
	res, body = doRequest(t, env.handler, http.MethodPost, "/api/v1/tasks/"+taskID+"/retry", token,
		map[string]any{"targetIds": []string{failedID}})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("重试期望 HTTP 200，实际 %d（%s）", res.StatusCode, string(body))
	}
	resp = decodeResp(t, body)
	if resp.Code != CodeOK {
		t.Fatalf("重试返回错误码 %d：%s", resp.Code, resp.Message)
	}
	rd, _ := resp.Data.(map[string]any)
	if rd["retried"] != float64(1) || rd["newFindings"] != float64(1) ||
		rd["remaining"] != float64(0) || rd["status"] != "done" {
		t.Fatalf("重试结果不符：%v（%s）", rd, string(body))
	}

	// 4) 重试后台账含两张表且不重复。
	res, body = doRequest(t, env.handler, http.MethodGet, "/api/v1/findings?taskId="+taskID, token, nil)
	resp = decodeResp(t, body)
	page, _ = resp.Data.(map[string]any)
	items, _ = page["items"].([]any)
	tables := map[string]int{}
	for _, it := range items {
		m, _ := it.(map[string]any)
		name, _ := m["tableName"].(string)
		tables[name]++
		if masked, _ := m["sampleMasked"].(string); masked == "13800008000" {
			t.Fatalf("台账样例泄露明文手机号：%v", masked)
		}
	}
	if tables["USERS"] != 1 || tables["ORDERS"] != 1 {
		t.Fatalf("台账应含两张表各一条，实际 %v", tables)
	}

	// 5) 重试后详情回到 done，失败清单清空、完成表数按断点统计为 2。
	res, body = doRequest(t, env.handler, http.MethodGet, "/api/v1/tasks/"+taskID, token, nil)
	resp = decodeResp(t, body)
	view, _ = resp.Data.(map[string]any)
	if view["status"] != "done" {
		t.Fatalf("重试后任务应为 done，实际 %v（%s）", view["status"], string(body))
	}
	summary, _ = view["summary"].(map[string]any)
	if len(objectsAfter(summary)) != 0 || summary["doneTables"] != float64(2) || summary["failedTables"] != float64(0) {
		t.Fatalf("重试后汇总不符：%v（%s）", summary, string(body))
	}
}

// objectsAfter 读取失败对象数组；omitempty 导致空清单字段缺省时按空数组处理。
func objectsAfter(summary map[string]any) []any {
	out, _ := summary["failedObjects"].([]any)
	return out
}

func TestRetryTaskEndpoint_状态与入参校验(t *testing.T) {
	env := newTestEnv(t)
	target := setupRetryTargetDB(t)
	insertSQLiteDataSource(t, env, "ds-retry2", target)
	env.server.scanSvc = scan.NewService(
		env.store,
		func(d port.Dialect) (port.Driver, error) { return dsdriver.NewDriver(d) },
		func(f port.ReportFormat) (port.Reporter, error) { return reporter.New(f) },
		func() ([]port.Rule, string, error) { return rule.BuildRules("") },
		env.cipher.Decrypt,
		apptesting.NewNopLogger(), 20, 2000,
	)
	token := env.login(t)

	// 任务不存在 → 404/1004。
	res, body := doRequest(t, env.handler, http.MethodPost, "/api/v1/tasks/not-exist/retry", token, map[string]any{})
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("不存在的任务期望 HTTP 404，实际 %d（%s）", res.StatusCode, string(body))
	}
	if resp := decodeResp(t, body); resp.Code != CodeNotFound {
		t.Fatalf("不存在的任务期望 code=%d，实际 %d", CodeNotFound, resp.Code)
	}

	// 未鉴权 → 401/1002。
	res, body = doRequest(t, env.handler, http.MethodPost, "/api/v1/tasks/x/retry", "", map[string]any{})
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("未鉴权期望 HTTP 401，实际 %d（%s）", res.StatusCode, string(body))
	}
	if resp := decodeResp(t, body); resp.Code != CodeUnauthorized {
		t.Fatalf("未鉴权期望 code=%d，实际 %d", CodeUnauthorized, resp.Code)
	}

	// 扫描成功（无失败表）→ 重试报「没有失败表可重试」→ 404/1004。
	res, body = doRequest(t, env.handler, http.MethodPost, "/api/v1/tasks", token,
		map[string]any{"dataSourceId": "ds-retry2"})
	resp := decodeResp(t, body)
	data, _ := resp.Data.(map[string]any)
	taskID, _ := data["taskId"].(string)
	// 该目标库存在失败表，先置为完成态以验证「无失败表」分支。
	if err := env.store.UpdateTaskStatus(t.Context(), taskID, port.TaskStatusDone, port.TaskSummary{}); err != nil {
		t.Fatalf("更新任务状态失败：%v", err)
	}
	res, body = doRequest(t, env.handler, http.MethodPost, "/api/v1/tasks/"+taskID+"/retry", token, map[string]any{})
	if res.StatusCode != http.StatusConflict {
		t.Fatalf("已完成任务重试期望 HTTP 409，实际 %d（%s）", res.StatusCode, string(body))
	}
	if resp := decodeResp(t, body); resp.Code != CodeTaskConflict {
		t.Fatalf("已完成任务重试期望 code=%d，实际 %d", CodeTaskConflict, resp.Code)
	}
}
