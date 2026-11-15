package datasource

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"dbdlp/internal/domain/port"
	_ "github.com/lib/pq"
)

// postgresDriver 实现 PostgreSQL 适配器（lib/pq，驱动名 postgres）。
// 仅用于只读资产扫描；只读性由 roConn 守卫保障（lib/pq 无 readOnly DSN 参数）。
type postgresDriver struct{}

func (d *postgresDriver) Dialect() port.Dialect { return port.DialectPostgres }

func (d *postgresDriver) Connect(ctx context.Context, src port.DataSourceConfig) (port.ReadOnlyConn, error) {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable connect_timeout=10",
		src.Host, src.Port, src.Username, src.Password, src.ServiceName)
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开 PostgreSQL 连接失败：%w", err)
	}
	if src.MaxOpenConns > 0 {
		db.SetMaxOpenConns(src.MaxOpenConns)
	}
	if src.QueryTimeout > 0 {
		db.SetConnMaxLifetime(src.QueryTimeout)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("PostgreSQL 连接测试失败（主机不可达或凭据错误）：%w", err)
	}
	return &roConn{db: db}, nil
}

func (d *postgresDriver) ListSchemas(ctx context.Context, conn port.ReadOnlyConn) ([]port.Schema, error) {
	q := "SELECT schema_name FROM information_schema.schemata WHERE schema_name NOT IN (" +
		inList(ExcludedSchemasPostgres) + ") AND schema_name NOT LIKE 'pg_%' AND schema_name NOT LIKE 'pgtoast%'"
	rows, err := conn.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, vals, err := scanToStrings(rows, 0)
	_ = cols
	if err != nil {
		return nil, err
	}
	out := make([]port.Schema, 0, len(vals))
	for _, v := range vals {
		if len(v) > 0 {
			out = append(out, port.Schema{Name: v[0]})
		}
	}
	return out, nil
}

func (d *postgresDriver) ListColumns(ctx context.Context, conn port.ReadOnlyConn, schema string) ([]port.ColumnMeta, error) {
	q := `SELECT table_schema, table_name, column_name, data_type, character_maximum_length, ordinal_position
FROM information_schema.columns WHERE table_schema = $1 ORDER BY ordinal_position`
	rows, err := conn.Query(ctx, q, schema)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, vals, err := scanToStrings(rows, 0)
	_ = cols
	if err != nil {
		return nil, err
	}
	out := make([]port.ColumnMeta, 0, len(vals))
	for _, v := range vals {
		if len(v) < 6 {
			continue
		}
		var dl int
		if v[4] != "" {
			dl, _ = strconv.Atoi(v[4])
		}
		out = append(out, port.ColumnMeta{
			SchemaName: v[0], TableName: v[1], ColumnName: v[2],
			DataType: v[3], DataLength: dl,
		})
	}
	return out, nil
}

func (d *postgresDriver) TableStats(ctx context.Context, conn port.ReadOnlyConn, schema, table string) (port.TableStats, error) {
	ts := port.TableStats{SchemaName: schema, TableName: table}
	q1 := `SELECT reltuples::bigint FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace WHERE n.nspname = $1 AND c.relname = $2`
	if rows, err := conn.Query(ctx, q1, schema, table); err == nil {
		if _, vals, e := scanToStrings(rows, 0); e == nil && len(vals) > 0 && len(vals[0]) > 0 {
			ts.EstRows, _ = strconv.ParseInt(vals[0][0], 10, 64)
		}
		rows.Close()
	}
	q2 := `SELECT a.attname FROM pg_index i
JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = ANY(i.indkey)
WHERE i.indrelid = ($1 || '.' || $2)::regclass AND i.indisprimary`
	if rows, err := conn.Query(ctx, q2, schema, table); err == nil {
		if _, vals, e := scanToStrings(rows, 0); e == nil && len(vals) > 0 && len(vals[0]) > 0 {
			ts.PrimaryKey = vals[0][0]
		}
		rows.Close()
	}
	return ts, nil
}

func (d *postgresDriver) SampleRows(ctx context.Context, conn port.ReadOnlyConn, q port.SampleQuery) (port.RowBatch, error) {
	cols := q.Columns
	if len(cols) == 0 {
		return port.RowBatch{}, fmt.Errorf("采样列不能为空")
	}
	var sb strings.Builder
	sb.WriteString("SELECT ")
	for i, c := range cols {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(quoteIdentPG(c))
	}
	sb.WriteString(" FROM ")
	sb.WriteString(quoteIdentPG(q.Schema))
	sb.WriteString(".")
	sb.WriteString(quoteIdentPG(q.Table))
	args := []any{}
	if q.PrimaryKey != "" && q.LastKey != "" {
		sb.WriteString(" WHERE ")
		sb.WriteString(quoteIdentPG(q.PrimaryKey))
		sb.WriteString(" > $1")
		args = append(args, q.LastKey)
	}
	if q.PrimaryKey != "" {
		sb.WriteString(" ORDER BY ")
		sb.WriteString(quoteIdentPG(q.PrimaryKey))
		sb.WriteString(" ASC")
	}
	args = append(args, q.BatchSize)
	sb.WriteString(fmt.Sprintf(" LIMIT $%d", len(args)))
	rows, err := conn.Query(ctx, sb.String(), args...)
	if err != nil {
		return port.RowBatch{}, err
	}
	defer rows.Close()
	colsOut, vals, err := scanToStrings(rows, q.MaxTextLen)
	if err != nil {
		return port.RowBatch{}, err
	}
	batch := port.RowBatch{Columns: colsOut, Values: vals, HasMore: len(vals) >= q.BatchSize}
	if len(vals) > 0 && q.PrimaryKey != "" && len(colsOut) > 0 {
		if idx := indexOfCol(colsOut, q.PrimaryKey); idx >= 0 {
			batch.LastKey = vals[len(vals)-1][idx]
		}
	}
	return batch, nil
}

func (d *postgresDriver) Placeholder(i int) string           { return fmt.Sprintf("$%d", i) }
func (d *postgresDriver) QuoteIdentifier(name string) string { return quoteIdentPG(name) }

// quoteIdentPG 用双引号引用标识符，内部双引号转义为双写。
func quoteIdentPG(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// ExcludedSchemasPostgres 系统模式排除清单（docs/02 §5.3.1）。
var ExcludedSchemasPostgres = []string{"pg_catalog", "information_schema"}
