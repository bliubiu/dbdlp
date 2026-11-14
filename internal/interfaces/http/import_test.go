package httpapi

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"testing"

	"dbdlp/internal/application/datasource"
	"dbdlp/internal/domain/port"
	"github.com/xuri/excelize/v2"
)

// 本文件覆盖批量导入：逐条门禁 + 部分成功 + 不整批回滚。

// fakeConn 伪只读连接，仅支持 Ping。
type fakeConn struct{}

// Ping 返回成功。
func (f *fakeConn) Ping(context.Context) error { return nil }

// Query 不支持。
func (f *fakeConn) Query(context.Context, string, ...any) (port.Rows, error) {
	return nil, errors.New("测试桩不支持查询")
}

// Close 返回成功。
func (f *fakeConn) Close() error { return nil }

// fakeDriver 伪驱动：连接与 Ping 均成功。
type fakeDriver struct{ d port.Dialect }

// Dialect 返回方言。
func (f *fakeDriver) Dialect() port.Dialect { return f.d }

// Connect 返回伪连接。
func (f *fakeDriver) Connect(context.Context, port.DataSourceConfig) (port.ReadOnlyConn, error) {
	return &fakeConn{}, nil
}

// ListSchemas 返回空。
func (f *fakeDriver) ListSchemas(context.Context, port.ReadOnlyConn) ([]port.Schema, error) {
	return nil, nil
}

// ListColumns 返回空。
func (f *fakeDriver) ListColumns(context.Context, port.ReadOnlyConn, string) ([]port.ColumnMeta, error) {
	return nil, nil
}

// TableStats 返回空。
func (f *fakeDriver) TableStats(context.Context, port.ReadOnlyConn, string, string) (port.TableStats, error) {
	return port.TableStats{}, nil
}

// SampleRows 返回空批次。
func (f *fakeDriver) SampleRows(context.Context, port.ReadOnlyConn, port.SampleQuery) (port.RowBatch, error) {
	return port.RowBatch{}, nil
}

// Placeholder 返回占位符。
func (f *fakeDriver) Placeholder(int) string { return "?" }

// QuoteIdentifier 返回标识符。
func (f *fakeDriver) QuoteIdentifier(name string) string { return name }

// newImportEnv 构造带伪驱动的导入测试环境。
func newImportEnv(t *testing.T) *testEnv {
	t.Helper()
	env := newTestEnv(t)
	driverFactory := func(d port.Dialect) (port.Driver, error) {
		if !dialectAllowed(string(d)) {
			return nil, errors.New("方言不支持：" + string(d))
		}
		return &fakeDriver{d: d}, nil
	}
	dsSvc := datasource.NewService(env.store, driverFactory, env.cipher, port.AlgAES256GCM, nil)
	srv, err := newServer(Deps{
		Store:         env.store,
		Cipher:        env.cipher,
		DsSvc:         dsSvc,
		SessionTTLMin: 60,
		ConfigEcho:    ConfigEcho{LogLevel: "INFO", CryptoAlgorithm: "aes256gcm"},
	})
	if err != nil {
		t.Fatalf("构造 API 处理器失败：%v", err)
	}
	env.handler = srv.routes()
	env.server = srv
	return env
}

// buildExcelImportFile 生成导入用 Excel（首行表头）。
func buildExcelImportFile(t *testing.T, rows [][]any) []byte {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
	const sheet = "数据源"
	if _, err := f.NewSheet(sheet); err != nil {
		t.Fatalf("创建工作表失败：%v", err)
	}
	f.DeleteSheet("Sheet1")
	for i, col := range templateColumns {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue(sheet, cell, col); err != nil {
			t.Fatalf("写表头失败：%v", err)
		}
	}
	for r, row := range rows {
		for c, v := range row {
			cell, _ := excelize.CoordinatesToCellName(c+1, r+2)
			if err := f.SetCellValue(sheet, cell, v); err != nil {
				t.Fatalf("写数据失败：%v", err)
			}
		}
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatalf("生成 Excel 失败：%v", err)
	}
	return buf.Bytes()
}

