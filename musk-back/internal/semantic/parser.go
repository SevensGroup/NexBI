package semantic

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
)

var (
	reLastNDays = regexp.MustCompile(`(?i)(?:last|past)\s*(\d+)\s*days?`)
	reISODate   = regexp.MustCompile(`\b(20\d{2}-\d{2}-\d{2})\b`)
)

type datePhrase struct {
	match string
	label string
	start time.Time
	end   time.Time
}

// Parse maps a natural-language advertising question onto a SemanticQuery.
func Parse(input string, catalog *Registry, now time.Time) (*Query, error) {
	if catalog == nil {
		return nil, fmt.Errorf("%w: catalog is required", ErrParse)
	}
	text := strings.TrimSpace(input)
	if text == "" {
		return nil, fmt.Errorf("%w: input is required", ErrParse)
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}

	q := &Query{}
	remaining := text

	if phrase, ok := matchDatePhrase(remaining, now); ok {
		q.TimeRange = TimeRange{Start: phrase.start, End: phrase.end, Label: phrase.label}
		remaining = strings.Replace(remaining, phrase.match, " ", 1)
	}

	type hit struct {
		start, end int
		name       string
	}
	var metricHits []hit
	for _, m := range catalog.ListMetrics() {
		for _, alias := range append([]string{m.Name, m.Key}, m.Aliases...) {
			if loc := findAlias(remaining, alias); loc >= 0 {
				metricHits = append(metricHits, hit{start: loc, end: loc + len(alias), name: m.Name})
			}
		}
	}
	sort.Slice(metricHits, func(i, j int) bool {
		if metricHits[i].start != metricHits[j].start {
			return metricHits[i].start < metricHits[j].start
		}
		return (metricHits[i].end - metricHits[i].start) > (metricHits[j].end - metricHits[j].start)
	})
	seenMetric := map[string]struct{}{}
	used := make([]bool, len(remaining))
	for _, h := range metricHits {
		if overlaps(used, h.start, h.end) {
			continue
		}
		if _, ok := seenMetric[h.name]; ok {
			continue
		}
		seenMetric[h.name] = struct{}{}
		q.Metrics = append(q.Metrics, h.name)
		mark(used, h.start, h.end)
	}

	type dimHit struct {
		start, end int
		name       string
		asFilter   bool
		value      string
	}
	var dimHits []dimHit
	for _, d := range catalog.ListDimensions() {
		for _, alias := range append([]string{d.Name, d.Key}, d.Aliases...) {
			if loc := findAlias(remaining, alias); loc >= 0 {
				dimHits = append(dimHits, dimHit{start: loc, end: loc + len(alias), name: d.Name})
			}
		}
		for canon, aliases := range d.Values {
			for _, alias := range append([]string{canon}, aliases...) {
				if loc := findAlias(remaining, alias); loc >= 0 {
					dimHits = append(dimHits, dimHit{
						start:    loc,
						end:      loc + len(alias),
						name:     d.Name,
						asFilter: true,
						value:    canon,
					})
				}
			}
		}
	}
	sort.Slice(dimHits, func(i, j int) bool {
		spanI := dimHits[i].end - dimHits[i].start
		spanJ := dimHits[j].end - dimHits[j].start
		if spanI != spanJ {
			return spanI > spanJ
		}
		return dimHits[i].start < dimHits[j].start
	})
	seenDim := map[string]struct{}{}
	seenFilter := map[string]struct{}{}
	for _, h := range dimHits {
		if overlaps(used, h.start, h.end) {
			continue
		}
		if h.asFilter {
			key := h.name + "=" + h.value
			if _, ok := seenFilter[key]; ok {
				continue
			}
			seenFilter[key] = struct{}{}
			q.Filters = append(q.Filters, Filter{Dimension: h.name, Op: "eq", Value: h.value})
			mark(used, h.start, h.end)
			continue
		}
		if strings.EqualFold(h.name, "Date") {
			continue
		}
		if _, ok := seenDim[h.name]; ok {
			continue
		}
		seenDim[h.name] = struct{}{}
		q.Dimensions = append(q.Dimensions, h.name)
		mark(used, h.start, h.end)
	}

	if len(q.Metrics) == 0 {
		return nil, fmt.Errorf("%w: no known metric in %q", ErrParse, input)
	}
	return q, nil
}

