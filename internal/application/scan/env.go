package scan

import (
	"context"
	"fmt"

	"dbdlp/internal/domain/detector"
	"dbdlp/internal/domain/port"
)

// 本文件实现扫描执行环境（目标库连接 + 识别引擎）的构造与释放。

// scanEnv 扫描执行环境。
type scanEnv struct {
	cfg    port.DataSourceConfig
	rules  []port.Rule
	ver    string
	driver port.Driver
	conn   port.ReadOnlyConn
	det    port.Detector
}

// newScanEnv 构造扫描执行环境：还原数据源配置、加载规则、连接目标库、构建识别引擎。
func (s *Service) newScanEnv(ctx context.Context, task port.ScanTask) (*scanEnv, error) {
	cfg, err := s.datasourceConfig(ctx, task.DataSourceID)
	if err != nil {
		return nil, err
	}
	rules, version, err := s.rulesLoader()
	if err != nil {
		return nil, fmt.Errorf("加载规则失败：%w", err)
	}
	det, err := detector.New(rules, version)
	if err != nil {
		return nil, fmt.Errorf("构建识别引擎失败：%w", err)
	}
	driver, err := s.driverFactory(cfg.Dialect)
	if err != nil {
		_ = det.Close()
		return nil, fmt.Errorf("方言不支持：%v", err)
	}
	conn, err := driver.Connect(ctx, cfg)
	if err != nil {
		_ = det.Close()
		return nil, fmt.Errorf("连接失败：%v", err)
	}
	return &scanEnv{cfg: cfg, rules: rules, ver: version, driver: driver, conn: conn, det: det}, nil
}

// Close 释放识别引擎与目标库连接。
func (e *scanEnv) Close() {
	if e.det != nil {
		_ = e.det.Close()
	}
	if e.conn != nil {
		_ = e.conn.Close()
	}
}
