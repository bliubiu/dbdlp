package mask

import "testing"

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
