package aggregator

import (
	"context"
	"testing"

	"dbdlp/internal/domain/port"
)

func TestClassifyRisk(t *testing.T) {
	a := New()

	// 高敏感 + 明文 + 校验通过 + 列名吻合 → high
	f := port.ColumnFinding{SensitiveTypes: []string{"ID_CARD"}, VerifyPassed: true, StorageForm: "plaintext", ColumnName: "id_card_no"}
	r, _ := a.ClassifyRisk(context.Background(), f)
	if r != port.RiskHigh {
		t.Errorf("期望 high，got %s", r)
	}

	// 高敏感 + 校验通过但列名不吻合（如负样本池碰巧过校验的纯数字串）→ medium（EV-03 不得为 high）
	f = port.ColumnFinding{SensitiveTypes: []string{"BANK_CARD"}, VerifyPassed: true, StorageForm: "plaintext", ColumnName: "sample_value"}
	r, _ = a.ClassifyRisk(context.Background(), f)
	if r != port.RiskMedium {
		t.Errorf("期望 medium（校验过但缺上下文吻合），got %s", r)
	}

	// 元数据弱命中（姓名）明文 → medium
	f = port.ColumnFinding{SensitiveTypes: []string{"NAME"}, VerifyPassed: false, StorageForm: "plaintext"}
	r, _ = a.ClassifyRisk(context.Background(), f)
	if r != port.RiskMedium {
		t.Errorf("期望 medium（元数据），got %s", r)
	}

	// 仅格式命中、未校验 → pending_review
	f = port.ColumnFinding{SensitiveTypes: []string{"FIXED_PHONE"}, VerifyPassed: false, StorageForm: "plaintext"}
	r, _ = a.ClassifyRisk(context.Background(), f)
	if r != port.RiskPendingReview {
		t.Errorf("期望 pending_review，got %s", r)
	}
}

func TestComputeConfidence(t *testing.T) {
	a := New()
	// 校验通过 + 列名吻合 + 高命中率 → high
	f := port.ColumnFinding{SensitiveTypes: []string{"MOBILE_PHONE"}, VerifyPassed: true, HitRatio: 0.8, ColumnName: "phone"}
	c, _ := a.ComputeConfidence(context.Background(), f)
	if c != port.ConfHigh {
		t.Errorf("期望 high，got %s", c)
	}
	// 仅正则命中、低命中率 → low
	f = port.ColumnFinding{SensitiveTypes: []string{"FIXED_PHONE"}, VerifyPassed: false, HitRatio: 0.1, ColumnName: "memo"}
	c, _ = a.ComputeConfidence(context.Background(), f)
	if c != port.ConfLow {
		t.Errorf("期望 low，got %s", c)
	}
}

func TestAggregate(t *testing.T) {
	a := New()
	meta := port.ColumnMeta{SchemaName: "SALES", TableName: "CUSTOMER", ColumnName: "mobile", DataType: "VARCHAR2(20)"}
	hits := []port.Hit{{SensitiveType: "MOBILE_PHONE", VerifyPassed: true, MaskedValue: "138****8000", Confidence: "high"}}
	f, err := a.Aggregate(context.Background(), meta, 20, hits)
	if err != nil {
		t.Fatal(err)
	}
	if f.SampleHit != 1 || f.SampleTotal != 20 {
		t.Errorf("采样统计错误：%d/%d", f.SampleHit, f.SampleTotal)
	}
	if f.HitRatio != 0.05 {
		t.Errorf("命中率错误：%v", f.HitRatio)
	}
	if f.RiskLevel == "" || f.Confidence == "" {
		t.Errorf("风险/置信度未填充")
	}
	if f.SampleMasked != "138****8000" {
		t.Errorf("掩码样例错误：%q", f.SampleMasked)
	}
}

func TestAggregatePendingReview(t *testing.T) {
	a := New()
	meta := port.ColumnMeta{SchemaName: "SALES", TableName: "NEG_POOL", ColumnName: "sample_value", DataType: "VARCHAR2(64)"}
	hits := []port.Hit{{SensitiveType: "FIXED_PHONE", VerifyPassed: false, MaskedValue: "010*****6666", Confidence: "low"}}
	f, err := a.Aggregate(context.Background(), meta, 20, hits)
	if err != nil {
		t.Fatal(err)
	}
	if f.RiskLevel != string(port.RiskPendingReview) {
		t.Errorf("期望 pending_review，got %s", f.RiskLevel)
	}
	if f.ReviewStatus != "pending_review" {
		t.Errorf("期望复核状态 pending_review，got %q", f.ReviewStatus)
	}
}
