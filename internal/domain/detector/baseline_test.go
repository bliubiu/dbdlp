// baseline_test.go 规则基线评测（docs/08 M3 判据：基线评测有记录，进版本库可复跑）。
//
// 样本来源：hitsz-ids/openDLP（Apache-2.0）tests/data，每类截取 200 对正负样本
// （testdata/baseline/openDLP_*.csv）。口径对齐 docs/07 EV-02/EV-03：
//   - 严格召回 = 正样本命中目标规则且校验通过；
//   - 高置信误报 = 负样本命中目标规则且校验通过（判高风险的来源）；
//   - openDLP 正样本含 +86/分隔符等变形，与整串正则的差距在报告中单列。
package detector_test

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dbdlp/internal/domain/detector"
	"dbdlp/internal/domain/rule"
)

// baselineTypes 基线类型与引擎规则 ID 映射（DATE 无对应规则，跳过并标注预留）。
var baselineTypes = []struct {
	CSV    string
	RuleID string
	Class  string // high：高置信校验类；mid：中置信格式类
}{
	{"openDLP_ID_CARD.csv", "ID_CARD", "high"},
	{"openDLP_BANK_CARD.csv", "BANK_CARD", "high"},
	{"openDLP_MOBILE_PHONE.csv", "MOBILE_PHONE", "high"},
	{"openDLP_EMAIL.csv", "EMAIL", "high"},
	{"openDLP_SOCIAL_CREDIT_CODE.csv", "SOCIAL_CREDIT_CODE", "high"},
	{"openDLP_IPV4.csv", "IPV4", "high"},
	{"openDLP_IPV6.csv", "IPV6", "high"},
	{"openDLP_PASSPORT.csv", "PASSPORT", "mid"},
	{"openDLP_LICENSE_PLATE.csv", "PLATE", "mid"},
	{"openDLP_POSTCODE.csv", "POSTCODE", "mid"},
	{"openDLP_TELEPHONE.csv", "FIXED_PHONE", "mid"},
	{"openDLP_DOMAIN_NAME.csv", "DOMAIN", "mid"},
	{"openDLP_MAC.csv", "MAC", "mid"},
}

func loadBaseline(t *testing.T, name string) (pos, neg []string) {
	t.Helper()
	f, err := os.Open(filepath.Join("..", "..", "..", "testdata", "baseline", name))
	if err != nil {
		t.Fatalf("打开基线样本：%v", err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.LazyQuotes = true
	first := true
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("解析 %s：%v", name, err)
		}
		if first {
			first = false
			continue
		}
		if len(row) >= 1 {
			pos = append(pos, strings.TrimSpace(row[0]))
		}
		if len(row) >= 2 {
			neg = append(neg, strings.TrimSpace(row[1]))
		}
	}
	return pos, neg
}

func TestRuleBaseline(t *testing.T) {
	rules, version, err := rule.BuildRules("")
	if err != nil {
		t.Fatalf("加载规则：%v", err)
	}
	det, err := detector.New(rules, version)
	if err != nil {
		t.Fatalf("构建引擎：%v", err)
	}
	defer det.Close()
	ctx := context.Background()
	var rows []string
	for _, bt := range baselineTypes {
		pos, neg := loadBaseline(t, bt.CSV)
		strict, ruleOnly, posTotal := 0, 0, len(pos)
		negFP, negFmt, negTotal := 0, 0, len(neg)
		for _, v := range pos {
			hits, err := det.DetectValue(ctx, v)
			if err != nil {
				t.Fatalf("识别 %q：%v", v, err)
			}
			for _, h := range hits {
				if h.SensitiveType != bt.RuleID {
					continue
				}
				ruleOnly++
				if h.VerifyPassed {
					strict++
				}
			}
		}
		for _, v := range neg {
			hits, _ := det.DetectValue(ctx, v)
			for _, h := range hits {
				if h.SensitiveType != bt.RuleID {
					continue
				}
				if h.VerifyPassed {
					negFP++
				} else {
					negFmt++
				}
			}
		}
		recall := 0.0
		if posTotal > 0 {
			recall = float64(strict) / float64(posTotal) * 100
		}
		fpRate := 0.0
		if negTotal > 0 {
			fpRate = float64(negFP) / float64(negTotal) * 100
		}
		rows = append(rows, fmt.Sprintf("| %s | %s | %d/%d | %.1f%% | %d/%d | %.1f%% | %d |",
			bt.RuleID, bt.Class, strict, posTotal, recall, negFP, negTotal, fpRate, negFmt))
		if strict == 0 {
			t.Errorf("基线健康检查失败：%s 严格召回为 0（引擎异常）", bt.RuleID)
		}
	}
	out := strings.Join(rows, "\n")
	t.Logf("规则基线（openDLP 样本每类 200 对，规则版本 %s）：\n| 规则 | 分层 | 严格召回 | 召回率 | 高置信误报 | 误报率 | 仅格式命中 |\n|---|---|---|---|---|---|---|\n%s", version, out)
}
