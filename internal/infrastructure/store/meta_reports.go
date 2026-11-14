package store

import (
	"context"
	"database/sql"
	"strings"

	"dbdlp/internal/domain/port"
)

// 本文件实现元数据、规则快照、报告产物与台账统计的 Store 方法。

// ListRuleSnapshots 分页列出规则版本快照摘要。
func (s *SQLiteStore) ListRuleSnapshots(ctx context.Context, p port.Pagination) (port.Page[port.RuleSnapshot], error) {
	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT rule_version) FROM rule_snapshot`).Scan(&total); err != nil {
		return port.Page[port.RuleSnapshot]{}, err
	}
	limit, offset := pageLimitOffset(p.Page, p.Limit)
	rows, err := s.db.QueryContext(ctx, `SELECT rule_version, MIN(created_at), COUNT(*) FROM rule_snapshot
		GROUP BY rule_version ORDER BY MIN(created_at) DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return port.Page[port.RuleSnapshot]{}, err
	}
	defer rows.Close()
	var items []port.RuleSnapshot
	for rows.Next() {
		var rs port.RuleSnapshot
		if err := rows.Scan(&rs.Version, &rs.CreatedAt, &rs.RuleCount); err != nil {
			return port.Page[port.RuleSnapshot]{}, err
		}
		items = append(items, rs)
	}
	return port.Page[port.RuleSnapshot]{Items: items, Total: total, Page: p.Page, Limit: limit, HasMore: offset+int64(len(items)) < total}, nil
}

// ListRulesByVersion 读取指定规则版本的快照行（快照表只存正则摘要，不存正则正文）。
func (s *SQLiteStore) ListRulesByVersion(ctx context.Context, version string) ([]port.Rule, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT rule_id,sensitive_type,enabled,custom,regex_digest,verifier,threshold
		FROM rule_snapshot WHERE rule_version=? ORDER BY rule_id`, version)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []port.Rule
	for rows.Next() {
		var r port.Rule
		var digest sql.NullString
		var enabled, custom int
		if err := rows.Scan(&r.RuleID, &r.SensitiveType, &enabled, &custom, &digest, &r.Verifier, &r.Threshold); err != nil {
			return nil, err
		}
		r.Enabled = enabled != 0
		r.Custom = custom != 0
		r.RegexDigest = digest.String
		r.Version = version
		out = append(out, r)
	}
	return out, nil
}

// ListReports 分页列出报告产物。
func (s *SQLiteStore) ListReports(ctx context.Context, taskID string, p port.Pagination) (port.Page[port.ReportArtifact], error) {
	where := "WHERE 1=1"
	args := []any{}
	if taskID != "" {
		where += " AND task_id=?"
		args = append(args, taskID)
	}
	var total int64
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM report_artifact "+where, args...).Scan(&total); err != nil {
		return port.Page[port.ReportArtifact]{}, err
	}
	limit, offset := pageLimitOffset(p.Page, p.Limit)
	args = append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx, `SELECT id,task_id,format,file_path,file_size,generated_by,created_at,updated_at
		FROM report_artifact `+where+` ORDER BY created_at DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return port.Page[port.ReportArtifact]{}, err
	}
	defer rows.Close()
	var items []port.ReportArtifact
	for rows.Next() {
		var a port.ReportArtifact
		var size sql.NullInt64
		if err := rows.Scan(&a.ID, &a.TaskID, &a.Format, &a.FilePath, &size, &a.GeneratedBy, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return port.Page[port.ReportArtifact]{}, err
		}
		a.FileSize = size.Int64
		items = append(items, a)
	}
	return port.Page[port.ReportArtifact]{Items: items, Total: total, Page: p.Page, Limit: limit, HasMore: offset+int64(len(items)) < total}, nil
}

// GetReportArtifact 按 ID 查报告产物。
func (s *SQLiteStore) GetReportArtifact(ctx context.Context, id string) (port.ReportArtifact, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id,task_id,format,file_path,file_size,generated_by,created_at,updated_at
		FROM report_artifact WHERE id=?`, id)
	var a port.ReportArtifact
	var size sql.NullInt64
	if err := row.Scan(&a.ID, &a.TaskID, &a.Format, &a.FilePath, &size, &a.GeneratedBy, &a.CreatedAt, &a.UpdatedAt); err != nil {
		return port.ReportArtifact{}, err
	}
	a.FileSize = size.Int64
	return a, nil
}

// ListSchemasByTask 按任务读取模式元数据。
func (s *SQLiteStore) ListSchemasByTask(ctx context.Context, taskID string) ([]port.SchemaMetaRow, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,schema_name,is_system,table_count FROM schema_meta WHERE task_id=? ORDER BY schema_name`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []port.SchemaMetaRow
	for rows.Next() {
		var r port.SchemaMetaRow
		var isSys int
		if err := rows.Scan(&r.ID, &r.SchemaName, &isSys, &r.TableCount); err != nil {
			return nil, err
		}
		r.IsSystem = isSys != 0
		out = append(out, r)
	}
	return out, nil
}

