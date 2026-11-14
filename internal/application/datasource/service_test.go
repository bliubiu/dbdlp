package datasource

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	apptesting "dbdlp/internal/application/testing"
	"dbdlp/internal/domain/port"
	"dbdlp/internal/infrastructure/store"
)

// ===== 测试替身 =====

type stubConn struct{ pingErr error }

func (c *stubConn) Ping(context.Context) error { return c.pingErr }
func (c *stubConn) Query(context.Context, string, ...any) (port.Rows, error) {
	return nil, fmt.Errorf("未实现")
}
func (c *stubConn) Close() error { return nil }

type stubDriver struct{ failNames map[string]bool }

func (d *stubDriver) Dialect() port.Dialect { return port.DialectOracle }
func (d *stubDriver) Connect(_ context.Context, src port.DataSourceConfig) (port.ReadOnlyConn, error) {
	if d.failNames[src.Name] {
		return nil, fmt.Errorf("主机不可达")
	}
	return &stubConn{}, nil
}
func (d *stubDriver) ListSchemas(context.Context, port.ReadOnlyConn) ([]port.Schema, error) {
	return nil, nil
}
func (d *stubDriver) ListColumns(context.Context, port.ReadOnlyConn, string) ([]port.ColumnMeta, error) {
	return nil, nil
}
func (d *stubDriver) TableStats(context.Context, port.ReadOnlyConn, string, string) (port.TableStats, error) {
	return port.TableStats{}, nil
}
func (d *stubDriver) SampleRows(context.Context, port.ReadOnlyConn, port.SampleQuery) (port.RowBatch, error) {
	return port.RowBatch{}, nil
}
func (d *stubDriver) Placeholder(int) string { return "?" }
func (d *stubDriver) QuoteIdentifier(n string) string {
	return `"` + strings.ReplaceAll(n, `"`, `""`) + `"`
}

// stubCipher 用可逆的 ENC() 包裹代替真实加密，验证「密文落库」契约。
type stubCipher struct{}

func (stubCipher) Encrypt(p string, _ port.Algorithm) (string, error) { return "ENC(" + p + ")", nil }
func (stubCipher) Decrypt(s string) (string, error) {
	return strings.TrimSuffix(strings.TrimPrefix(s, "ENC("), ")"), nil
}
func (stubCipher) IsEncrypted(s string) bool { return strings.HasPrefix(s, "ENC(") }

func newSvc(t *testing.T, failNames map[string]bool) (*Service, *store.SQLiteStore) {
	t.Helper()
	st, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "ds.db"))
	if err != nil {
		t.Fatalf("打开存储：%v", err)
	}
	svc := NewService(st, func(port.Dialect) (port.Driver, error) { return &stubDriver{failNames: failNames}, nil },
		stubCipher{}, port.AlgAES256GCM, apptesting.NewNopLogger())
	return svc, st
}

// ===== 用例 =====

func TestAdd_连接测试未通过则不落库(t *testing.T) {
	ctx := context.Background()
	svc, st := newSvc(t, map[string]bool{"bad": true})
	defer st.Close()

	id, err := svc.Add(ctx, AddInput{Name: "bad", Dialect: "oracle", Host: "10.0.0.9", Username: "u", Password: "p"})
	if err == nil {
		t.Fatalf("预期连接测试失败，实际成功并返回 ID=%s", id)
	}
	page, err := st.ListDataSources(ctx, port.DataSourceFilter{})
	if err != nil {
		t.Fatalf("ListDataSources：%v", err)
	}
	if page.Total != 0 {
		t.Fatalf("连接测试未通过却落了库，预期 0 条，实际 %d", page.Total)
	}
}

func TestAdd_凭据密文落库(t *testing.T) {
	ctx := context.Background()
	svc, st := newSvc(t, nil)
	defer st.Close()

	id, err := svc.Add(ctx, AddInput{Name: "good", Dialect: "oracle", Host: "10.0.0.9", Username: "u", Password: "Tiger123"})
	if err != nil {
		t.Fatalf("Add：%v", err)
	}
	ds, err := st.GetDataSource(ctx, id)
	if err != nil {
		t.Fatalf("GetDataSource：%v", err)
	}
	if ds.PasswordEnc != "ENC(Tiger123)" {
		t.Fatalf("口令未以密文落库：%q", ds.PasswordEnc)
	}
	if ds.HostEnc != "ENC(10.0.0.9)" {
		t.Fatalf("主机未以密文落库：%q", ds.HostEnc)
	}
	if !ds.LastTestOk {
		t.Fatalf("应标记连接测试通过")
	}
}

func TestImport_部分成功(t *testing.T) {
	ctx := context.Background()
	svc, st := newSvc(t, map[string]bool{"bad": true})
	defer st.Close()

	ids, failures, err := svc.Import(ctx, []AddInput{
		{Name: "good1", Dialect: "oracle", Host: "h1", Password: "p1"},
		{Name: "bad", Dialect: "oracle", Host: "h2", Password: "p2"},
		{Name: "good2", Dialect: "oracle", Host: "h3", Password: "p3"},
	})
	if err != nil {
		t.Fatalf("Import：%v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("预期 2 条成功，实际 %d", len(ids))
	}
	if len(failures) != 1 || failures[0].Name != "bad" {
		t.Fatalf("失败明细不匹配：%+v", failures)
	}
	page, err := st.ListDataSources(ctx, port.DataSourceFilter{})
	if err != nil {
		t.Fatalf("ListDataSources：%v", err)
	}
	if page.Total != 2 {
		t.Fatalf("预期落库 2 条，实际 %d", page.Total)
	}
}
