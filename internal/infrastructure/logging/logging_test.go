package logging

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dbdlp/internal/domain/port"
)

func newTestLogger(t *testing.T, level port.LogLevel, dir string) port.Logger {
	t.Helper()
	l, err := New(Options{Level: level, Dir: dir, Console: false, RetentionDays: 32, FilePrefix: "dbdlp"})
	if err != nil {
		t.Fatalf("New：%v", err)
	}
	return l
}

// 读取当天日志文件内容。
func readToday(t *testing.T, dir string) string {
	t.Helper()
	name := "dbdlp-" + time.Now().Format("20060102") + ".log"
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("读取日志文件 %s：%v", name, err)
	}
	return string(data)
}

func Test格式与内容(t *testing.T) {
	dir := t.TempDir()
	l := newTestLogger(t, port.LevelINFO, dir)
	defer l.Close()

	ctx := WithTraceID(context.Background(), "trace-abc-123")
	l.Info(ctx, "扫描任务开始", port.Field{Key: "数据源", Value: "orcl_prod"})

	content := readToday(t, dir)
	if !strings.Contains(content, "INFO") {
		t.Fatalf("缺少级别：%q", content)
	}
	if !strings.Contains(content, "trace-abc-123") {
		t.Fatalf("缺少 trace_id：%q", content)
	}
	if !strings.Contains(content, "扫描任务开始") {
		t.Fatalf("缺少日志内容：%q", content)
	}
	if !strings.Contains(content, "数据源=orcl_prod") {
		t.Fatalf("缺少附加字段：%q", content)
	}
	// 格式：时间戳  级别  trace_id  PID  模块:行号 - 内容
	parts := strings.SplitN(strings.TrimSpace(content), "  ", 5)
	if len(parts) < 5 {
		t.Fatalf("格式不符合规范（应为 5 段）：%q", content)
	}
	if _, err := time.Parse("2006-01-02 15:04:05.000", parts[0]); err != nil {
		t.Fatalf("时间戳格式错误：%q", parts[0])
	}
	if !strings.Contains(parts[4], " - ") {
		t.Fatalf("缺少「模块:行号 - 内容」段：%q", parts[4])
	}
	if !strings.Contains(parts[4], ".go:") {
		t.Fatalf("缺少模块:行号：%q", parts[4])
	}
}

func Test级别过滤(t *testing.T) {
	dir := t.TempDir()
	l := newTestLogger(t, port.LevelERROR, dir)
	defer l.Close()

	l.Info(context.Background(), "这条不应出现")
	l.Warn(context.Background(), "这条也不应出现")
	l.Error(context.Background(), "这条应该出现")

	content := readToday(t, dir)
	if strings.Contains(content, "这条不应出现") || strings.Contains(content, "这条也不应出现") {
		t.Fatalf("低于级别的日志被写出：%q", content)
	}
	if !strings.Contains(content, "这条应该出现") {
		t.Fatalf("ERROR 日志缺失：%q", content)
	}
}

func Test敏感信息脱敏(t *testing.T) {
	dir := t.TempDir()
	l := newTestLogger(t, port.LevelINFO, dir)
	defer l.Close()

	l.Info(context.Background(), "用户手机号 13800008000 已登记",
		port.Field{Key: "password", Value: "Tiger123"},
		port.Field{Key: "身份证", Value: "110101199003071234"},
		port.Field{Key: "主机", Value: "192.168.10.180"})

	content := readToday(t, dir)
	if strings.Contains(content, "Tiger123") {
		t.Fatalf("日志泄露明文口令：%q", content)
	}
	if strings.Contains(content, "13800008000") {
		t.Fatalf("日志泄露完整手机号：%q", content)
	}
	if strings.Contains(content, "110101199003071234") {
		t.Fatalf("日志泄露完整身份证号：%q", content)
	}
	if !strings.Contains(content, "password=******") {
		t.Fatalf("口令字段未掩码：%q", content)
	}
	if !strings.Contains(content, "138****8000") {
		t.Fatalf("手机号未脱敏：%q", content)
	}
	if !strings.Contains(content, "*.*.10.180") {
		t.Fatalf("IPv4 未脱敏：%q", content)
	}
}

