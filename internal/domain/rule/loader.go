package rule

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"dbdlp/internal/domain/port"
	"gopkg.in/yaml.v3"
)

// yamlGlobals 对应 rules.yaml 的 globals 段（docs/02 §6.3）。
type yamlGlobals struct {
	DefaultThreshold float64  `yaml:"default_threshold"`
	DisabledRules    []string `yaml:"disabled_rules"`
}

// yamlRule 对应 rules.yaml 中单条规则。
type yamlRule struct {
	RuleID        string  `yaml:"rule_id"`
	SensitiveType string  `yaml:"sensitive_type"`
	Enabled       bool    `yaml:"enabled"`
	Custom        bool    `yaml:"custom"`
	Regex         string  `yaml:"regex"`
	Verifier      string  `yaml:"verifier"`
	Threshold     float64 `yaml:"threshold"`
	Bytescan      struct {
		MinLen      int      `yaml:"min_len"`
		MaxLen      int      `yaml:"max_len"`
		DigitsOnly  bool     `yaml:"digits_only"`
		PrefixBytes []string `yaml:"prefix_bytes"`
	} `yaml:"bytescan"`
}

type yamlFile struct {
	Version string      `yaml:"version"`
	Globals yamlGlobals `yaml:"globals"`
	Rules   []yamlRule  `yaml:"rules"`
}

// defaultThreshold 规则未声明 threshold 时的兜底置信阈值。
const defaultThreshold = 0.5

// BuildRules 合并内置规则与（可选的）自定义 YAML 规则，校验后计算版本摘要。
// path 为空时仅用内置规则；非空且文件不存在返回中文错误。
//
// 合并语义（docs/02 §6.1 第4层「与内置同构按 rule_id 去重合并」）：
//   - YAML 内 rule_id 互重复 → 报错；
//   - YAML 规则与内置规则同 rule_id → 视为「覆盖内置定义」，不报错；
//   - globals.disabled_rules 列出的 rule_id → 置为未启用（禁用≠删除，仍进快照）。
func BuildRules(path string) ([]port.Rule, string, error) {
	rules := BuiltinRules()

	fileVersion := ""
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, "", fmt.Errorf("规则文件读取失败：%w", err)
		}
		var f yamlFile
		if err := yaml.Unmarshal(data, &f); err != nil {
			return nil, "", fmt.Errorf("规则文件解析失败：%w", err)
		}
		if strings.TrimSpace(f.Version) == "" {
			return nil, "", fmt.Errorf("规则文件 version 不能为空")
		}
		fileVersion = strings.TrimSpace(f.Version)

		merged, err := mergeYAML(rules, f)
		if err != nil {
			return nil, "", err
		}
		rules = merged
	}

	version := computeVersion(rules, fileVersion)
	for i := range rules {
		rules[i].Version = version
	}
	return rules, version, nil
}

// mergeYAML 校验并合并 YAML 规则到内置规则集。
func mergeYAML(base []port.Rule, f yamlFile) ([]port.Rule, error) {
	idx := make(map[string]int, len(base))
	for i, r := range base {
		idx[r.RuleID] = i
	}

	seenInFile := map[string]bool{}
	globalThreshold := f.Globals.DefaultThreshold
	if globalThreshold <= 0 {
		globalThreshold = defaultThreshold
	}

	for _, yr := range f.Rules {
		ruleID := strings.TrimSpace(yr.RuleID)
		if ruleID == "" {
			return nil, fmt.Errorf("存在 rule_id 为空的规则")
		}
		if seenInFile[ruleID] {
			return nil, fmt.Errorf("规则 rule_id 重复：%s", ruleID)
		}
		seenInFile[ruleID] = true

		if _, err := regexp.Compile(yr.Regex); err != nil {
			return nil, fmt.Errorf("规则 %s 正则编译失败：%w", ruleID, err)
		}
		verifier := strings.TrimSpace(yr.Verifier)
		if verifier != "" && GetVerifier(verifier) == nil {
			return nil, fmt.Errorf("规则 %s 引用了未注册的校验器 %s", ruleID, verifier)
		}

		threshold := yr.Threshold
		if threshold <= 0 {
			threshold = globalThreshold
		}

		sensitiveType := strings.TrimSpace(yr.SensitiveType)
		if sensitiveType == "" {
			sensitiveType = ruleID
		}

		merged := port.Rule{
			RuleID:        ruleID,
			SensitiveType: sensitiveType,
			Enabled:       yr.Enabled,
			Custom:        yr.Custom,
			Regex:         yr.Regex,
			Verifier:      verifier,
			Threshold:     threshold,
			Bytescan: port.BytescanSpec{
				MinLen:      yr.Bytescan.MinLen,
				MaxLen:      yr.Bytescan.MaxLen,
				DigitsOnly:  yr.Bytescan.DigitsOnly,
				PrefixBytes: yr.Bytescan.PrefixBytes,
			},
		}

		if i, ok := idx[ruleID]; ok {
			// 覆盖内置规则：保留内置位置，仅替换定义。
			base[i] = merged
			continue
		}
		idx[ruleID] = len(base)
		base = append(base, merged)
	}

	// globals.disabled_rules → 置未启用（不删除，仍进规则快照）。
	for _, disabled := range f.Globals.DisabledRules {
		id := strings.TrimSpace(disabled)
		if id == "" {
			continue
		}
		if i, ok := idx[id]; ok {
			base[i].Enabled = false
		}
	}

	return base, nil
}

// computeVersion 对规则内容做稳定摘要；同内容恒得同版本，支持历史报告复现。
// fileVersion 为 rules.yaml 声明的 version（无文件时为空串）。
func computeVersion(rules []port.Rule, fileVersion string) string {
	type line struct {
		id, st, rx, vf string
		en             bool
		cu             bool
		th             float64
		bs             string
	}
	lines := make([]line, 0, len(rules))
	for _, r := range rules {
		lines = append(lines, line{
			r.RuleID, r.SensitiveType, r.Regex, r.Verifier,
			r.Enabled, r.Custom, r.Threshold, bytescanDigest(r.Bytescan),
		})
	}
	sort.Slice(lines, func(i, j int) bool { return lines[i].id < lines[j].id })
	h := sha256.New()
	fmt.Fprintf(h, "file_version=%s\n", fileVersion)
	for _, l := range lines {
		fmt.Fprintf(h, "%s|%s|%s|%s|%v|%v|%v|%s\n",
			l.id, l.st, l.rx, l.vf, l.en, l.cu, l.th, l.bs)
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// bytescanDigest 字节扫描参数的稳定摘要（顺序无关）。
func bytescanDigest(b port.BytescanSpec) string {
	prefixes := append([]string(nil), b.PrefixBytes...)
	sort.Strings(prefixes)
	return fmt.Sprintf("%d-%d-%t-%s", b.MinLen, b.MaxLen, b.DigitsOnly, strings.Join(prefixes, ","))
}