func matchDatePhrase(text string, now time.Time) (datePhrase, bool) {
	now = now.In(time.UTC)
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	type cand struct {
		match string
		label string
		start time.Time
		end   time.Time
	}
	var found []cand

	lower := text
	add := func(match, label string, start, end time.Time) {
		if match == "" {
			return
		}
		found = append(found, cand{match: match, label: label, start: start, end: end})
	}

	if m := reLastNDays.FindStringSubmatch(text); len(m) == 2 {
		n := 0
		fmt.Sscanf(m[1], "%d", &n)
		if n > 0 {
			add(m[0], "last_"+m[1]+"_days", day.AddDate(0, 0, -n), day.AddDate(0, 0, 1))
		}
	}
	if loc := findAlias(lower, "yesterday"); loc >= 0 {
		add("yesterday", "yesterday", day.AddDate(0, 0, -1), day)
	}
	if loc := findAlias(lower, "today"); loc >= 0 {
		add("today", "today", day, day.AddDate(0, 0, 1))
	}
	if loc := findAlias(lower, "last week"); loc >= 0 {
		weekday := int(day.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		thisMonday := day.AddDate(0, 0, -(weekday - 1))
		add("last week", "last_week", thisMonday.AddDate(0, 0, -7), thisMonday)
	}
	if dates := reISODate.FindAllString(text, 2); len(dates) == 1 {
		if t, err := time.ParseInLocation("2006-01-02", dates[0], time.UTC); err == nil {
			add(dates[0], dates[0], t, t.AddDate(0, 0, 1))
		}
	} else if len(dates) >= 2 {
		s, err1 := time.ParseInLocation("2006-01-02", dates[0], time.UTC)
		e, err2 := time.ParseInLocation("2006-01-02", dates[1], time.UTC)
		if err1 == nil && err2 == nil {
			add(dates[0]+" "+dates[1], dates[0]+"_to_"+dates[1], s, e.AddDate(0, 0, 1))
		}
	}

	if len(found) == 0 {
		return datePhrase{}, false
	}
	sort.Slice(found, func(i, j int) bool {
		return len(found[i].match) > len(found[j].match)
	})
	best := found[0]
	return datePhrase{match: best.match, label: best.label, start: best.start, end: best.end}, true
}

func findAlias(text, alias string) int {
	alias = strings.TrimSpace(alias)
	if alias == "" {
		return -1
	}
	if isASCIIWord(alias) {
		re := regexp.MustCompile(`(?i)(^|[^A-Za-z0-9_])` + regexp.QuoteMeta(alias) + `([^A-Za-z0-9_]|$)`)
		loc := re.FindStringIndex(text)
		if loc == nil {
			return -1
		}
		start := loc[0]
		if start < len(text) && !isASCIIWordChar(rune(text[start])) {
			start++
		}
		return start
	}
	return strings.Index(text, alias)
}

func isASCIIWord(s string) bool {
	for _, r := range s {
		if r > unicode.MaxASCII || (!unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != ' ') {
			return false
		}
	}
	return true
}

func isASCIIWordChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

func overlaps(used []bool, start, end int) bool {
	if start < 0 {
		return true
	}
	if end > len(used) {
		end = len(used)
	}
	for i := start; i < end && i < len(used); i++ {
		if used[i] {
			return true
		}
	}
	return false
}

func mark(used []bool, start, end int) {
	if start < 0 {
		return
	}
	if end > len(used) {
		end = len(used)
	}
	for i := start; i < end; i++ {
		used[i] = true
	}
}
