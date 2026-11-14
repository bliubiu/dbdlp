// Package scan 扫描用例：元数据→采样→识别→聚合→台账→报告 的端到端编排。
package scan

import (
	"context"
	"fmt"

	"dbdlp/internal/application"
	"dbdlp/internal/domain/detector"
	"dbdlp/internal/domain/port"
)

// Service 扫描编排服务；依赖通过构造函数注入，便于测试替换驱动/报告器。
type Service struct {
	store           port.Store
	driverFactory   func(dialect port.Dialect) (port.Driver, error)
	reporterFactory func(format port.ReportFormat) (port.Reporter, error)
	rulesLoader     func() ([]port.Rule, string, error)
	decrypt         func(stored string) (string, error)
	logger          port.Logger
	sampleRows      int
	maxTextLen      int
	batchIntervalMs int
	lowPeakWindow   string
	auditActor      string
}

// 本文件实现扫描编排：元数据 → 采样 → 识别 → 聚合 → 台账 → 报告。

func NewService(
	store port.Store,
	driverFactory func(port.Dialect) (port.Driver, error),
	reporterFactory func(port.ReportFormat) (port.Reporter, error),
	rulesLoader func() ([]port.Rule, string, error),
	decrypt func(string) (string, error),
	logger port.Logger,
	sampleRows, maxTextLen int,
) *Service {
	if sampleRows <= 0 {
		sampleRows = 20
	}
	if maxTextLen <= 0 {
		maxTextLen = 2000
	}
	return &Service{
		store: store, driverFactory: driverFactory, reporterFactory: reporterFactory,
		rulesLoader: rulesLoader, decrypt: decrypt, logger: logger,
		sampleRows: sampleRows, maxTextLen: maxTextLen,
	}
}

// log 输出日志；未注入日志器时静默跳过。

