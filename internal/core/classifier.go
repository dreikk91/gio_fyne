package core

import "strings"

func DeterminePriority(code string) int {
	uc := strings.ToUpper(code)
	switch {
	case strings.HasPrefix(uc, "E1"):
		return 4
	case strings.HasPrefix(uc, "R4"):
		return 1
	case strings.HasPrefix(uc, "E4"):
		return 2
	case strings.HasPrefix(uc, "R"):
		return 3
	default:
		return 0
	}
}

func Classify(code, typ, desc string) string {
	_ = typ
	_ = desc
	raw := strings.ToUpper(strings.TrimSpace(code))
	if raw == "" {
		return "other"
	}
	if len(raw) >= 2 && (raw[0] == 'E' || raw[0] == 'R') {
		if n, ok := parseCodeNumber(raw[1:]); ok {
			if n >= 100 && n <= 199 {
				return "alarm"
			}
			if n >= 300 && n <= 380 {
				return "fault"
			}
			if n >= 400 && n <= 499 {
				if raw[0] == 'E' {
					return "disguard"
				}
				return "guard"
			}
		}
	}
	if n, ok := parseCodeNumber(raw); ok {
		switch {
		case n >= 200 && n <= 205:
			return "alarm"
		case n >= 300 && n <= 380:
			return "fault"
		case n >= 400 && n <= 410:
			return "disguard"
		case n >= 500 && n <= 570:
			return "fault"
		case n >= 600 && n <= 699:
			return "test"
		}
	}
	return "other"
}

func parseCodeNumber(s string) (int, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	n := 0
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0, false
		}
		n = n*10 + int(ch-'0')
	}
	return n, true
}
