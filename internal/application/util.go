// Package application 应用层：编排领域服务与基础设施端口，实现用例。
package application

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// GenID 生成进程内唯一标识（时间纳秒 + 随机十六进制）。
func GenID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), hex.EncodeToString(b))
}

// ContainsToken 判断 csv（逗号分隔）是否包含 tok（忽略大小写与空格）。
func ContainsToken(csv, tok string) bool {
	for _, p := range strings.Split(csv, ",") {
		if strings.EqualFold(strings.TrimSpace(p), strings.TrimSpace(tok)) {
			return true
		}
	}
	return false
}

// SplitCSV 拆分逗号分隔串为去空白切片。
func SplitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
