package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dbdlp/internal/application/datasource"
	"dbdlp/internal/application/scan"
	"dbdlp/internal/domain/port"
)

// ConfigEcho 是 GET /api/v1/config 的只读回显（不含密钥与 DSN）。
type ConfigEcho struct {
	LogLevel        string `json:"logLevel"`
	SampleRows      int    `json:"sampleRows"`
	StorageDriver   string `json:"storageDriver"`
	CryptoAlgorithm string `json:"cryptoAlgorithm"`
	RetentionDays   int    `json:"retentionDays"`
}

// Deps 是 API 层依赖。
type Deps struct {
	Store           port.Store
	ScanSvc         *scan.Service
	DsSvc           *datasource.Service
	ReporterFactory func(format port.ReportFormat) (port.Reporter, error)
	RulesLoader     func() ([]port.Rule, string, error)
	Logger          port.Logger
	SessionTTLMin   int
	ConfigEcho      ConfigEcho
	StaticDir       string // 静态资源目录；为空则不托管前端

	// 以下为可选增强项，不填时使用内置默认值。
	Cipher           port.Cipher    // 必填：解密数据源凭据（缺失直接启动失败，绝不明文落库）
	DefaultAlgorithm port.Algorithm // 凭据落库使用的加密算法标签；缺省为 AES-256-GCM
	ReportOutDir     string         // 报告输出目录，默认 "reports"
	MaxImportRows    int            // 批量导入行数上限，默认 5000（docs/06 §6.6）
	MaxUploadBytes   int64          // 上传文件体积上限，默认 10MB
	LockThreshold    int            // 连续登录失败锁定阈值，默认 5 次
	LockMinutes      int            // 登录锁定时长（分钟），默认 15 分钟
}

// Server 是 API 层处理器。
type Server struct {
	store           port.Store
	scanSvc         *scan.Service
	dsSvc           *datasource.Service
	reporterFactory func(format port.ReportFormat) (port.Reporter, error)
	rulesLoader     func() ([]port.Rule, string, error)
	logger          port.Logger
	cipher          port.Cipher
	defaultAlg      port.Algorithm
	configEcho      ConfigEcho
	staticDir       string
	reportOutDir    string
	sessionTTL      time.Duration
	maxImportRows   int
	maxUploadBytes  int64
	lockThreshold   int
	lockDuration    time.Duration
}

// New 构造 API 处理器。依赖缺失直接返回错误，避免运行期出现凭据明文落库等不可逆后果。
func New(deps Deps) (http.Handler, error) {
	s, err := newServer(deps)
	if err != nil {
		return nil, err
	}
	return s.routes(), nil
}

// newServer 构造处理器实例（New 与单元测试共用）。
func newServer(deps Deps) (*Server, error) {
	if deps.Store == nil {
		return nil, fmt.Errorf("缺少依赖：Store（port.Store）未注入，无法访问本地存储")
	}
	if deps.Cipher == nil {
		return nil, fmt.Errorf("缺少依赖：Cipher（port.Cipher）未注入，无法解密数据源凭据；请在接线时传入已构建的加解密器")
	}
	s := &Server{
		store:           deps.Store,
		scanSvc:         deps.ScanSvc,
		dsSvc:           deps.DsSvc,
		reporterFactory: deps.ReporterFactory,
		rulesLoader:     deps.RulesLoader,
		logger:          deps.Logger,
		cipher:          deps.Cipher,
		configEcho:      deps.ConfigEcho,
		staticDir:       deps.StaticDir,
		reportOutDir:    "reports",
		sessionTTL:      8 * time.Hour,
		maxImportRows:   5000,
		maxUploadBytes:  10 << 20,
		lockThreshold:   5,
		lockDuration:    15 * time.Minute,
	}
	if deps.SessionTTLMin > 0 {
		s.sessionTTL = time.Duration(deps.SessionTTLMin) * time.Minute
	}
	if deps.ReportOutDir != "" {
		s.reportOutDir = deps.ReportOutDir
	}
	if deps.MaxImportRows > 0 {
		s.maxImportRows = deps.MaxImportRows
	}
	if deps.MaxUploadBytes > 0 {
		s.maxUploadBytes = deps.MaxUploadBytes
	}
	if deps.LockThreshold > 0 {
		s.lockThreshold = deps.LockThreshold
	}
	if deps.LockMinutes > 0 {
		s.lockDuration = time.Duration(deps.LockMinutes) * time.Minute
	}
	if deps.DefaultAlgorithm == port.AlgSM4GCM {
		s.defaultAlg = port.AlgSM4GCM
	} else if deps.DefaultAlgorithm == port.AlgAES256GCM {
		s.defaultAlg = port.AlgAES256GCM
	} else if strings.EqualFold(deps.ConfigEcho.CryptoAlgorithm, "sm4gcm") {
		s.defaultAlg = port.AlgSM4GCM
	}
	if s.staticDir != "" {
		if abs, err := filepath.Abs(s.staticDir); err == nil {
			s.staticDir = abs
		}
	}
	return s, nil
}

