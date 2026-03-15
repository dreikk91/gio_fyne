package ui

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"
	"time"

	"cid_fyne/internal/config"
	"cid_fyne/internal/core"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

func prepend(dst, batch []core.EventDTO, maxN int) []core.EventDTO {
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

func eventColor(cat string, row int) color.NRGBA {
	switch strings.ToLower(strings.TrimSpace(cat)) {
	case "alarm":
		return cBadSoft
	case "test":
		return cWarnSoft
	case "fault":
		return color.NRGBA{R: 255, G: 238, B: 214, A: 255}
	case "guard":
		return cGoodSoft
	case "disguard":
		return cAccentSoft
	default:
		return firstColor(row%2 == 0, cPanel2, cPanel)
	}
}

func eventTextColor(cat string) color.NRGBA {
	switch strings.ToLower(strings.TrimSpace(cat)) {
	case "alarm":
		return cBad
	case "test":
		return cWarn
	case "fault":
		return color.NRGBA{R: 168, G: 95, B: 0, A: 255}
	case "guard":
		return cGood
	case "disguard":
		return cAccent
	default:
		return cText
	}
}

func eventTextColorName(cat string) fyne.ThemeColorName {
	switch strings.ToLower(strings.TrimSpace(cat)) {
	case "alarm":
		return theme.ColorNameError
	case "test":
		return theme.ColorNameWarning
	case "fault":
		return theme.ColorNameWarning
	case "guard":
		return theme.ColorNameSuccess
	case "disguard":
		return theme.ColorNamePrimary
	default:
		return theme.ColorNameForeground
	}
}

func rowAltColor(i int) color.NRGBA {
	return firstColor(i%2 == 0, cPanel2, cPanel)
}

func relayTextColor(blocked bool) color.NRGBA {
	if blocked {
		return cBad
	}
	return cGood
}

func isStale(t time.Time, d time.Duration) bool { return t.IsZero() || time.Since(t) > d }

func atoiOr(def int, s string) int {
	v, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return def
	}
	return v
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func boolText(c bool, t, f string) string {
	if c {
		return t
	}
	return f
}

func firstColor(c bool, t, f color.NRGBA) color.NRGBA {
	if c {
		return t
	}
	return f
}

func firstNonEmpty(s, fallback string) string {
	if strings.TrimSpace(s) != "" {
		return s
	}
	return fallback
}

func eventBelongsToDevice(deviceID string, id int) bool {
	if n, err := strconv.Atoi(strings.TrimSpace(deviceID)); err == nil {
		return n == id
	}
	trimmed := strings.TrimSpace(deviceID)
	return trimmed == strconv.Itoa(id) || trimmed == fmt.Sprintf("%03d", id)
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
	lines := strings.Split(text, "\n")
	out := make([]config.AccountRange, 0, len(lines))
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) != 2 {
			return nil, fmt.Errorf("line %d: expected From-To:Delta", i+1)
		}
		rng := strings.Split(strings.TrimSpace(parts[0]), "-")
		if len(rng) != 2 {
			return nil, fmt.Errorf("line %d: invalid range", i+1)
		}
		from, err := strconv.Atoi(strings.TrimSpace(rng[0]))
		if err != nil {
			return nil, fmt.Errorf("line %d: invalid from", i+1)
		}
		to, err := strconv.Atoi(strings.TrimSpace(rng[1]))
		if err != nil {
			return nil, fmt.Errorf("line %d: invalid to", i+1)
		}
		delta, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil {
			return nil, fmt.Errorf("line %d: invalid delta", i+1)
		}
		if from > to {
			from, to = to, from
		}
		out = append(out, config.AccountRange{From: from, To: to, Delta: delta})
	}
	return out, nil
}

func intsToStrings(ints []int) []string {
	out := make([]string, len(ints))
	for i, v := range ints {
		out[i] = strconv.Itoa(v)
	}
	return out
}

func parseGroupsLine(text string) []int {
	parts := strings.FieldsFunc(text, func(r rune) bool {
		return r == ',' || r == ' ' || r == ';'
	})
	out := []int{}
	for _, p := range parts {
		if v, err := strconv.Atoi(p); err == nil {
			out = append(out, v)
		}
	}
	return out
}

func formatEventLine(e core.EventDTO) string {
	relay := "OK"
	if e.RelayBlocked {
		relay = "Blocked"
	}
	var b strings.Builder
	b.Grow(128)
	if !e.Time.IsZero() {
		b.WriteString(e.Time.Format("2006-01-02 15:04:05"))
	}
	if e.DeviceID != "" {
		if b.Len() > 0 {
			b.WriteString(" | ")
		}
		b.WriteString(e.DeviceID)
	}
	if e.Code != "" {
		if b.Len() > 0 {
			b.WriteString(" | ")
		}
		b.WriteString(e.Code)
	}
	if e.Type != "" {
		if b.Len() > 0 {
			b.WriteString(" | ")
		}
		b.WriteString(e.Type)
	}
	if strings.TrimSpace(e.Desc) != "" {
		if b.Len() > 0 {
			b.WriteString(" | ")
		}
		b.WriteString(strings.TrimSpace(e.Desc))
	}
	if strings.TrimSpace(e.Zone) != "" {
		if b.Len() > 0 {
			b.WriteString(" | ")
		}
		b.WriteString(strings.TrimSpace(e.Zone))
	}
	if b.Len() > 0 {
		b.WriteString(" | ")
	}
	b.WriteString(relay)
	return b.String()
}

func filterTone(filter string) (color.NRGBA, color.NRGBA) {
	switch strings.ToLower(strings.TrimSpace(filter)) {
	case "alarm":
		return cBadSoft, cBad
	case "test":
		return cWarnSoft, cWarn
	case "fault":
		return color.NRGBA{R: 255, G: 238, B: 214, A: 255}, color.NRGBA{R: 168, G: 95, B: 0, A: 255}
	case "guard":
		return cGoodSoft, cGood
	case "disguard":
		return cAccentSoft, cAccent
	case "other":
		return cPanel3, cSoft
	default:
		return cAccent2, cAccent
	}
}
