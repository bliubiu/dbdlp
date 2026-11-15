package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"dbdlp/internal/domain/mask"
	"dbdlp/internal/domain/port"
)

// 本文件定义对外视图结构：凭据永不出后端，密码恒为占位串，主机按 IP 规则脱敏。

// passwordPlaceholder 密码占位串（固定值，前端不得回显真实凭据）。
const passwordPlaceholder = "******"

// DataSourceView 数据源对外视图。
type DataSourceView struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Dialect        string `json:"dialect"`
	Host           string `json:"host"`
	Port           int    `json:"port"`
	ServiceName    string `json:"serviceName"`
	Username       string `json:"username"`
	Password       string `json:"password"`
	DSN            string `json:"dsn"`
	IncludeSchemas string `json:"includeSchemas"`
	ExcludeTables  string `json:"excludeTables"`
	ExcludeColumns string `json:"excludeColumns"`
	ConnMaxOpen    int    `json:"connMaxOpen"`
	ConnMaxIdle    int    `json:"connMaxIdle"`
	ConnTimeoutMs  int    `json:"connTimeoutMs"`
	QueryTimeoutMs int    `json:"queryTimeoutMs"`
	LastTestAt     string `json:"lastTestAt"`
	LastTestOk     bool   `json:"lastTestOk"`
	Enabled        bool   `json:"enabled"`
	Remark         string `json:"remark"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
}

// newDataSourceView 由领域实体构造对外视图。
// host 为解密后的明文主机名／IP，脱敏后回显；dsn 含凭据，一律不回显。
func (s *Server) newDataSourceView(ds port.DataSource) DataSourceView {
	host := ""
	if s.cipher != nil {
		if plain, err := s.cipher.Decrypt(ds.HostEnc); err == nil {
			host = mask.MaskIPv4(plain)
		}
	}
	return DataSourceView{
		ID: ds.ID, Name: ds.Name, Dialect: string(ds.Dialect), Host: host, Port: ds.Port,
		ServiceName: ds.ServiceName, Username: ds.Username, Password: passwordPlaceholder, DSN: "",
		IncludeSchemas: ds.IncludeSchemas, ExcludeTables: ds.ExcludeTables, ExcludeColumns: ds.ExcludeColumns,
		ConnMaxOpen: ds.ConnMaxOpen, ConnMaxIdle: ds.ConnMaxIdle, ConnTimeoutMs: ds.ConnTimeoutMs,
		QueryTimeoutMs: ds.QueryTimeoutMs, LastTestAt: ds.LastTestAt, LastTestOk: ds.LastTestOk,
		Enabled: ds.Enabled, Remark: ds.Remark, CreatedAt: ds.CreatedAt, UpdatedAt: ds.UpdatedAt,
	}
}

// newDataSourceViews 批量构造视图。
func (s *Server) newDataSourceViews(items []port.DataSource) []DataSourceView {
	out := make([]DataSourceView, 0, len(items))
	for _, ds := range items {
		out = append(out, s.newDataSourceView(ds))
	}
	return out
}

// TaskSummaryView 任务汇总（解析自 summary 字段）。
//
// 前六个字段为后端原始计数（报告与仪表盘消费）；后七个字段为控制台契约字段，
// 与原始计数同源，避免前端再维护一套命名导致「命中列恒为 0」类静默失配。
type TaskSummaryView struct {
	SchemaCount    int    `json:"schemaCount"`
	TableCount     int    `json:"tableCount"`
	ColumnCount    int    `json:"columnCount"`
	HitColumnCount int    `json:"hitColumnCount"`
	FailedCount    int    `json:"failedCount"`
	Message        string `json:"message"`

	TotalTables   int                `json:"totalTables"`
	DoneTables    int                `json:"doneTables"`
	FailedTables  int                `json:"failedTables"`
	SkippedTables int                `json:"skippedTables"`
	TotalColumns  int                `json:"totalColumns"`
	HitColumns    int                `json:"hitColumns"`
	FailedObjects []FailedObjectView `json:"failedObjects,omitempty"`
}

// FailedObjectView 失败表视图；ID 为断点键「模式/表」，可直接回传 retry 的 targetIds。
type FailedObjectView struct {
	ID         string `json:"id"`
	SchemaName string `json:"schemaName"`
	TableName  string `json:"tableName"`
	Reason     string `json:"reason"`
}

// 断点表级状态（与 scan 用例的 checkpointState* 一致；表级断点 column_name 为空）。
const (
	checkpointStateDone   = "done"
	checkpointStateFailed = "failed"
)

// TaskView 扫描任务对外视图。
type TaskView struct {
	ID            string          `json:"id"`
	DataSourceID  string          `json:"dataSourceId"`
	InstanceID    string          `json:"instanceId"`
	Scope         string          `json:"scope"`
	Status        string          `json:"status"`
	RuleVersion   string          `json:"ruleVersion"`
	SampleRows    int             `json:"sampleRows"`
	BatchSize     int             `json:"batchSize"`
	Concurrency   int             `json:"concurrency"`
	LowPeakWindow string          `json:"lowPeakWindow"`
	TriggeredBy   string          `json:"triggeredBy"`
	Summary       TaskSummaryView `json:"summary"`
	ErrorMessage  string          `json:"errorMessage"`
	StartedAt     string          `json:"startedAt"`
	FinishedAt    string          `json:"finishedAt"`
	CreatedAt     string          `json:"createdAt"`
	UpdatedAt     string          `json:"updatedAt"`
}

// newTaskView 由领域实体构造视图；summary 为 JSON 字符串，解析失败不阻断返回。
func newTaskView(t port.ScanTask) TaskView {
	v := TaskView{
		ID: t.ID, DataSourceID: t.DataSourceID, InstanceID: t.DataSourceID, Scope: t.Scope,
		Status: string(t.Status), RuleVersion: t.RuleVersion, SampleRows: t.SampleRows,
		BatchSize: t.BatchSize, Concurrency: t.Concurrency, LowPeakWindow: t.LowPeakWindow,
		TriggeredBy: t.TriggeredBy, ErrorMessage: t.ErrorMessage, StartedAt: t.StartedAt,
		FinishedAt: t.FinishedAt, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt,
	}
	if t.Summary != "" {
		var s port.TaskSummary
		if err := json.Unmarshal([]byte(t.Summary), &s); err == nil {
			doneTables := s.TableCount - s.FailedCount
			if doneTables < 0 {
				doneTables = 0
			}
			v.Summary = TaskSummaryView{
				SchemaCount: s.SchemaCount, TableCount: s.TableCount, ColumnCount: s.ColumnCount,
				HitColumnCount: s.HitColumnCount, FailedCount: s.FailedCount, Message: s.Message,
				TotalTables: s.TableCount, DoneTables: doneTables, FailedTables: s.FailedCount,
				TotalColumns: s.ColumnCount, HitColumns: s.HitColumnCount,
			}
		}
	}
	return v
}

// newTaskViews 批量构造任务视图。
func newTaskViews(items []port.ScanTask) []TaskView {
	out := make([]TaskView, 0, len(items))
	for _, t := range items {
		out = append(out, newTaskView(t))
	}
	return out
}

// enrichTaskDetailView 用断点表补齐任务详情的完成/失败表数与失败对象清单。
// 仅详情接口调用：列表接口不逐任务查断点，避免 N+1；查询失败时退回汇总计数。
func (s *Server) enrichTaskDetailView(ctx context.Context, taskID string, v TaskView) TaskView {
	cps, err := s.store.ListCheckpoints(ctx, taskID)
	if err != nil {
		return v
	}
	done, failed := 0, 0
	objects := make([]FailedObjectView, 0)
	for _, cp := range cps {
		if cp.ColumnName != "" {
			continue // 只统计表级断点
		}
		switch cp.State {
		case checkpointStateDone:
			done++
		case checkpointStateFailed:
			failed++
			schemaName, tableName, _ := strings.Cut(cp.TableMetaID, "/")
			objects = append(objects, FailedObjectView{
				ID: cp.TableMetaID, SchemaName: schemaName, TableName: tableName, Reason: cp.ErrorMessage,
			})
		}
	}
	v.Summary.DoneTables = done
	v.Summary.FailedTables = failed
	v.Summary.FailedObjects = objects
	return v
}

// FindingView 字段台账对外视图（仅含掩码样例）。
type FindingView struct {
	ID             string   `json:"id"`
	TaskID         string   `json:"taskId"`
	DataSourceID   string   `json:"dataSourceId"`
	InstanceID     string   `json:"instanceId"`
	ColumnMetaID   string   `json:"columnMetaId"`
	SchemaName     string   `json:"schemaName"`
	TableName      string   `json:"tableName"`
	ColumnName     string   `json:"columnName"`
	DataType       string   `json:"dataType"`
	ColumnComment  string   `json:"columnComment"`
	SensitiveTypes []string `json:"sensitiveTypes"`
	SampleTotal    int      `json:"sampleTotal"`
	SampleHit      int      `json:"sampleHit"`
	HitRatio       float64  `json:"hitRatio"`
	VerifyPassed   bool     `json:"verifyPassed"`
	Confidence     string   `json:"confidence"`
	StorageForm    string   `json:"storageForm"`
	RiskLevel      string   `json:"riskLevel"`
	ReviewStatus   string   `json:"reviewStatus"`
	SampleMasked   string   `json:"sampleMasked"`
	CreatedAt      string   `json:"createdAt"`
	UpdatedAt      string   `json:"updatedAt"`
}

// newFindingView 由领域实体构造视图。
func newFindingView(f port.ColumnFinding) FindingView {
	types := f.SensitiveTypes
	if types == nil {
		types = []string{}
	}
	return FindingView{
		ID: f.ID, TaskID: f.TaskID, DataSourceID: f.DataSourceID, InstanceID: f.DataSourceID,
		ColumnMetaID: f.ColumnMetaID, SchemaName: f.SchemaName, TableName: f.TableName,
		ColumnName: f.ColumnName, DataType: f.DataType, ColumnComment: f.ColumnComment,
		SensitiveTypes: types, SampleTotal: f.SampleTotal, SampleHit: f.SampleHit,
		HitRatio: f.HitRatio, VerifyPassed: f.VerifyPassed, Confidence: f.Confidence,
		StorageForm: f.StorageForm, RiskLevel: f.RiskLevel, ReviewStatus: f.ReviewStatus,
		SampleMasked: f.SampleMasked, CreatedAt: f.CreatedAt, UpdatedAt: f.UpdatedAt,
	}
}

// newFindingViews 批量构造台账视图。
func newFindingViews(items []port.ColumnFinding) []FindingView {
	out := make([]FindingView, 0, len(items))
	for _, f := range items {
		out = append(out, newFindingView(f))
	}
	return out
}

// AuditLogView 审计日志对外视图（IP 已脱敏）。
type AuditLogView struct {
	ID           string `json:"id"`
	Actor        string `json:"actor"`
	Action       string `json:"action"`
	TargetType   string `json:"targetType"`
	TargetID     string `json:"targetId"`
	Detail       string `json:"detail"`
	Result       string `json:"result"`
	ErrorMessage string `json:"errorMessage"`
	IPMasked     string `json:"ipMasked"`
	CreatedAt    string `json:"createdAt"`
}

// newAuditLogViews 批量构造审计视图。
func newAuditLogViews(items []port.AuditLog) []AuditLogView {
	out := make([]AuditLogView, 0, len(items))
	for _, a := range items {
		out = append(out, AuditLogView{
			ID: a.ID, Actor: a.Actor, Action: a.Action, TargetType: a.TargetType, TargetID: a.TargetID,
			Detail: a.Detail, Result: a.Result, ErrorMessage: a.ErrorMessage, IPMasked: a.IPMasked,
			CreatedAt: a.CreatedAt,
		})
	}
	return out
}

// RuleView 规则对外视图。
type RuleView struct {
	RuleID        string  `json:"ruleId"`
	SensitiveType string  `json:"sensitiveType"`
	Enabled       bool    `json:"enabled"`
	Custom        bool    `json:"custom"`
	Regex         string  `json:"regex"`
	RegexDigest   string  `json:"regexDigest"`
	Verifier      string  `json:"verifier"`
	Threshold     float64 `json:"threshold"`
}

// newRuleViews 批量构造规则视图。
func newRuleViews(items []port.Rule) []RuleView {
	out := make([]RuleView, 0, len(items))
	for _, r := range items {
		out = append(out, RuleView{
			RuleID: r.RuleID, SensitiveType: r.SensitiveType, Enabled: r.Enabled, Custom: r.Custom,
			Regex: r.Regex, RegexDigest: r.RegexDigest, Verifier: r.Verifier, Threshold: r.Threshold,
		})
	}
	return out
}

// dialectAllowed 判断方言是否在支持范围内。
func dialectAllowed(d string) bool {
	switch port.Dialect(d) {
	case port.DialectOracle, port.DialectDameng, port.DialectMySQL,
		port.DialectPostgres, port.DialectOpenGauss:
		return true
	}
	return false
}

// normalizeName 规整实例标识。
func normalizeName(s string) string { return strings.TrimSpace(s) }

// describeDialect 返回方言中文名。
func describeDialect(d string) string {
	switch port.Dialect(d) {
	case port.DialectOracle:
		return "Oracle"
	case port.DialectDameng:
		return "达梦"
	case port.DialectMySQL:
		return "MySQL"
	case port.DialectPostgres:
		return "PostgreSQL"
	case port.DialectOpenGauss:
		return "openGauss"
	default:
		return fmt.Sprintf("%s", d)
	}
}
