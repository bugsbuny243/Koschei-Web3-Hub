package services

import "strings"

func RedactSecret(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "missing"
	}
	if len(value) <= 8 {
		return "***"
	}
	return value[:4] + "..." + value[len(value)-4:]
}

func SafeLogKey(name string, value string) string {
	if IsSensitiveEnvName(name) {
		return RedactSecret(value)
	}
	if strings.TrimSpace(value) == "" {
		return "missing"
	}
	return strings.TrimSpace(value)
}

func IsSensitiveEnvName(name string) bool {
	upper := strings.ToUpper(strings.TrimSpace(name))
	if upper == "" {
		return false
	}
	needles := []string{"API_KEY", "SECRET", "PRIVATE", "TOKEN", "PASSWORD", "DATABASE_URL", "WEBHOOK"}
	for _, needle := range needles {
		if strings.Contains(upper, needle) {
			return true
		}
	}
	return false
}
