package datasource

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"dbdlp/internal/domain/port"
)

// sqliteDriver 测试用 SQLite 目标适配器（modernc.org/sqlite，驱动名 sqlite）。
// 仅用于端到端验证引擎链路；非产品方言。
type sqliteDriver struct{}

func (d *sqliteDriver) Dialect() port.Dialect { return port.DialectSQLite }

func (d *sqliteDriver) Connect(ctx context.Context, src port.DataSourceConfig) (port.ReadOnlyConn, error) {
	path := src.DSN
	if path == "" {
		path = src.Host
	}
	if path == "" {
		return nil, fmt.Errorf("SQLite 目标缺少文件路径")
	}
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开 SQLite 失败：%w", err)
	}
	db.SetMaxOpenConns(1)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("SQLite 连接测试失败：%w", err)
	}
	return &roConn{db: db}, nil
}

func (d *sqliteDriver) ListSchemas(ctx context.Context, conn port.ReadOnlyConn) ([]port.Schema, error) {
	return []port.Schema{{Name: "main"}}, nil
}

func (d *sqliteDriver) ListColumns(ctx context.Context, conn port.ReadOnlyConn, schema string) ([]port.ColumnMeta, error) {
	tables, err := d.tables(ctx, conn)
	if err != nil {
		return nil, err
	}
	var out []port.ColumnMeta
	for _, t := range tables {
		rows, err := conn.Query(ctx, fmt.Sprintf("PRAGMA table_info('%s')", strings.ReplaceAll(t, "'", "''")))
		if err != nil {
			return nil, err
		}
		_, vals, err := scanToStrings(rows, 0)
		rows.Close()
		if err != nil {
			return nil, err
		}
		for _, v := range vals {
			// cid, name, type, notnull, dflt, pk
			if len(v) < 3 {
				continue
			}
			out = append(out, port.ColumnMeta{
				SchemaName: schema, TableName: t, ColumnName: v[1], DataType: v[2],
			})
		}
	}
	return out, nil
}

func (d *sqliteDriver) tables(ctx context.Context, conn port.ReadOnlyConn) ([]string, error) {
	rows, err := conn.Query(ctx, "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	_, vals, err := scanToStrings(rows, 0)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(vals))
	for _, v := range vals {
		if len(v) > 0 {
			out = append(out, v[0])
		}
	}
	return out, nil
}

func (d *sqliteDriver) TableStats(ctx context.Context, conn port.ReadOnlyConn, schema, table string) (port.TableStats, error) {
	ts := port.TableStats{SchemaName: schema, TableName: table, PrimaryKey: "rowid"}
	rows, err := conn.Query(ctx, fmt.Sprintf("PRAGMA table_info('%s')", strings.ReplaceAll(table, "'", "''")))
	if err != nil {
		return ts, err
	}
	_, vals, err := scanToStrings(rows, 0)
	rows.Close()
	if err != nil {
		return ts, err
	}
	for _, v := range vals {
		if len(v) >= 6 && v[5] == "1" {
			ts.PrimaryKey = v[1] // 首个主键列
			break
		}
	}
	return ts, nil
}

func (d *sqliteDriver) SampleRows(ctx context.Context, conn port.ReadOnlyConn, q port.SampleQuery) (port.RowBatch, error) {
	if len(q.Columns) == 0 {
		return port.RowBatch{}, fmt.Errorf("采样列不能为空")
	}
	var sb strings.Builder
	sb.WriteString("SELECT ")
	for i, c := range q.Columns {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(quoteIdent(c))
	}
	sb.WriteString(" FROM ")
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
		} else if q.PrimaryKey == "rowid" {
			batch.LastKey = strconv.Itoa(len(vals)) // rowid 退化为行序号，仅供测试
		}
	}
	return batch, nil
}

func (d *sqliteDriver) Placeholder(i int) string           { return "?" }
func (d *sqliteDriver) QuoteIdentifier(name string) string { return quoteIdent(name) }
