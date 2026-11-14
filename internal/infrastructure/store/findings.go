package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"dbdlp/internal/domain/port"
)

// 本文件实现元数据、字段台账与命中明细的持久化。

// SaveMetadata 保存扫描元数据（模式／表／字段）。
//
// 幂等：同一任务重复保存不会产生重复行——先复用该任务已存在的模式 ID 与表 ID，
// 字段行按 (task_id, table_meta_id, column_name) 唯一约束更新。
// 续扫场景需要重复保存元数据，这是幂等性的前提。
func (s *SQLiteStore) SaveMetadata(ctx context.Context, taskID string, schemas []port.Schema, tables []port.TableMeta, columns []port.ColumnMeta) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	schemaIDs, err := loadSchemaIDs(ctx, tx, taskID)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	for _, sc := range schemas {
		if _, ok := schemaIDs[sc.Name]; ok {
			continue
		}
		id := newID()
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_meta (id,task_id,data_source_id,schema_name,is_system,table_count,created_at,updated_at)
			VALUES (?,?,?,?,?,?,?,?)`, id, taskID, "", sc.Name, boolToInt(sc.IsSystem), 0, nowText(), nowText()); err != nil {
			_ = tx.Rollback()
			return err
		}
		schemaIDs[sc.Name] = id
	}
	tableIDs, err := loadTableIDs(ctx, tx, taskID)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	for _, tb := range tables {
		sid := schemaIDs[tb.SchemaName]
		key := sid + "/" + tb.TableName
		if _, ok := tableIDs[key]; ok {
			continue
		}
		id := newID()
		if _, err := tx.ExecContext(ctx, `INSERT INTO table_meta (id,task_id,schema_meta_id,table_name,table_comment,est_rows,primary_key,is_large,is_huge,degraded,created_at,updated_at)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`, id, taskID, sid, tb.TableName, tb.TableComment, tb.EstRows, tb.PrimaryKey, boolToInt(tb.IsLarge), boolToInt(tb.IsHuge), boolToInt(tb.Degraded), nowText(), nowText()); err != nil {
			_ = tx.Rollback()
			return err
		}
		tableIDs[key] = id
	}
	for _, c := range columns {
		tid := tableIDs[c.SchemaName+"/"+c.TableName]
		if _, err := tx.ExecContext(ctx, `INSERT INTO column_meta (id,task_id,table_meta_id,column_name,data_type,data_length,column_comment,nullable,is_encrypted,is_binary,scan_state,created_at,updated_at)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)
			ON CONFLICT(task_id,table_meta_id,column_name) DO UPDATE SET data_type=excluded.data_type,data_length=excluded.data_length,
			column_comment=excluded.column_comment,nullable=excluded.nullable,is_encrypted=excluded.is_encrypted,is_binary=excluded.is_binary,updated_at=excluded.updated_at`,
			newID(), taskID, tid, c.ColumnName, c.DataType, c.DataLength, c.Comment, boolToInt(c.Nullable), boolToInt(c.IsEncrypted), boolToInt(c.IsBinary), "pending", nowText(), nowText()); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// loadSchemaIDs 读取任务已存在的模式 ID（模式名 → ID）。
func loadSchemaIDs(ctx context.Context, tx *sql.Tx, taskID string) (map[string]string, error) {
	out := map[string]string{}
	rows, err := tx.QueryContext(ctx, `SELECT id, schema_name FROM schema_meta WHERE task_id=?`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		out[name] = id
	}
	return out, rows.Err()
}

// loadTableIDs 读取任务已存在的表 ID（模式 ID／表名 → ID）。
func loadTableIDs(ctx context.Context, tx *sql.Tx, taskID string) (map[string]string, error) {
	out := map[string]string{}
	rows, err := tx.QueryContext(ctx, `SELECT id,schema_meta_id,table_name FROM table_meta WHERE task_id=?`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, sid, name string
		if err := rows.Scan(&id, &sid, &name); err != nil {
			return nil, err
		}
		out[sid+"/"+name] = id
	}
	return out, rows.Err()
}

func (s *SQLiteStore) SaveFindings(ctx context.Context, findings []port.ColumnFinding) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	for _, f := range findings {
		if f.TaskID == "" {
			_ = tx.Rollback()
			return fmt.Errorf("字段台账缺少 task_id")
		}
		findingID := f.ID
		if findingID == "" {
			findingID = newID()
		}
		reviewStatus := f.ReviewStatus
		if reviewStatus == "" {
			reviewStatus = "pending"
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO finding_column
			(id,task_id,column_meta_id,data_source_id,schema_name,table_name,column_name,data_type,column_comment,sensitive_types,
			 sample_total,sample_hit,hit_ratio,verify_passed,confidence,storage_form,risk_level,review_status,sample_masked,is_encrypted,is_huge,note,created_at,updated_at)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			findingID, f.TaskID, f.ColumnMetaID, f.DataSourceID, f.SchemaName, f.TableName, f.ColumnName, f.DataType, f.ColumnComment,
			strings.Join(f.SensitiveTypes, ","), f.SampleTotal, f.SampleHit, f.HitRatio, boolToInt(f.VerifyPassed), f.Confidence,
			f.StorageForm, f.RiskLevel, reviewStatus, f.SampleMasked, boolToInt(false), boolToInt(false), "", nowText(), nowText())
		if err != nil {
			_ = tx.Rollback()
			return err
		}
		for _, h := range f.Hits {
			if _, err := tx.ExecContext(ctx, `INSERT INTO finding_hit
				(id,finding_id,task_id,rule_id,sensitive_type,verify_passed,confidence,masked_value,source_form,created_at,updated_at)
				VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
				newID(), findingID, f.TaskID, h.RuleID, h.SensitiveType, boolToInt(h.VerifyPassed), h.Confidence, h.MaskedValue, h.SourceForm, nowText(), nowText()); err != nil {
				_ = tx.Rollback()
				return err
			}
		}
	}
	return tx.Commit()
}

