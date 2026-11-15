// Package aggregator 字段级聚合：风险分级与置信度计算（唯一入口）。
package aggregator

import (
	"context"
	"strings"

	"dbdlp/internal/domain/port"
	"dbdlp/internal/domain/rule"
)

type aggregator struct{}

// New 构造聚合器。
func New() port.Aggregator { return &aggregator{} }

// Aggregate 汇总单字段命中，计算比值/类型/验证/风险/置信度/样例，返回完整台账项。
func (a *aggregator) Aggregate(_ context.Context, meta port.ColumnMeta, sampleTotal int, hits []port.Hit) (port.ColumnFinding, error) {
	f := port.ColumnFinding{
		SchemaName:    meta.SchemaName,
		TableName:     meta.TableName,
		ColumnName:    meta.ColumnName,
		DataType:      meta.DataType,
		ColumnComment: meta.Comment,
		SampleTotal:   sampleTotal,
		SampleHit:     len(hits),
		Hits:          hits,
		StorageForm:   "plaintext",
	}
	if sampleTotal > 0 {
		// 同一采样值命中多条规则时 hits 会多于采样行数（如 18 位数字串同命中
		// 身份证/银行卡/信用代码），命中率按采样行数封顶 100%（docs/04 §4.3 口径）。
		hitted := len(hits)
		if hitted > sampleTotal {
			hitted = sampleTotal
		}
		f.HitRatio = float64(hitted) / float64(sampleTotal)
	}
	types := map[string]bool{}
	verify := false
	var sampleMasked []string
	for _, h := range hits {
		types[h.SensitiveType] = true
		if h.VerifyPassed {
			verify = true
		}
		if h.MaskedValue != "" && len(sampleMasked) < 5 {
			sampleMasked = append(sampleMasked, h.MaskedValue)
		}
	}
	for t := range types {
		f.SensitiveTypes = append(f.SensitiveTypes, t)
	}
	f.VerifyPassed = verify
	f.SampleMasked = strings.Join(sampleMasked, " | ")
	if risk, err := a.ClassifyRisk(context.Background(), f); err == nil {
		f.RiskLevel = string(risk)
		if risk == port.RiskPendingReview {
			// 仅格式命中/校验未过 → 进人工复核（docs/07 EV-04）
			f.ReviewStatus = "pending_review"
		}
	}
	if conf, err := a.ComputeConfidence(context.Background(), f); err == nil {
		f.Confidence = string(conf)
	}
	return f, nil
}

// columnMatches 列名/注释与敏感类型是否吻合（docs/02 §14.4 置信度维度）。
func columnMatches(f port.ColumnFinding) bool {
	matched := rule.MatchColumnMeta(f.ColumnName, f.ColumnComment)
	for _, t := range f.SensitiveTypes {
		for _, m := range matched {
			if m == t {
				return true
			}
		}
	}
	return false
}

// ClassifyRisk 风险分级（docs/02 §14.3）。
// high 须同时满足列名/注释吻合：负样本池中碰巧通过校验的纯数字串
// （如订单号过 Luhn）因缺乏上下文吻合降为 medium，交人工确认（docs/07 EV-03/EV-04）。
func (a *aggregator) ClassifyRisk(_ context.Context, f port.ColumnFinding) (port.RiskLevel, error) {
	hasHigh := false
	hasMeta := false
	for _, t := range f.SensitiveTypes {
		if rule.HighSensitivity[t] {
			hasHigh = true
		}
		if t == "NAME" || t == "ADDRESS" || t == "COMPANY" {
			hasMeta = true
		}
	}
	plaintext := f.StorageForm == "plaintext" || f.StorageForm == ""
	colMatch := columnMatches(f)
	switch {
	case hasHigh && f.VerifyPassed && colMatch && plaintext:
		return port.RiskHigh, nil
	case hasHigh && f.VerifyPassed && plaintext:
		return port.RiskMedium, nil
	case hasMeta && plaintext && f.VerifyPassed:
		return port.RiskMedium, nil
	case f.StorageForm == "hash" || f.StorageForm == "encrypted":
		return port.RiskMediumLow, nil
	case f.VerifyPassed && plaintext:
		return port.RiskMediumLow, nil
	case len(f.SensitiveTypes) > 0 && !f.VerifyPassed:
		return port.RiskPendingReview, nil
	default:
		return port.RiskLow, nil
	}
}

// ComputeConfidence 置信度（docs/02 §14.4）。
func (a *aggregator) ComputeConfidence(_ context.Context, f port.ColumnFinding) (port.Confidence, error) {
	colMatch := columnMatches(f)
	switch {
	case f.VerifyPassed && colMatch && f.HitRatio >= 0.5:
		return port.ConfHigh, nil
	case f.VerifyPassed || (f.HitRatio >= 0.5):
		return port.ConfMedium, nil
	default:
		return port.ConfLow, nil
	}
}
