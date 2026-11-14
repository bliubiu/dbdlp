// Package scan 扫描用例：元数据→采样→识别→聚合→台账→报告 的端到端编排。
package scan

import (
	"context"
	"fmt"
	"strings"
	"time"

	"dbdlp/internal/domain/port"
)

// 本文件实现已存数据源到运行时配置的还原（含凭据解密）。

// datasourceConfig 由已存数据源还原运行时配置（自动解密凭据）。
func (s *Service) datasourceConfig(ctx context.Context, dataSourceID string) (port.DataSourceConfig, error) {
	ds, err := s.store.GetDataSource(ctx, dataSourceID)
	if err != nil {
		return port.DataSourceConfig{}, fmt.Errorf("读取数据源失败：%w", err)
	}
	host, err := s.decrypt(ds.HostEnc)
	if err != nil {
		return port.DataSourceConfig{}, fmt.Errorf("解密主机失败：%w", err)
	}
	pass, err := s.decrypt(ds.PasswordEnc)
	if err != nil {
		return port.DataSourceConfig{}, fmt.Errorf("解密口令失败：%w", err)
	}
	dsn, err := s.decrypt(ds.DSNEnc)
	if err != nil {
		return port.DataSourceConfig{}, fmt.Errorf("解密 DSN 失败：%w", err)
	}
	return port.DataSourceConfig{
		ID: ds.ID, Name: ds.Name, Dialect: ds.Dialect, Host: host, Port: ds.Port,
		ServiceName: ds.ServiceName, Username: ds.Username, Password: pass, DSN: dsn,
		MaxOpenConns: ds.ConnMaxOpen, MaxIdleConns: ds.ConnMaxIdle,
		QueryTimeout: time.Duration(ds.QueryTimeoutMs) * time.Millisecond,
	}, nil
}

// splitTableKey 拆分「模式/表」键。
func splitTableKey(key string) (string, string) {
	if i := strings.Index(key, "/"); i > 0 {
		return key[:i], key[i+1:]
	}
	return "", key
}
