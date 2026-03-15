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
	text := strings.ToLower(code + " " + typ + " " + desc)
	switch {
	case strings.Contains(text, "test") || strings.Contains(text, "тест"):
		return "test"
	case strings.Contains(text, "alarm") || strings.Contains(text, "трив"):
		return "alarm"
	case strings.Contains(text, "fault") || strings.Contains(text, "несправ") || strings.Contains(text, "помил"):
		return "fault"
	case strings.Contains(text, "disguard") || strings.Contains(text, "disarm") || strings.Contains(text, "знят"):
		return "disguard"
	case strings.Contains(text, "guard") || strings.Contains(text, "постанов"):
		return "guard"
	default:
		return "other"
	}
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
