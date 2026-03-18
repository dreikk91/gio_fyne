package tea

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"cid_fyne/internal/config"
	"cid_fyne/internal/core"
)

var eventFilters = []string{"all", "alarm", "test", "fault", "guard", "disguard", "other"}
var logLevels = []string{"trace", "debug", "info", "warn", "error", "fatal"}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func atoiOr(def int, s string) int {
	v, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return def
	}
	return v
}

func parseBool(s string) bool {
	s = strings.TrimSpace(strings.ToLower(s))
	return s == "1" || s == "true" || s == "yes" || s == "y" || s == "on"
}

func boolText(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

func firstNonEmpty(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

func isStaleTime(ts time.Time, timeout time.Duration) bool {
	return ts.IsZero() || time.Since(ts) > timeout
}

func matchesEventFilter(evt core.EventDTO, filter string, hideTests bool, query string) bool {
	filter = strings.ToLower(strings.TrimSpace(filter))
	if filter != "" && filter != "all" && !strings.EqualFold(evt.Category, filter) {
		return false
	}
	if hideTests && strings.EqualFold(evt.Category, "test") {
		return false
	}
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return true
	}
	hay := strings.ToLower(strings.TrimSpace(evt.DeviceID + " " + evt.Code + " " + evt.Type + " " + evt.Desc + " " + evt.Zone))
	return strings.Contains(hay, query)
}

func filterDevices(all map[int]core.DeviceDTO, query string, isInactive func(core.DeviceDTO) bool) ([]core.DeviceDTO, int, int) {
	query = strings.ToLower(strings.TrimSpace(query))
	res := make([]core.DeviceDTO, 0, len(all))
	active, inactive := 0, 0
	for _, d := range all {
		stale := isInactive(d)
		if stale {
			inactive++
		} else {
			active++
		}
		if query != "" {
			hay := strings.ToLower(fmt.Sprintf("%d %s %s", d.ID, d.ClientAddr, d.LastEvent))
			if !strings.Contains(hay, query) {
				continue
			}
		}
		res = append(res, d)
	}

	sort.Slice(res, func(i, j int) bool {
		return res[i].ID < res[j].ID
	})

	return res, active, inactive
}

func filterEvents(all []core.EventDTO, filter string, hideTests bool, query string, limit int) []core.EventDTO {
	res := make([]core.EventDTO, 0, minInt(len(all), limit))
	for _, e := range all {
		if matchesEventFilter(e, filter, hideTests, query) {
			res = append(res, e)
		}
		if len(res) >= limit {
			break
		}
	}
	return res
}

func prependEvents(dst, batch []core.EventDTO, maxN int) []core.EventDTO {
	if len(batch) == 0 {
		if len(dst) > maxN {
			return dst[:maxN]
		}
		return dst
	}
	if len(batch) >= maxN {
		out := make([]core.EventDTO, maxN)
		for i := 0; i < maxN; i++ {
			out[i] = batch[len(batch)-1-i]
		}
		return out
	}
	keep := maxN - len(batch)
	if keep > len(dst) {
		keep = len(dst)
	}
	out := make([]core.EventDTO, len(batch)+keep)
	for i := 0; i < len(batch); i++ {
		out[i] = batch[len(batch)-1-i]
	}
	copy(out[len(batch):], dst[:keep])
	return out
}

func formatAccountRanges(ranges []config.AccountRange) string {
	if len(ranges) == 0 {
		return "2000-2200:+2100"
	}
	lines := make([]string, 0, len(ranges))
	for _, r := range ranges {
		sign := ""
		if r.Delta >= 0 {
			sign = "+"
		}
		lines = append(lines, fmt.Sprintf("%d-%d:%s%d", r.From, r.To, sign, r.Delta))
	}
	return strings.Join(lines, "\n")
}

func parseAccountRanges(text string) ([]config.AccountRange, error) {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	out := make([]config.AccountRange, 0, len(lines))
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) != 2 {
			return nil, fmt.Errorf("рядок %d: очікується формат From-To:Delta", i+1)
		}
		rng := strings.Split(strings.TrimSpace(parts[0]), "-")
		if len(rng) != 2 {
			return nil, fmt.Errorf("рядок %d: некоректний діапазон", i+1)
		}
		from, err := strconv.Atoi(strings.TrimSpace(rng[0]))
		if err != nil {
			return nil, fmt.Errorf("рядок %d: некоректне значення From", i+1)
		}
		to, err := strconv.Atoi(strings.TrimSpace(rng[1]))
		if err != nil {
			return nil, fmt.Errorf("рядок %d: некоректне значення To", i+1)
		}
		delta, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil {
			return nil, fmt.Errorf("рядок %d: некоректне значення Delta", i+1)
		}
		if from > to {
			from, to = to, from
		}
		out = append(out, config.AccountRange{From: from, To: to, Delta: delta})
	}
	return out, nil
}

func parseIntCSV(text string) []int {
	parts := strings.Split(text, ",")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		v := strings.TrimSpace(p)
		if v == "" {
			continue
		}
		n, err := strconv.Atoi(v)
		if err == nil {
			out = append(out, n)
		}
	}
	sort.Ints(out)
	return out
}

func parseCodesCSV(text string) []string {
	parts := strings.Split(text, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, p := range parts {
		v := strings.ToUpper(strings.TrimSpace(p))
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

func formatIntCSV(items []int) string {
	if len(items) == 0 {
		return ""
	}
	parts := make([]string, 0, len(items))
	for _, it := range items {
		parts = append(parts, strconv.Itoa(it))
	}
	return strings.Join(parts, ",")
}

func formatCodeCSV(items []string) string {
	if len(items) == 0 {
		return ""
	}
	vals := append([]string(nil), items...)
	sort.Strings(vals)
	return strings.Join(vals, ",")
}

func parseObjectCodesMap(text string) map[int][]string {
	out := map[int][]string{}
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		id, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil {
			continue
		}
		codes := parseCodesCSV(parts[1])
		if len(codes) > 0 {
			out[id] = codes
		}
	}
	return out
}

func formatObjectCodesMap(m map[int][]string) string {
	if len(m) == 0 {
		return ""
	}
	ids := make([]int, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	lines := make([]string, 0, len(ids))
	for _, id := range ids {
		codes := append([]string(nil), m[id]...)
		sort.Strings(codes)
		lines = append(lines, fmt.Sprintf("%d:%s", id, strings.Join(codes, ",")))
	}
	return strings.Join(lines, "\n")
}

func parseHexColor(s string) string {
	s = strings.TrimSpace(strings.ToUpper(s))
	if len(s) == 0 {
		return ""
	}
	if strings.HasPrefix(s, "#") {
		s = s[1:]
	}
	if len(s) != 6 {
		return ""
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'F')) {
			return ""
		}
	}
	return "#" + s
}
