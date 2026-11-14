package rule

import "dbdlp/internal/domain/port"

// BuiltinRules 内置识别规则（自研，对应 docs/02 §6.2/§6.3）。
// 校验器名须已在 registry 注册；Version 由加载器统一填充。
func BuiltinRules() []port.Rule {
	return []port.Rule{
		{
			RuleID: "MOBILE_PHONE", SensitiveType: "MOBILE_PHONE", Enabled: true,
			Regex: `^1[3-9][0-9]{9}$`, Verifier: "mobile_segment", Threshold: 0.8,
			Bytescan: port.BytescanSpec{MinLen: 11, MaxLen: 11, DigitsOnly: true},
		},
		{
			RuleID: "ID_CARD", SensitiveType: "ID_CARD", Enabled: true,
			Regex:    `^[1-9][0-9]{5}(?:19|20)[0-9]{2}(?:0[1-9]|1[0-2])(?:0[1-9]|[12][0-9]|3[01])[0-9]{3}[0-9Xx]$`,
			Verifier: "id_card_checksum", Threshold: 0.8,
			Bytescan: port.BytescanSpec{MinLen: 18, MaxLen: 18},
		},
		{
			RuleID: "BANK_CARD", SensitiveType: "BANK_CARD", Enabled: true,
			Regex: `^[0-9]{16,19}$`, Verifier: "luhn", Threshold: 0.8,
			Bytescan: port.BytescanSpec{MinLen: 16, MaxLen: 19, DigitsOnly: true},
		},
		{
			RuleID: "SOCIAL_CREDIT_CODE", SensitiveType: "SOCIAL_CREDIT_CODE", Enabled: true,
			Regex: `^[0-9A-HJ-NP-RTUWXY]{18}$`, Verifier: "social_credit_code", Threshold: 0.8,
			Bytescan: port.BytescanSpec{MinLen: 18, MaxLen: 18},
		},
		{
			RuleID: "EMAIL", SensitiveType: "EMAIL", Enabled: true,
			Regex:    `^[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}$`,
			Verifier: "email_format", Threshold: 0.8,
			Bytescan: port.BytescanSpec{MinLen: 6, MaxLen: 254},
		},
		{
			RuleID: "IPV4", SensitiveType: "IPV4", Enabled: true,
			Regex: `^(?:\d{1,3}\.){3}\d{1,3}$`, Verifier: "ipv4", Threshold: 0.8,
			Bytescan: port.BytescanSpec{MinLen: 7, MaxLen: 15},
		},
		{
			RuleID: "PASSPORT", SensitiveType: "PASSPORT", Enabled: true,
			Regex: `^[A-Za-z][0-9]{8}$`, Verifier: "passport", Threshold: 0.6,
			Bytescan: port.BytescanSpec{MinLen: 9, MaxLen: 9},
		},
		{
			RuleID: "PLATE", SensitiveType: "PLATE", Enabled: true,
			Regex:    `^[京津沪渝冀豫云辽黑湘皖鲁新苏浙赣鄂桂甘晋蒙陕吉闽贵粤青藏川宁琼使领][A-Z][A-Z0-9]{5,6}$`,
			Verifier: "plate", Threshold: 0.6,
		},
		{
			RuleID: "POSTCODE", SensitiveType: "POSTCODE", Enabled: true,
			Regex: `^[0-9]{6}$`, Verifier: "postcode", Threshold: 0.6,
			Bytescan: port.BytescanSpec{MinLen: 6, MaxLen: 6, DigitsOnly: true},
		},
		{
			RuleID: "FIXED_PHONE", SensitiveType: "FIXED_PHONE", Enabled: true,
			Regex: `^0\d{2,3}-?\d{7,8}$`, Threshold: 0.5,
			Bytescan: port.BytescanSpec{MinLen: 11, MaxLen: 13},
		},
		{
			RuleID: "IPV6", SensitiveType: "IPV6", Enabled: true,
			Regex:    `^(?:[0-9A-Fa-f]{1,4}:){7}[0-9A-Fa-f]{1,4}|^(?:[0-9A-Fa-f]{1,4}:){1,7}:|^(?:[0-9A-Fa-f]{1,4}:){1,6}:[0-9A-Fa-f]{1,4}|^(?:[0-9A-Fa-f]{1,4}:){1,5}(?::[0-9A-Fa-f]{1,4}){1,2}|^(?:[0-9A-Fa-f]{1,4}:){1,4}(?::[0-9A-Fa-f]{1,4}){1,3}|^(?:[0-9A-Fa-f]{1,4}:){1,3}(?::[0-9A-Fa-f]{1,4}){1,4}|^(?:[0-9A-Fa-f]{1,4}:){1,2}(?::[0-9A-Fa-f]{1,4}){1,5}|^[0-9A-Fa-f]{1,4}:(?::[0-9A-Fa-f]{1,4}){1,6}|^:(?::[0-9A-Fa-f]{1,4}){1,7}|^::(?:[fF]{4}:)?(?:[0-9]{1,3}\.){3}[0-9]{1,3}|^(?:[0-9A-Fa-f]{1,4}:){1,4}:(?:[0-9]{1,3}\.){3}[0-9]{1,3}`,
			Verifier: "ipv6", Threshold: 0.6,
			Bytescan: port.BytescanSpec{MinLen: 2, MaxLen: 45},
		},
		{
			RuleID: "MAC", SensitiveType: "MAC", Enabled: true,
			Regex:    `^[0-9A-Fa-f]{2}(?::[0-9A-Fa-f]{2}){5}$|^[0-9A-Fa-f]{2}(?:-[0-9A-Fa-f]{2}){5}$`,
			Verifier: "mac", Threshold: 0.6,
			Bytescan: port.BytescanSpec{MinLen: 14, MaxLen: 17},
		},
		{
			RuleID: "DOMAIN", SensitiveType: "DOMAIN", Enabled: true,
			Regex:    `^(?:[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,63}$`,
			Verifier: "domain", Threshold: 0.6,
			Bytescan: port.BytescanSpec{MinLen: 4, MaxLen: 253},
		},
	}
}

