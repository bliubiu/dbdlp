// Package mask 脱敏策略实现（对应 docs/02 §14.2 硬规定）。
// 适用范围三处：SQL 输出字面量、运行日志、报告文件样例与明细。
package mask

import (
	"regexp"
	"strings"
)

// ipv4Re 匹配 IPv4 地址。
var ipv4Re = regexp.MustCompile(`^(?:\d{1,3}\.){3}\d{1,3}$`)

// MaskIPv4 前两段替换成 *，保留后两段；回环地址与 localhost 保留完整。
func MaskIPv4(v string) string {
	if v == "127.0.0.1" || v == "localhost" || v == "::1" {
		return v
	}
	if !ipv4Re.MatchString(v) {
		return MaskGeneric(v)
	}
	parts := strings.Split(v, ".")
	if len(parts) != 4 {
		return MaskGeneric(v)
	}
	return "*.*." + parts[2] + "." + parts[3]
}

// MaskIDCard 身份证：保留前 6 位与后 4 位，中间掩码。
func MaskIDCard(v string) string {
	if len(v) < 10 {
		return strings.Repeat("*", len(v))
	}
	return v[:6] + strings.Repeat("*", len(v)-10) + v[len(v)-4:]
}

// MaskPhone 手机号：保留前 3 位与后 4 位。
func MaskPhone(v string) string {
	if len(v) < 7 {
		return strings.Repeat("*", len(v))
	}
	return v[:3] + strings.Repeat("*", len(v)-7) + v[len(v)-4:]
}

// MaskEmail 邮箱：保留首字符与 @ 后域名，中间掩码。
func MaskEmail(v string) string {
	at := strings.Index(v, "@")
	if at <= 0 || at == len(v)-1 {
		return strings.Repeat("*", len(v))
	}
	local := v[:at]
	domain := v[at:]
	if len(local) <= 1 {
		return "*" + domain
	}
	return local[:1] + strings.Repeat("*", len(local)-1) + domain
}

// MaskPassword 密码恒为固定占位串。
func MaskPassword(_ string) string {
	return "******"
}

// MaskGeneric 兜底：保留首尾各 1 字符，中间以 * 替换（长度<=2 全替换）。
func MaskGeneric(v string) string {
	if len(v) <= 2 {
		return strings.Repeat("*", len(v))
	}
	return v[:1] + strings.Repeat("*", len(v)-2) + v[len(v)-1:]
}

// ByType 按敏感类型选择掩码策略；未知类型走兜底。
func ByType(sensitiveType, v string) string {
	switch sensitiveType {
	case "IPV4", "IPV6", "IP":
		if ipv4Re.MatchString(v) {
			return MaskIPv4(v)
		}
		return MaskGeneric(v)
	case "ID_CARD", "ID_CARD_OLD":
		return MaskIDCard(v)
	case "MOBILE_PHONE", "PHONE", "FIXED_PHONE":
		return MaskPhone(v)
	case "EMAIL":
		return MaskEmail(v)
	case "PASSWORD", "PASSWD":
		return MaskPassword(v)
	default:
		return MaskGeneric(v)
	}
}
