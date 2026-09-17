package handlers

import (
	"regexp"
	"strings"
)

var (
	reportSecretQueryPattern = regexp.MustCompile(`(?i)([?&](?:api[-_]?key|apikey|access[-_]?token|token|secret|client[-_]?secret|key)=)[^&\\s"'<>]+`)
	reportBearerPattern      = regexp.MustCompile(`(?i)(bearer\\s+)[A-Za-z0-9._~+/=-]+`)
	reportSecretAssignPattern = regexp.MustCompile(`(?i)((?:HELIUS|ALCHEMY|QUICKNODE|SOLANA)?_?(?:API[_-]?KEY|ACCESS[_-]?TOKEN|TOKEN|SECRET)\\s*=\\s*)[^\\s,;]+`)
)

func redactSensitiveErrorText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = reportSecretQueryPattern.ReplaceAllString(value, `${1}[REDACTED]`)
	value = reportBearerPattern.ReplaceAllString(value, `${1}[REDACTED]`)
	value = reportSecretAssignPattern.ReplaceAllString(value, `${1}[REDACTED]`)
	return value
}
