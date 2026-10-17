package board

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

var englishFields = map[string]string{
	"日期": "date", "渠道": "channel", "计划": "campaign", "花费": "spend", "收入": "revenue",
	"展示": "impressions", "曝光": "impressions", "点击": "clicks", "点击率": "ctr", "转化": "conversions",
	"ROAS": "roas",
}

var englishValues = map[string]string{
	"品牌-秋季上新": "Brand Autumn Launch",
	"拉新-短视频":  "Acquisition Short Video",
	"效果-再营销":  "Performance Remarketing",
	"品牌-搜索":   "Brand Search",
	"效果-购物":   "Performance Shopping",
}

// Normalize turns model-supplied labels into the English table the board renders.
func Normalize(metrics []Metric, columns []string, rows []map[string]any) ([]Metric, []string, []map[string]string) {
	cols := englishColumns(columns)
	engRows := englishRows(rows)
	clean := cleanColumns(cols, engRows)
	return metrics, clean, stringifyRows(engRows, clean)
}

func englishColumns(columns []string) []string {
	out := make([]string, len(columns))
	for i, col := range columns {
		out[i] = englishToken(col)
	}
	return out
}

func englishRows(rows []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		item := make(map[string]any, len(row))
		for key, value := range row {
			item[englishToken(key)] = englishCell(value)
		}
		out = append(out, item)
	}
	return out
}

func englishToken(key string) string {
	key = strings.TrimSpace(key)
	if mapped, ok := englishFields[key]; ok {
		return mapped
	}
	return key
}

func englishCell(value any) any {
	text, ok := value.(string)
	if !ok {
		return value
	}
	text = strings.TrimSpace(text)
	if mapped, ok := englishValues[text]; ok {
		return mapped
	}
	return text
}

func cleanColumns(columns []string, rows []map[string]any) []string {
	out := make([]string, 0, len(columns))
	seen := map[string]struct{}{}
	for _, col := range columns {
		col = strings.TrimSpace(col)
		if col == "" {
			continue
		}
		if _, ok := seen[col]; ok {
			continue
		}
		seen[col] = struct{}{}
		out = append(out, col)
	}
	if len(out) > 0 || len(rows) == 0 {
		return out
	}
	for key := range rows[0] {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		out = append(out, key)
	}
	return out
}

func stringifyRows(rows []map[string]any, columns []string) []map[string]string {
	out := make([]map[string]string, 0, len(rows))
	for _, row := range rows {
		item := map[string]string{}
		if len(columns) == 0 {
			for k, v := range row {
				item[k] = cellText(v)
			}
		} else {
			for _, col := range columns {
				if v, ok := row[col]; ok && v != nil {
					item[col] = cellText(v)
				} else {
					item[col] = ""
				}
			}
		}
		out = append(out, item)
	}
	return out
}

func cellText(v any) string {
	switch n := v.(type) {
	case float64:
		if math.IsNaN(n) || math.IsInf(n, 0) {
			return ""
		}
		if n == math.Trunc(n) && n >= math.MinInt64 && n <= math.MaxInt64 {
			return strconv.FormatInt(int64(n), 10)
		}
		return strconv.FormatFloat(n, 'f', -1, 64)
	case float32:
		return cellText(float64(n))
	case int, int64, int32:
		return fmt.Sprint(n)
	default:
		text := strings.TrimSpace(fmt.Sprint(v))
		if text == "<nil>" {
			return ""
		}
		return text
	}
}

func money(v float64) string { return fmt.Sprintf("%.2f", v) }

func ratio(num, den float64) string {
	if den == 0 {
		return "0.00"
	}
	return fmt.Sprintf("%.2f", num/den)
}

func pct(num, den float64) string {
	if den == 0 {
		return "0.00%"
	}
	return fmt.Sprintf("%.2f%%", num/den*100)
}
