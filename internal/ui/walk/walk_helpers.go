//go:build windows

package walk

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dreikk91/gio_fyne/internal/config"
	"github.com/dreikk91/gio_fyne/internal/core"

	"github.com/lxn/walk"
)

var (
	colorWindow      = walk.RGB(238, 243, 247)
	colorSurface     = walk.RGB(252, 254, 255)
	colorSurfaceAlt  = walk.RGB(246, 250, 253)
	colorWhite       = colorSurface
	colorRowAlt      = walk.RGB(242, 247, 252)
	colorText        = walk.RGB(18, 26, 36)
	colorSoft        = walk.RGB(78, 92, 108)
	colorGoodBg      = walk.RGB(226, 248, 237)
	colorGoodText    = walk.RGB(15, 119, 73)
	colorWarnBg      = walk.RGB(255, 246, 222)
	colorWarnText    = walk.RGB(128, 82, 0)
	colorBadBg       = walk.RGB(254, 233, 232)
	colorBadText     = walk.RGB(176, 45, 36)
	colorFaultBg     = walk.RGB(255, 236, 214)
	colorFaultText   = walk.RGB(142, 82, 24)
	colorAccentBg    = walk.RGB(226, 241, 255)
	colorAccentText  = walk.RGB(14, 88, 173)
	colorSelectedBg  = walk.RGB(214, 235, 255)
	colorSelectedTxt = walk.RGB(14, 73, 142)

	colorHeroStart       = walk.RGB(18, 51, 86)
	colorHeroEnd         = walk.RGB(24, 72, 118)
	colorHeroTitle       = walk.RGB(243, 249, 255)
	colorHeroSubtitle    = walk.RGB(194, 214, 233)
	colorHeroChipText    = walk.RGB(250, 252, 255)
	colorHeroChipMetric  = walk.RGB(46, 109, 166)
	colorHeroChipOnline  = walk.RGB(20, 140, 92)
	colorHeroChipOffline = walk.RGB(173, 56, 49)
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

func boolText(condition bool, yes, no string) string {
	if condition {
		return yes
	}
	return no
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

func matchesEventFilter(evt core.EventDTO, filter string, hideTests bool, hideBlocked bool, query string) bool {
	filter = strings.ToLower(strings.TrimSpace(filter))
	if filter != "" && filter != "all" && !strings.EqualFold(evt.Category, filter) {
		return false
	}
	if hideTests && strings.EqualFold(evt.Category, "test") {
		return false
	}
	// Note: hideBlocked is not directly in core.EventDTO,
	// but the backend should have filtered it already if requested.
	// Here we just match the interface needs.
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

func filterEvents(all []core.EventDTO, filter string, hideTests bool, hideBlocked bool, query string, limit int) []core.EventDTO {
	res := make([]core.EventDTO, 0, minInt(len(all), limit))
	for _, e := range all {
		if matchesEventFilter(e, filter, hideTests, hideBlocked, query) {
			res = append(res, e)
		}
		if len(res) >= limit {
			break
		}
	}
	return res
}

func priorityColors(app *walkApp, category string, row int) (walk.Color, walk.Color) {
	cat := strings.ToLower(strings.TrimSpace(category))
	if app != nil {
		bg, bgOk := app.categoryColors[cat]
		fg, fgOk := app.categoryFontColors[cat]
		if bgOk {
			if !fgOk {
				fg = colorText
			}
			return tintByRow(bg, row), fg
		}
	}

	switch cat {
	case "alarm":
		return tintByRow(colorBadBg, row), colorBadText
	case "test":
		return tintByRow(colorWarnBg, row), colorWarnText
	case "fault":
		return tintByRow(colorFaultBg, row), colorFaultText
	case "guard":
		return tintByRow(colorGoodBg, row), colorGoodText
	case "disguard":
		return tintByRow(colorAccentBg, row), colorAccentText
	default:
		if row%2 == 0 {
			return colorRowAlt, colorText
		}
		return colorWhite, colorText
	}
}

func tintByRow(c walk.Color, row int) walk.Color {
	if row%2 == 0 {
		return c
	}
	return shiftColor(c, -9)
}

func shiftColor(c walk.Color, delta int) walk.Color {
	r := clampInt(int(uint8(c)), 0, 255)
	g := clampInt(int(uint8(c>>8)), 0, 255)
	b := clampInt(int(uint8(c>>16)), 0, 255)
	r = clampInt(r+delta, 0, 255)
	g = clampInt(g+delta, 0, 255)
	b = clampInt(b+delta, 0, 255)
	return walk.RGB(uint8(r), uint8(g), uint8(b))
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
	return strings.Join(lines, "\r\n")
}

func intsToStrings(ints []int) []string {
	res := make([]string, len(ints))
	for i, v := range ints {
		res[i] = strconv.Itoa(v)
	}
	return res
}

func parseGroupsLine(text string) []int {
	parts := strings.Split(text, ",")
	res := make([]int, 0, len(parts))
	for _, p := range parts {
		v := strings.TrimSpace(p)
		if v == "" {
			continue
		}
		if n, err := strconv.Atoi(v); err == nil {
			res = append(res, n)
		}
	}
	return res
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