// HighSensitivity 高敏感类型（明文 + 高敏感 → 风险 high）。
var HighSensitivity = map[string]bool{
	"ID_CARD":            true,
	"MOBILE_PHONE":       true,
	"BANK_CARD":          true,
	"PASSPORT":           true,
	"SOCIAL_CREDIT_CODE": true,
}

// columnNameHints 字段名/注释关键词 → 敏感类型（元数据辅助，亦用于列名吻合度判断）。
var columnNameHints = []struct {
	Type     string
	Keywords []string
}{
	{"MOBILE_PHONE", []string{"phone", "mobile", "tel", "手机", "电话", "mobilephone", "sj"}},
	{"ID_CARD", []string{"id", "身份证", "sfz", "idcard", "证件", "id_no"}},
	{"BANK_CARD", []string{"bank", "card", "银行卡", "卡号", "bankcard", "yhk"}},
	{"EMAIL", []string{"email", "mail", "邮箱", "yx"}},
	{"SOCIAL_CREDIT_CODE", []string{"credit", "信用代码", "usc", "统一社会信用", "shxydm", "tyshxydm"}},
	{"PASSPORT", []string{"passport", "护照", "hz"}},
	{"PLATE", []string{"plate", "车牌", "carno", "号牌", "cp"}},
	{"POSTCODE", []string{"postcode", "zip", "邮编", "yb"}},
	{"FIXED_PHONE", []string{"tel", "固话", "座机", "gh"}},
	{"NAME", []string{"name", "姓名", "客户名", "用户名", "realname", "xm"}},
	{"ADDRESS", []string{"address", "地址", "住址", "位置", "dz"}},
	{"COMPANY", []string{"company", "公司", "企业", "单位", "corp", "gs"}},
}

// MatchColumnMeta 按字段名/注释关键词匹配元数据弱命中类型；无匹配返回空。
func MatchColumnMeta(columnName, comment string) []string {
	var out []string
	hay := lowerASCII(columnName + " " + comment)
	for _, h := range columnNameHints {
		for _, kw := range h.Keywords {
			if containsToken(hay, kw) {
				out = append(out, h.Type)
				break
			}
		}
	}
	return out
}

func lowerASCII(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 'a' - 'A'
		}
	}
	return string(b)
}

// containsToken 以单词边界（非字母数字）判断关键词是否存在，避免子串误命中。
func containsToken(hay, kw string) bool {
	kw = lowerASCII(kw)
	if kw == "" {
		return false
	}
	idx := 0
	for idx < len(hay) {
		i := indexOf(hay, kw, idx)
		if i < 0 {
			return false
		}
		before := i == 0 || !isAlnum(hay[i-1])
		after := i+len(kw) >= len(hay) || !isAlnum(hay[i+len(kw)])
		if before && after {
			return true
		}
		idx = i + len(kw)
	}
	return false
}

func indexOf(s, sub string, from int) int {
	for i := from; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func isAlnum(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')
}