// ListTablesByTask 按任务读取表元数据。
func (s *SQLiteStore) ListTablesByTask(ctx context.Context, taskID string) ([]port.TableMetaRow, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT t.id, COALESCE(s.schema_name,''), t.table_name, COALESCE(t.table_comment,''),
		t.est_rows, COALESCE(t.primary_key,''), t.is_large, t.is_huge, t.degraded
		FROM table_meta t LEFT JOIN schema_meta s ON s.id=t.schema_meta_id
		WHERE t.task_id=? ORDER BY s.schema_name, t.table_name`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []port.TableMetaRow
	for rows.Next() {
		var r port.TableMetaRow
		var large, huge, degraded int
		if err := rows.Scan(&r.ID, &r.SchemaName, &r.TableName, &r.TableComment, &r.EstRows, &r.PrimaryKey, &large, &huge, &degraded); err != nil {
			return nil, err
		}
		r.IsLarge = large != 0
		r.IsHuge = huge != 0
		r.Degraded = degraded != 0
		out = append(out, r)
	}
	return out, nil
}

// ListColumnsByTask 按任务读取字段元数据。
func (s *SQLiteStore) ListColumnsByTask(ctx context.Context, taskID string) ([]port.ColumnMetaRow, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,table_meta_id,column_name,data_type,COALESCE(data_length,0),
		COALESCE(column_comment,''),nullable,is_encrypted,is_binary,scan_state
		FROM column_meta WHERE task_id=? ORDER BY table_meta_id, column_name`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []port.ColumnMetaRow
	for rows.Next() {
		var r port.ColumnMetaRow
		var nullable, enc, bin int
		if err := rows.Scan(&r.ID, &r.TableMetaID, &r.ColumnName, &r.DataType, &r.DataLength, &r.ColumnComment,
			&nullable, &enc, &bin, &r.ScanState); err != nil {
			return nil, err
		}
		r.Nullable = nullable != 0
		r.IsEncrypted = enc != 0
		r.IsBinary = bin != 0
		out = append(out, r)
	}
	return out, nil
}

// CountFindingsByRisk 按风险等级统计字段台账条数。
func (s *SQLiteStore) CountFindingsByRisk(ctx context.Context, taskID string) (map[string]int, error) {
	return s.countFindingsBy(ctx, "risk_level", taskID)
}

// CountFindingsBySchema 按模式统计字段台账条数。
func (s *SQLiteStore) CountFindingsBySchema(ctx context.Context, taskID string) (map[string]int, error) {
	return s.countFindingsBy(ctx, "schema_name", taskID)
}

// countFindingsBy 按单列分组统计字段台账条数。
func (s *SQLiteStore) countFindingsBy(ctx context.Context, column, taskID string) (map[string]int, error) {
	where := "WHERE deleted_at IS NULL"
	args := []any{}
	if taskID != "" {
		where += " AND task_id=?"
		args = append(args, taskID)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+column+`, COUNT(*) FROM finding_column `+where+` GROUP BY `+column, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var key sql.NullString
		var n int
		if err := rows.Scan(&key, &n); err != nil {
			return nil, err
		}
		if key.String == "" {
			continue
		}
		out[key.String] = n
	}
	return out, nil
}

// CountFindingsByType 按敏感类型统计字段台账条数（sensitive_types 为逗号分隔多值，需展开计数）。
func (s *SQLiteStore) CountFindingsByType(ctx context.Context, taskID string) (map[string]int, error) {
	where := "WHERE deleted_at IS NULL"
	args := []any{}
	if taskID != "" {
		where += " AND task_id=?"
		args = append(args, taskID)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT sensitive_types FROM finding_column `+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var types sql.NullString
		if err := rows.Scan(&types); err != nil {
			return nil, err
		}
		for _, t := range strings.Split(types.String, ",") {
			t = strings.TrimSpace(t)
			if t == "" {
				continue
			}
			out[t]++
		}
	}
	return out, nil
}

// scanTaskFromRows 从结果集当前行读出任务。
func scanTaskFromRows(rows *sql.Rows) (port.ScanTask, error) {
	var t port.ScanTask
	var scope, summary, errMsg, started, finished, deleted sql.NullString
	if err := rows.Scan(&t.ID, &t.DataSourceID, &scope, &t.Status, &t.RuleVersion, &t.SampleRows, &t.BatchSize,
		&t.Concurrency, &t.LowPeakWindow, &t.TriggeredBy, &summary, &errMsg, &started, &finished,
		&t.CreatedAt, &t.UpdatedAt, &deleted); err != nil {
		return port.ScanTask{}, err
	}
	t.Scope = scope.String
	t.Summary = summary.String
	t.ErrorMessage = errMsg.String
	t.StartedAt = started.String
	t.FinishedAt = finished.String
	return t, nil
}
