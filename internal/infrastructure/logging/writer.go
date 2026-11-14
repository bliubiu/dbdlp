package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// writer 负责日志文件写入：按日轮转、保留期自动清理、终端同步输出。
// 同一时刻只持有当天的文件句柄，跨天时切换。
type writer struct {
	mu            sync.Mutex
	dir           string
	prefix        string
	retentionDays int
	console       bool
	day           string
	file          *os.File
	closed        bool
}

func newWriter(dir, prefix string, retentionDays int, console bool) (*writer, error) {
	if dir == "" {
		return nil, fmt.Errorf("日志目录不能为空")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("创建日志目录失败：%w", err)
	}
	w := &writer{dir: dir, prefix: prefix, retentionDays: retentionDays, console: console}
	w.cleanup()
	return w, nil
}

// fileName 返回日志文件名：dbdlp-20261114.log
func (w *writer) fileName(day string) string {
	return fmt.Sprintf("%s-%s.log", w.prefix, day)
}

// write 写入一行日志；文件不可用时返回错误（调用方忽略即可，不影响主流程）。
func (w *writer) write(line string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil
	}
	if err := w.rotateLocked(time.Now()); err != nil {
		return err
	}
	if _, err := w.file.WriteString(line + "\n"); err != nil {
		return fmt.Errorf("写日志文件失败：%w", err)
	}
	if w.console {
		// 终端输出走 stderr，避免与 CLI 的标准输出混在一起。
		fmt.Fprintln(os.Stderr, line)
	}
	return nil
}

// rotateLocked 跨天时切换文件句柄；切换当天顺带清理过期日志。
func (w *writer) rotateLocked(now time.Time) error {
	day := now.Format("20060102")
	if w.file != nil && w.day == day {
		return nil
	}
	if w.file != nil {
		_ = w.file.Close()
		w.file = nil
	}
	f, err := os.OpenFile(filepath.Join(w.dir, w.fileName(day)), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("打开日志文件失败：%w", err)
	}
	w.file = f
	w.day = day
	w.cleanup()
	return nil
}

// cleanup 删除超出保留期的历史日志文件。
func (w *writer) cleanup() {
	if w.retentionDays <= 0 {
		return
	}
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		return
	}
	// 保留最近 retentionDays 天（含今日）：早于截止日的文件删除。
	cutoff := time.Now().AddDate(0, 0, -(w.retentionDays - 1)).Format("20060102")
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		day, ok := parseLogDay(w.prefix, e.Name())
		if !ok {
			continue
		}
		if day < cutoff {
			_ = os.Remove(filepath.Join(w.dir, e.Name()))
		}
	}
}

// parseLogDay 从文件名解析日期串，形如 dbdlp-20261114.log → 20261114。
func parseLogDay(prefix, name string) (string, bool) {
	if !strings.HasPrefix(name, prefix+"-") || !strings.HasSuffix(name, ".log") {
		return "", false
	}
	day := strings.TrimSuffix(strings.TrimPrefix(name, prefix+"-"), ".log")
	if len(day) != 8 {
		return "", false
	}
	for _, c := range day {
		if c < '0' || c > '9' {
			return "", false
		}
	}
	return day, true
}

func (w *writer) close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.closed = true
	if w.file != nil {
		err := w.file.Close()
		w.file = nil
		return err
	}
	return nil
}
