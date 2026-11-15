// Package cli 命令行入口：cobra 子命令编排应用层用例。
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"dbdlp/internal/application/datasource"
	"dbdlp/internal/application/scan"
	"dbdlp/internal/domain/port"
	"dbdlp/internal/infrastructure/crypto"
	"github.com/spf13/cobra"
)

// Deps 命令依赖（由 main 注入）。
type Deps struct {
	ConfigDir       string
	ReportOutDir    string // 报告输出目录默认值（已由 main 解析为绝对路径）
	Listen          string // HTTP 监听地址
	StaticDir       string // 前端静态资源目录
	HTTPServer      http.Handler
	Store           port.Store
	ScanSvc         *scan.Service
	DsSvc           *datasource.Service
	ReporterFactory func(format port.ReportFormat) (port.Reporter, error)
	RulesLoader     func() ([]port.Rule, string, error)
	Logger          port.Logger
}

// log 输出日志；未注入日志器时静默跳过。
func (d *Deps) log(ctx context.Context, level port.LogLevel, msg string, fields ...port.Field) {
	if d.Logger == nil {
		return
	}
	switch level {
	case port.LevelTRACE:
		d.Logger.Trace(ctx, msg, fields...)
	case port.LevelDEBUG:
		d.Logger.Debug(ctx, msg, fields...)
	case port.LevelINFO:
		d.Logger.Info(ctx, msg, fields...)
	case port.LevelWARN:
		d.Logger.Warn(ctx, msg, fields...)
	case port.LevelERROR:
		d.Logger.Error(ctx, msg, fields...)
	}
}

// reportOut 返回报告输出目录默认值（优先 main 解析后的绝对路径）。
func (d *Deps) reportOut() string {
	if d.ReportOutDir != "" {
		return d.ReportOutDir
	}
	return "reports"
}

// NewRootCmd 构造根命令。
func NewRootCmd(d *Deps) *cobra.Command {
	root := &cobra.Command{
		Use:   "dbdlp",
		Short: "数据库敏感数据发现平台（只读扫描）",
	}
	root.SetOut(os.Stdout)
	root.SetErr(os.Stderr)
	root.PersistentFlags().StringVar(&d.ConfigDir, "config", d.ConfigDir, "配置目录（含 dbdlp.toml 与 .dbdlp/key）")

	root.AddCommand(newInitCmd(d))
	root.AddCommand(newDatasourceCmd(d))
	root.AddCommand(newScanCmd(d))
	root.AddCommand(newReportCmd(d))
	root.AddCommand(newRetryCmd(d))
	root.AddCommand(newResumeCmd(d))
	root.AddCommand(newServeCmd(d))
	return root
}

func newInitCmd(d *Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "初始化工作区：生成 32 字节密钥与本地存储",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ks := crypto.NewKeyStore(d.ConfigDir)
			if _, err := ks.LoadOrInit(); err != nil {
				return fmt.Errorf("生成密钥失败：%w", err)
			}
			cmd.Printf("初始化完成：密钥位于 %s\n", ks.Path())
			return nil
		},
	}
}

func newDatasourceCmd(d *Deps) *cobra.Command {
	cmd := &cobra.Command{Use: "datasource", Short: "数据源管理（add/test/import）"}
	var (
		name, dialect, host, service, username, password, dsn, inc, exT, exC string
		portNum                                                              int
		filePath                                                             string
	)
	add := &cobra.Command{
		Use:   "add",
		Short: "新增数据源（先连接测试，通过后才加密落库）",
		RunE: func(cmd *cobra.Command, _ []string) error {
			id, err := d.DsSvc.Add(context.Background(), datasource.AddInput{
				Name: name, Dialect: dialect, Host: host, Port: portNum, ServiceName: service,
				Username: username, Password: password, DSN: dsn,
				IncludeSchemas: inc, ExcludeTables: exT, ExcludeColumns: exC,
			})
			if err != nil {
				return err
			}
			cmd.Printf("数据源已创建：%s\n", id)
			return nil
		},
	}
	add.Flags().StringVar(&name, "name", "", "数据源名称（必填）")
	add.Flags().StringVar(&dialect, "dialect", "", "方言：oracle|dameng|mysql|postgres|sqlite（必填）")
	add.Flags().StringVar(&host, "host", "", "主机")
	add.Flags().IntVar(&portNum, "port", 0, "端口")
	add.Flags().StringVar(&service, "service", "", "Oracle 服务名 / 达梦库名")
	add.Flags().StringVar(&username, "username", "", "用户名")
	add.Flags().StringVar(&password, "password", "", "口令")
	add.Flags().StringVar(&dsn, "dsn", "", "完整连接串（可选）")
	add.Flags().StringVar(&inc, "include-schemas", "", "仅扫描的模式（逗号分隔）")
	add.Flags().StringVar(&exT, "exclude-tables", "", "跳过表（逗号分隔）")
	add.Flags().StringVar(&exC, "exclude-columns", "", "跳过字段（逗号分隔）")
	add.MarkFlagRequired("name")
	add.MarkFlagRequired("dialect")

	test := &cobra.Command{
		Use:   "test",
		Short: "仅测试数据源连通性（不落库）",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := port.DataSourceConfig{Dialect: port.Dialect(dialect), Host: host, Port: portNum,
				ServiceName: service, Username: username, Password: password, DSN: dsn}
			if err := d.DsSvc.Test(context.Background(), cfg); err != nil {
				return fmt.Errorf("连接测试失败：%w", err)
			}
			cmd.Println("连接测试通过")
			return nil
		},
	}
	test.Flags().StringVar(&dialect, "dialect", "", "方言：oracle|dameng|mysql|postgres|sqlite（必填）")
	test.Flags().StringVar(&host, "host", "", "主机")
	test.Flags().IntVar(&portNum, "port", 0, "端口")
	test.Flags().StringVar(&service, "service", "", "服务名 / 库名")
	test.Flags().StringVar(&username, "username", "", "用户名")
	test.Flags().StringVar(&password, "password", "", "口令")
	test.Flags().StringVar(&dsn, "dsn", "", "完整连接串")
	test.MarkFlagRequired("dialect")

	imp := &cobra.Command{
		Use:   "import",
		Short: "批量导入数据源（JSON 数组，逐项连接测试，部分成功）",
		RunE: func(cmd *cobra.Command, _ []string) error {
			raw, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("读取导入文件失败：%w", err)
			}
			var items []datasource.AddInput
			if err := json.Unmarshal(raw, &items); err != nil {
				return fmt.Errorf("解析导入文件失败：%w", err)
			}
			ids, failures, err := d.DsSvc.Import(context.Background(), items)
			if err != nil {
				return err
			}
			cmd.Printf("成功导入 %d 个，失败 %d 个\n", len(ids), len(failures))
			for _, id := range ids {
				cmd.Printf("  [OK]   %s\n", id)
			}
			for _, f := range failures {
				cmd.Printf("  [FAIL] #%d %s: %s\n", f.Index, f.Name, f.Error)
			}
			return nil
		},
	}
	imp.Flags().StringVar(&filePath, "file", "", "JSON 数组文件路径（必填）")
	imp.MarkFlagRequired("file")

	cmd.AddCommand(add, test, imp)
	return cmd
}