func (s *Service) log(ctx context.Context, level port.LogLevel, msg string, fields ...port.Field) {
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

// Run 经已存数据源执行扫描（自动解密凭据并建任务）。

func (s *Service) Run(ctx context.Context, dataSourceID, reportFormat, outDir string) (string, error) {
	ds, err := s.store.GetDataSource(ctx, dataSourceID)
	if err != nil {
		return "", fmt.Errorf("读取数据源失败：%w", err)
	}
	cfg, err := s.datasourceConfig(ctx, dataSourceID)
	if err != nil {
		return "", err
	}
	scope := port.ScanScope{IncludeSchemas: ds.IncludeSchemas, ExcludeTables: ds.ExcludeTables, ExcludeColumns: ds.ExcludeColumns}
	return s.runPipeline(ctx, cfg, scope, reportFormat, outDir)
}

// RunConfig 以明文配置直接执行扫描（单次调用与测试友好，跳过存储与解密）。

func (s *Service) RunConfig(ctx context.Context, cfg port.DataSourceConfig, scope port.ScanScope, reportFormat, outDir string) (string, error) {
	return s.runPipeline(ctx, cfg, scope, reportFormat, outDir)
}

func (s *Service) runPipeline(ctx context.Context, cfg port.DataSourceConfig, scope port.ScanScope, reportFormat, outDir string) (string, error) {
	rules, version, err := s.rulesLoader()
	if err != nil {
		return "", fmt.Errorf("加载规则失败：%w", err)
	}
	det, err := detector.New(rules, version)
	if err != nil {
		return "", fmt.Errorf("构建识别引擎失败：%w", err)
	}
	defer det.Close()
	taskID := application.GenID()
	if err := s.store.CreateTask(ctx, port.ScanTask{
		ID: taskID, DataSourceID: cfg.ID, Status: port.TaskStatusRunning,
		RuleVersion: version, SampleRows: s.sampleRows,
	}); err != nil {
		return "", fmt.Errorf("创建任务失败：%w", err)
	}
	fail := func(reason string) (string, error) {
		_ = s.store.UpdateTaskStatus(ctx, taskID, port.TaskStatusFailed, port.TaskSummary{Message: reason})
		s.log(ctx, port.LevelERROR, "扫描任务失败", port.Field{Key: "任务", Value: taskID}, port.Field{Key: "原因", Value: reason})
		s.auditTask(ctx, "task.start", taskID, "数据源="+cfg.Name, "failed", reason)
		return "", fmt.Errorf("%s", reason)
	}
	s.log(ctx, port.LevelINFO, "扫描任务开始",
		port.Field{Key: "任务", Value: taskID},
		port.Field{Key: "数据源", Value: cfg.Name},
		port.Field{Key: "方言", Value: string(cfg.Dialect)},
		port.Field{Key: "规则版本", Value: version},
		port.Field{Key: "采样行数", Value: fmt.Sprintf("%d", s.sampleRows)})
	driver, err := s.driverFactory(cfg.Dialect)
	if err != nil {
		return fail(fmt.Sprintf("方言不支持：%v", err))
	}
	conn, err := driver.Connect(ctx, cfg)
	if err != nil {
		return fail(fmt.Sprintf("连接失败：%v", err))
	}
	defer conn.Close()
	schemas, err := driver.ListSchemas(ctx, conn)
	if err != nil {
		return fail(fmt.Sprintf("列举模式失败：%v", err))
	}
	kept := filterSchemas(schemas, scope)
	s.log(ctx, port.LevelINFO, "扫描范围确定",
		port.Field{Key: "任务", Value: taskID},
		port.Field{Key: "模式数", Value: fmt.Sprintf("%d", len(kept))})
	// 逐表扫描：单表失败不阻断整体，失败表留存 failed 断点。
	out := s.scanTables(ctx, driver, conn, det, taskID, cfg.ID, kept, scope, nil)
	if err := s.store.SaveRuleSnapshot(ctx, version, rules); err != nil {
		return fail(fmt.Sprintf("保存规则快照失败：%v", err))
	}
	summary := port.TaskSummary{
		SchemaCount: len(kept), TableCount: len(out.Tables),
		ColumnCount: len(out.Columns), HitColumnCount: len(out.Findings),
		Message: fmt.Sprintf("发现 %d 个敏感字段", len(out.Findings)),
	}
	// 部分失败：有表失败但其余表已产出 → 任务状态为 partial，可单独重试失败表。
	status := port.TaskStatusDone
	if len(out.Failed) > 0 {
		summary.FailedCount = len(out.Failed)
		summary.Message = fmt.Sprintf("发现 %d 个敏感字段；%d 张表扫描失败可重试", len(out.Findings), len(out.Failed))
		status = port.TaskStatusPartial
		if out.Scanned == 0 {
			status = port.TaskStatusFailed
		}
	}
	if err := s.store.UpdateTaskStatus(ctx, taskID, status, summary); err != nil {
		return "", fmt.Errorf("更新任务状态失败：%w", err)
	}
	s.log(ctx, port.LevelINFO, "扫描任务完成",
		port.Field{Key: "任务", Value: taskID},
		port.Field{Key: "状态", Value: string(status)},
		port.Field{Key: "模式数", Value: fmt.Sprintf("%d", len(kept))},
		port.Field{Key: "表数", Value: fmt.Sprintf("%d", len(out.Tables))},
		port.Field{Key: "字段数", Value: fmt.Sprintf("%d", len(out.Columns))},
		port.Field{Key: "命中字段数", Value: fmt.Sprintf("%d", len(out.Findings))},
		port.Field{Key: "失败表数", Value: fmt.Sprintf("%d", len(out.Failed))})
	if reportFormat != "" && outDir != "" {
		if err := s.exportReport(ctx, taskID, cfg.ID, version, out.Findings, rules, reportFormat, outDir); err != nil {
			// 扫描本体已成功（台账与任务状态已落库），但报告交付失败：返回任务 ID 供人工补救。
			s.auditTask(ctx, "task.start", taskID,
				fmt.Sprintf("状态=%s，表=%d，命中=%d", status, len(out.Tables), len(out.Findings)),
				"success", "报告导出失败："+err.Error())
			return taskID, err
		}
	}
	s.auditTask(ctx, "task.start", taskID,
		fmt.Sprintf("状态=%s，表=%d，命中=%d", status, len(out.Tables), len(out.Findings)),
		"success", "")
	return taskID, nil
}