// postMultipart 上传文件并返回响应体。
func postMultipart(t *testing.T, h http.Handler, path, token, field, filename string, content []byte) (int, []byte) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile(field, filename)
	if err != nil {
		t.Fatalf("创建表单文件失败：%v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("写表单文件失败：%v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("关闭表单失败：%v", err)
	}
	req := newRequest(http.MethodPost, path)
	req.Header.Set("Content-Type", w.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Body = http.NoBody
	req.Body = io.NopCloser(&buf)
	rec := newRecorder()
	h.ServeHTTP(rec, req)
	res := rec.Result()
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	return res.StatusCode, raw
}

// TestImportPartialSuccess 校验逐条门禁：成功行入库、失败行不入库、逐行中文回显。
func TestImportPartialSuccess(t *testing.T) {
	env := newImportEnv(t)
	token := env.login(t)
	content := buildExcelImportFile(t, [][]any{
		{"ORA-PROD-01", "oracle", "db01.intra", 1521, "ORCL", "ro_scan", "Secret@1", "", "", "", "", ""},
		{"BAD-DIALECT", "notadb", "db02.intra", 1521, "ORCL", "ro_scan", "Secret@1", "", "", "", "", ""},
		{"ORA-PROD-02", "oracle", "db03.intra", 1521, "ORCL", "ro_scan", "Secret@1", "", "", "", "", ""},
	})
	status, body := postMultipart(t, env.handler, "/api/v1/datasources/import", token, "file",
		"datasources.xlsx", content)
	if status != http.StatusOK {
		t.Fatalf("导入期望 HTTP 200，实际 %d（%s）", status, string(body))
	}
	resp := decodeResp(t, body)
	if resp.Code != CodeOK {
		t.Fatalf("部分成功期望 code=%d，实际 %d（%s）", CodeOK, resp.Code, string(body))
	}
	data, _ := resp.Data.(map[string]any)
	total, _ := data["total"].(float64)
	succeeded, _ := data["succeeded"].(float64)
	failed, _ := data["failed"].(float64)
	if total != 3 || succeeded != 2 || failed != 1 {
		t.Fatalf("期望 total=3 succeeded=2 failed=1，实际 %v", data)
	}
	if int(total) != int(succeeded)+int(failed) {
		t.Fatalf("total 必须等于 succeeded+failed，实际 %v", data)
	}
	errs, _ := data["errors"].([]any)
	if len(errs) != 1 {
		t.Fatalf("期望 1 条错误明细，实际 %d", len(errs))
	}
	item, _ := errs[0].(map[string]any)
	row, _ := item["row"].(float64)
	if int(row) != 3 {
		t.Fatalf("失败行号期望 3（表头占第 1 行），实际 %v", item["row"])
	}
	msg, _ := item["message"].(string)
	if msg == "" {
		t.Fatal("失败行缺少中文原因")
	}
	// 成功行应已落库，失败行不得落库
	_, lbody := doRequest(t, env.handler, http.MethodGet, "/api/v1/datasources", token, nil)
	lresp := decodeResp(t, lbody)
	page, _ := lresp.Data.(map[string]any)
	lt, _ := page["total"].(float64)
	if int(lt) != 2 {
		t.Fatalf("期望落库 2 条，实际 %v（%s）", page["total"], string(lbody))
	}
}

// TestImportIdempotentSkip 校验重复导入按实例标识幂等去重。
func TestImportIdempotentSkip(t *testing.T) {
	env := newImportEnv(t)
	token := env.login(t)
	row := []any{"ORA-PROD-01", "oracle", "db01.intra", 1521, "ORCL", "ro_scan", "Secret@1", "", "", "", "", ""}
	content := buildExcelImportFile(t, [][]any{row})
	status, body := postMultipart(t, env.handler, "/api/v1/datasources/import", token, "file",
		"datasources.xlsx", content)
	if status != http.StatusOK {
		t.Fatalf("首次导入期望 HTTP 200，实际 %d（%s）", status, string(body))
	}
	// 再次导入同一文件：应幂等跳过，不产生重复记录
	_, body = postMultipart(t, env.handler, "/api/v1/datasources/import", token, "file",
		"datasources.xlsx", content)
	resp := decodeResp(t, body)
	data, _ := resp.Data.(map[string]any)
	succeeded, _ := data["succeeded"].(float64)
	failed, _ := data["failed"].(float64)
	if int(succeeded) != 1 || int(failed) != 0 {
		t.Fatalf("重复导入期望 succeeded=1 failed=0，实际 %v", data)
	}
	_, lbody := doRequest(t, env.handler, http.MethodGet, "/api/v1/datasources", token, nil)
	lresp := decodeResp(t, lbody)
	page, _ := lresp.Data.(map[string]any)
	if total, _ := page["total"].(float64); int(total) != 1 {
		t.Fatalf("重复导入后期望库内 1 条，实际 %v", page["total"])
	}
}

// TestImportAllFailed 校验全批失败时零写入并返回 2001。
func TestImportAllFailed(t *testing.T) {
	env := newImportEnv(t)
	token := env.login(t)
	content := buildExcelImportFile(t, [][]any{
		{"BAD-01", "notadb", "db01.intra", 1521, "ORCL", "ro_scan", "Secret@1", "", "", "", "", ""},
	})
	status, body := postMultipart(t, env.handler, "/api/v1/datasources/import", token, "file",
		"datasources.xlsx", content)
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("全批失败期望 HTTP 422，实际 %d（%s）", status, string(body))
	}
	resp := decodeResp(t, body)
	if resp.Code != CodeConnTestFail {
		t.Fatalf("全批失败期望 code=%d，实际 %d", CodeConnTestFail, resp.Code)
	}
	data, _ := resp.Data.(map[string]any)
	if succeeded, _ := data["succeeded"].(float64); int(succeeded) != 0 {
		t.Fatalf("全批失败期望 succeeded=0，实际 %v", data)
	}
	_, lbody := doRequest(t, env.handler, http.MethodGet, "/api/v1/datasources", token, nil)
	lresp := decodeResp(t, lbody)
	page, _ := lresp.Data.(map[string]any)
	if total, _ := page["total"].(float64); int(total) != 0 {
		t.Fatalf("全批失败期望零写入，实际 %v", page["total"])
	}
}

// TestImportTemplateDownload 校验 Excel 与 YAML 模板可下载。
func TestImportTemplateDownload(t *testing.T) {
	env := newTestEnv(t)
	token := env.login(t)

	res, body := doRequest(t, env.handler, http.MethodGet, "/api/v1/datasources/import/template?format=excel", token, nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("Excel 模板期望 HTTP 200，实际 %d", res.StatusCode)
	}
	if len(body) < 4 || string(body[:2]) != "PK" {
		t.Fatalf("Excel 模板内容不是合法 xlsx（前两字节 %q）", string(body[:min(2, len(body))]))
	}

	res, body = doRequest(t, env.handler, http.MethodGet, "/api/v1/datasources/import/template?format=yaml", token, nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("YAML 模板期望 HTTP 200，实际 %d", res.StatusCode)
	}
	if !bytes.Contains(body, []byte("datasources:")) {
		t.Fatalf("YAML 模板缺少 datasources 列表：%s", string(body))
	}

	// 模板不得包含任何真实凭据
	if bytes.Contains(body, []byte("Secret@")) {
		t.Fatal("模板中不得出现凭据值")
	}
}
