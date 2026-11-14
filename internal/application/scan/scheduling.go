// scheduling.go 限速与低峰窗口的编排消费（docs/02 §8.6：分批限流、低峰执行）。
package scan

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"dbdlp/internal/domain/port"
)

// WithScheduling 注入批次间歇（毫秒）与低峰窗口（"22:00-06:00"，跨午夜合法）。
// 间歇 0 表示不限速；窗口空或非法表示不限低峰。
func (s *Service) WithScheduling(batchIntervalMs int, lowPeakWindow string) *Service {
	if batchIntervalMs > 0 {
		s.batchIntervalMs = batchIntervalMs
	}
	if lp := strings.TrimSpace(lowPeakWindow); lp != "" {
		s.lowPeakWindow = lp
	}
	return s
}

// inLowPeakWindow 判断 now 是否处于低峰窗口内（窗口内才执行查询）。
// 窗口格式 "HH:MM-HH:MM"，支持跨午夜；解析失败视为不限（恒 true）。
func inLowPeakWindow(now time.Time, window string) bool {
	parts := strings.Split(window, "-")
	if len(parts) != 2 {
		return true
	}
	start, err1 := parseHM(parts[0])
	end, err2 := parseHM(parts[1])
	if err1 != nil || err2 != nil {
		return true
	}
	cur := now.Hour()*60 + now.Minute()
	s, e := start, end
	if s == e {
		return true
	}
	if s < e {
		return cur >= s && cur < e
	}
	return cur >= s || cur < e // 跨午夜
}

// parseHM 解析 "HH:MM" 为当日分钟数。
func parseHM(v string) (int, error) {
	v = strings.TrimSpace(v)
	colon := strings.Index(v, ":")
	if colon <= 0 || colon == len(v)-1 {
		return 0, fmt.Errorf("时间格式非法：%s", v)
	}
	h, err1 := strconv.Atoi(v[:colon])
	m, err2 := strconv.Atoi(v[colon+1:])
	if err1 != nil || err2 != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, fmt.Errorf("时间格式非法：%s", v)
	}
	return h*60 + m, nil
}

// waitLowPeak 低峰窗口外暂停查询：轮询等待直至进入窗口或 ctx 取消。
func (s *Service) waitLowPeak(ctx context.Context) error {
	if s.lowPeakWindow == "" {
		return nil
	}
	if inLowPeakWindow(time.Now(), s.lowPeakWindow) {
		return nil
	}
	s.log(ctx, port.LevelINFO, "处于低峰窗口外，暂停采样查询",
		port.Field{Key: "低峰窗口", Value: s.lowPeakWindow})
	const poll = 30 * time.Second
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(poll):
		}
		if inLowPeakWindow(time.Now(), s.lowPeakWindow) {
			s.log(ctx, port.LevelINFO, "进入低峰窗口，恢复采样查询",
				port.Field{Key: "低峰窗口", Value: s.lowPeakWindow})
			return nil
		}
	}
}

// paceBatch 批次间歇限速：批与批之间等待 batchIntervalMs（ctx 可取消）。
func (s *Service) paceBatch(ctx context.Context) error {
	if s.batchIntervalMs <= 0 {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(time.Duration(s.batchIntervalMs) * time.Millisecond):
		return nil
	}
}
