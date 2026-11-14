// Package datasource 数据源用例：连接测试前置校验、加密落库、批量导入。
package datasource

import (
	"context"
	"fmt"
	"strings"
	"time"

	"dbdlp/internal/application"
	"dbdlp/internal/domain/port"
)

// Service 数据源用例服务。
type Service struct {
	store         port.Store
	driverFactory func(dialect port.Dialect) (port.Driver, error)
	cipher        port.Cipher
	defaultAlg    port.Algorithm
	logger        port.Logger
}

// NewService 构造数据源服务。
func NewService(store port.Store, driverFactory func(port.Dialect) (port.Driver, error), c port.Cipher, alg port.Algorithm, logger port.Logger) *Service {
	return &Service{store: store, driverFactory: driverFactory, cipher: c, defaultAlg: alg, logger: logger}
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

// audit 写审计留痕（docs/06：CLI 操作与 HTTP 控制台同构留痕）；
// 审计写入失败只记日志，不影响主流程。actor 固定 cli，无请求 IP。
func (s *Service) audit(ctx context.Context, action, targetID, detail, result, errMsg string) {
	if s.store == nil {
		return
	}
	entry := port.AuditLog{
		Actor: "cli", Action: action, TargetType: "data_source", TargetID: targetID,
		Detail: detail, Result: result, ErrorMessage: errMsg,
	}
	if err := s.store.AppendAudit(ctx, entry); err != nil {
		s.log(ctx, port.LevelWARN, "审计日志写入失败",
			port.Field{Key: "动作", Value: action},
			port.Field{Key: "原因", Value: err.Error()})
	}
}

// AddInput 新增数据源入参（明文仅内存瞬时）。
type AddInput struct {
	Name           string
	Dialect        string
	Host           string
	Port           int
	ServiceName    string
	Username       string
	Password       string
	DSN            string
	IncludeSchemas string
	ExcludeTables  string
	ExcludeColumns string
}

// Add 连接测试通过后，加密落库；返回新数据源 ID。
func (s *Service) Add(ctx context.Context, in AddInput) (string, error) {
	if in.Name == "" || in.Dialect == "" {
		return "", fmt.Errorf("名称与方言不能为空")
	}
	cfg := s.toConfig(in, "")
	// 连接测试前置门禁：未通过不得落库
	if err := s.Test(ctx, cfg); err != nil {
		s.log(ctx, port.LevelWARN, "数据源连接测试未通过，未保存",
			port.Field{Key: "数据源", Value: in.Name},
			port.Field{Key: "原因", Value: err.Error()})
		s.audit(ctx, "datasource.test", "", "名称="+in.Name+"，方言="+in.Dialect, "failed", err.Error())
		return "", fmt.Errorf("连接测试未通过：%w", err)
	}
	ds, err := s.encryptAndBuild(in)
	if err != nil {
		return "", err
	}
	if err := s.store.CreateDataSource(ctx, ds); err != nil {
		return "", fmt.Errorf("保存数据源失败：%w", err)
	}
	s.log(ctx, port.LevelINFO, "数据源已创建",
		port.Field{Key: "数据源", Value: in.Name},
		port.Field{Key: "数据源ID", Value: ds.ID},
		port.Field{Key: "方言", Value: in.Dialect})
	s.audit(ctx, "datasource.create", ds.ID, "名称="+in.Name+"，方言="+in.Dialect, "success", "")
	return ds.ID, nil
}

// Test 仅做连通性校验（Ping），不落库。
func (s *Service) Test(ctx context.Context, cfg port.DataSourceConfig) error {
	if cfg.Dialect == "" {
		return fmt.Errorf("方言不能为空")
	}
	driver, err := s.driverFactory(cfg.Dialect)
	if err != nil {
		return err
	}
	conn, err := driver.Connect(ctx, cfg)
	if err != nil {
		return err
	}
	defer conn.Close()
	return conn.Ping(ctx)
}

// Import 批量导入：逐项连接测试，通过的整批事务落库（部分成功）。
// 返回成功 ID 列表与失败项（含原因）。
func (s *Service) Import(ctx context.Context, items []AddInput) (succeededIDs []string, failures []ImportFailure, err error) {
	type tested struct {
		in port.DataSource
	}
	var ok []tested
	for i, in := range items {
		cfg := s.toConfig(in, "")
		if terr := s.Test(ctx, cfg); terr != nil {
			failures = append(failures, ImportFailure{Index: i, Name: in.Name, Error: terr.Error()})
			continue
		}
		ds, eerr := s.encryptAndBuild(in)
		if eerr != nil {
			failures = append(failures, ImportFailure{Index: i, Name: in.Name, Error: eerr.Error()})
			continue
		}
		ok = append(ok, tested{in: ds})
	}
	if len(ok) == 0 {
		return nil, failures, nil
	}
	batch := make([]port.DataSource, 0, len(ok))
	for _, t := range ok {
		batch = append(batch, t.in)
		succeededIDs = append(succeededIDs, t.in.ID)
	}
	if berr := s.store.BatchCreateDataSources(ctx, batch); berr != nil {
		return nil, failures, fmt.Errorf("批量落库失败：%w", berr)
	}
	s.log(ctx, port.LevelINFO, "数据源批量导入完成",
		port.Field{Key: "总数", Value: fmt.Sprintf("%d", len(items))},
		port.Field{Key: "成功", Value: fmt.Sprintf("%d", len(succeededIDs))},
		port.Field{Key: "失败", Value: fmt.Sprintf("%d", len(failures))})
	detail := fmt.Sprintf("总数=%d，成功=%d，失败=%d", len(items), len(succeededIDs), len(failures))
	if len(failures) > 0 {
		var lines []string
		for _, f := range failures {
			lines = append(lines, fmt.Sprintf("#%d %s：%s", f.Index, f.Name, f.Error))
		}
		detail += "；失败明细：" + strings.Join(lines, "；")
	}
	result := "success"
	errMsg := ""
	if len(failures) > 0 {
		result = "partial"
		errMsg = fmt.Sprintf("%d 行未入库", len(failures))
	}
	s.audit(ctx, "datasource.import", "", detail, result, errMsg)
	return succeededIDs, failures, nil
}

// ImportFailure 单条导入失败明细。
type ImportFailure struct {
	Index int
	Name  string
	Error string
}

func (s *Service) toConfig(in AddInput, _ string) port.DataSourceConfig {
	return port.DataSourceConfig{
		Name: in.Name, Dialect: port.Dialect(in.Dialect), Host: in.Host, Port: in.Port,
		ServiceName: in.ServiceName, Username: in.Username, Password: in.Password, DSN: in.DSN,
	}
}

func (s *Service) encryptAndBuild(in AddInput) (port.DataSource, error) {
	hostEnc, err := s.encrypt(in.Host)
	if err != nil {
		return port.DataSource{}, fmt.Errorf("加密主机失败：%w", err)
	}
	passEnc, err := s.encrypt(in.Password)
	if err != nil {
		return port.DataSource{}, fmt.Errorf("加密口令失败：%w", err)
	}
	dsnEnc, err := s.encrypt(in.DSN)
	if err != nil {
		return port.DataSource{}, fmt.Errorf("加密 DSN 失败：%w", err)
	}
	now := time.Now().Format("2006-01-02 15:04:05.000")
	return port.DataSource{
		ID: application.GenID(), Name: in.Name, Dialect: port.Dialect(in.Dialect),
		HostEnc: hostEnc, Port: in.Port, ServiceName: in.ServiceName, Username: in.Username,
		PasswordEnc: passEnc, DSNEnc: dsnEnc, IncludeSchemas: in.IncludeSchemas,
		ExcludeTables: in.ExcludeTables, ExcludeColumns: in.ExcludeColumns,
		ConnMaxOpen: 2, ConnMaxIdle: 1, ConnTimeoutMs: 10000, QueryTimeoutMs: 30000,
		LastTestAt: now, LastTestOk: true, Enabled: true, Remark: "", CreatedAt: now, UpdatedAt: now,
	}, nil
}

func (s *Service) encrypt(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}
	return s.cipher.Encrypt(plain, s.defaultAlg)
}