func newScanCmd(d *Deps) *cobra.Command {
	var dsID, report, out string
	cmd := &cobra.Command{
		Use:   "scan",
		Short: "执行只读敏感数据扫描",
		RunE: func(cmd *cobra.Command, _ []string) error {
			taskID, err := d.ScanSvc.Run(context.Background(), dsID, report, out)
			if taskID != "" {
				cmd.Printf("扫描任务：%s\n", taskID)
			}
			if err != nil {
				return err
			}
			cmd.Printf("扫描完成，任务：%s\n", taskID)
			if report != "" && out != "" {
				cmd.Printf("报告目录：%s\n", out)
			}
			d.log(context.Background(), port.LevelINFO, "命令行扫描完成",
				port.Field{Key: "任务", Value: taskID},
				port.Field{Key: "数据源ID", Value: dsID},
				port.Field{Key: "报告格式", Value: report})
			return nil
		},
	}
	cmd.Flags().StringVar(&dsID, "datasource", "", "数据源 ID（必填）")
	cmd.Flags().StringVar(&report, "report", "", "报告格式：excel|html|json（可选，支持逗号分隔多格式）")
	cmd.Flags().StringVar(&out, "out", d.reportOut(), "报告输出目录")
	cmd.MarkFlagRequired("datasource")
	return cmd
}

func newReportCmd(d *Deps) *cobra.Command {
	var taskID, format, out string
	cmd := &cobra.Command{
		Use:   "report",
		Short: "基于已存台账重新导出报告",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := context.Background()
			task, err := d.Store.GetTask(ctx, taskID)
			if err != nil {
				return fmt.Errorf("读取任务失败：%w", err)
			}
			page, err := d.Store.ListFindings(ctx, port.FindingFilter{TaskID: taskID})
			if err != nil {
				return fmt.Errorf("读取台账失败：%w", err)
			}
			rules, version, err := d.RulesLoader()
			if err != nil {
				return fmt.Errorf("加载规则失败：%w", err)
			}
			_ = version
			bundle := port.ReportBundle{Task: task, Findings: page.Items, Rules: rules}
			var paths []string
			for _, f := range splitFormats(format) {
				r, err := d.ReporterFactory(port.ReportFormat(f))
				if err != nil {
					return fmt.Errorf("构建 %s 报告器失败：%w", f, err)
				}
				path, err := r.Export(ctx, bundle, out)
				if err != nil {
					return fmt.Errorf("导出 %s 报告失败：%w", f, err)
				}
				paths = append(paths, path)
				d.log(ctx, port.LevelINFO, "命令行报告导出完成",
					port.Field{Key: "任务", Value: taskID},
					port.Field{Key: "格式", Value: f},
					port.Field{Key: "文件", Value: path})
			}
			for _, p := range paths {
				cmd.Println("报告已生成：" + p)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&taskID, "task", "", "任务 ID（必填）")
	cmd.Flags().StringVar(&format, "format", "excel", "报告格式：excel|html|json（支持逗号分隔多格式）")
	cmd.Flags().StringVar(&out, "out", d.reportOut(), "报告输出目录")
	cmd.MarkFlagRequired("task")
	return cmd
}

// splitFormats 解析逗号分隔的格式列表（大小写不敏感，忽略空项）。
func splitFormats(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		p := strings.ToLower(strings.TrimSpace(part))
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
