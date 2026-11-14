package rule

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeRules 写临时规则文件并返回路径。
func writeRules(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "rules.yaml")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatalf("写规则文件失败：%v", err)
	}
	return p
}

// Test内置规则版本非空且稳定：同内容恒得同版本。
func Test内置规则版本非空且稳定(t *testing.T) {
	_, v1, err := BuildRules("")
	if err != nil {
		t.Fatalf("BuildRules 内置规则失败：%v", err)
	}
	if len(v1) != 16 {
		t.Fatalf("规则版本摘要长度应为 16，实际 %d（%q）", len(v1), v1)
	}
	_, v2, _ := BuildRules("")
	if v1 != v2 {
		t.Fatalf("内置规则版本不稳定：%s != %s", v1, v2)
	}
}

// Test自定义规则合并：YAML 规则按 rule_id 去重合并进内置规则集。
func Test自定义规则合并(t *testing.T) {
	p := writeRules(t, `
version: "1.0.0"
rules:
  - rule_id: "EMPLOYEE_NO"
    sensitive_type: "EMPLOYEE_NO"
    enabled: true
    custom: true
    regex: "^EMP[0-9]{6}$"
    threshold: 0.6
    bytescan:
      min_len: 9
      max_len: 9
      prefix_bytes: ["EMP"]
`)
	rules, version, err := BuildRules(p)
	if err != nil {
		t.Fatalf("BuildRules 失败：%v", err)
	}
	if version == "" {
		t.Fatal("规则版本不能为空")
	}
	idx := -1
	for i, r := range rules {
		if r.RuleID == "EMPLOYEE_NO" {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatal("自定义规则 EMPLOYEE_NO 未合并进规则集")
	}
	r := rules[idx]
	if r.SensitiveType != "EMPLOYEE_NO" {
		t.Fatalf("敏感类型错误：%s", r.SensitiveType)
	}
	if !r.Enabled || !r.Custom {
		t.Fatalf("自定义规则标记错误：enabled=%v custom=%v", r.Enabled, r.Custom)
	}
	if r.Threshold != 0.6 {
		t.Fatalf("阈值错误：%v", r.Threshold)
	}
	if r.Bytescan.MinLen != 9 || r.Bytescan.MaxLen != 9 || len(r.Bytescan.PrefixBytes) != 1 {
		t.Fatalf("字节扫描参数解析错误：%+v", r.Bytescan)
	}
	if r.Version != version {
		t.Fatalf("规则版本未回填：rule.Version=%s version=%s", r.Version, version)
	}
	// 版本随文件内容变化
	_, v2, err := BuildRules(writeRules(t, `
version: "1.0.0"
rules:
  - rule_id: "EMPLOYEE_NO"
    sensitive_type: "EMPLOYEE_NO"
    enabled: true
    custom: true
    regex: "^EMP[0-9]{7}$"
    threshold: 0.6
`))
	if err != nil {
		t.Fatalf("BuildRules 变更后失败：%v", err)
	}
	if v2 == version {
		t.Fatal("规则内容变更后 rule_version 应变化")
	}
}

// Test覆盖内置规则：YAML 与内置同 rule_id 时覆盖而非报重复。
func Test覆盖内置规则(t *testing.T) {
	p := writeRules(t, `
version: "1.0.0"
rules:
  - rule_id: "MOBILE_PHONE"
    sensitive_type: "MOBILE_PHONE"
    enabled: true
    regex: "^1[3-9][0-9]{9}$"
    verifier: "mobile_segment"
    threshold: 0.95
`)
	rules, _, err := BuildRules(p)
	if err != nil {
		t.Fatalf("覆盖内置规则应成功，实际报错：%v", err)
	}
	count := 0
	for _, r := range rules {
		if r.RuleID == "MOBILE_PHONE" {
			count++
			if r.Threshold != 0.95 {
				t.Fatalf("内置规则未被覆盖，threshold=%v", r.Threshold)
			}
		}
	}
	if count != 1 {
		t.Fatalf("MOBILE_PHONE 规则应只有一条，实际 %d 条", count)
	}
}

// Test全局禁用规则：globals.disabled_rules 置未启用但仍保留在快照中。
func Test全局禁用规则(t *testing.T) {
	p := writeRules(t, `
version: "1.0.0"
globals:
  default_threshold: 0.55
  disabled_rules: ["PASSPORT"]
rules:
  - rule_id: "CONTRACT_NO"
    sensitive_type: "CONTRACT_NO"
    enabled: true
    regex: "^HT-20[0-9]{2}-[0-9]{10}$"
`)
	rules, _, err := BuildRules(p)
	if err != nil {
		t.Fatalf("BuildRules 失败：%v", err)
	}
	var passportFound, contractFound bool
	for _, r := range rules {
		switch r.RuleID {
		case "PASSPORT":
			passportFound = true
			if r.Enabled {
				t.Fatal("PASSPORT 被 globals.disabled_rules 禁用后应为未启用")
			}
		case "CONTRACT_NO":
			contractFound = true
			// 未声明 threshold → 取 globals.default_threshold
			if r.Threshold != 0.55 {
				t.Fatalf("缺省阈值应取 globals.default_threshold，实际 %v", r.Threshold)
			}
		}
	}
	if !passportFound {
		t.Fatal("被禁用的规则不得从规则集删除")
	}
	if !contractFound {
		t.Fatal("自定义规则 CONTRACT_NO 未合并")
	}
}

// Test非法规则文件一律拒绝启动：version 空 / rule_id 重复 / 正则不可编译 / 校验器未注册。
func Test非法规则文件一律拒绝启动(t *testing.T) {
	cases := []struct {
		name    string
		content string
		wantErr string
	}{
		{
			name:    "version为空",
			content: "rules:\n  - rule_id: \"X\"\n    regex: \"^a$\"\n",
			wantErr: "version",
		},
		{
			name: "rule_id重复",
			content: `version: "1.0.0"
rules:
  - rule_id: "DUP"
    regex: "^a$"
  - rule_id: "DUP"
    regex: "^b$"
`,
			wantErr: "重复",
		},
		{
			name:    "正则不可编译",
			content: "version: \"1.0.0\"\nrules:\n  - rule_id: \"X\"\n    regex: \"^([a-z]$\"\n",
			wantErr: "正则编译失败",
		},
		{
			name:    "校验器未注册",
			content: "version: \"1.0.0\"\nrules:\n  - rule_id: \"X\"\n    regex: \"^a$\"\n    verifier: \"not_registered\"\n",
			wantErr: "未注册的校验器",
		},
		{
			name:    "rule_id为空",
			content: "version: \"1.0.0\"\nrules:\n  - rule_id: \"  \"\n    regex: \"^a$\"\n",
			wantErr: "rule_id 为空",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := BuildRules(writeRules(t, tc.content))
			if err == nil {
				t.Fatalf("应拒绝启动，实际无错误")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("错误信息应包含 %q，实际：%v", tc.wantErr, err)
			}
		})
	}
}

// Test规则文件不存在时返回中文错误。
func Test规则文件不存在时返回中文错误(t *testing.T) {
	_, _, err := BuildRules(filepath.Join(t.TempDir(), "not-exist.yaml"))
	if err == nil {
		t.Fatal("规则文件不存在应返回错误")
	}
	if !strings.Contains(err.Error(), "规则文件读取失败") {
		t.Fatalf("错误信息不正确：%v", err)
	}
}
