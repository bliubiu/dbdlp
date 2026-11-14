package httpapi

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"dbdlp/internal/domain/port"
	"dbdlp/internal/infrastructure/crypto"
	"dbdlp/internal/infrastructure/store"
	"golang.org/x/crypto/bcrypt"
)

// 本文件提供 API 层测试夹具与端到端用例。

// testAdminPassword 测试用管理员口令。
const testAdminPassword = "Test@12345"

// testEnv 测试环境：真实 SQLite 存储 + 真实加密器（避免自造桩件掩盖真实行为）。
type testEnv struct {
	handler http.Handler
	server  *Server
	store   *store.SQLiteStore
	cipher  port.Cipher
}

// newTestEnv 构造测试环境。
func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	dir := t.TempDir()
	st, err := store.NewSQLiteStore(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("创建测试存储失败：%v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	hash, herr := bcrypt.GenerateFromPassword([]byte(testAdminPassword), bcrypt.MinCost)
	if herr != nil {
		t.Fatalf("生成测试口令哈希失败：%v", herr)
	}
	now := port.NowText()
	if _, ierr := st.DB().Exec(`INSERT INTO web_user (id,username,password_hash,display_name,role,enabled,created_at,updated_at)
		VALUES (?,?,?,?,?,1,?,?)`, "u-admin", "admin", string(hash), "系统管理员", "admin", now, now); ierr != nil {
		t.Fatalf("写入测试账号失败：%v", ierr)
	}

	key := make([]byte, 32)
	if _, rerr := rand.Read(key); rerr != nil {
		t.Fatalf("生成测试密钥失败：%v", rerr)
	}
	c, cerr := crypto.NewCipher(key, port.AlgAES256GCM)
	if cerr != nil {
		t.Fatalf("构建测试加密器失败：%v", cerr)
	}
	srv, nerr := newServer(Deps{
		Store:         st,
		Cipher:        c,
		SessionTTLMin: 60,
		ConfigEcho: ConfigEcho{
			LogLevel: "INFO", SampleRows: 20, StorageDriver: "sqlite",
			CryptoAlgorithm: "aes256gcm", RetentionDays: 32,
		},
	})
	if nerr != nil {
		t.Fatalf("构造 API 处理器失败：%v", nerr)
	}
	return &testEnv{handler: srv.routes(), server: srv, store: st, cipher: c}
}

// insertDataSource 写入一条数据源（凭据为真实密文）。
func (e *testEnv) insertDataSource(t *testing.T, id, name, host string) {
	t.Helper()
	hostEnc, err := e.cipher.Encrypt(host, port.AlgAES256GCM)
	if err != nil {
		t.Fatalf("加密主机失败：%v", err)
	}
	passEnc, err := e.cipher.Encrypt("Secret@123", port.AlgAES256GCM)
	if err != nil {
		t.Fatalf("加密口令失败：%v", err)
	}
	now := port.NowText()
	if _, ierr := e.store.DB().Exec(`INSERT INTO data_source
		(id,name,dialect,host_enc,port,service_name,username,password_enc,dsn_enc,include_schemas,exclude_tables,exclude_columns,
		 conn_max_open,conn_max_idle,conn_timeout_ms,query_timeout_ms,last_test_at,last_test_ok,enabled,remark,created_at,updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		id, name, "oracle", hostEnc, 1521, "ORCL", "ro_scan", passEnc, "",
		"", "", "", 2, 1, 10000, 30000, now, 1, 1, "", now, now); ierr != nil {
		t.Fatalf("写入测试数据源失败：%v", ierr)
	}
}

// newRequest 构造测试请求。
func newRequest(method, path string) *http.Request {
	return httptest.NewRequest(method, path, nil)
}

// newRecorder 构造响应记录器。
func newRecorder() *httptest.ResponseRecorder {
	return httptest.NewRecorder()
}

// decodeResp 解析统一响应体。
func decodeResp(t *testing.T, body []byte) Resp {
	t.Helper()
	var resp Resp
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("解析响应体失败：%v（内容：%s）", err, string(body))
	}
	return resp
}

// doRequest 发起一次请求并返回响应与响应体。
func doRequest(t *testing.T, h http.Handler, method, path, token string, body any) (*http.Response, []byte) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("序列化请求体失败：%v", err)
		}
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	res := rec.Result()
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("读取响应体失败：%v", err)
	}
	return res, raw
}

// login 取会话令牌。
func (e *testEnv) login(t *testing.T) string {
	t.Helper()
	res, body := doRequest(t, e.handler, http.MethodPost, "/api/v1/auth/login", "",
		map[string]string{"username": "admin", "password": testAdminPassword})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("登录失败：HTTP %d，响应：%s", res.StatusCode, string(body))
	}
	resp := decodeResp(t, body)
	if resp.Code != CodeOK {
		t.Fatalf("登录返回错误码 %d：%s", resp.Code, resp.Message)
	}
	data, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("登录响应 data 结构非法：%v", resp.Data)
	}
	token, _ := data["token"].(string)
	if token == "" {
		t.Fatal("登录响应缺少 token")
	}
	return token
}

// TestEndToEndLoginAndList 端到端：登录取 token → 带 token 查列表 → 不带 token 返回 1002。
func TestEndToEndLoginAndList(t *testing.T) {
	env := newTestEnv(t)
	env.insertDataSource(t, "ds-1", "ORA-PROD-01", "192.168.10.3")
	srv := httptest.NewServer(env.handler)
	defer srv.Close()

	// 1. 登录取 token
	loginResp, err := http.Post(srv.URL+"/api/v1/auth/login", "application/json; charset=utf-8",
		bytes.NewReader([]byte(`{"username":"admin","password":"`+testAdminPassword+`"}`)))
	if err != nil {
		t.Fatalf("登录请求失败：%v", err)
	}
	defer loginResp.Body.Close()
	loginBody, _ := io.ReadAll(loginResp.Body)
	if loginResp.StatusCode != http.StatusOK {
		t.Fatalf("登录 HTTP 状态码异常：%d（响应：%s）", loginResp.StatusCode, string(loginBody))
	}
	loginParsed := decodeResp(t, loginBody)
	if loginParsed.Code != CodeOK {
		t.Fatalf("登录 code 期望 0，实际 %d", loginParsed.Code)
	}
	loginData, _ := loginParsed.Data.(map[string]any)
	token, _ := loginData["token"].(string)
	if token == "" {
		t.Fatal("登录未返回 token")
	}

	// 2. 带 token 查询数据源列表
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/datasources?page=1&limit=20", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	listResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("列表请求失败：%v", err)
	}
	defer listResp.Body.Close()
	listBody, _ := io.ReadAll(listResp.Body)
	parsed := decodeResp(t, listBody)
	if parsed.Code != CodeOK {
		t.Fatalf("带 token 列表期望 code=0，实际 %d（%s）", parsed.Code, string(listBody))
	}
	page, ok := parsed.Data.(map[string]any)
	if !ok {
		t.Fatalf("列表 data 结构非法：%v", parsed.Data)
	}
	for _, key := range []string{"items", "total", "page", "limit", "hasMore"} {
		if _, exist := page[key]; !exist {
			t.Fatalf("分页 data 缺少字段 %s：%v", key, parsed.Data)
		}
	}
	items, ok := page["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("列表 items 期望 1 条，实际 %v", page["items"])
	}

	// 3. 不带 token 访问受保护端点
	noAuthResp, err := http.Get(srv.URL + "/api/v1/datasources")
	if err != nil {
		t.Fatalf("未认证请求失败：%v", err)
	}
	defer noAuthResp.Body.Close()
	noAuthBody, _ := io.ReadAll(noAuthResp.Body)
	noAuth := decodeResp(t, noAuthBody)
	if noAuth.Code != CodeUnauthorized {
		t.Fatalf("未认证期望 code=%d，实际 %d", CodeUnauthorized, noAuth.Code)
	}
	if noAuthResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("未认证期望 HTTP 401，实际 %d", noAuthResp.StatusCode)
	}
}
