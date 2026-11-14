package store

import (
	"context"
	"database/sql"

	"dbdlp/internal/domain/port"
)

// 本文件实现数据源的增删改查、批量创建与行扫描辅助。

func (s *SQLiteStore) CreateDataSource(ctx context.Context, d port.DataSource) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO data_source
		(id,name,dialect,host_enc,port,service_name,username,password_enc,dsn_enc,include_schemas,exclude_tables,exclude_columns,
		 conn_max_open,conn_max_idle,conn_timeout_ms,query_timeout_ms,last_test_at,last_test_ok,enabled,remark,created_at,updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		d.ID, d.Name, string(d.Dialect), d.HostEnc, d.Port, d.ServiceName, d.Username, d.PasswordEnc, d.DSNEnc,
		d.IncludeSchemas, d.ExcludeTables, d.ExcludeColumns, d.ConnMaxOpen, d.ConnMaxIdle, d.ConnTimeoutMs, d.QueryTimeoutMs,
		d.LastTestAt, boolToInt(d.LastTestOk), boolToInt(d.Enabled), d.Remark, nowText(), nowText())
	return err
}

func (s *SQLiteStore) UpdateDataSource(ctx context.Context, d port.DataSource) error {
	_, err := s.db.ExecContext(ctx, `UPDATE data_source SET
		name=?,dialect=?,host_enc=?,port=?,service_name=?,username=?,password_enc=?,dsn_enc=?,include_schemas=?,exclude_tables=?,exclude_columns=?,
		conn_max_open=?,conn_max_idle=?,conn_timeout_ms=?,query_timeout_ms=?,last_test_at=?,last_test_ok=?,enabled=?,remark=?,updated_at=?
		WHERE id=? AND deleted_at IS NULL`,
		d.Name, string(d.Dialect), d.HostEnc, d.Port, d.ServiceName, d.Username, d.PasswordEnc, d.DSNEnc,
		d.IncludeSchemas, d.ExcludeTables, d.ExcludeColumns, d.ConnMaxOpen, d.ConnMaxIdle, d.ConnTimeoutMs, d.QueryTimeoutMs,
		d.LastTestAt, boolToInt(d.LastTestOk), boolToInt(d.Enabled), d.Remark, nowText(), d.ID)
	return err
}

func (s *SQLiteStore) DeleteDataSource(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE data_source SET deleted_at=? WHERE id=? AND deleted_at IS NULL`, nowText(), id)
	return err
}

func (s *SQLiteStore) GetDataSource(ctx context.Context, id string) (port.DataSource, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id,name,dialect,host_enc,port,service_name,username,password_enc,dsn_enc,
		include_schemas,exclude_tables,exclude_columns,conn_max_open,conn_max_idle,conn_timeout_ms,query_timeout_ms,
		last_test_at,last_test_ok,enabled,remark,created_at,updated_at,deleted_at FROM data_source WHERE id=? AND deleted_at IS NULL`, id)
	return scanDataSource(row)
}

func (s *SQLiteStore) ListDataSources(ctx context.Context, f port.DataSourceFilter) (port.Page[port.DataSource], error) {
	where := "WHERE deleted_at IS NULL"
	args := []any{}
	if f.Dialect != "" {
		where += " AND dialect=?"
		args = append(args, string(f.Dialect))
	}
	if f.Keyword != "" {
		where += " AND name LIKE ?"
		args = append(args, "%"+f.Keyword+"%")
	}
	var total int64
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM data_source "+where, args...).Scan(&total); err != nil {
		return port.Page[port.DataSource]{}, err
	}
	limit, offset := pageLimitOffset(f.Page, f.Limit)
	args = append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx, `SELECT id,name,dialect,host_enc,port,service_name,username,password_enc,dsn_enc,
		include_schemas,exclude_tables,exclude_columns,conn_max_open,conn_max_idle,conn_timeout_ms,query_timeout_ms,
		last_test_at,last_test_ok,enabled,remark,created_at,updated_at,deleted_at FROM data_source `+where+` ORDER BY created_at DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return port.Page[port.DataSource]{}, err
	}
	defer rows.Close()
	var items []port.DataSource
	for rows.Next() {
		d, err := scanDataSourceFromRows(rows)
		if err != nil {
			return port.Page[port.DataSource]{}, err
		}
		items = append(items, d)
	}
	return port.Page[port.DataSource]{Items: items, Total: total, Page: f.Page, Limit: limit, HasMore: offset+int64(len(items)) < total}, nil
}

func (s *SQLiteStore) BatchCreateDataSources(ctx context.Context, items []port.DataSource) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	for _, d := range items {
		_, err := tx.ExecContext(ctx, `INSERT INTO data_source
			(id,name,dialect,host_enc,port,service_name,username,password_enc,dsn_enc,include_schemas,exclude_tables,exclude_columns,
			 conn_max_open,conn_max_idle,conn_timeout_ms,query_timeout_ms,enabled,remark,created_at,updated_at)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			d.ID, d.Name, string(d.Dialect), d.HostEnc, d.Port, d.ServiceName, d.Username, d.PasswordEnc, d.DSNEnc,
			d.IncludeSchemas, d.ExcludeTables, d.ExcludeColumns, d.ConnMaxOpen, d.ConnMaxIdle, d.ConnTimeoutMs, d.QueryTimeoutMs,
			boolToInt(d.Enabled), d.Remark, nowText(), nowText())
		if err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func scanDataSource(row *sql.Row) (port.DataSource, error) {
	var d port.DataSource
	var dialect, deleted, lta sql.NullString
	var lto int
	if err := row.Scan(&d.ID, &d.Name, &dialect, &d.HostEnc, &d.Port, &d.ServiceName, &d.Username, &d.PasswordEnc, &d.DSNEnc,
		&d.IncludeSchemas, &d.ExcludeTables, &d.ExcludeColumns, &d.ConnMaxOpen, &d.ConnMaxIdle, &d.ConnTimeoutMs, &d.QueryTimeoutMs,
		&lta, &lto, &d.Enabled, &d.Remark, &d.CreatedAt, &d.UpdatedAt, &deleted); err != nil {
		return port.DataSource{}, err
	}
	d.Dialect = port.Dialect(dialect.String)
	d.LastTestAt = lta.String
	d.LastTestOk = lto != 0
	return d, nil
}

func scanDataSourceFromRows(rows *sql.Rows) (port.DataSource, error) {
	var d port.DataSource
	var dialect, deleted, lta sql.NullString
	var lto int
	if err := rows.Scan(&d.ID, &d.Name, &dialect, &d.HostEnc, &d.Port, &d.ServiceName, &d.Username, &d.PasswordEnc, &d.DSNEnc,
		&d.IncludeSchemas, &d.ExcludeTables, &d.ExcludeColumns, &d.ConnMaxOpen, &d.ConnMaxIdle, &d.ConnTimeoutMs, &d.QueryTimeoutMs,
		&lta, &lto, &d.Enabled, &d.Remark, &d.CreatedAt, &d.UpdatedAt, &deleted); err != nil {
		return port.DataSource{}, err
	}
	d.Dialect = port.Dialect(dialect.String)
	d.LastTestAt = lta.String
	d.LastTestOk = lto != 0
	return d, nil
}
