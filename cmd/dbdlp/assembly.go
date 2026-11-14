package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"dbdlp/internal/application/datasource"
	"dbdlp/internal/application/scan"
	"dbdlp/internal/domain/port"
	"dbdlp/internal/domain/rule"
	"dbdlp/internal/infrastructure/config"
	"dbdlp/internal/infrastructure/crypto"
	dsdriver "dbdlp/internal/infrastructure/datasource"
	"dbdlp/internal/infrastructure/logging"
	"dbdlp/internal/infrastructure/reporter"
	"dbdlp/internal/infrastructure/store"
	"dbdlp/internal/interfaces/cli"
	httpapi "dbdlp/internal/interfaces/http"
	"github.com/spf13/cobra"
)

// App 进程级装配结果：持有根命令与需释放的资源。
type App struct {
	Root   *cobra.Command
	logger port.Logger
	store  *store.SQLiteStore
}

// Close 释放进程级资源（日志器与本地存储）。
func (a *App) Close() {
	if a.logger != nil {
		_ = a.logger.Close()
	}
	if a.store != nil {
		_ = a.store.Close()
	}
}

// NewApp 装配配置、密钥、日志、存储、应用用例与 HTTP API。
// 任一环节失败即返回错误，绝不带着半成品继续启动。
func NewApp(configDir string) (*App, error) {
	cfg, err := config.Load(configDir)
	if err != nil {
		return nil, fmt.Errorf("加载配置失败：%w", err)
	}
	ks := crypto.NewKeyStore(configDir)
	key, err := ks.LoadOrInit()
	if err != nil {
		return nil, fmt.Errorf("初始化密钥失败：%w", err)
	}
	cipher, err := crypto.NewCipher(key, cfg.DefaultAlgorithm())
	if err != nil {
		return nil, fmt.Errorf("构建加解密器失败：%w", err)
	}
	level, err := logging.ParseLevel(cfg.Log.Level)
	if err != nil {
		return nil, fmt.Errorf("配置日志级别非法：%w", err)
	}
	// 日志目录默认为项目根目录 logs/（相对配置目录解析）。
	logger, err := logging.New(logging.Options{
		Level: level, Dir: resolveUnder(configDir, cfg.Log.Dir), Console: cfg.Log.Console,
		RetentionDays: cfg.Log.RetentionDays, FilePrefix: cfg.Log.FilePrefix,
	})
	if err != nil {
		return nil, fmt.Errorf("初始化日志失败：%w", err)
	}
	app := &App{logger: logger}

	// 存储与报告目录均相对配置目录解析，避免 --config 指向其他目录时写错位置。
	sqlitePath := resolveUnder(configDir, cfg.Storage.SQLitePath)
	reportOutDir := resolveUnder(configDir, cfg.Report.OutDir)
	if err := os.MkdirAll(filepath.Dir(sqlitePath), 0o700); err != nil {
		app.Close()
		return nil, fmt.Errorf("创建存储目录失败：%w", err)
	}
	st, err := store.NewSQLiteStore(sqlitePath)
	if err != nil {
		app.Close()
		return nil, fmt.Errorf("打开本地存储失败：%w", err)
	}
	app.store = st

	driverFactory := func(d port.Dialect) (port.Driver, error) { return dsdriver.NewDriver(d) }
	reporterFactory := func(f port.ReportFormat) (port.Reporter, error) { return reporter.New(f) }
	rulesLoader := func() ([]port.Rule, string, error) { return rule.BuildRules(cfg.RulesFilePath()) }

	scanSvc := scan.NewService(st, driverFactory, reporterFactory, rulesLoader, cipher.Decrypt, logger, cfg.Scan.SampleRows, cfg.Scan.MaxTextLen).
		WithScheduling(cfg.Scan.BatchIntervalMs, cfg.Scan.LowPeakWindow).
		WithAuditActor("cli")
	dsSvc := datasource.NewService(st, driverFactory, cipher, cfg.DefaultAlgorithm(), logger)

	// 控制台管理员引导：仅在提供环境变量且账号尚不存在时创建，已存在则不覆盖口令。
	if err := bootstrapAdmin(st, logger); err != nil {
		app.Close()
		return nil, err
	}

	// HTTP API 与前端静态资源托管（[web].enabled=false 时不装配）。
	var httpHandler http.Handler
	staticDir := ""
	if cfg.Web.Enabled {
		staticDir = resolveUnder(configDir, cfg.Web.StaticDir)
		h, err := httpapi.New(httpapi.Deps{
			Store:           st,
			ScanSvc:         scanSvc,
			DsSvc:           dsSvc,
			ReporterFactory: reporterFactory,
			RulesLoader:     rulesLoader,
			Logger:          logger,
			SessionTTLMin:   cfg.Web.SessionTTLMin,
			ConfigEcho: httpapi.ConfigEcho{
				LogLevel:        cfg.Log.Level,
				SampleRows:      cfg.Scan.SampleRows,
				StorageDriver:   cfg.Storage.Driver,
				CryptoAlgorithm: cfg.Crypto.DefaultAlgorithm,
				RetentionDays:   cfg.Log.RetentionDays,
			},
			StaticDir:        staticDir,
			Cipher:           cipher,
			DefaultAlgorithm: cfg.DefaultAlgorithm(),
			ReportOutDir:     reportOutDir,
		})
		if err != nil {
			app.Close()
			return nil, fmt.Errorf("装配 HTTP API 失败：%w", err)
		}
		httpHandler = h
	}

	app.Root = cli.NewRootCmd(&cli.Deps{
		ConfigDir:       configDir,
		ReportOutDir:    reportOutDir,
		Listen:          cfg.Web.Listen,
		StaticDir:       staticDir,
		HTTPServer:      httpHandler,
		Store:           st,
		ScanSvc:         scanSvc,
		DsSvc:           dsSvc,
		ReporterFactory: reporterFactory,
		RulesLoader:     rulesLoader,
		Logger:          logger,
	})
	return app, nil
}

// bootstrapAdmin 按环境变量引导控制台管理员账号（DBDLP_ADMIN_USER / DBDLP_ADMIN_PASSWORD）。
// 口令只从环境变量读取，不落配置文件、不进日志；账号已存在时不覆盖其口令。
func bootstrapAdmin(st *store.SQLiteStore, logger port.Logger) error {
	user := strings.TrimSpace(os.Getenv("DBDLP_ADMIN_USER"))
	if user == "" {
		return nil
	}
	pass := os.Getenv("DBDLP_ADMIN_PASSWORD")
	if pass == "" {
		return fmt.Errorf("已设置 DBDLP_ADMIN_USER，但缺少 DBDLP_ADMIN_PASSWORD")
	}
	created, err := st.EnsureAdminUser(context.Background(), user, pass, "系统管理员")
	if err != nil {
		return fmt.Errorf("引导控制台管理员账号失败：%w", err)
	}
	if created && logger != nil {
		logger.Info(context.Background(), "已创建控制台管理员账号", port.Field{Key: "账号", Value: user})
	}
	return nil
}

// resolveUnder 相对路径挂到基准目录下；已是绝对路径则原样返回。
func resolveUnder(base, p string) string {
	if p == "" {
		p = "."
	}
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(base, p)
}
