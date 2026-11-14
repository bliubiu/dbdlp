package rule

import "testing"

// makeID 按加权模 11 计算身份证校验位，生成合法测试样本。
func makeID(p17 string) string {
	weights := []int{7, 9, 10, 5, 8, 4, 2, 1, 6, 3, 7, 9, 10, 5, 8, 4, 2}
	checkCodes := "10X98765432"
	sum := 0
	for i := 0; i < 17; i++ {
		sum += int(p17[i]-'0') * weights[i]
	}
	return p17 + string(checkCodes[sum%11])
}

// makeUSC 按 GB32100 计算统一社会信用代码校验位，生成合法测试样本。
func makeUSC(p17 string) string {
	const body = "0123456789ABCDEFGHJKLMNPQRTUWXY"
	weights := []int{1, 3, 9, 27, 19, 26, 16, 17, 20, 29, 25, 13, 8, 24, 10, 30, 28}
	sum := 0
	for i := 0; i < 17; i++ {
		idx := -1
		for j := 0; j < len(body); j++ {
			if body[j] == p17[i] {
				idx = j
				break
			}
		}
		sum += idx * weights[i]
	}
	return p17 + string(body[sum%31])
}

func TestIDCardChecksum(t *testing.T) {
	valid := makeID("11010119900101123")
	if !idCardChecksum(valid) {
		t.Errorf("合法身份证应过校验：%s", valid)
	}
	// 篡改校验位
	bad := valid[:17] + "0"
	if idCardChecksum(bad) {
		t.Errorf("篡改校验位应不通过：%s", bad)
	}
	// 非法出生日期
	if idCardChecksum("110101199013011230") {
		t.Errorf("非法日期应不通过")
	}
}

func TestLuhn(t *testing.T) {
	if !luhn("4111111111111111") {
		t.Errorf("合法银行卡号应过 Luhn")
	}
	if luhn("4111111111111112") {
		t.Errorf("错误校验位应不通过 Luhn")
	}
	if luhn("12345") {
		t.Errorf("过短应不通过")
	}
}

func TestMobileSegment(t *testing.T) {
	if !mobileSegment("13800138000") {
		t.Errorf("合法手机号应过")
	}
	if mobileSegment("12345678901") {
		t.Errorf("非法号段应不通过")
	}
	if mobileSegment("11111111111") {
		t.Errorf("全同号应不通过")
	}
}

func TestEmailFormat(t *testing.T) {
	if !emailFormat("alice@example.com") {
		t.Errorf("合法邮箱应过")
	}
	if emailFormat("abc") || emailFormat("a@b") {
		t.Errorf("缺 @ 或缺顶级域应不通过")
	}
}

func TestSocialCreditCode(t *testing.T) {
	valid := makeUSC("91110108MA01ABCDE")
	if !socialCreditCode(valid) {
		t.Errorf("合法统一社会信用代码应过：%s", valid)
	}
	if socialCreditCode(valid[:17] + "0") {
		t.Errorf("篡改校验位应不通过")
	}
}

func TestPassportPlatePostcodeIP(t *testing.T) {
	if !passport("E12345678") {
		t.Errorf("合法护照应过")
	}
	if passport("123456789") {
		t.Errorf("数字串非护照")
	}
	if !plate("京A12345") {
		t.Errorf("合法车牌应过")
	}
	if plate("A12345") {
		t.Errorf("缺省份车牌应不通过")
	}
	if !postcode("100000") {
		t.Errorf("合法邮编应过")
	}
	if postcode("1234") {
		t.Errorf("短邮编应不通过")
	}
	if !ipv4("192.168.1.1") {
		t.Errorf("合法 IPv4 应过")
	}
	if ipv4("999.1.1.1") {
		t.Errorf("超段 IPv4 应不通过")
	}
}

func TestGetVerifierUnknown(t *testing.T) {
	if GetVerifier("no_such") != nil {
		t.Errorf("未注册校验器应返回 nil")
	}
}
