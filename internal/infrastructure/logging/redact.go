package logging

import (
	"regexp"
	"strings"

	"dbdlp/internal/domain/mask"
)

// redactedPlaceholder 敏感值的统一占位串（与 mask.MaskPassword 保持一致）。
const redactedPlaceholder = "******"

// sensitiveKeys 命中即整体替换，不做任何部分保留。
// 键名统一小写并把 '-'/' ' 归一为 '_' 后比对。
var sensitiveKeys = map[string]bool{
	"password": true, "passwd": true, "pwd": true, "pass": true,
	"口令": true, "密码": true, "密碼": true,
	"secret": true, "token": true, "authorization": true,
	"apikey": true, "api_key": true, "access_key": true, "secret_key": true,
	"key": true, "dsn": true, "credential": true, "credentials": true,
	"privatekey": true, "private_key": true, "cookie": true, "session": true,
}

var (
	// 身份证 18 位（末位可为 X）。
	idCardRe = regexp.MustCompile(`\b[1-9]\d{5}(?:19|20)\d{2}(?:0[1-9]|1[0-2])(?:0[1-9]|[12]\d|3[01])\d{3}[\dXx]\b`)
	// 手机号 11 位。
	phoneRe = regexp.MustCompile(`\b1[3-9]\d{9}\b`)
	// 银行卡候选 16~19 位；还需再过 Luhn 校验才认定为银行卡，避免误伤系统流水号/长数字 ID。
	bankRe = regexp.MustCompile(`\b\d{16,19}\b`)
	// IPv4 地址。
	ipv4Re = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
)

// RedactFieldValue 按字段名脱敏：敏感键整体替换，其余按键值内容做模式脱敏。
func RedactFieldValue(key, value string) string {
	if isSensitiveKey(key) {
		return redactedPlaceholder
	}
	return RedactText(value)
}

// RedactText 对自由文本中的敏感模式脱敏（身份证、手机号、银行卡、IPv4）。
// 脱敏顺序有意义：身份证先于银行卡处理，避免 18 位身份证被银行卡规则二次命中。
func RedactText(s string) string {
	if s == "" {
		return s
	}
	s = idCardRe.ReplaceAllStringFunc(s, func(m string) string { return mask.MaskIDCard(m) })
	s = phoneRe.ReplaceAllStringFunc(s, func(m string) string { return mask.MaskPhone(m) })
	s = bankRe.ReplaceAllStringFunc(s, func(m string) string {
		if !luhnOK(m) {
			return m // 非 Luhn 有效位的长数字视为系统标识，不脱敏
		}
		return mask.MaskGeneric(m)
	})
	s = ipv4Re.ReplaceAllStringFunc(s, func(m string) string { return mask.MaskIPv4(m) })
	return s
}

// luhnOK Luhn 模 10 校验；仅校验位通过的 16~19 位数字才按银行卡脱敏。
// 此处内联实现，避免基础设施层反向依赖领域校验器注册表。
func luhnOK(s string) bool {
	if len(s) < 16 || len(s) > 19 {
		return false
	}
	sum := 0
	alt := false
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
		d := int(s[i] - '0')
		if alt {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
		alt = !alt
	}
	return sum%10 == 0
}

// isSensitiveKey 判断字段名是否属于禁止明文记录的敏感字段。
func isSensitiveKey(key string) bool {
	k := strings.ToLower(strings.TrimSpace(key))
	k = strings.ReplaceAll(k, "-", "_")
	k = strings.ReplaceAll(k, " ", "_")
	return sensitiveKeys[k]
}
