package datasource

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"dbdlp/internal/domain/port"
	_ "github.com/go-sql-driver/mysql"
)

// mysqlDriver 实现 MySQL 适配器（go-sql-driver/mysql，驱动名 mysql）。
// 仅用于只读资产扫描；只读性由 roConn 守卫保障（go-sql-driver/mysql 不支持 readOnly DSN 参数）。
type mysqlDriver struct{}

func (d *mysqlDriver) Dialect() port.Dialect { return port.DialectMySQL }

func (d *mysqlDriver) Connect(ctx context.Context, src port.DataSourceConfig) (port.ReadOnlyConn, error) {
	// charset=utf8mb4 避免中文乱码；只读性由 roConn 守卫保障，不依赖驱动参数。
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true&loc=Local",
		src.Username, src.Password, src.Host, src.Port, src.ServiceName)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开 MySQL 连接失败：%w", err)
	}
	if src.MaxOpenConns > 0 {
		db.SetMaxOpenConns(src.MaxOpenConns)
	}
	if src.QueryTimeout > 0 {
		db.SetConnMaxLifetime(src.QueryTimeout)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("MySQL 连接测试失败（主机不可达或凭据错误）：%w", err)
	}
	return &roConn{db: db}, nil
}

func (d *mysqlDriver) ListSchemas(ctx context.Context, conn port.ReadOnlyConn) ([]port.Schema, error) {
	q := "SELECT SCHEMA_NAME FROM information_schema.SCHEMATA WHERE SCHEMA_NAME NOT IN (" + inList(ExcludedSchemasMySQL) + ")"
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

func (d *mysqlDriver) ListColumns(ctx context.Context, conn port.ReadOnlyConn, schema string) ([]port.ColumnMeta, error) {
	q := `SELECT TABLE_SCHEMA, TABLE_NAME, COLUMN_NAME, DATA_TYPE, CHARACTER_MAXIMUM_LENGTH, ORDINAL_POSITION
FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = ? ORDER BY ORDINAL_POSITION`
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

func (d *mysqlDriver) TableStats(ctx context.Context, conn port.ReadOnlyConn, schema, table string) (port.TableStats, error) {
	ts := port.TableStats{SchemaName: schema, TableName: table}
	q1 := "SELECT TABLE_ROWS FROM information_schema.TABLES WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?"
	if rows, err := conn.Query(ctx, q1, schema, table); err == nil {
		if _, vals, e := scanToStrings(rows, 0); e == nil && len(vals) > 0 && len(vals[0]) > 0 {
			ts.EstRows, _ = strconv.ParseInt(vals[0][0], 10, 64)
		}
		rows.Close()
	}
	q2 := `SELECT kcu.COLUMN_NAME FROM information_schema.TABLE_CONSTRAINTS tc
JOIN information_schema.KEY_COLUMN_USAGE kcu ON tc.CONSTRAINT_NAME = kcu.CONSTRAINT_NAME AND tc.TABLE_SCHEMA = kcu.TABLE_SCHEMA
WHERE tc.CONSTRAINT_TYPE = 'PRIMARY KEY' AND tc.TABLE_SCHEMA = ? AND tc.TABLE_NAME = ? LIMIT 1`
	if rows, err := conn.Query(ctx, q2, schema, table); err == nil {
		if _, vals, e := scanToStrings(rows, 0); e == nil && len(vals) > 0 && len(vals[0]) > 0 {
			ts.PrimaryKey = vals[0][0]
		}
		rows.Close()
	}
	return ts, nil
}

func (d *mysqlDriver) SampleRows(ctx context.Context, conn port.ReadOnlyConn, q port.SampleQuery) (port.RowBatch, error) {
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
		sb.WriteString(quoteIdentMySQL(c))
	}
	sb.WriteString(" FROM ")
	sb.WriteString(quoteIdentMySQL(q.Schema))
	sb.WriteString(".")
	sb.WriteString(quoteIdentMySQL(q.Table))
	args := []any{}
	if q.PrimaryKey != "" && q.LastKey != "" {
		sb.WriteString(" WHERE ")
		sb.WriteString(quoteIdentMySQL(q.PrimaryKey))
		sb.WriteString(" > ?")
		args = append(args, q.LastKey)
	}
	if q.PrimaryKey != "" {
		sb.WriteString(" ORDER BY ")
		sb.WriteString(quoteIdentMySQL(q.PrimaryKey))
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

func (d *mysqlDriver) Placeholder(i int) string           { return "?" }
func (d *mysqlDriver) QuoteIdentifier(name string) string { return quoteIdentMySQL(name) }

// quoteIdentMySQL 用反引号引用标识符，内部反引号转义为双反引号。
func quoteIdentMySQL(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}

// ExcludedSchemasMySQL 系统库排除清单（docs/02 §5.3.1）。
var ExcludedSchemasMySQL = []string{"information_schema", "mysql", "performance_schema", "sys"}
