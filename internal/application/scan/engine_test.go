package scan

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	apptesting "dbdlp/internal/application/testing"
	"dbdlp/internal/domain/port"
	"dbdlp/internal/domain/rule"
	dsdriver "dbdlp/internal/infrastructure/datasource"
	"dbdlp/internal/infrastructure/reporter"
	"dbdlp/internal/infrastructure/store"

	_ "modernc.org/sqlite"
)

// setupTargetDB 创建一个含敏感数据的 SQLite 目标库，返回文件路径。
func setupTargetDB(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "target.db")
	dsn := "file:" + path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("打开目标库：%v", err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE USERS (ID INTEGER PRIMARY KEY, PHONE VARCHAR(20), NAME VARCHAR(20))`); err != nil {
		t.Fatalf("建表：%v", err)
	}
	for i := 0; i < 10; i++ {
		phone := "1380000800" + string(rune('0'+i)) // 13800008000 ~ 13800008009
		if _, err := db.Exec(`INSERT INTO USERS (ID, PHONE, NAME) VALUES (?,?,?)`, i+1, phone, "用户"+string(rune('0'+i))); err != nil {
			t.Fatalf("插数：%v", err)
		}
	}
	return path
}

func newEngineService(t *testing.T) (*Service, *store.SQLiteStore) {
	t.Helper()
	st, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "meta.db"))
	if err != nil {
		t.Fatalf("打开元数据库：%v", err)
	}
	svc := NewService(
		st,
		func(d port.Dialect) (port.Driver, error) { return dsdriver.NewDriver(d) },
		func(f port.ReportFormat) (port.Reporter, error) { return reporter.New(f) },
		func() ([]port.Rule, string, error) { return rule.BuildRules("") },
		func(s string) (string, error) { return s, nil }, // 明文配置，无需解密
		apptesting.NewNopLogger(), 20, 2000,
	)
	return svc, st
}

func TestEngineEndToEnd(t *testing.T) {
	ctx := context.Background()
	target := setupTargetDB(t)
	svc, st := newEngineService(t)
	defer st.Close()

	cfg := port.DataSourceConfig{ID: "ds-test", Name: "target", Dialect: port.DialectSQLite, DSN: target}

	// 第一次扫描：导出 Excel 报告
	taskID, err := svc.RunConfig(ctx, cfg, port.ScanScope{}, string(port.FormatExcel), t.TempDir())
	if err != nil {
		t.Fatalf("RunConfig(excel)：%v", err)
	}

	// 台账落库校验
	page, err := st.ListFindings(ctx, port.FindingFilter{TaskID: taskID})
	if err != nil {
		t.Fatalf("ListFindings：%v", err)
	}
	if page.Total != 1 {
		t.Fatalf("预期 1 条敏感字段台账，实际 %d", page.Total)
	}
	f := page.Items[0]
	if f.ColumnName != "PHONE" || f.TableName != "USERS" {
		t.Fatalf("台账字段不匹配：%+v", f)
	}
	if f.SampleTotal != 10 || f.SampleHit != 10 {
		t.Fatalf("采样统计不匹配：total=%d hit=%d", f.SampleTotal, f.SampleHit)
	}
	if f.HitRatio != 1.0 {
		t.Fatalf("命中率应为 1.0，实际 %v", f.HitRatio)
	}
	if !f.VerifyPassed {
		t.Fatalf("应验证通过")
	}
	if f.RiskLevel != "high" {
		t.Fatalf("风险应为 high（高敏感+验证通过+明文），实际 %v", f.RiskLevel)
	}
	if f.Confidence != "high" {
		t.Fatalf("置信度应为 high（列名吻合+验证通过+命中率≥0.5），实际 %v", f.Confidence)
	}
	if !strings.Contains(f.SampleMasked, "138****8000") {
		t.Fatalf("脱敏样例不匹配：%q", f.SampleMasked)
	}
	if strings.Contains(f.SampleMasked, "13800008000") {
		t.Fatalf("脱敏样例泄露明文：%q", f.SampleMasked)
	}

	// 命中记录脱敏校验
	hits, err := st.ListHits(ctx, f.ID, port.Pagination{Page: 1, Limit: 50})
	if err != nil {
		t.Fatalf("ListHits：%v", err)
	}
	if hits.Total != 10 || hits.Items[0].MaskedValue != "138****8000" {
		t.Fatalf("命中记录不匹配：%+v", hits)
	}

	// 第二次扫描：导出 JSON 报告，验证仅含脱敏数据
	jsonDir := t.TempDir()
	if _, err := svc.RunConfig(ctx, cfg, port.ScanScope{}, string(port.FormatJSON), jsonDir); err != nil {
		t.Fatalf("RunConfig(json)：%v", err)
	}
	entries, _ := os.ReadDir(jsonDir)
	foundJSON := false
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".json") {
			foundJSON = true
			data, _ := os.ReadFile(filepath.Join(jsonDir, e.Name()))
			if strings.Contains(string(data), "13800008000") {
				t.Fatalf("JSON 报告泄露明文手机号原值")
			}
			if !strings.Contains(string(data), "138****8000") {
				t.Fatalf("JSON 报告未包含脱敏样例")
			}
		}
	}
	if !foundJSON {
		t.Fatalf("未生成 JSON 报告")
	}
}
