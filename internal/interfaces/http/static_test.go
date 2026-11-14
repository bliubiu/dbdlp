package httpapi

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dbdlp/internal/domain/port"
)

// 本文件覆盖静态资源同端口托管与 SPA 回落。

// newStaticEnv 构造带静态目录的测试环境。
func newStaticEnv(t *testing.T, staticDir string) *testEnv {
	t.Helper()
	env := newTestEnv(t)
	srv, err := newServer(Deps{
		Store:         env.store,
		Cipher:        env.cipher,
		SessionTTLMin: 60,
		StaticDir:     staticDir,
		ConfigEcho:    ConfigEcho{LogLevel: "INFO"},
	})
	if err != nil {
		t.Fatalf("构造 API 处理器失败：%v", err)
	}
	env.handler = srv.routes()
	env.server = srv
	return env
}

// TestStaticServingAndSPAFallback 校验静态目录存在时的首页、SPA 回落与 API 优先。
func TestStaticServingAndSPAFallback(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>dbdlp 控制台</html>"), 0o600); err != nil {
		t.Fatalf("写入首页失败：%v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "asset.txt"), []byte("静态资源"), 0o600); err != nil {
		t.Fatalf("写入静态文件失败：%v", err)
	}
	env := newStaticEnv(t, dir)

	// 首页
	res, body := doRequest(t, env.handler, http.MethodGet, "/", "", nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("首页期望 HTTP 200，实际 %d", res.StatusCode)
	}
	if !strings.Contains(string(body), "dbdlp 控制台") {
		t.Fatalf("首页内容不符：%s", string(body))
	}

	// 静态资源
	res, body = doRequest(t, env.handler, http.MethodGet, "/asset.txt", "", nil)
	if res.StatusCode != http.StatusOK || !strings.Contains(string(body), "静态资源") {
		t.Fatalf("静态资源期望 200 且内容匹配，实际 %d：%s", res.StatusCode, string(body))
	}

	// SPA 回落：未知路径返回首页
	res, body = doRequest(t, env.handler, http.MethodGet, "/datasources/import", "", nil)
	if res.StatusCode != http.StatusOK || !strings.Contains(string(body), "dbdlp 控制台") {
		t.Fatalf("SPA 回落期望 200 且返回首页，实际 %d：%s", res.StatusCode, string(body))
	}

	// API 优先于静态资源
	token := env.login(t)
	res, body = doRequest(t, env.handler, http.MethodGet, "/api/v1/config", token, nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("配置端点期望 HTTP 200，实际 %d", res.StatusCode)
	}
	resp := decodeResp(t, body)
	if resp.Code != CodeOK {
		t.Fatalf("配置端点期望 code=0，实际 %d", resp.Code)
	}
}

// TestStaticDirMissingReturns404 校验静态目录不存在时返回 404 且不报错。
func TestStaticDirMissingReturns404(t *testing.T) {
	env := newStaticEnv(t, filepath.Join(t.TempDir(), "not-exist"))
	res, _ := doRequest(t, env.handler, http.MethodGet, "/", "", nil)
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("静态目录缺失期望 HTTP 404，实际 %d", res.StatusCode)
	}
	// API 仍应可用
	token := env.login(t)
	res, body := doRequest(t, env.handler, http.MethodGet, "/api/v1/config", token, nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("静态目录缺失时 API 期望 HTTP 200，实际 %d（%s）", res.StatusCode, string(body))
	}
}

// TestStaticPathTraversalBlocked 校验目录穿越被阻断。
func TestStaticPathTraversalBlocked(t *testing.T) {
	root := t.TempDir()
	staticDir := filepath.Join(root, "web")
	if err := os.MkdirAll(staticDir, 0o700); err != nil {
		t.Fatalf("创建静态目录失败：%v", err)
	}
	if err := os.WriteFile(filepath.Join(staticDir, "index.html"), []byte("<html>首页</html>"), 0o600); err != nil {
		t.Fatalf("写入首页失败：%v", err)
	}
	secret := filepath.Join(root, "secret.txt")
	if err := os.WriteFile(secret, []byte("机密内容"), 0o600); err != nil {
		t.Fatalf("写入机密文件失败：%v", err)
	}
	env := newStaticEnv(t, staticDir)
	res, body := doRequest(t, env.handler, http.MethodGet, "/../secret.txt", "", nil)
	if strings.Contains(string(body), "机密内容") {
		t.Fatalf("目录穿越被穿透，响应含机密内容：%s", string(body))
	}
	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusNotFound && res.StatusCode != http.StatusTemporaryRedirect {
		t.Fatalf("目录穿越期望 404／回落首页／路径规范化重定向，实际 %d", res.StatusCode)
	}
	// 路径解析函数须直接阻断越界路径
	if _, ok := env.server.safeStaticPath("../secret.txt"); ok {
		t.Fatal("safeStaticPath 未阻断越界路径")
	}
	_ = port.NowText
}
