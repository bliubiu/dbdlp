package datasource

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"dbdlp/internal/domain/port"
)

// oracleDriver 实现 Oracle 适配器（go-ora/v2，驱动名 oracle）。
type oracleDriver struct{}

func (d *oracleDriver) Dialect() port.Dialect { return port.DialectOracle }

func (d *oracleDriver) Connect(ctx context.Context, src port.DataSourceConfig) (port.ReadOnlyConn, error) {
	dsn := fmt.Sprintf("oracle://%s:%s@%s:%d/%s", src.Username, src.Password, src.Host, src.Port, src.ServiceName)
	db, err := sql.Open("oracle", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开 Oracle 连接失败：%w", err)
	}
	if src.QueryTimeout > 0 {
		db.SetConnMaxLifetime(src.QueryTimeout)
	}
	if src.MaxOpenConns > 0 {
		db.SetMaxOpenConns(src.MaxOpenConns)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("Oracle 连接测试失败（主机不可达或凭据错误）：%w", err)
	}
	return &roConn{db: db}, nil
}

func (d *oracleDriver) ListSchemas(ctx context.Context, conn port.ReadOnlyConn) ([]port.Schema, error) {
	q := "SELECT username FROM all_users WHERE username NOT IN (" + inList(ExcludedSchemasOracle) + ")"
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

func (d *oracleDriver) ListColumns(ctx context.Context, conn port.ReadOnlyConn, schema string) ([]port.ColumnMeta, error) {
	q := `SELECT owner, table_name, column_name, data_type, data_length, column_id
FROM all_tab_columns WHERE owner = :1 ORDER BY column_id`
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

func (d *oracleDriver) TableStats(ctx context.Context, conn port.ReadOnlyConn, schema, table string) (port.TableStats, error) {
	ts := port.TableStats{SchemaName: schema, TableName: table}
	q1 := "SELECT NVL(num_rows,0) FROM all_tables WHERE owner = :1 AND table_name = :2"
	if rows, err := conn.Query(ctx, q1, schema, table); err == nil {
		if _, vals, e := scanToStrings(rows, 0); e == nil && len(vals) > 0 && len(vals[0]) > 0 {
			ts.EstRows, _ = strconv.ParseInt(vals[0][0], 10, 64)
		}
		rows.Close()
	}
	// 主键（取首个主键列）
	q2 := `SELECT cc.column_name FROM all_constraints c
JOIN all_cons_columns cc ON c.constraint_name = cc.constraint_name AND c.owner = cc.owner
WHERE c.owner = :1 AND c.table_name = :2 AND c.constraint_type = 'P'`
	if rows, err := conn.Query(ctx, q2, schema, table); err == nil {
		if _, vals, e := scanToStrings(rows, 0); e == nil && len(vals) > 0 && len(vals[0]) > 0 {
			ts.PrimaryKey = vals[0][0]
		}
		rows.Close()
	}
	return ts, nil
}

func (d *oracleDriver) SampleRows(ctx context.Context, conn port.ReadOnlyConn, q port.SampleQuery) (port.RowBatch, error) {
	cols := q.Columns
	if len(cols) == 0 {
		return port.RowBatch{}, fmt.Errorf("采样列不能为空")
	}
	var sb strings.Builder
	args := []any{}
	var inner strings.Builder
	inner.WriteString("SELECT ")
	for i, c := range cols {
		if i > 0 {
			inner.WriteString(", ")
		}
		inner.WriteString(quoteIdent(c))
	}
	inner.WriteString(" FROM ")
	inner.WriteString(quoteIdent(q.Schema))
	inner.WriteString(".")
	inner.WriteString(quoteIdent(q.Table))
	if q.PrimaryKey != "" && q.LastKey != "" {
		inner.WriteString(" WHERE ")
		inner.WriteString(quoteIdent(q.PrimaryKey))
		inner.WriteString(" > :1")
		args = append(args, q.LastKey)
	}
	if q.PrimaryKey != "" {
		inner.WriteString(" ORDER BY ")
		inner.WriteString(quoteIdent(q.PrimaryKey))
		inner.WriteString(" ASC")
	}
	// Oracle 11g 不支持 FETCH FIRST（12c 语法），采用 ROWNUM 三明治分页。
	paramRownum := fmt.Sprintf(":%d", len(args)+1)
	args = append(args, q.BatchSize)
	var colsQuoted []string
	for _, c := range cols {
		colsQuoted = append(colsQuoted, quoteIdent(c))
	}
	colList := strings.Join(colsQuoted, ", ")
	sb.WriteString("SELECT ")
	sb.WriteString(colList)
	sb.WriteString(" FROM (SELECT ")
	sb.WriteString(colList)
	sb.WriteString(", ROWNUM AS \"__RN\" FROM (")
	sb.WriteString(inner.String())
	sb.WriteString(") WHERE ROWNUM <= ")
	sb.WriteString(paramRownum)
	sb.WriteString(")")
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
		// 主键值取主键列所在位置
		idx := indexOfCol(colsOut, q.PrimaryKey)
		if idx >= 0 {
			batch.LastKey = vals[len(vals)-1][idx]
		}
	}
	return batch, nil
}

func (d *oracleDriver) Placeholder(i int) string           { return fmt.Sprintf(":%d", i) }
func (d *oracleDriver) QuoteIdentifier(name string) string { return quoteIdent(name) }

// inList 生成 ('A','B') 形式的 IN 列表（系统模式名为常量，安全）。
func inList(items []string) string {
	parts := make([]string, len(items))
	for i, it := range items {
		parts[i] = "'" + strings.ReplaceAll(it, "'", "''") + "'"
	}
	return strings.Join(parts, ",")
}

func indexOfCol(cols []string, name string) int {
	for i, c := range cols {
		if strings.EqualFold(c, name) {
			return i
		}
	}
	return -1
}
