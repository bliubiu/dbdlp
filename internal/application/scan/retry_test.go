package scan

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	apptesting "dbdlp/internal/application/testing"
	"dbdlp/internal/domain/port"
	"dbdlp/internal/domain/rule"
	dsdriver "dbdlp/internal/infrastructure/datasource"
	"dbdlp/internal/infrastructure/reporter"
	"dbdlp/internal/infrastructure/store"

	_ "modernc.org/sqlite"
)

// 本文件覆盖 docs/07 IT-06：单表失败不阻断整体，失败表可单独重试。

// flakyDriver 包装真实驱动：对指定表的前 failCount 次采样返回错误，用于模拟目标库瞬时故障。
type flakyDriver struct {
	port.Driver
	failTable string
	failCount int
	calls     int
}

// SampleRows 在命中失败条件时返回模拟错误。
func (d *flakyDriver) SampleRows(ctx context.Context, conn port.ReadOnlyConn, q port.SampleQuery) (port.RowBatch, error) {
	if q.Table == d.failTable && d.calls < d.failCount {
		d.calls++
		return port.RowBatch{}, errors.New("模拟采样超时")
	}
	return d.Driver.SampleRows(ctx, conn, q)
}

// setupTwoTableDB 建两个含敏感数据的表：USERS 与 ORDERS。
func setupTwoTableDB(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "target.db")
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatalf("打开目标库：%v", err)
	}
	defer db.Close()
	stmts := []string{
		`CREATE TABLE USERS (ID INTEGER PRIMARY KEY, PHONE VARCHAR(20), NAME VARCHAR(20))`,
		`CREATE TABLE ORDERS (ID INTEGER PRIMARY KEY, PHONE VARCHAR(20), AMOUNT INTEGER)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("建表：%v", err)
		}
	}
	for i := 0; i < 5; i++ {
		phone := "1380000800" + string(rune('0'+i))
		if _, err := db.Exec(`INSERT INTO USERS (ID, PHONE, NAME) VALUES (?,?,?)`, i+1, phone, "用户"); err != nil {
			t.Fatalf("插数：%v", err)
		}
		if _, err := db.Exec(`INSERT INTO ORDERS (ID, PHONE, AMOUNT) VALUES (?,?,?)`, i+1, phone, i*10); err != nil {
			t.Fatalf("插数：%v", err)
		}
	}
	return path
}

// newFlakyService 构造使用 flaky 驱动的扫描服务。
func newFlakyService(t *testing.T, flaky *flakyDriver) (*Service, *store.SQLiteStore) {
	t.Helper()
	st, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "meta.db"))
	if err != nil {
		t.Fatalf("打开元数据库：%v", err)
	}
	svc := NewService(
		st,
		func(d port.Dialect) (port.Driver, error) { return flaky, nil },
		func(f port.ReportFormat) (port.Reporter, error) { return reporter.New(f) },
		func() ([]port.Rule, string, error) { return rule.BuildRules("") },
		func(s string) (string, error) { return s, nil },
		apptesting.NewNopLogger(), 20, 2000,
	)
	return svc, st
}

// seedDataSource 在元数据库中登记数据源（测试的解密函数为恒等函数，故密文字段直接写明文）。
func seedDataSource(t *testing.T, st *store.SQLiteStore, id, dsn string) {
	t.Helper()
	err := st.CreateDataSource(context.Background(), port.DataSource{
		ID: id, Name: "target", Dialect: port.DialectSQLite,
		DSNEnc: dsn, Enabled: true, QueryTimeoutMs: 30000, ConnMaxOpen: 1,
	})
	if err != nil {
		t.Fatalf("登记数据源：%v", err)
	}
}

// findingTables 返回台账中的「表/字段」集合。
func findingTables(t *testing.T, st *store.SQLiteStore, taskID string) map[string]int {
	t.Helper()
	page, err := st.ListFindings(context.Background(), port.FindingFilter{TaskID: taskID, Page: 1, Limit: 100})
	if err != nil {
		t.Fatalf("ListFindings：%v", err)
	}
	out := map[string]int{}
	for _, f := range page.Items {
		out[f.TableName+"/"+f.ColumnName]++
	}
	if page.Total != int64(len(page.Items)) {
		t.Fatalf("台账条数与总数不一致：total=%d items=%d", page.Total, len(page.Items))
	}
	return out
}

func TestIT06_单表失败不阻断且失败表可重试(t *testing.T) {
	ctx := context.Background()
	target := setupTwoTableDB(t)
	base, err := dsdriver.NewDriver(port.DialectSQLite)
	if err != nil {
		t.Fatalf("构造驱动：%v", err)
	}
	flaky := &flakyDriver{Driver: base, failTable: "ORDERS", failCount: 1}
	svc, st := newFlakyService(t, flaky)
	defer st.Close()

	seedDataSource(t, st, "ds-it06", target)
	taskID, err := svc.Run(ctx, "ds-it06", "", "")
	if err != nil {
		t.Fatalf("RunConfig：%v", err)
	}

	// 1) 任务状态为部分失败，失败表数为 1。
	task, err := st.GetTask(ctx, taskID)
	if err != nil {
		t.Fatalf("GetTask：%v", err)
	}
	if task.Status != port.TaskStatusPartial {
		t.Fatalf("任务状态应为 partial，实际 %s", task.Status)
	}
	var summary port.TaskSummary
	if err := json.Unmarshal([]byte(task.Summary), &summary); err != nil {
		t.Fatalf("解析汇总：%v", err)
	}
	if summary.FailedCount != 1 {
		t.Fatalf("失败表数应为 1，实际 %d", summary.FailedCount)
	}

	// 2) 失败表不产生台账，其余表正常产出。
	got := findingTables(t, st, taskID)
	if len(got) != 1 || got["USERS/PHONE"] != 1 {
		t.Fatalf("台账应仅含 USERS/PHONE 一条，实际 %v", got)
	}

	// 3) 失败表留存 failed 断点。
	cps, err := st.ListCheckpoints(ctx, taskID)
	if err != nil {
		t.Fatalf("ListCheckpoints：%v", err)
	}
	failedKeys := map[string]bool{}
	for _, cp := range cps {
		if cp.State == checkpointStateFailed {
			failedKeys[cp.TableMetaID] = true
		}
	}
	if !failedKeys["main/ORDERS"] {
		t.Fatalf("应存在 main/ORDERS 的失败断点，实际断点 %v", failedKeys)
	}

	// 4) 重试失败表：仅重扫 ORDERS，补齐台账且不重复。
	res, err := svc.RetryFailed(ctx, taskID, "", "")
	if err != nil {
		t.Fatalf("RetryFailed：%v", err)
	}
	if res.Retried != 1 || res.NewFindings != 1 || res.Remaining != 0 {
		t.Fatalf("重试结果不符：retried=%d new=%d remaining=%d", res.Retried, res.NewFindings, res.Remaining)
	}
	if res.Status != port.TaskStatusDone {
		t.Fatalf("重试成功后任务状态应为 done，实际 %s", res.Status)
	}
	got = findingTables(t, st, taskID)
	if got["USERS/PHONE"] != 1 || got["ORDERS/PHONE"] != 1 {
		t.Fatalf("重试后台账应含两张表各一条且不重复，实际 %v", got)
	}

	// 5) 断点全部转 done，任务已完成：再次重试应被状态校验拦下。
	if _, err := svc.RetryFailed(ctx, taskID, "", ""); !errors.Is(err, ErrTaskNotRetryable) {
		t.Fatalf("任务已完成后重试应返回 ErrTaskNotRetryable，实际 %v", err)
	}

	// 6) 任务仍为部分失败但已无失败断点时，应报「没有失败表可重试」。
	if err := st.UpdateTaskStatus(ctx, taskID, port.TaskStatusPartial, port.TaskSummary{FailedCount: 0}); err != nil {
		t.Fatalf("更新任务状态：%v", err)
	}
	if _, err := svc.RetryFailed(ctx, taskID, "", ""); !errors.Is(err, ErrNoFailedTable) {
		t.Fatalf("无失败表时应返回 ErrNoFailedTable，实际 %v", err)
	}
}

func TestIT06_重试指定对象且成功表不重跑(t *testing.T) {
	ctx := context.Background()
	target := setupTwoTableDB(t)
	base, err := dsdriver.NewDriver(port.DialectSQLite)
	if err != nil {
		t.Fatalf("构造驱动：%v", err)
	}
	flaky := &flakyDriver{Driver: base, failTable: "ORDERS", failCount: 1}
	svc, st := newFlakyService(t, flaky)
	defer st.Close()

	seedDataSource(t, st, "ds-it06b", target)
	taskID, err := svc.Run(ctx, "ds-it06b", "", "")
	if err != nil {
		t.Fatalf("RunConfig：%v", err)
	}
	// 指定一个并不在失败清单中的对象 → ErrTargetNotFailed。
	if _, err := svc.RetryFailedTables(ctx, taskID, []string{"main/USERS"}, "", ""); !errors.Is(err, ErrTargetNotFailed) {
		t.Fatalf("指定非失败对象时应返回 ErrTargetNotFailed，实际 %v", err)
	}
	// 指定失败清单中的对象 → 只重扫该表。
	res, err := svc.RetryFailedTables(ctx, taskID, []string{"main/ORDERS"}, "", "")
	if err != nil {
		t.Fatalf("RetryFailedTables：%v", err)
	}
	if res.Retried != 1 || res.NewFindings != 1 {
		t.Fatalf("重试结果不符：retried=%d new=%d", res.Retried, res.NewFindings)
	}
	got := findingTables(t, st, taskID)
	if got["USERS/PHONE"] != 1 || got["ORDERS/PHONE"] != 1 {
		t.Fatalf("台账应含两张表各一条，实际 %v", got)
	}
}

func TestIT06_续扫跳过已完成表(t *testing.T) {
	ctx := context.Background()
	target := setupTwoTableDB(t)
	base, err := dsdriver.NewDriver(port.DialectSQLite)
	if err != nil {
		t.Fatalf("构造驱动：%v", err)
	}
	flaky := &flakyDriver{Driver: base, failTable: "ORDERS", failCount: 1}
	svc, st := newFlakyService(t, flaky)
	defer st.Close()

	seedDataSource(t, st, "ds-it06c", target)
	taskID, err := svc.Run(ctx, "ds-it06c", "", "")
	if err != nil {
		t.Fatalf("RunConfig：%v", err)
	}
	// 续扫：已完成的 USERS 不重扫（不产生重复台账），ORDERS 重扫成功。
	res, err := svc.Resume(ctx, taskID, "", "")
	if err != nil {
		t.Fatalf("Resume：%v", err)
	}
	if res.NewFindings != 1 || res.Remaining != 0 || res.Status != port.TaskStatusDone {
		t.Fatalf("续扫结果不符：new=%d remaining=%d status=%s", res.NewFindings, res.Remaining, res.Status)
	}
	got := findingTables(t, st, taskID)
	if got["USERS/PHONE"] != 1 || got["ORDERS/PHONE"] != 1 {
		t.Fatalf("续扫后台账应含两张表各一条且不重复，实际 %v", got)
	}
}
