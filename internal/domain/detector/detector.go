// Package detector 敏感识别引擎：四层流水线（字节扫描→正则→校验器→聚合）。
package detector

import (
	"context"
	"regexp"
	"strings"

	"dbdlp/internal/domain/mask"
	"dbdlp/internal/domain/port"
	"dbdlp/internal/domain/rule"
)

type detector struct {
	rules    []port.Rule
	compiled map[string]*regexp.Regexp
	version  string
}

// New 构造检测器。rules 须已通过 rule.BuildRules 校验。
func New(rules []port.Rule, version string) (port.Detector, error) {
	d := &detector{rules: rules, compiled: make(map[string]*regexp.Regexp), version: version}
	for _, r := range rules {
		if r.Regex == "" {
			continue
		}
		re, err := regexp.Compile(r.Regex)
		if err != nil {
			return nil, err
		}
		d.compiled[r.RuleID] = re
	}
	return d, nil
}

func (d *detector) RuleVersion() string { return d.version }

func (d *detector) Close() error { return nil }

// DetectValue 对单值执行四层识别，返回命中的规则（可能多条）。
func (d *detector) DetectValue(_ context.Context, value string) ([]port.Hit, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	var hits []port.Hit
	for _, r := range d.rules {
		if !r.Enabled || r.Regex == "" {
			continue
		}
		if !passByteScan(r.Bytescan, value) {
			continue
		}
		re := d.compiled[r.RuleID]
		if re == nil || !re.MatchString(value) {
			continue
		}
		verify := true
		if r.Verifier != "" {
			fn := rule.GetVerifier(r.Verifier)
			if fn == nil {
				verify = false
			} else {
				verify = fn(value)
			}
		}
		conf := "low"
		if verify {
			conf = "high"
		}
		hits = append(hits, port.Hit{
			RuleID:        r.RuleID,
			SensitiveType: r.SensitiveType,
			Confidence:    conf,
			MaskedValue:   mask.ByType(r.SensitiveType, value),
			SourceForm:    "plaintext",
			VerifyPassed:  verify,
		})
	}
	return hits, nil
}

// DetectRow 对一行多列取值识别，扁平返回全部命中。
func (d *detector) DetectRow(_ context.Context, kv map[string]string) ([]port.Hit, error) {
	var all []port.Hit
	for _, v := range kv {
		hits, err := d.DetectValue(context.Background(), v)
		if err != nil {
			return nil, err
		}
		all = append(all, hits...)
	}
	return all, nil
}

// passByteScan 第 1 层：廉价字符特征筛选。
func passByteScan(spec port.BytescanSpec, v string) bool {
	if spec.MinLen > 0 && len(v) < spec.MinLen {
		return false
	}
	if spec.MaxLen > 0 && len(v) > spec.MaxLen {
		return false
	}
	if spec.DigitsOnly {
		for i := 0; i < len(v); i++ {
			if v[i] < '0' || v[i] > '9' {
				return false
			}
		}
	}
	if len(spec.PrefixBytes) > 0 {
		ok := false
		for _, p := range spec.PrefixBytes {
			if strings.HasPrefix(v, p) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	return true
}
