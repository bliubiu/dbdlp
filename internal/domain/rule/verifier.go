// Package rule 敏感识别规则与校验器（自研，不依赖 godlp）。
package rule

import (
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// verifierFunc 校验器实现：返回候选值是否为合法业务值。
type verifierFunc func(candidate string) bool

// registry 校验器注册表（注册式，启动时校验规则引用的校验器已注册）。
var registry = map[string]verifierFunc{
	"id_card_checksum":   idCardChecksum,
	"luhn":               luhn,
	"mobile_segment":     mobileSegment,
	"email_format":       emailFormat,
	"social_credit_code": socialCreditCode,
	"passport":           passport,
	"plate":              plate,
	"postcode":           postcode,
	"ipv4":               ipv4,
	"ipv6":               ipv6,
	"mac":                mac,
	"domain":             domainVerifier,
}

// GetVerifier 取已注册校验器；未注册返回 nil。
func GetVerifier(name string) func(string) bool {
	if name == "" {
		return nil
	}
	return registry[name]
}

// RegisteredVerifiers 返回全部已注册校验器名（用于规则加载校验与快照）。
func RegisteredVerifiers() []string {
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	return names
}

// ===== 各校验器实现 =====

// idCardChecksum 18 位身份证：出生日期合法 + 加权模 11 校验位。
func idCardChecksum(s string) bool {
	if len(s) != 18 {
		return false
	}
	// 出生日期 YYYYMMDD
	birth := s[6:14]
	if _, err := time.Parse("20060102", birth); err != nil {
		return false
	}
	weights := []int{7, 9, 10, 5, 8, 4, 2, 1, 6, 3, 7, 9, 10, 5, 8, 4, 2}
	checkCodes := "10X98765432"
	sum := 0
	for i := 0; i < 17; i++ {
		d, err := strconv.Atoi(string(s[i]))
		if err != nil {
			return false
		}
		sum += d * weights[i]
	}
	want := string(checkCodes[sum%11])
	got := strings.ToUpper(string(s[17]))
	return want == got
}

// luhn 银行卡号 Luhn 模 10。
func luhn(s string) bool {
	if len(s) < 13 || len(s) > 19 {
		return false
	}
	sum := 0
	alt := false
	for i := len(s) - 1; i >= 0; i-- {
		c := s[i]
		if c < '0' || c > '9' {
			return false
		}
		d := int(c - '0')
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

// mobileSegment 合法号段 + 11 位 + 非全同号。
func mobileSegment(s string) bool {
	if len(s) != 11 {
		return false
	}
	if !regexp.MustCompile(`^1[3-9][0-9]{9}$`).MatchString(s) {
		return false
	}
	allSame := true
	for i := 1; i < len(s); i++ {
		if s[i] != s[0] {
			allSame = false
			break
		}
	}
	return !allSame
}

// emailFormat 本地域/域名结构，MX 非必须。
func emailFormat(s string) bool {
	at := strings.Index(s, "@")
	if at <= 0 || at != strings.LastIndex(s, "@") {
		return false
	}
	domain := s[at+1:]
	dot := strings.LastIndex(domain, ".")
	if dot <= 0 || dot == len(domain)-1 {
		return false
	}
	if strings.ContainsAny(s[:at], "@ ") || strings.Contains(domain, "..") {
		return false
	}
	return true
}

// socialCreditCode 统一社会信用代码（GB32100-2015）校验位。
func socialCreditCode(s string) bool {
	const body = "0123456789ABCDEFGHJKLMNPQRTUWXY"
	if len(s) != 18 {
		return false
	}
	weights := []int{1, 3, 9, 27, 19, 26, 16, 17, 20, 29, 25, 13, 8, 24, 10, 30, 28}
	sum := 0
	for i := 0; i < 17; i++ {
		idx := strings.IndexByte(body, s[i])
		if idx < 0 {
			return false
		}
		sum += idx * weights[i]
	}
	want := body[sum%31]
	return want == s[17]
}

// passport 护照：1-2 位字母前缀 + 7~8 位数字（含中国电子护照 E/G 与
// 双字母前缀形态，openDLP 基线回流）。
func passport(s string) bool {
	return regexp.MustCompile(`^[A-Za-z]{1,2}[0-9]{7,8}$`).MatchString(strings.ToUpper(s))
}

// plate 中国大陆车牌：省份简称 + 字母 + 5 位字母数字（含新能源 6 位）。
func plate(s string) bool {
	return regexp.MustCompile(`^[京津沪渝冀豫云辽黑湘皖鲁新苏浙赣鄂桂甘晋蒙陕吉闽贵粤青藏川宁琼使领][A-Z][A-Z0-9]{5,6}$`).MatchString(strings.ToUpper(s))
}

// postcode 中国邮编：6 位数字。
func postcode(s string) bool {
	return regexp.MustCompile(`^[0-9]{6}$`).MatchString(s)
}

// ipv4 标准 IPv4 格式（每段 0-255）。
func ipv4(s string) bool {
	parts := strings.Split(s, ".")
	if len(parts) != 4 {
		return false
	}
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 || n > 255 {
			return false
		}
	}
	return true
}

// ipv6 合法 IPv6（含 :: 压缩与内嵌 IPv4 映射形态），交给标准库精确判定。
func ipv6(s string) bool {
	if s == "::" { // 全压缩空地址（未指定地址）不作为业务数据值
		return false
	}
	ip := net.ParseIP(s)
	return ip != nil && strings.Contains(s, ":")
}

// mac MAC 地址：冒号或连字符分隔的 6 组十六进制，分隔符一致。
func mac(s string) bool {
	if len(s) != 17 {
		return false
	}
	sep := byte(':')
	if s[2] == '-' {
		sep = '-'
	} else if s[2] != ':' {
		return false
	}
	for i := 0; i < 17; i++ {
		c := s[i]
		if i%3 == 2 {
			if c != sep {
				return false
			}
			continue
		}
		if !isHexDigit(c) {
			return false
		}
	}
	return true
}

func isHexDigit(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

// domainVerifier 域名：总长与标签长度合法、标签字符集 [a-zA-Z0-9-] 且首尾非连字符、
// TLD 为纯字母，并排除 IPv4 形态误命中。
func domainVerifier(s string) bool {
	if len(s) > 253 || net.ParseIP(s) != nil {
		return false
	}
	labels := strings.Split(s, ".")
	if len(labels) < 2 {
		return false
	}
	for i, l := range labels {
		if len(l) == 0 || len(l) > 63 {
			return false
		}
		if l[0] == '-' || l[len(l)-1] == '-' {
			return false
		}
		for _, r := range l {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-') {
				return false
			}
		}
		if i == len(labels)-1 {
			if len(l) < 2 { // 顶级域至少 2 个字母
				return false
			}
			for _, r := range l {
				if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')) {
					return false
				}
			}
		}
	}
	return true
}
