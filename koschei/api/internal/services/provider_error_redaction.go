package services

import (
	"regexp"
	"strings"
)

var providerCredentialPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)((?:https?|wss?)://[^\s"]+/v2/)[^\s"]+`),
	regexp.MustCompile(`(?i)([?&](?:api[_-]?key|apikey|access[_-]?token|token|secret|client[_-]?secret|key)=)[^&\s"'<>]+`),
	regexp.MustCompile(`(?i)(bearer\s+)[A-Za-z0-9._~+/=-]+`),
	regexp.MustCompile(`(?i)((?:HELIUS|ALCHEMY|QUICKNODE|SOLANA)?_?(?:API[_-]?KEY|ACCESS[_-]?TOKEN|TOKEN|SECRET)\s*=\s*)[^&\s,;"'<>]+`),
}

func SafeProviderError(err error) string {
	if err == nil {
		return ""
	}
	message := redactProviderCredentials(err.Error())
	if len(message) > 240 {
		message = message[:240]
	}
	return message
}

func RedactProviderCredentials(message string) string {
	return redactProviderCredentials(message)
}

func safeProviderError(err error) string {
	return SafeProviderError(err)
}

func redactProviderCredentials(message string) string {
	for _, pattern := range providerCredentialPatterns {
		message = pattern.ReplaceAllString(message, `${1}[redacted]`)
	}
	return message
}
