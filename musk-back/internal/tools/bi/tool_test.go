package bi

import "testing"

func TestValidateCreateRejectsPlaceholderBoard(t *testing.T) {
	err := validateCreate(map[string]any{
		"name":    "广告看板",
		"summary": "广告投放核心指标看板：花费、曝光、点击、转化与 ROAS/CTR/CPC/CPA",
		"metrics": []any{
			map[string]any{"label": "花费", "value": "-"},
			map[string]any{"label": "ROAS", "value": "-"},
			map[string]any{"label": "点击", "value": "-"},
		},
		"rows": []any{},
	})
	if err == nil {
		t.Fatal("placeholder board was accepted")
	}
}

func TestValidateCreateAcceptsObservedBoard(t *testing.T) {
	err := validateCreate(map[string]any{
		"name":    "广告看板",
		"summary": "Meta 品牌-秋季上新花费最高，两日合计花费 274200，整体 ROAS 1.79",
		"metrics": []any{
			map[string]any{"label": "花费", "value": "274200.00"},
			map[string]any{"label": "收入", "value": "491900.00"},
			map[string]any{"label": "ROAS", "value": "1.79"},
		},
		"rows": []any{
			map[string]any{"渠道": "Meta", "花费": "42000.00"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
}
