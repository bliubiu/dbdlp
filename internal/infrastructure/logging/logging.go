// Package logging 日志系统实现（AGENTS.md「日志系统」章）。
// 级别 TRACE/DEBUG/INFO/WARN/ERROR，默认 INFO；文件按日轮转、保留期自动清理；
// 日志内容一律中文，且强制脱敏，禁止出现密码、完整身份证号、手机号等敏感数据。
package logging

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"dbdlp/internal/domain/port"
)

// Options 日志器构造参数。
type Options struct {
	Level         port.LogLevel // 最低输出级别
	Dir           string        // 日志目录（默认项目根目录 logs/）
	Console       bool          // 是否同步输出到终端
	RetentionDays int           // 日志保留天数
	FilePrefix    string        // 文件名前缀（默认 dbdlp）
}

// New 构造日志器。
func New(opts Options) (port.Logger, error) {
	level := opts.Level
	if level == "" {
		level = port.LevelINFO
	}
	if !validLevel(level) {
		return nil, fmt.Errorf("非法日志级别：%s", level)
	}
	prefix := opts.FilePrefix
	if prefix == "" {
		prefix = "dbdlp"
	}
	w, err := newWriter(opts.Dir, prefix, opts.RetentionDays, opts.Console)
	if err != nil {
		return nil, err
	}
	return &logger{w: w, level: level}, nil
}

// logger port.Logger 实现。
type logger struct {
	w      *writer
	level  port.LogLevel
	fields []port.Field
}

func (l *logger) Trace(ctx context.Context, msg string, fields ...port.Field) {
	l.log(ctx, port.LevelTRACE, msg, fields)
}

func (l *logger) Debug(ctx context.Context, msg string, fields ...port.Field) {
	l.log(ctx, port.LevelDEBUG, msg, fields)
}

func (l *logger) Info(ctx context.Context, msg string, fields ...port.Field) {
	l.log(ctx, port.LevelINFO, msg, fields)
}

func (l *logger) Warn(ctx context.Context, msg string, fields ...port.Field) {
	l.log(ctx, port.LevelWARN, msg, fields)
}

func (l *logger) Error(ctx context.Context, msg string, fields ...port.Field) {
	l.log(ctx, port.LevelERROR, msg, fields)
}

// With 派生带附加字段的日志器；字段在写入时统一脱敏。
func (l *logger) With(fields ...port.Field) port.Logger {
	merged := make([]port.Field, 0, len(l.fields)+len(fields))
	merged = append(merged, l.fields...)
	merged = append(merged, fields...)
	return &logger{w: l.w, level: l.level, fields: merged}
}

func (l *logger) Close() error { return l.w.close() }

// log 组装并输出一行日志。
// 格式：时间戳  级别  trace_id  进程ID  模块:行号 - 内容 | 字段=值 ...
func (l *logger) log(ctx context.Context, lvl port.LogLevel, msg string, fields []port.Field) {
	if levelValue(lvl) < levelValue(l.level) {
		return
	}
	module, line := callerInfo(3)

	var sb strings.Builder
	sb.WriteString(time.Now().Format("2006-01-02 15:04:05.000"))
	sb.WriteString("  ")
	sb.WriteString(string(lvl))
	sb.WriteString("  ")
	sb.WriteString(orDash(TraceIDFrom(ctx)))
	sb.WriteString("  ")
	sb.WriteString(strconv.Itoa(os.Getpid()))
	sb.WriteString("  ")
	sb.WriteString(module)
	sb.WriteString(":")
	sb.WriteString(strconv.Itoa(line))
	sb.WriteString(" - ")
	sb.WriteString(RedactText(msg))

	all := make([]port.Field, 0, len(l.fields)+len(fields))
	all = append(all, l.fields...)
	all = append(all, fields...)
	for _, f := range all {
		sb.WriteString(" | ")
		sb.WriteString(f.Key)
		sb.WriteString("=")
		sb.WriteString(RedactFieldValue(f.Key, f.Value))
	}
	_ = l.w.write(sb.String())
}

// callerInfo 返回调用方所在文件名与行号。
// skip=0 指向 runtime.Caller 自身，1 指向 log，2 指向级别方法，3 指向业务调用点。
func callerInfo(skip int) (string, int) {
	_, file, line, ok := runtime.Caller(skip)
	if !ok {
		return "unknown", 0
	}
	return filepath.Base(file), line
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// levelValue 级别序：TRACE < DEBUG < INFO < WARN < ERROR。
func levelValue(l port.LogLevel) int {
	switch l {
	case port.LevelTRACE:
		return 0
	case port.LevelDEBUG:
		return 1
	case port.LevelINFO:
		return 2
	case port.LevelWARN:
		return 3
	case port.LevelERROR:
		return 4
	default:
		return 2
	}
}

func validLevel(l port.LogLevel) bool {
	switch l {
	case port.LevelTRACE, port.LevelDEBUG, port.LevelINFO, port.LevelWARN, port.LevelERROR:
		return true
	default:
		return false
	}
}

// ParseLevel 解析配置中的日志级别字符串。
func ParseLevel(s string) (port.LogLevel, error) {
	l := port.LogLevel(strings.ToUpper(strings.TrimSpace(s)))
	if !validLevel(l) {
		return "", fmt.Errorf("非法日志级别：%s（仅允许 TRACE/DEBUG/INFO/WARN/ERROR）", s)
	}
	return l, nil
}

// ===== trace_id =====

type ctxKey struct{ name string }

var traceIDKey = ctxKey{"trace_id"}

// WithTraceID 把链路追踪 ID 放入上下文，供日志输出使用。
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey, traceID)
}

// TraceIDFrom 读取上下文中的链路追踪 ID；不存在时返回空串。
func TraceIDFrom(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(traceIDKey).(string); ok {
		return v
	}
	return ""
}