// routes 注册全部端点；/api/v1/* 优先于静态资源。
func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()

	// 认证与会话（登录为唯一免鉴权端点）
	mux.HandleFunc("POST /api/v1/auth/login", s.handleLogin)
	mux.HandleFunc("POST /api/v1/auth/logout", s.authed(s.handleLogout))
	mux.HandleFunc("GET /api/v1/auth/me", s.authed(s.handleMe))

	// 数据源
	mux.HandleFunc("GET /api/v1/datasources", s.authed(s.handleListDataSources))
	mux.HandleFunc("POST /api/v1/datasources", s.authed(s.handleCreateDataSource))
	mux.HandleFunc("GET /api/v1/datasources/import/template", s.authed(s.handleImportTemplate))
	mux.HandleFunc("POST /api/v1/datasources/import", s.authed(s.handleImportDataSources))
	mux.HandleFunc("GET /api/v1/datasources/{id}", s.authed(s.handleGetDataSource))
	mux.HandleFunc("PATCH /api/v1/datasources/{id}", s.authed(s.handleUpdateDataSource))
	mux.HandleFunc("DELETE /api/v1/datasources/{id}", s.authed(s.handleDeleteDataSource))
	mux.HandleFunc("POST /api/v1/datasources/{id}/test", s.authed(s.handleTestDataSource))
	mux.HandleFunc("GET /api/v1/datasources/{id}/metadata", s.authed(s.handleDataSourceMetadata))

	// 扫描任务
	mux.HandleFunc("GET /api/v1/tasks", s.authed(s.handleListTasks))
	mux.HandleFunc("POST /api/v1/tasks", s.authed(s.handleCreateTask))
	mux.HandleFunc("GET /api/v1/tasks/{id}", s.authed(s.handleGetTask))
	mux.HandleFunc("GET /api/v1/tasks/{id}/progress", s.authed(s.handleTaskProgress))
	mux.HandleFunc("POST /api/v1/tasks/{id}/pause", s.authed(s.handlePauseTask))
	mux.HandleFunc("POST /api/v1/tasks/{id}/resume", s.authed(s.handleResumeTask))
	mux.HandleFunc("POST /api/v1/tasks/{id}/retry", s.authed(s.handleRetryTask))

	// 结果台账与复核
	mux.HandleFunc("GET /api/v1/findings", s.authed(s.handleListFindings))
	mux.HandleFunc("GET /api/v1/findings/{id}", s.authed(s.handleGetFinding))
	mux.HandleFunc("GET /api/v1/findings/{id}/hits", s.authed(s.handleListHits))
	mux.HandleFunc("POST /api/v1/reviews", s.authed(s.handleCreateReview))
	mux.HandleFunc("GET /api/v1/reviews", s.authed(s.handleListReviews))

	// 规则
	mux.HandleFunc("GET /api/v1/rules", s.authed(s.handleListRules))
	mux.HandleFunc("GET /api/v1/rules/snapshots", s.authed(s.handleListRuleSnapshots))

	// 报告
	mux.HandleFunc("POST /api/v1/reports", s.authed(s.handleCreateReport))
	mux.HandleFunc("GET /api/v1/reports", s.authed(s.handleListReports))
	mux.HandleFunc("GET /api/v1/reports/{id}/download", s.authed(s.handleDownloadReport))

	// 仪表盘／审计／配置
	mux.HandleFunc("GET /api/v1/dashboard/summary", s.authed(s.handleDashboardSummary))
	mux.HandleFunc("GET /api/v1/audit-logs", s.authed(s.handleListAuditLogs))
	mux.HandleFunc("GET /api/v1/config", s.authed(s.handleGetConfig))

	// 静态资源（兜底路由）
	mux.HandleFunc("/", s.handleStatic)

	return s.recoverPanic(mux)
}

// recoverPanic 兜底捕获未处理异常，避免进程崩溃并统一返回 5000。
func (s *Server) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				s.log(r.Context(), port.LevelERROR, "请求处理发生未捕获异常",
					port.Field{Key: "路径", Value: r.URL.Path},
					port.Field{Key: "方法", Value: r.Method})
				writeAPIError(w, errInternal("系统内部错误，请联系管理员"))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// log 输出日志；未注入日志器时静默跳过。
func (s *Server) log(ctx context.Context, level port.LogLevel, msg string, fields ...port.Field) {
	if s.logger == nil {
		return
	}
	switch level {
	case port.LevelTRACE:
		s.logger.Trace(ctx, msg, fields...)
	case port.LevelDEBUG:
		s.logger.Debug(ctx, msg, fields...)
	case port.LevelINFO:
		s.logger.Info(ctx, msg, fields...)
	case port.LevelWARN:
		s.logger.Warn(ctx, msg, fields...)
	case port.LevelERROR:
		s.logger.Error(ctx, msg, fields...)
	}
}

// staticExists 判断静态资源目录是否存在。
func (s *Server) staticExists() bool {
	if s.staticDir == "" {
		return false
	}
	info, err := os.Stat(s.staticDir)
	return err == nil && info.IsDir()
}
