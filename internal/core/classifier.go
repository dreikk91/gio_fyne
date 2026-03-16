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
	// We prioritize 'typ' (category from the catalog) as requested.
	// 'code' and 'desc' are kept for fallback or specific logic if needed.
	category := strings.ToLower(typ)
	description := strings.ToLower(desc)
	fullText := strings.ToLower(code + " " + typ + " " + desc)

	switch {
	// 1. Test - high priority
	case strings.Contains(category, "тест") || strings.Contains(category, "test"):
		return "test"

	// 2. Alarm/Fire/Intrusion
	case strings.Contains(category, "тривога") ||
		strings.Contains(category, "пожежа") ||
		strings.Contains(category, "напад") ||
		strings.Contains(category, "вторгнення") ||
		strings.Contains(category, "паніка"):
		return "alarm"

	// 3. Fault/Problem/Failure
	case strings.Contains(category, "несправність") ||
		strings.Contains(category, "проблема") ||
		strings.Contains(category, "помилка") ||
		strings.Contains(category, "втрата") ||
		strings.Contains(category, "кз") ||
		strings.Contains(category, "відсутність") ||
		strings.Contains(category, "невдача"):
		return "fault"

	// 4. Guard/Arming
	case strings.Contains(category, "постановка") ||
		strings.Contains(category, "взяття") ||
		strings.Contains(category, "guard") ||
		strings.Contains(description, "постанов"):
		return "guard"

	// 5. Disguard/Disarming/Restore
	case strings.Contains(category, "зняття") ||
		strings.Contains(category, "скасування") ||
		strings.Contains(category, "скидання") ||
		strings.Contains(category, "відновлення") ||
		strings.Contains(category, "норма") ||
		strings.Contains(category, "disguard") ||
		strings.Contains(category, "disarm") ||
		strings.Contains(description, "знят"):
		return "disguard"

	// Fallback to full text search if category is too vague (e.g. "Система" or "Різне")
	case strings.Contains(fullText, "трив") || strings.Contains(fullText, "alarm"):
		return "alarm"
	case strings.Contains(fullText, "тест") || strings.Contains(fullText, "test"):
		return "test"
	case strings.Contains(fullText, "помил") || strings.Contains(fullText, "несправ"):
		return "fault"

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
