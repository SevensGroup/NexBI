package board

import "sort"

type adRow struct {
	date        string
	channel     string
	campaign    string
	spend       float64
	revenue     float64
	impressions int64
	clicks      int64
	conversions int64
}

func seedAds() []adRow {
	rows := []adRow{
		{"2026-10-13", "Google", "Brand Search", 17600, 49800, 390000, 11800, 350},
		{"2026-10-13", "Google", "Performance Shopping", 8900, 18600, 280000, 4900, 140},
		{"2026-10-13", "Meta", "Brand Autumn Launch", 39800, 81200, 1720000, 19800, 640},
		{"2026-10-13", "Meta", "Performance Remarketing", 24100, 38600, 880000, 9100, 260},
		{"2026-10-13", "TikTok", "Acquisition Short Video", 28700, 43100, 2210000, 30900, 470},
		{"2026-10-13", "TikTok", "Performance Remarketing", 14200, 12100, 690000, 5700, 80},
		{"2026-10-14", "Google", "Brand Search", 18400, 55200, 420000, 12600, 380},
		{"2026-10-14", "Google", "Performance Shopping", 9600, 21400, 310000, 5400, 160},
		{"2026-10-14", "Meta", "Brand Autumn Launch", 42000, 79800, 1860000, 21400, 620},
		{"2026-10-14", "Meta", "Performance Remarketing", 26800, 35100, 940000, 9800, 240},
		{"2026-10-14", "TikTok", "Acquisition Short Video", 31500, 47200, 2400000, 33600, 510},
		{"2026-10-14", "TikTok", "Performance Remarketing", 12600, 9800, 760000, 6100, 90},
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].date != rows[j].date {
			return rows[i].date > rows[j].date
		}
		return rows[i].spend > rows[j].spend
	})
	return rows
}
