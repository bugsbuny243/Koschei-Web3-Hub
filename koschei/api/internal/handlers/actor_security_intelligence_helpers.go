package handlers

import "strings"

func shortHolderIntelligence(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 18 {
		return value
	}
	return value[:8] + "…" + value[len(value)-6:]
}
