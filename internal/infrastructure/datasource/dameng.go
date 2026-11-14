package datasource

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"dbdlp/internal/domain/port"
)

// damengDriver 实现达梦适配器（chunanyong/dm，驱动名 dm；SQL 与 Oracle 兼容）。
type damengDriver struct{}

func (d *damengDriver) Dialect() port.Dialect { return port.DialectDameng }

func (d *damengDriver) Connect(ctx context.Context, src port.DataSourceConfig) (port.ReadOnlyConn, error) {
	dsn := fmt.Sprintf("dm://%s:%s@%s:%d", src.Username, src.Password, src.Host, src.Port)
	if src.ServiceName != "" {
		dsn += "?schema=" + src.ServiceName
	}
	db, err := sql.Open("dm", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开达梦连接失败：%w", err)
	}
	if src.MaxOpenConns > 0 {
		db.SetMaxOpenConns(src.MaxOpenConns)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("达梦连接测试失败（主机不可达或凭据错误）：%w", err)
	}
	return &roConn{db: db}, nil
}

func (d *damengDriver) ListSchemas(ctx context.Context, conn port.ReadOnlyConn) ([]port.Schema, error) {
	q := "SELECT NAME FROM SYSOBJECTS WHERE TYPE$='SCH' AND NAME NOT IN (" + inList(ExcludedSchemasDameng) + ")"
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

func (d *damengDriver) ListColumns(ctx context.Context, conn port.ReadOnlyConn, schema string) ([]port.ColumnMeta, error) {
	q := `SELECT owner, table_name, column_name, data_type, data_length, column_id
FROM all_tab_columns WHERE owner = ? ORDER BY column_id`
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
		dl, _ := strconv.Atoi(v[4])
		out = append(out, port.ColumnMeta{
			SchemaName: v[0], TableName: v[1], ColumnName: v[2],
			DataType: v[3], DataLength: dl,
		})
	}
	return out, nil
}

func (d *damengDriver) TableStats(ctx context.Context, conn port.ReadOnlyConn, schema, table string) (port.TableStats, error) {
	ts := port.TableStats{SchemaName: schema, TableName: table}
	q1 := "SELECT NVL(num_rows,0) FROM all_tables WHERE owner = ? AND table_name = ?"
	if rows, err := conn.Query(ctx, q1, schema, table); err == nil {
		if _, vals, e := scanToStrings(rows, 0); e == nil && len(vals) > 0 && len(vals[0]) > 0 {
			ts.EstRows, _ = strconv.ParseInt(vals[0][0], 10, 64)
		}
		rows.Close()
	}
	q2 := `SELECT cc.column_name FROM all_constraints c
JOIN all_cons_columns cc ON c.constraint_name = cc.constraint_name AND c.owner = cc.owner
WHERE c.owner = ? AND c.table_name = ? AND c.constraint_type = 'P'`
	if rows, err := conn.Query(ctx, q2, schema, table); err == nil {
		if _, vals, e := scanToStrings(rows, 0); e == nil && len(vals) > 0 && len(vals[0]) > 0 {
			ts.PrimaryKey = vals[0][0]
		}
		rows.Close()
	}
	return ts, nil
}

func (d *damengDriver) SampleRows(ctx context.Context, conn port.ReadOnlyConn, q port.SampleQuery) (port.RowBatch, error) {
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
		sb.WriteString(quoteIdent(c))
	}
	sb.WriteString(" FROM ")
	sb.WriteString(quoteIdent(q.Schema))
	sb.WriteString(".")
	sb.WriteString(quoteIdent(q.Table))
	args := []any{}
	if q.PrimaryKey != "" && q.LastKey != "" {
		sb.WriteString(" WHERE ")
		sb.WriteString(quoteIdent(q.PrimaryKey))
		sb.WriteString(" > ?")
		args = append(args, q.LastKey)
	}
	if q.PrimaryKey != "" {
		sb.WriteString(" ORDER BY ")
		sb.WriteString(quoteIdent(q.PrimaryKey))
		sb.WriteString(" ASC")
	}
	sb.WriteString(fmt.Sprintf(" LIMIT %d", q.BatchSize))
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

func (d *damengDriver) Placeholder(i int) string           { return "?" }
func (d *damengDriver) QuoteIdentifier(name string) string { return quoteIdent(name) }