func (s *SQLiteStore) ListFindings(ctx context.Context, f port.FindingFilter) (port.Page[port.ColumnFinding], error) {
	where := "WHERE 1=1"
	args := []any{}
	if f.TaskID != "" {
		where += " AND task_id=?"
		args = append(args, f.TaskID)
	}
	if f.RiskLevel != "" {
		where += " AND risk_level=?"
		args = append(args, string(f.RiskLevel))
	}
	if f.ReviewStatus != "" {
		where += " AND review_status=?"
		args = append(args, f.ReviewStatus)
	}
	if f.Keyword != "" {
		where += " AND (schema_name LIKE ? OR table_name LIKE ? OR column_name LIKE ?)"
		args = append(args, "%"+f.Keyword+"%", "%"+f.Keyword+"%", "%"+f.Keyword+"%")
	}
	var total int64
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM finding_column "+where, args...).Scan(&total); err != nil {
		return port.Page[port.ColumnFinding]{}, err
	}
	limit, offset := pageLimitOffset(f.Page, f.Limit)
	args = append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx, `SELECT id,task_id,column_meta_id,data_source_id,schema_name,table_name,column_name,data_type,column_comment,sensitive_types,
		sample_total,sample_hit,hit_ratio,verify_passed,confidence,storage_form,risk_level,review_status,sample_masked FROM finding_column `+where+` ORDER BY risk_level, schema_name, table_name LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return port.Page[port.ColumnFinding]{}, err
	}
	defer rows.Close()
	var items []port.ColumnFinding
	for rows.Next() {
		var f port.ColumnFinding
		var sens, cm string
		var vp int
		if err := rows.Scan(&f.ID, &f.TaskID, &f.ColumnMetaID, &f.DataSourceID, &f.SchemaName, &f.TableName, &f.ColumnName, &f.DataType, &cm, &sens,
			&f.SampleTotal, &f.SampleHit, &f.HitRatio, &vp, &f.Confidence, &f.StorageForm, &f.RiskLevel, &f.ReviewStatus, &f.SampleMasked); err != nil {
			return port.Page[port.ColumnFinding]{}, err
		}
		f.ColumnComment = cm
		f.VerifyPassed = vp != 0
		if sens != "" {
			f.SensitiveTypes = strings.Split(sens, ",")
		}
		items = append(items, f)
	}
	return port.Page[port.ColumnFinding]{Items: items, Total: total, Page: f.Page, Limit: limit, HasMore: offset+int64(len(items)) < total}, nil
}

func (s *SQLiteStore) ListHits(ctx context.Context, findingID string, p port.Pagination) (port.Page[port.Hit], error) {
	var total int64
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM finding_hit WHERE finding_id=?", findingID).Scan(&total); err != nil {
		return port.Page[port.Hit]{}, err
	}
	limit, offset := pageLimitOffset(p.Page, p.Limit)
	rows, err := s.db.QueryContext(ctx, `SELECT id,finding_id,task_id,rule_id,sensitive_type,verify_passed,confidence,masked_value,source_form
		FROM finding_hit WHERE finding_id=? LIMIT ? OFFSET ?`, findingID, limit, offset)
	if err != nil {
		return port.Page[port.Hit]{}, err
	}
	defer rows.Close()
	var items []port.Hit
	for rows.Next() {
		var h port.Hit
		var id, fid, tid, ruleID, sens, conf, masked, source string
		var vp int
		if err := rows.Scan(&id, &fid, &tid, &ruleID, &sens, &vp, &conf, &masked, &source); err != nil {
			return port.Page[port.Hit]{}, err
		}
		h.RuleID = ruleID
		h.SensitiveType = sens
		h.Confidence = conf
		h.MaskedValue = masked
		h.SourceForm = source
		h.VerifyPassed = vp != 0
		items = append(items, h)
	}
	return port.Page[port.Hit]{Items: items, Total: total, Page: p.Page, Limit: limit, HasMore: offset+int64(len(items)) < total}, nil
}
