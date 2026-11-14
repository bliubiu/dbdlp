// Package testing 提供应用层测试替身（docs/07 §1.3：替身不得定义在 domain 内）。
package testing

import (
	"context"

	"dbdlp/internal/domain/port"
)

// NopLogger 丢弃全部日志的替身；用于应用层单测避免真实落盘。
type NopLogger struct{}

// NewNopLogger 构造空日志器替身。
func NewNopLogger() *NopLogger { return &NopLogger{} }

// Trace 实现 port.Logger。
func (NopLogger) Trace(context.Context, string, ...port.Field) {}

// Debug 实现 port.Logger。
func (NopLogger) Debug(context.Context, string, ...port.Field) {}

// Info 实现 port.Logger。
func (NopLogger) Info(context.Context, string, ...port.Field) {}

// Warn 实现 port.Logger。
func (NopLogger) Warn(context.Context, string, ...port.Field) {}

// Error 实现 port.Logger。
func (NopLogger) Error(context.Context, string, ...port.Field) {}

// With 实现 port.Logger（返回自身）。
func (l NopLogger) With(...port.Field) port.Logger { return l }

// Close 实现 port.Logger。
func (NopLogger) Close() error { return nil }

var _ port.Logger = NopLogger{}
