package store

import (
	"context"
	"path/filepath"
	"testing"

	"dbdlp/internal/domain/port"
)

// 编译期断言：SQLiteStore 满足 port.Store。
var _ port.Store = (*SQLiteStore)(nil)

func newTestStore(t *testing.T) (*SQLiteStore, func()) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	return s, func() { _ = s.Close() }
}

func TestDataSourceCRUD(t *testing.T) {
	ctx := context.Background()
	s, cleanup := newTestStore(t)
	defer cleanup()

	ds := port.DataSource{
		ID: "ds1", Name: "orcl", Dialect: port.DialectOracle,
		HostEnc: "ENC(x)", Port: 1521, ServiceName: "orcl",
		Username: "scott", PasswordEnc: "ENC(y)", ConnMaxOpen: 2, ConnMaxIdle: 1,
		ConnTimeoutMs: 10000, QueryTimeoutMs: 30000, Enabled: true,
	}
	if err := s.CreateDataSource(ctx, ds); err != nil {
		t.Fatalf("CreateDataSource: %v", err)
	}
	got, err := s.GetDataSource(ctx, "ds1")
	if err != nil {
		t.Fatalf("GetDataSource: %v", err)
	}
	if got.Name != "orcl" || got.Dialect != port.DialectOracle || got.Port != 1521 {
		t.Fatalf("字段不匹配: %+v", got)
	}
	// 逻辑删除后查不到
	if err := s.DeleteDataSource(ctx, "ds1"); err != nil {
		t.Fatalf("DeleteDataSource: %v", err)
	}
	if _, err := s.GetDataSource(ctx, "ds1"); err == nil {
		t.Fatalf("删除后仍可取回，预期错误")
	}
}

func TestMetadataAndFindings(t *testing.T) {
	ctx := context.Background()
	s, cleanup := newTestStore(t)
	defer cleanup()

	if err := s.CreateTask(ctx, port.ScanTask{ID: "t1", DataSourceID: "ds1", Status: port.TaskStatusRunning, SampleRows: 20}); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	schemas := []port.Schema{{Name: "SCOTT", IsSystem: false}}
	tables := []port.TableMeta{{SchemaName: "SCOTT", TableName: "USERS", EstRows: 100, PrimaryKey: "ID"}}
	cols := []port.ColumnMeta{
		{SchemaName: "SCOTT", TableName: "USERS", ColumnName: "PHONE", DataType: "VARCHAR2", Comment: "手机号", Nullable: true},
	}
	if err := s.SaveMetadata(ctx, "t1", schemas, tables, cols); err != nil {
		t.Fatalf("SaveMetadata: %v", err)
	}

	findings := []port.ColumnFinding{{
		TaskID: "t1", DataSourceID: "ds1", SchemaName: "SCOTT", TableName: "USERS", ColumnName: "PHONE",
		DataType: "VARCHAR2", ColumnComment: "手机号", SampleTotal: 10, SampleHit: 10, HitRatio: 1.0,
		SensitiveTypes: []string{"MOBILE_PHONE"}, VerifyPassed: true, Confidence: "high",
		StorageForm: "plaintext", RiskLevel: "high", ReviewStatus: "pending", SampleMasked: "138****8000",
		Hits: []port.Hit{{
			RuleID: "MOBILE_PHONE", SensitiveType: "MOBILE_PHONE", Confidence: "high",
			MaskedValue: "138****8000", SourceForm: "plaintext", VerifyPassed: true,
		}},
	}}
	if err := s.SaveFindings(ctx, findings); err != nil {
		t.Fatalf("SaveFindings: %v", err)
	}

	page, err := s.ListFindings(ctx, port.FindingFilter{TaskID: "t1"})
	if err != nil {
		t.Fatalf("ListFindings: %v", err)
	}
	if page.Total != 1 {
		t.Fatalf("预期 1 条台账，实际 %d", page.Total)
	}
	f := page.Items[0]
	if f.ColumnName != "PHONE" || f.DataType != "VARCHAR2" || f.ColumnComment != "手机号" {
		t.Fatalf("台账字段不匹配: %+v", f)
	}
	if f.RiskLevel != "high" || f.ReviewStatus != "pending" {
		t.Fatalf("风险/复核状态不匹配: %+v", f)
	}

	hits, err := s.ListHits(ctx, f.ID, port.Pagination{Page: 1, Limit: 20})
	if err != nil {
		t.Fatalf("ListHits: %v", err)
	}
	if hits.Total != 1 || hits.Items[0].MaskedValue != "138****8000" {
		t.Fatalf("命中记录不匹配: %+v", hits)
	}
}

func TestCheckpointAndRuleSnapshot(t *testing.T) {
	ctx := context.Background()
	s, cleanup := newTestStore(t)
	defer cleanup()

	cp := port.ScanCheckpoint{TaskID: "t1", TableMetaID: "tm1", ColumnName: "PHONE", LastKey: "100", DoneRows: 100, State: "scanning"}
	if err := s.SaveCheckpoint(ctx, cp); err != nil {
		t.Fatalf("SaveCheckpoint: %v", err)
	}
	pending, err := s.ListPendingCheckpoints(ctx, "t1")
	if err != nil {
		t.Fatalf("ListPendingCheckpoints: %v", err)
	}
	if len(pending) != 1 || pending[0].LastKey != "100" {
		t.Fatalf("断点不匹配: %+v", pending)
	}

	rules := []port.Rule{{RuleID: "MOBILE_PHONE", SensitiveType: "MOBILE_PHONE", Enabled: true, Regex: `\d+`, Threshold: 0.5}}
	if err := s.SaveRuleSnapshot(ctx, "v1", rules); err != nil {
		t.Fatalf("SaveRuleSnapshot: %v", err)
	}
}

func TestTaskStatusUpdate(t *testing.T) {
	ctx := context.Background()
	s, cleanup := newTestStore(t)
	defer cleanup()

	if err := s.CreateTask(ctx, port.ScanTask{ID: "t1", DataSourceID: "ds1", Status: port.TaskStatusPending}); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	summary := port.TaskSummary{SchemaCount: 1, TableCount: 2, ColumnCount: 5, HitColumnCount: 3}
	if err := s.UpdateTaskStatus(ctx, "t1", port.TaskStatusDone, summary); err != nil {
		t.Fatalf("UpdateTaskStatus: %v", err)
	}
}
