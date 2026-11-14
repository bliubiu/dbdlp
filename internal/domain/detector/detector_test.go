package detector

import (
	"context"
	"testing"

	"dbdlp/internal/domain/port"
	"dbdlp/internal/domain/rule"
)

func makeID(p17 string) string {
	weights := []int{7, 9, 10, 5, 8, 4, 2, 1, 6, 3, 7, 9, 10, 5, 8, 4, 2}
	checkCodes := "10X98765432"
	sum := 0
	for i := 0; i < 17; i++ {
		sum += int(p17[i]-'0') * weights[i]
	}
	return p17 + string(checkCodes[sum%11])
}

func newTestDetector(t *testing.T) port.Detector {
	rules, version, err := rule.BuildRules("")
	if err != nil {
		t.Fatalf("BuildRules: %v", err)
	}
	d, err := New(rules, version)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return d
}

func TestDetectMobile(t *testing.T) {
	d := newTestDetector(t)
	hits, err := d.DetectValue(context.Background(), "13800138000")
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 {
		t.Fatal("手机号应命中")
	}
	h := hits[0]
	if h.SensitiveType != "MOBILE_PHONE" || !h.VerifyPassed {
		t.Errorf("期望 MOBILE_PHONE 校验通过，got %+v", h)
	}
	if h.MaskedValue != "138****8000" {
		t.Errorf("期望掩码 138****8000，got %q", h.MaskedValue)
	}
}

func TestDetectIDCardValidAndInvalid(t *testing.T) {
	d := newTestDetector(t)
	valid := makeID("11010119900101123")
	hits, _ := d.DetectValue(context.Background(), valid)
	if len(hits) == 0 || !hits[0].VerifyPassed {
		t.Fatalf("合法身份证应校验通过，got %+v", hits)
	}
	bad := valid[:17] + "0"
	hits, _ = d.DetectValue(context.Background(), bad)
	if len(hits) == 0 {
		t.Fatal("格式命中仍应返回命中")
	}
	if hits[0].VerifyPassed {
		t.Errorf("篡改校验位应为未通过校验")
	}
	if hits[0].Confidence != "low" {
		t.Errorf("未通过校验置信度应为 low，got %q", hits[0].Confidence)
	}
}

func TestDetectBankCardAndEmail(t *testing.T) {
	d := newTestDetector(t)
	if hits, _ := d.DetectValue(context.Background(), "4111111111111111"); len(hits) == 0 || !hits[0].VerifyPassed {
		t.Errorf("合法银行卡应命中且校验通过")
	}
	if hits, _ := d.DetectValue(context.Background(), "alice@example.com"); len(hits) == 0 {
		t.Errorf("邮箱应命中")
	}
}

func TestDetectNonSensitive(t *testing.T) {
	d := newTestDetector(t)
	if hits, _ := d.DetectValue(context.Background(), "普通商品名称"); len(hits) != 0 {
		t.Errorf("非敏感值不应命中，got %+v", hits)
	}
}

func TestDetectRow(t *testing.T) {
	d := newTestDetector(t)
	kv := map[string]string{"phone": "13800138000", "memo": "备注"}
	hits, err := d.DetectRow(context.Background(), kv)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 {
		t.Errorf("一行应仅命中 phone，got %d", len(hits))
	}
}

func TestRuleVersion(t *testing.T) {
	d := newTestDetector(t)
	if d.RuleVersion() == "" {
		t.Errorf("规则版本不应为空")
	}
}
