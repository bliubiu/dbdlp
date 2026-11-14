package store

import (
	"context"
	"database/sql"
	"strings"

	"dbdlp/internal/domain/port"
)

// 本文件实现面向控制台查询的 Store 方法（任务、台账、元数据、规则快照、报告）。

// GetDataSourceByName 按名称查数据源（含软删除行，用于名称占用判定）。
func (s *SQLiteStore) GetDataSourceByName(ctx context.Context, name string) (port.DataSource, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id,name,dialect,host_enc,port,service_name,username,password_enc,dsn_enc,
		include_schemas,exclude_tables,exclude_columns,conn_max_open,conn_max_idle,conn_timeout_ms,query_timeout_ms,
		last_test_at,last_test_ok,enabled,remark,created_at,updated_at,deleted_at FROM data_source WHERE name=?`, name)
	return scanDataSource(row)
}

// ListTasks 按条件分页查询任务列表。
func (s *SQLiteStore) ListTasks(ctx context.Context, f port.TaskFilter) (port.Page[port.ScanTask], error) {
	where := "WHERE deleted_at IS NULL"
	args := []any{}
	if f.DataSourceID != "" {
		where += " AND data_source_id=?"
		args = append(args, f.DataSourceID)
	}
	if f.Status != "" {
		where += " AND status=?"
		args = append(args, string(f.Status))
	}
	var total int64
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM scan_task "+where, args...).Scan(&total); err != nil {
		return port.Page[port.ScanTask]{}, err
	}
	limit, offset := pageLimitOffset(f.Page, f.Limit)
	args = append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx, `SELECT id,data_source_id,scope,status,rule_version,sample_rows,batch_size,concurrency,
		low_peak_window,triggered_by,summary,error_message,started_at,finished_at,created_at,updated_at,deleted_at
		FROM scan_task `+where+` ORDER BY created_at DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return port.Page[port.ScanTask]{}, err
	}
	defer rows.Close()
	var items []port.ScanTask
	for rows.Next() {
		t, err := scanTaskFromRows(rows)
		if err != nil {
			return port.Page[port.ScanTask]{}, err
		}
		items = append(items, t)
	}
	return port.Page[port.ScanTask]{Items: items, Total: total, Page: f.Page, Limit: limit, HasMore: offset+int64(len(items)) < total}, nil
}

// UpdateTaskState 仅迁移任务状态与错误信息，不覆盖既有 summary。
func (s *SQLiteStore) UpdateTaskState(ctx context.Context, id string, status port.TaskStatus, errorMessage string) error {
	var finished *string
	switch status {
	case port.TaskStatusDone, port.TaskStatusFailed, port.TaskStatusPartial:
		v := nowText()
		finished = &v
	}
	_, err := s.db.ExecContext(ctx, `UPDATE scan_task SET status=?,error_message=?,finished_at=?,updated_at=? WHERE id=?`,
		string(status), errorMessage, finished, nowText(), id)
	return err
}

// CountRunningTasksBySource 统计某数据源下处于运行中的任务数。
func (s *SQLiteStore) CountRunningTasksBySource(ctx context.Context, dataSourceID string) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM scan_task WHERE data_source_id=? AND status=? AND deleted_at IS NULL`,
		dataSourceID, string(port.TaskStatusRunning)).Scan(&n)
	return n, err
}

// CountTaskProgress 统计任务已落库的表数、字段数与已完成断点数。
func (s *SQLiteStore) CountTaskProgress(ctx context.Context, taskID string) (port.TaskProgressStats, error) {
	var st port.TaskProgressStats
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM table_meta WHERE task_id=?`, taskID).Scan(&st.Tables); err != nil {
		return st, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM column_meta WHERE task_id=?`, taskID).Scan(&st.Columns); err != nil {
		return st, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM scan_checkpoint WHERE task_id=? AND state='done'`, taskID).Scan(&st.CheckpointsDone); err != nil {
		return st, err
	}
	return st, nil
}

// GetFinding 按 ID 查单条字段台账。
func (s *SQLiteStore) GetFinding(ctx context.Context, id string) (port.ColumnFinding, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id,task_id,column_meta_id,data_source_id,schema_name,table_name,column_name,data_type,
		column_comment,sensitive_types,sample_total,sample_hit,hit_ratio,verify_passed,confidence,storage_form,risk_level,
		review_status,sample_masked,created_at,updated_at FROM finding_column WHERE id=? AND deleted_at IS NULL`, id)
	var f port.ColumnFinding
	var sens, cm sql.NullString
	var vp int
	if err := row.Scan(&f.ID, &f.TaskID, &f.ColumnMetaID, &f.DataSourceID, &f.SchemaName, &f.TableName, &f.ColumnName,
		&f.DataType, &cm, &sens, &f.SampleTotal, &f.SampleHit, &f.HitRatio, &vp, &f.Confidence, &f.StorageForm,
		&f.RiskLevel, &f.ReviewStatus, &f.SampleMasked, &f.CreatedAt, &f.UpdatedAt); err != nil {
		return port.ColumnFinding{}, err
	}
	f.ColumnComment = cm.String
	f.VerifyPassed = vp != 0
	if sens.String != "" {
		f.SensitiveTypes = strings.Split(sens.String, ",")
	}
	return f, nil
}

// UpdateFindingReviewStatus 更新字段台账的复核状态。
func (s *SQLiteStore) UpdateFindingReviewStatus(ctx context.Context, id string, status string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE finding_column SET review_status=?,updated_at=? WHERE id=?`, status, nowText(), id)
	return err
}

// ListReviews 分页列出复核记录。
func (s *SQLiteStore) ListReviews(ctx context.Context, findingID string, p port.Pagination) (port.Page[port.ReviewRecord], error) {
	where := "WHERE 1=1"
	args := []any{}
	if findingID != "" {
		where += " AND finding_id=?"
		args = append(args, findingID)
	}
	var total int64
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM review_record "+where, args...).Scan(&total); err != nil {
		return port.Page[port.ReviewRecord]{}, err
	}
	limit, offset := pageLimitOffset(p.Page, p.Limit)
	args = append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx, `SELECT id,finding_id,reviewer,conclusion,comment,reviewed_at,created_at,updated_at
		FROM review_record `+where+` ORDER BY created_at DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return port.Page[port.ReviewRecord]{}, err
	}
	defer rows.Close()
	var items []port.ReviewRecord
	for rows.Next() {
		var r port.ReviewRecord
		var comment sql.NullString
		if err := rows.Scan(&r.ID, &r.FindingID, &r.Reviewer, &r.Conclusion, &comment, &r.ReviewedAt, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return port.Page[port.ReviewRecord]{}, err
		}
		r.Comment = comment.String
		items = append(items, r)
	}
	return port.Page[port.ReviewRecord]{Items: items, Total: total, Page: p.Page, Limit: limit, HasMore: offset+int64(len(items)) < total}, nil
}
