package store

import (
	"context"
	"database/sql"
	"encoding/json"

	"dbdlp/internal/domain/port"
)

// 本文件实现扫描任务与断点的持久化。

func (s *SQLiteStore) CreateTask(ctx context.Context, t port.ScanTask) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO scan_task
		(id,data_source_id,scope,status,rule_version,sample_rows,batch_size,concurrency,low_peak_window,triggered_by,summary,error_message,started_at,finished_at,created_at,updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		t.ID, t.DataSourceID, t.Scope, string(t.Status), t.RuleVersion, t.SampleRows, t.BatchSize, t.Concurrency,
		t.LowPeakWindow, t.TriggeredBy, t.Summary, t.ErrorMessage, t.StartedAt, t.FinishedAt, nowText(), nowText())
	return err
}

func (s *SQLiteStore) GetTask(ctx context.Context, id string) (port.ScanTask, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id,data_source_id,scope,status,rule_version,sample_rows,batch_size,concurrency,low_peak_window,triggered_by,summary,error_message,started_at,finished_at,created_at,updated_at,deleted_at
		FROM scan_task WHERE id=?`, id)
	var t port.ScanTask
	var scope, summary, errMsg, started, finished, deleted sql.NullString
	if err := row.Scan(&t.ID, &t.DataSourceID, &scope, &t.Status, &t.RuleVersion, &t.SampleRows, &t.BatchSize, &t.Concurrency,
		&t.LowPeakWindow, &t.TriggeredBy, &summary, &errMsg, &started, &finished, &t.CreatedAt, &t.UpdatedAt, &deleted); err != nil {
		return port.ScanTask{}, err
	}
	t.Scope = scope.String
	t.Summary = summary.String
	t.ErrorMessage = errMsg.String
	t.StartedAt = started.String
	t.FinishedAt = finished.String
	return t, nil
}

func (s *SQLiteStore) UpdateTaskStatus(ctx context.Context, id string, status port.TaskStatus, summary port.TaskSummary) error {
	b, _ := json.Marshal(summary)
	var finished *string
	if status == port.TaskStatusDone || status == port.TaskStatusFailed || status == port.TaskStatusPartial {
		v := nowText()
		finished = &v
	}
	_, err := s.db.ExecContext(ctx, `UPDATE scan_task SET status=?,summary=?,finished_at=?,updated_at=? WHERE id=?`,
		string(status), string(b), finished, nowText(), id)
	return err
}

func (s *SQLiteStore) SaveCheckpoint(ctx context.Context, cp port.ScanCheckpoint) error {
	if cp.ID == "" {
		cp.ID = newID()
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO scan_checkpoint
		(id,task_id,table_meta_id,column_name,last_key,done_rows,state,error_message,created_at,updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(task_id,table_meta_id,column_name) DO UPDATE SET last_key=excluded.last_key,done_rows=excluded.done_rows,state=excluded.state,error_message=excluded.error_message,updated_at=excluded.updated_at`,
		cp.ID, cp.TaskID, cp.TableMetaID, cp.ColumnName, cp.LastKey, cp.DoneRows, cp.State, cp.ErrorMessage, nowText(), nowText())
	return err
}

func (s *SQLiteStore) ListPendingCheckpoints(ctx context.Context, taskID string) ([]port.ScanCheckpoint, error) {
	return s.queryCheckpoints(ctx, `SELECT id,task_id,table_meta_id,column_name,last_key,done_rows,state,error_message FROM scan_checkpoint WHERE task_id=? AND state<>'done'`, taskID)
}

// ListCheckpoints 列出任务的全部断点（含已完成），供断点续扫识别已完成的表。
func (s *SQLiteStore) ListCheckpoints(ctx context.Context, taskID string) ([]port.ScanCheckpoint, error) {
	return s.queryCheckpoints(ctx, `SELECT id,task_id,table_meta_id,column_name,last_key,done_rows,state,error_message FROM scan_checkpoint WHERE task_id=?`, taskID)
}

// queryCheckpoints 按给定 SQL 查询断点并映射为值对象。
func (s *SQLiteStore) queryCheckpoints(ctx context.Context, query, taskID string) ([]port.ScanCheckpoint, error) {
	rows, err := s.db.QueryContext(ctx, query, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []port.ScanCheckpoint
	for rows.Next() {
		var cp port.ScanCheckpoint
		if err := rows.Scan(&cp.ID, &cp.TaskID, &cp.TableMetaID, &cp.ColumnName, &cp.LastKey, &cp.DoneRows, &cp.State, &cp.ErrorMessage); err != nil {
			return nil, err
		}
		out = append(out, cp)
	}
	return out, rows.Err()
}
