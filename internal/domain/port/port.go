// Package port 定义 dbdlp 领域核心契约（接口与值类型）。
// 本包不依赖任何内部包，是分层依赖的最内层；基础设施层实现这些接口，
// 应用层与接口层只依赖本包，保证依赖方向只向内。
//
// 文件职责：
//   - port.go  本文件：端口接口（依赖倒置的接缝），供基础设施层实现；
//   - values.go：值类型与实体，供各层按值传递。
package port

import (
	"context"
	"time"
)

// ===== 方言 =====

// Dialect 被监控数据库类型。首批 Oracle 与 Dameng，余为可扩展枚举。
type Dialect string

const (
	DialectOracle    Dialect = "oracle"
	DialectDameng    Dialect = "dameng"
	DialectMySQL     Dialect = "mysql"
	DialectPostgres  Dialect = "postgres"
	DialectOpenGauss Dialect = "opengauss"
	// DialectSQLite 仅供内部端到端测试使用，非产品方言。
	DialectSQLite Dialect = "sqlite"
)

// ===== 只读连接 =====

// ReadOnlyConn 只读连接句柄，封装 *sql.DB 与连接池，禁暴露写方法。
type ReadOnlyConn interface {
	Ping(ctx context.Context) error
	Query(ctx context.Context, q string, args ...any) (Rows, error) // 禁 Exec / DDL
	Close() error
}

// Rows 流式结果集抽象，逐行 Scan，避免一次性加载大结果集。
type Rows interface {
	Next() bool
	Scan(dest ...any) error
	Columns() ([]string, error)
	Err() error
	Close() error
}

// ===== 驱动端口 =====

// Driver 统一数据源驱动端口；上层采集/采样/聚合只依赖本接口。
type Driver interface {
	Dialect() Dialect
	Connect(ctx context.Context, src DataSourceConfig) (ReadOnlyConn, error)
	ListSchemas(ctx context.Context, conn ReadOnlyConn) ([]Schema, error)
	ListColumns(ctx context.Context, conn ReadOnlyConn, schema string) ([]ColumnMeta, error)
	TableStats(ctx context.Context, conn ReadOnlyConn, schema, table string) (TableStats, error)
	SampleRows(ctx context.Context, conn ReadOnlyConn, q SampleQuery) (RowBatch, error)
	Placeholder(i int) string // Oracle :1 / PG $1 / MySQL·DM ? / sqlite ?
	QuoteIdentifier(name string) string
}

// ===== 识别引擎接口 =====

// Detector 敏感识别引擎，四层流水线：字节扫描→正则→校验器→自定义规则。
type Detector interface {
	DetectValue(ctx context.Context, value string) ([]Hit, error)
	DetectRow(ctx context.Context, kv map[string]string) ([]Hit, error)
	RuleVersion() string
	Close() error
}

// Verifier 候选值校验器（注册式）。
type Verifier interface {
	Name() string
	Verify(candidate string) bool
}

// ===== 聚合 =====

// Aggregator 聚合：按字段汇总命中并分级、算置信度。
type Aggregator interface {
	Aggregate(ctx context.Context, meta ColumnMeta, sampleTotal int, hits []Hit) (ColumnFinding, error)
	ClassifyRisk(ctx context.Context, f ColumnFinding) (RiskLevel, error)
	ComputeConfidence(ctx context.Context, f ColumnFinding) (Confidence, error)
}

// ===== 存储 =====

