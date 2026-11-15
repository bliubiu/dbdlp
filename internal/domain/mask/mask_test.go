package mask

import (
	"testing"
	"unicode/utf8"
)

func TestMaskIPv4(t *testing.T) {
	cases := map[string]string{
		"192.168.10.3": "*.*.10.3",
		"127.0.0.1":    "127.0.0.1", // 回环保留
		"localhost":    "localhost",
		"10.0.0.255":   "*.*.0.255",
	}
	for in, want := range cases {
		if got := MaskIPv4(in); got != want {
			t.Errorf("MaskIPv4(%q)=%q, want %q", in, got, want)
		}
	}
}

func TestMaskIDCard(t *testing.T) {
	got := MaskIDCard("110101199001011234")
	want := "110101********1234"
	if got != want {
		t.Errorf("MaskIDCard=%q, want %q", got, want)
	}
}

func TestMaskPhone(t *testing.T) {
	if got := MaskPhone("13800138000"); got != "138****8000" {
		t.Errorf("MaskPhone=%q", got)
	}
}

func TestMaskEmail(t *testing.T) {
	if got := MaskEmail("alice@example.com"); got != "a****@example.com" {
		t.Errorf("MaskEmail=%q", got)
	}
}

func TestMaskPassword(t *testing.T) {
	if got := MaskPassword("secret"); got != "******" {
		t.Errorf("MaskPassword=%q", got)
	}
}

func TestByType(t *testing.T) {
	if got := ByType("MOBILE_PHONE", "13800138000"); got != "138****8000" {
		t.Errorf("ByType phone=%q", got)
	}
	if got := ByType("UNKNOWN", "abcdef"); got == "abcdef" {
		t.Errorf("ByType unknown 应被掩码")
	}
}

// TestMaskGenericMultibyte 验证根因修复：多字节字符（中文）按 rune 处理，
// 不得破坏 UTF-8 序列；否则脱敏样例会含孤立首字节（"京"(e4 ba ac) 仅保留 e4），
// 使 HTML 报告成为非法编码文件。
func TestMaskGenericMultibyte(t *testing.T) {
	got := MaskGeneric("京A12345")
	want := "京*****5"
	if got != want {
		t.Errorf("MaskGeneric(中文)=%q, want %q", got, want)
	}
	if !utf8.ValidString(got) {
		t.Errorf("MaskGeneric 输出含非法 UTF-8 字节：%q", got)
	}
}

// TestMaskEmailMultibyteLocal 验证邮箱本地部分含非 ASCII 时仍产生合法 UTF-8。
func TestMaskEmailMultibyteLocal(t *testing.T) {
	got := MaskEmail("用户@example.com")
	if !utf8.ValidString(got) {
		t.Errorf("MaskEmail 多字节本地部分输出非法 UTF-8：%q", got)
	}
}
