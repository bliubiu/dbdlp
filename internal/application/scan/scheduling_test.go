// scheduling_test.go 低峰窗口判断单测。
package scan

import (
	"testing"
	"time"
)

func TestInLowPeakWindow(t *testing.T) {
	mk := func(h, m int) time.Time { return time.Date(2026, 11, 14, h, m, 0, 0, time.Local) }
	cases := []struct {
		name   string
		window string
		now    time.Time
		want   bool
	}{
		{"窗口内-常规", "09:00-18:00", mk(12, 0), true},
		{"窗口外-常规", "09:00-18:00", mk(20, 0), false},
		{"跨午夜-深夜内", "22:00-06:00", mk(23, 30), true},
		{"跨午夜-凌晨内", "22:00-06:00", mk(5, 59), true},
		{"跨午夜-白日外", "22:00-06:00", mk(12, 0), false},
		{"边界-起点含", "22:00-06:00", mk(22, 0), true},
		{"边界-终点排他", "22:00-06:00", mk(6, 0), false},
		{"非法窗口-不限", "abc-def", mk(12, 0), true},
		{"空窗口-不限", "", mk(12, 0), true},
	}
	for _, c := range cases {
		if got := inLowPeakWindow(c.now, c.window); got != c.want {
			t.Errorf("%s：期望 %v，got %v", c.name, c.want, got)
		}
	}
}

func TestParseHM(t *testing.T) {
	if v, err := parseHM("22:05"); err != nil || v != 22*60+5 {
		t.Errorf("解析错误：%v %v", v, err)
	}
	for _, bad := range []string{"", "22", "24:00", "12:60", "ab:cd", "12:"} {
		if _, err := parseHM(bad); err == nil {
			t.Errorf("期望拒绝：%q", bad)
		}
	}
}