// Store 持久化端口。
type Store interface {
	// 数据源
	CreateDataSource(ctx context.Context, d DataSource) error
	UpdateDataSource(ctx context.Context, d DataSource) error
	DeleteDataSource(ctx context.Context, id string) error
	GetDataSource(ctx context.Context, id string) (DataSource, error)
	ListDataSources(ctx context.Context, f DataSourceFilter) (Page[DataSource], error)
	BatchCreateDataSources(ctx context.Context, items []DataSource) error // 整批事务
	// GetDataSourceByName 按名称查数据源（含软删除行，用于名称占用判定）。
	GetDataSourceByName(ctx context.Context, name string) (DataSource, error)
	// 任务与断点
	CreateTask(ctx context.Context, t ScanTask) error
	GetTask(ctx context.Context, id string) (ScanTask, error)
	UpdateTaskStatus(ctx context.Context, id string, status TaskStatus, summary TaskSummary) error
	// UpdateTaskState 仅迁移任务状态与错误信息，不覆盖既有 summary。
	UpdateTaskState(ctx context.Context, id string, status TaskStatus, errorMessage string) error
	SaveCheckpoint(ctx context.Context, cp ScanCheckpoint) error
	ListPendingCheckpoints(ctx context.Context, taskID string) ([]ScanCheckpoint, error)
	// ListCheckpoints 列出任务的全部断点（含 done），供断点续扫识别已完成的表。
	ListCheckpoints(ctx context.Context, taskID string) ([]ScanCheckpoint, error)
	// ListTasks 按条件分页查询任务列表。
	ListTasks(ctx context.Context, f TaskFilter) (Page[ScanTask], error)
	// CountRunningTasksBySource 统计某数据源下处于运行中的任务数（删除前引用检查用）。
	CountRunningTasksBySource(ctx context.Context, dataSourceID string) (int, error)
	// CountTaskProgress 统计任务已落库的表数、字段数与已完成断点数。
	CountTaskProgress(ctx context.Context, taskID string) (TaskProgressStats, error)
	// 元数据与结果
	SaveMetadata(ctx context.Context, taskID string, schemas []Schema, tables []TableMeta, columns []ColumnMeta) error
	SaveFindings(ctx context.Context, findings []ColumnFinding) error
	ListFindings(ctx context.Context, f FindingFilter) (Page[ColumnFinding], error)
	// GetFinding 按 ID 查单条字段台账（含掩码样例）。
	GetFinding(ctx context.Context, id string) (ColumnFinding, error)
	// UpdateFindingReviewStatus 更新字段台账的复核状态。
	UpdateFindingReviewStatus(ctx context.Context, id string, status string) error
	ListHits(ctx context.Context, findingID string, p Pagination) (Page[Hit], error)
	// ListSchemasByTask / ListTablesByTask / ListColumnsByTask 按任务读取元数据。
	ListSchemasByTask(ctx context.Context, taskID string) ([]SchemaMetaRow, error)
	ListTablesByTask(ctx context.Context, taskID string) ([]TableMetaRow, error)
	ListColumnsByTask(ctx context.Context, taskID string) ([]ColumnMetaRow, error)
	// 规则/复核/审计/报告
	SaveRuleSnapshot(ctx context.Context, version string, rules []Rule) error
	// ListRuleSnapshots 分页列出规则版本快照摘要。
	ListRuleSnapshots(ctx context.Context, p Pagination) (Page[RuleSnapshot], error)
	// ListRulesByVersion 读取指定规则版本的快照行（仅摘要，正文不落库）。
	ListRulesByVersion(ctx context.Context, version string) ([]Rule, error)
	SaveReview(ctx context.Context, r ReviewRecord) error
	// ListReviews 分页列出复核记录（可按 findingID 过滤）。
	ListReviews(ctx context.Context, findingID string, p Pagination) (Page[ReviewRecord], error)
	AppendAudit(ctx context.Context, a AuditLog) error
	// ListAuditLogs 分页列出审计日志。
	ListAuditLogs(ctx context.Context, f AuditFilter) (Page[AuditLog], error)
	// CountAuditEventsSinceSuccess 统计 actor 的指定动作失败次数（自最近一次成功事件之后）。
	CountAuditEventsSinceSuccess(ctx context.Context, actor, action string) (int, error)
	// LastEventAt 返回 actor 的指定动作最近一次发生时间；无记录返回零值。
	LastEventAt(ctx context.Context, actor, action string) (time.Time, error)
	SaveReportArtifact(ctx context.Context, r ReportArtifact) error
	// ListReports 分页列出报告产物（可按 taskID 过滤）。
	ListReports(ctx context.Context, taskID string, p Pagination) (Page[ReportArtifact], error)
	// GetReportArtifact 按 ID 查报告产物。
	GetReportArtifact(ctx context.Context, id string) (ReportArtifact, error)
	// CountFindingsByRisk / ByType / BySchema 按维度统计字段台账（仪表盘用）。
	CountFindingsByRisk(ctx context.Context, taskID string) (map[string]int, error)
	CountFindingsByType(ctx context.Context, taskID string) (map[string]int, error)
	CountFindingsBySchema(ctx context.Context, taskID string) (map[string]int, error)
	// 会话
	GetUserByName(ctx context.Context, username string) (WebUser, error)
	// GetUserByID 按 ID 查控制台用户。
	GetUserByID(ctx context.Context, id string) (WebUser, error)
	// UpdateUserLastLogin 更新用户最近登录时间。
	UpdateUserLastLogin(ctx context.Context, id string) error
	CreateSession(ctx context.Context, s WebSession) error
	GetValidSession(ctx context.Context, tokenHash string, now time.Time) (WebSession, error)
	DeleteSession(ctx context.Context, id string) error
	// 迁移
	Migrate(ctx context.Context) error
}

// ===== 报告 =====

// Reporter 报告导出器。
type Reporter interface {
	Format() ReportFormat
	Export(ctx context.Context, bundle ReportBundle, outDir string) (filePath string, err error)
}

// ===== 加解密 =====

// Cipher 对称加解密；自动识别 ENC() 包裹与算法标签；非 ENC() 按明文返回。
type Cipher interface {
	Encrypt(plaintext string, alg Algorithm) (string, error)
	Decrypt(stored string) (string, error)
	IsEncrypted(s string) bool
}

// KeyStore 密钥存储：首次生成 32 字节，0600 写入；失败必须返回错误。
type KeyStore interface {
	LoadOrInit() ([]byte, error)
	Path() string
}

// ===== 日志 =====

// Logger 统一日志接口（中文、UTF-8、统一脱敏）。
type Logger interface {
	Trace(ctx context.Context, msg string, fields ...Field)
	Debug(ctx context.Context, msg string, fields ...Field)
	Info(ctx context.Context, msg string, fields ...Field)
	Warn(ctx context.Context, msg string, fields ...Field)
	Error(ctx context.Context, msg string, fields ...Field)
	With(fields ...Field) Logger
	Close() error
}