func TestWith派生字段(t *testing.T) {
	dir := t.TempDir()
	l := newTestLogger(t, port.LevelINFO, dir)
	defer l.Close()

	l.With(port.Field{Key: "任务", Value: "task-1"}).
		With(port.Field{Key: "库", Value: "SCOTT"}).
		Info(context.Background(), "开始采样")

	content := readToday(t, dir)
	if !strings.Contains(content, "任务=task-1") || !strings.Contains(content, "库=SCOTT") {
		t.Fatalf("派生字段缺失：%q", content)
	}
}

func Test按日轮转与保留期清理(t *testing.T) {
	dir := t.TempDir()
	// 造一个 100 天前的过期日志文件
	old := filepath.Join(dir, "dbdlp-"+time.Now().AddDate(0, 0, -100).Format("20060102")+".log")
	if err := os.WriteFile(old, []byte("过期"), 0o600); err != nil {
		t.Fatalf("写过期日志：%v", err)
	}
	// 一个无关文件，不应被删除
	keep := filepath.Join(dir, "readme.txt")
	if err := os.WriteFile(keep, []byte("保留"), 0o600); err != nil {
		t.Fatalf("写无关文件：%v", err)
	}

	l := newTestLogger(t, port.LevelINFO, dir)
	defer l.Close()
	l.Info(context.Background(), "触发一次写入")

	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatalf("过期日志未被清理")
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatalf("无关文件被误删：%v", err)
	}
}

func Test文件名规范(t *testing.T) {
	dir := t.TempDir()
	l := newTestLogger(t, port.LevelINFO, dir)
	defer l.Close()
	l.Info(context.Background(), "写一行")

	want := "dbdlp-" + time.Now().Format("20060102") + ".log"
	if _, err := os.Stat(filepath.Join(dir, want)); err != nil {
		t.Fatalf("日志文件名不符合「项目名-YYYYMMDD.log」：%v", err)
	}
}

func Test非法级别(t *testing.T) {
	if _, err := New(Options{Level: port.LogLevel("VERBOSE"), Dir: t.TempDir()}); err == nil {
		t.Fatalf("非法日志级别应报错")
	}
	if _, err := ParseLevel("warn"); err != nil {
		t.Fatalf("ParseLevel 应大小写不敏感：%v", err)
	}
}

// Test银行卡脱敏需过Luhn：真实卡号脱敏，系统长数字 ID 保持可读。
func Test银行卡脱敏需过Luhn(t *testing.T) {
	dir := t.TempDir()
	l := newTestLogger(t, port.LevelINFO, dir)
	defer l.Close()

	// 4111111111111111 为 Luhn 有效测试卡号；1794621557670604800 为系统流水 ID。
	l.Info(context.Background(), "数据源已创建",
		port.Field{Key: "卡号", Value: "4111111111111111"},
		port.Field{Key: "数据源ID", Value: "1794621557670604800"})

	content := readToday(t, dir)
	if strings.Contains(content, "4111111111111111") {
		t.Fatalf("日志泄露完整银行卡号：%q", content)
	}
	if !strings.Contains(content, "卡号=4**************1") {
		t.Fatalf("银行卡未脱敏：%q", content)
	}
	if !strings.Contains(content, "数据源ID=1794621557670604800") {
		t.Fatalf("系统长数字 ID 被误脱敏，影响排障：%q", content)
	}
}

// Test长数字非Luhn不脱敏：订单号等系统标识保持完整。
func Test长数字非Luhn不脱敏(t *testing.T) {
	// 订单 20261114000000001 经 Luhn 校验不通过，视为系统标识，不脱敏。
	in := "订单 1794621557670604800 已处理"
	got := RedactText(in)
	if got != in {
		t.Fatalf("非 Luhn 长数字不应脱敏：%q", got)
	}
	// Luhn 有效则必须脱敏
	card := "4111111111111111"
	if strings.Contains(RedactText(card), card) {
		t.Fatalf("Luhn 有效卡号必须脱敏")
	}
}
