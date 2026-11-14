package port

import "time"

// ===== 元数据与采样值类型 =====

// DataSourceConfig 运行时数据源配置（明文仅内存瞬时，禁落库/日志/报告）。
type DataSourceConfig struct {
	ID           string
	Name         string
	Dialect      Dialect
	Host         string // 明文（内存瞬时）
	Port         int
	ServiceName  string
	Username     string
	Password     string // 明文（内存瞬时），禁落库/日志/报告
	DSN          string // 可选完整连接串（内存瞬时）
	MaxOpenConns int
	MaxIdleConns int
	QueryTimeout time.Duration
}

// ScanScope 扫描范围过滤（逗号分隔；空表示默认：全部非系统模式）。
type ScanScope struct {
	IncludeSchemas string // 仅扫描这些模式；空=全部非系统
	ExcludeTables  string // 跳过匹配表
	ExcludeColumns string // 跳过匹配字段
}

// Schema 模式（库）元数据。
type Schema struct {
	Name     string
	IsSystem bool
}

// ColumnMeta 字段（列）元数据。
type ColumnMeta struct {
	SchemaName  string
	TableName   string
	ColumnName  string
	DataType    string
	Comment     string
	DataLength  int
	Nullable    bool
	IsEncrypted bool
	IsBinary    bool
}

// TableStats 表统计与分页依据。
type TableStats struct {
	SchemaName string
	TableName  string
	EstRows    int64
	PrimaryKey string // 空表示无主键
	IsLarge    bool
	IsHuge     bool
}

// SampleQuery 主键范围分页采样请求。
type SampleQuery struct {
	Schema     string
	Table      string
	Columns    []string
	PrimaryKey string // 空表示无主键，退化为受限行数扫描
	LastKey    string // 空表示从头
	BatchSize  int
	MaxTextLen int
}

// RowBatch 一批采样结果。
type RowBatch struct {
	Columns []string
	Values  [][]string // 每行各列字符串值；文本截断 MaxTextLen，二进制为空串
	LastKey string
	HasMore bool
}

// ===== 识别引擎值类型 =====

// Candidate 字节扫描候选片段。Raw 仅内存瞬时。
type Candidate struct {
	Raw        string
	Candidates []string
	Offset     int
	Length     int
}

// Hit 单次命中结果（不含明文原值）。
type Hit struct {
	RuleID        string
	SensitiveType string
	Confidence    string // high|medium|low
	MaskedValue   string // 仅掩码形态，不含完整明文
	SourceForm    string // plaintext|hash|masked|encrypted
	VerifyPassed  bool
}

// ===== 风险与置信度 =====

// RiskLevel 风险等级。
type RiskLevel string

const (
	RiskHigh          RiskLevel = "high"
	RiskMediumHigh    RiskLevel = "medium_high"
	RiskMedium        RiskLevel = "medium"
	RiskMediumLow     RiskLevel = "medium_low"
	RiskLow           RiskLevel = "low"
	RiskPendingReview RiskLevel = "pending_review"
)

// Confidence 置信度。
type Confidence string

const (
	ConfHigh   Confidence = "high"
	ConfMedium Confidence = "medium"
	ConfLow    Confidence = "low"
)

// ===== 字段台账 =====

// ColumnFinding 字段台账核心结果。
type ColumnFinding struct {
	ID             string
	TaskID         string
	DataSourceID   string
	ColumnMetaID   string
	SchemaName     string
	TableName      string
	ColumnName     string
	DataType       string
	ColumnComment  string
	SensitiveTypes []string
	SampleTotal    int
	SampleHit      int
	HitRatio       float64
	VerifyPassed   bool
	Confidence     string
	StorageForm    string
	RiskLevel      string
	ReviewStatus   string
	SampleMasked   string // 脱敏样例，不含完整明文
	Hits           []Hit
	CreatedAt      string
	UpdatedAt      string
}

// ===== 通用分页 =====

// Page 通用分页结果。
type Page[T any] struct {
	Items   []T
	Total   int64
	Page    int
	Limit   int
	HasMore bool
}

// ===== 报告 =====

// ReportFormat 报告格式。
type ReportFormat string

const (
	FormatExcel ReportFormat = "excel"
	FormatHTML  ReportFormat = "html"
	FormatJSON  ReportFormat = "json"
)

// ReportBundle 报告数据包，三种格式共享。
type ReportBundle struct {
	Task        ScanTask
	Findings    []ColumnFinding
	Rules       []Rule
	Reviews     []ReviewRecord
	Checkpoints []ScanCheckpoint
}

// ===== 加解密 =====

// Algorithm 加密算法标签。
type Algorithm byte

const (
	AlgAES256GCM Algorithm = 0x00 // AES-256-GCM，密钥用完整 32 字节
	AlgSM4GCM    Algorithm = 0x01 // SM4-GCM，密钥用前 16 字节
)

// ===== 日志 =====

// LogLevel 日志级别。
type LogLevel string

const (
	LevelTRACE LogLevel = "TRACE"
	LevelDEBUG LogLevel = "DEBUG"
	LevelINFO  LogLevel = "INFO"
	LevelWARN  LogLevel = "WARN"
	LevelERROR LogLevel = "ERROR"
)

// Field 结构化字段；值写入前统一脱敏。
type Field struct {
	Key   string
	Value string
}
