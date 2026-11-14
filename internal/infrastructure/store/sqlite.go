// Package store SQLite 持久化实现（modernc.org/sqlite，纯 Go、无 CGO）。
// 对应 docs/02 §10.3 的 14 张表；SQLite 单写者（max_open_conns=1）。
package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// ddl 14 张表建表语句（与 docs/02 §10.3 完全一致）。
const ddl = `
PRAGMA journal_mode = WAL;
PRAGMA foreign_keys = ON;
CREATE TABLE IF NOT EXISTS data_source (
    id TEXT PRIMARY KEY, name TEXT NOT NULL UNIQUE, dialect TEXT NOT NULL,
    host_enc TEXT NOT NULL, port INTEGER NOT NULL, service_name TEXT,
    username TEXT NOT NULL, password_enc TEXT NOT NULL, dsn_enc TEXT,
    include_schemas TEXT, exclude_tables TEXT, exclude_columns TEXT,
    conn_max_open INTEGER NOT NULL DEFAULT 2, conn_max_idle INTEGER NOT NULL DEFAULT 1,
    conn_timeout_ms INTEGER NOT NULL DEFAULT 10000, query_timeout_ms INTEGER NOT NULL DEFAULT 30000,
    last_test_at TEXT, last_test_ok INTEGER NOT NULL DEFAULT 0, enabled INTEGER NOT NULL DEFAULT 1,
    remark TEXT, created_at TEXT NOT NULL, updated_at TEXT NOT NULL, deleted_at TEXT);
CREATE INDEX IF NOT EXISTS idx_data_source_dialect ON data_source(dialect);
CREATE INDEX IF NOT EXISTS idx_data_source_deleted ON data_source(deleted_at);
CREATE TABLE IF NOT EXISTS scan_task (
    id TEXT PRIMARY KEY, data_source_id TEXT NOT NULL, scope TEXT, status TEXT NOT NULL, rule_version TEXT NOT NULL,
    sample_rows INTEGER NOT NULL DEFAULT 20, batch_size INTEGER NOT NULL DEFAULT 20,
    concurrency INTEGER NOT NULL DEFAULT 1, low_peak_window TEXT, triggered_by TEXT NOT NULL,
    summary TEXT, error_message TEXT, started_at TEXT, finished_at TEXT,
    created_at TEXT NOT NULL, updated_at TEXT NOT NULL, deleted_at TEXT);
CREATE INDEX IF NOT EXISTS idx_scan_task_status ON scan_task(status);
CREATE INDEX IF NOT EXISTS idx_scan_task_source ON scan_task(data_source_id);
CREATE TABLE IF NOT EXISTS schema_meta (
    id TEXT PRIMARY KEY, task_id TEXT NOT NULL, data_source_id TEXT NOT NULL, schema_name TEXT NOT NULL,
    is_system INTEGER NOT NULL DEFAULT 0, table_count INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL, updated_at TEXT NOT NULL, UNIQUE(task_id, schema_name));
CREATE INDEX IF NOT EXISTS idx_schema_meta_task ON schema_meta(task_id);
CREATE TABLE IF NOT EXISTS table_meta (
    id TEXT PRIMARY KEY, task_id TEXT NOT NULL, schema_meta_id TEXT NOT NULL, table_name TEXT NOT NULL,
    table_comment TEXT, est_rows INTEGER NOT NULL DEFAULT 0, primary_key TEXT,
    is_large INTEGER NOT NULL DEFAULT 0, is_huge INTEGER NOT NULL DEFAULT 0, degraded INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL, updated_at TEXT NOT NULL, UNIQUE(task_id, schema_meta_id, table_name));
CREATE INDEX IF NOT EXISTS idx_table_meta_task ON table_meta(task_id);
CREATE TABLE IF NOT EXISTS column_meta (
    id TEXT PRIMARY KEY, task_id TEXT NOT NULL, table_meta_id TEXT NOT NULL, column_name TEXT NOT NULL,
    data_type TEXT NOT NULL, data_length INTEGER, column_comment TEXT,
    nullable INTEGER NOT NULL DEFAULT 1, is_encrypted INTEGER NOT NULL DEFAULT 0, is_binary INTEGER NOT NULL DEFAULT 0,
    scan_state TEXT NOT NULL DEFAULT 'pending', skip_reason TEXT, created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
    UNIQUE(task_id, table_meta_id, column_name));
CREATE INDEX IF NOT EXISTS idx_column_meta_task ON column_meta(task_id);
CREATE TABLE IF NOT EXISTS finding_column (
    id TEXT PRIMARY KEY, task_id TEXT NOT NULL, column_meta_id TEXT NOT NULL, data_source_id TEXT NOT NULL,
    schema_name TEXT NOT NULL, table_name TEXT NOT NULL, column_name TEXT NOT NULL,
    data_type TEXT, column_comment TEXT, sensitive_types TEXT, sample_total INTEGER NOT NULL DEFAULT 0,
    sample_hit INTEGER NOT NULL DEFAULT 0, hit_ratio REAL NOT NULL DEFAULT 0, verify_passed INTEGER NOT NULL DEFAULT 0,
    confidence TEXT, storage_form TEXT, risk_level TEXT, review_status TEXT NOT NULL DEFAULT 'pending', sample_masked TEXT,
    is_encrypted INTEGER NOT NULL DEFAULT 0, is_huge INTEGER NOT NULL DEFAULT 0, note TEXT,
    created_at TEXT NOT NULL, updated_at TEXT NOT NULL, deleted_at TEXT);
CREATE INDEX IF NOT EXISTS idx_finding_task ON finding_column(task_id);
CREATE INDEX IF NOT EXISTS idx_finding_risk ON finding_column(risk_level);
CREATE INDEX IF NOT EXISTS idx_finding_source ON finding_column(data_source_id);
CREATE TABLE IF NOT EXISTS finding_hit (
    id TEXT PRIMARY KEY, finding_id TEXT NOT NULL, task_id TEXT NOT NULL, rule_id TEXT NOT NULL,
    sensitive_type TEXT NOT NULL, verify_passed INTEGER NOT NULL DEFAULT 0, confidence TEXT,
    masked_value TEXT NOT NULL, source_form TEXT, created_at TEXT NOT NULL, updated_at TEXT NOT NULL);
CREATE INDEX IF NOT EXISTS idx_hit_finding ON finding_hit(finding_id);
CREATE INDEX IF NOT EXISTS idx_hit_task ON finding_hit(task_id);
CREATE INDEX IF NOT EXISTS idx_hit_rule ON finding_hit(rule_id);
CREATE TABLE IF NOT EXISTS rule_snapshot (
    id TEXT PRIMARY KEY, rule_version TEXT NOT NULL, rule_id TEXT NOT NULL, sensitive_type TEXT NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 1, custom INTEGER NOT NULL DEFAULT 0, regex_digest TEXT, verifier TEXT,
    threshold REAL NOT NULL DEFAULT 0.5, created_at TEXT NOT NULL, updated_at TEXT NOT NULL, UNIQUE(rule_version, rule_id));
CREATE TABLE IF NOT EXISTS review_record (
    id TEXT PRIMARY KEY, finding_id TEXT NOT NULL, reviewer TEXT NOT NULL, conclusion TEXT NOT NULL, comment TEXT, reviewed_at TEXT NOT NULL,
    created_at TEXT NOT NULL, updated_at TEXT NOT NULL);
CREATE INDEX IF NOT EXISTS idx_review_finding ON review_record(finding_id);
CREATE TABLE IF NOT EXISTS audit_log (
    id TEXT PRIMARY KEY, actor TEXT NOT NULL, action TEXT NOT NULL, target_type TEXT, target_id TEXT,
    detail TEXT, result TEXT NOT NULL, error_message TEXT, ip_masked TEXT, created_at TEXT NOT NULL, updated_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS web_user (
    id TEXT PRIMARY KEY, username TEXT NOT NULL UNIQUE, password_hash TEXT NOT NULL, display_name TEXT, role TEXT NOT NULL DEFAULT 'admin',
    enabled INTEGER NOT NULL DEFAULT 1, last_login_at TEXT, created_at TEXT NOT NULL, updated_at TEXT NOT NULL, deleted_at TEXT);
CREATE TABLE IF NOT EXISTS web_session (
    id TEXT PRIMARY KEY, user_id TEXT NOT NULL, token_hash TEXT NOT NULL UNIQUE, expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL, updated_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS scan_checkpoint (
    id TEXT PRIMARY KEY, task_id TEXT NOT NULL, table_meta_id TEXT NOT NULL, column_name TEXT NOT NULL,
    last_key TEXT, done_rows INTEGER NOT NULL DEFAULT 0, state TEXT NOT NULL, error_message TEXT,
    created_at TEXT NOT NULL, updated_at TEXT NOT NULL, UNIQUE(task_id, table_meta_id, column_name));
CREATE TABLE IF NOT EXISTS report_artifact (
    id TEXT PRIMARY KEY, task_id TEXT NOT NULL, format TEXT NOT NULL, file_path TEXT NOT NULL, file_size INTEGER, generated_by TEXT NOT NULL,
    created_at TEXT NOT NULL, updated_at TEXT NOT NULL);
`

// SQLiteStore port.Store 的 SQLite 实现。
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore 打开 SQLite 并执行迁移。
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=synchronous(NORMAL)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开本地存储失败：%w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	s := &SQLiteStore{db: db}
	if err := s.Migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// DB 暴露底层 *sql.DB（供测试与其他存储适配复用）。
func (s *SQLiteStore) DB() *sql.DB { return s.db }

// Migrate 执行建表（幂等）。
func (s *SQLiteStore) Migrate(_ context.Context) error {
	for _, stmt := range strings.Split(ddl, ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" || strings.HasPrefix(stmt, "PRAGMA") {
			if stmt != "" {
				if _, err := s.db.Exec(stmt); err != nil {
					return fmt.Errorf("执行 PRAGMA 失败：%w", err)
				}
			}
			continue
		}
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("建表失败：%w", err)
		}
	}
	return nil
}

// newID 生成唯一标识（时间纳秒 + 随机十六进制）。
func newID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), hex.EncodeToString(b))
}

func nowText() string { return time.Now().Format("2006-01-02 15:04:05.000") }

// boolToInt / intToBool 布尔与 INTEGER 互转。
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (s *SQLiteStore) Close() error { return s.db.Close() }
