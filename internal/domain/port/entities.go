package port

import "time"

// ===== 持久化实体（与 docs/02 §10.3 的 14 张表一一对应） =====

// DataSource 数据源实例（敏感字段以 *_enc 密文存储）。
type DataSource struct {
	ID             string
	Name           string
	Dialect        Dialect
	HostEnc        string // ENC(...) 密文
	Port           int
	ServiceName    string
	Username       string
	PasswordEnc    string // ENC(...) 密文
	DSNEnc         string // ENC(...) 密文，可选
	IncludeSchemas string
	ExcludeTables  string
	ExcludeColumns string
	ConnMaxOpen    int
	ConnMaxIdle    int
	ConnTimeoutMs  int
	QueryTimeoutMs int
	LastTestAt     string
	LastTestOk     bool
	Enabled        bool
	Remark         string
	CreatedAt      string
	UpdatedAt      string
	DeletedAt      string
}

// TaskStatus 任务状态。
type TaskStatus string

const (
	TaskStatusPending TaskStatus = "pending"
	TaskStatusRunning TaskStatus = "running"
	TaskStatusPaused  TaskStatus = "paused"
	TaskStatusDone    TaskStatus = "done"
	TaskStatusFailed  TaskStatus = "failed"
	TaskStatusPartial TaskStatus = "partial"
)

// ScanTask 扫描任务。
type ScanTask struct {
	ID            string
	DataSourceID  string
	Scope         string
	Status        TaskStatus
	RuleVersion   string
	SampleRows    int
	BatchSize     int
	Concurrency   int
	LowPeakWindow string
	TriggeredBy   string
	Summary       string
	ErrorMessage  string
	StartedAt     string
	FinishedAt    string
	CreatedAt     string
	UpdatedAt     string
	DeletedAt     string
}

// TaskSummary 任务结果汇总。
type TaskSummary struct {
	SchemaCount    int
	TableCount     int
	ColumnCount    int
	HitColumnCount int
	FailedCount    int
	Message        string
}

// TableMeta 表元数据。
type TableMeta struct {
	ID           string
	TaskID       string
	SchemaMetaID string
	SchemaName   string
	TableName    string
	TableComment string
	EstRows      int64
	PrimaryKey   string
	IsLarge      bool
	IsHuge       bool
	Degraded     bool
}

// Rule 识别规则。
type Rule struct {
	RuleID        string
	SensitiveType string
	Enabled       bool
	Custom        bool
	Regex         string
	Verifier      string
	Threshold     float64
	Bytescan      BytescanSpec
	Version       string // 规则内容版本（加载时填充）
	RegexDigest   string // 快照摘要：历史版本只落摘要不落正则正文
}

// BytescanSpec 第 1 层字节扫描特征。
type BytescanSpec struct {
	MinLen      int
	MaxLen      int
	DigitsOnly  bool
	PrefixBytes []string
}

// ReviewRecord 复核记录。
type ReviewRecord struct {
	ID         string
	FindingID  string
	Reviewer   string
	Conclusion string
	Comment    string
	ReviewedAt string
	CreatedAt  string
	UpdatedAt  string
}

// AuditLog 审计日志。
type AuditLog struct {
	ID           string
	Actor        string
	Action       string
	TargetType   string
	TargetID     string
	Detail       string
	Result       string
	ErrorMessage string
	IPMasked     string
	CreatedAt    string
	UpdatedAt    string
}

// WebUser 控制台用户。
type WebUser struct {
	ID           string
	Username     string
	PasswordHash string
	DisplayName  string
	Role         string
	Enabled      bool
	LastLoginAt  string
	CreatedAt    string
	UpdatedAt    string
	DeletedAt    string
}

// WebSession 控制台会话。
type WebSession struct {
	ID        string
	UserID    string
	TokenHash string
	ExpiresAt string
	CreatedAt string
	UpdatedAt string
}

// ScanCheckpoint 断点续扫检查点。
type ScanCheckpoint struct {
	ID           string
	TaskID       string
	TableMetaID  string
	ColumnName   string
	LastKey      string
	DoneRows     int
	State        string
	ErrorMessage string
	CreatedAt    string
	UpdatedAt    string
}

// ReportArtifact 报告产物。
type ReportArtifact struct {
	ID          string
	TaskID      string
	Format      string
	FilePath    string
	FileSize    int64
	GeneratedBy string
	CreatedAt   string
	UpdatedAt   string
}

// ===== 过滤与分页 =====

// DataSourceFilter 数据源列表过滤。
type DataSourceFilter struct {
	Dialect Dialect
	Keyword string
	Page    int
	Limit   int
}

// FindingFilter 字段台账过滤。
type FindingFilter struct {
	TaskID       string
	RiskLevel    RiskLevel
	ReviewStatus string
	Keyword      string
	Page         int
	Limit        int
	Sort         string
	Order        string
}

// Pagination 通用分页参数。
type Pagination struct {
	Page  int
	Limit int
}

// TaskFilter 任务列表过滤。
type TaskFilter struct {
	DataSourceID string
	Status       TaskStatus
	Page         int
	Limit        int
}

// AuditFilter 审计日志过滤。
type AuditFilter struct {
	Actor  string
	Action string
	Page   int
	Limit  int
}

// RuleSnapshot 规则版本快照摘要（一版本一行）。
type RuleSnapshot struct {
	Version   string `json:"version"`
	CreatedAt string `json:"createdAt"`
	RuleCount int    `json:"ruleCount"`
}

// SchemaMetaRow 模式（库）元数据行。
type SchemaMetaRow struct {
	ID         string `json:"id"`
	SchemaName string `json:"schemaName"`
	IsSystem   bool   `json:"isSystem"`
	TableCount int    `json:"tableCount"`
}

// TableMetaRow 表元数据行。
type TableMetaRow struct {
	ID           string `json:"id"`
	SchemaName   string `json:"schemaName"`
	TableName    string `json:"tableName"`
	TableComment string `json:"tableComment"`
	EstRows      int64  `json:"estRows"`
	PrimaryKey   string `json:"primaryKey"`
	IsLarge      bool   `json:"isLarge"`
	IsHuge       bool   `json:"isHuge"`
	Degraded     bool   `json:"degraded"`
}

// ColumnMetaRow 字段（列）元数据行。
type ColumnMetaRow struct {
	ID            string `json:"id"`
	TableMetaID   string `json:"tableMetaId"`
	ColumnName    string `json:"columnName"`
	DataType      string `json:"dataType"`
	DataLength    int    `json:"dataLength"`
	ColumnComment string `json:"columnComment"`
	Nullable      bool   `json:"nullable"`
	IsEncrypted   bool   `json:"isEncrypted"`
	IsBinary      bool   `json:"isBinary"`
	ScanState     string `json:"scanState"`
}

// TaskProgressStats 任务进度统计（表/字段/已完成断点数）。
type TaskProgressStats struct {
	Tables          int
	Columns         int
	CheckpointsDone int
}

// Now 统一时间格式（与日志一致）：YYYY-MM-DD HH:MM:SS.SSS。
func NowText() string {
	return time.Now().Format("2006-01-02 15:04:05.000")
}
