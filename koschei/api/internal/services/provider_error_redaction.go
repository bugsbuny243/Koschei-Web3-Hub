package services

import (
	"regexp"
	"strings"
)

var providerCredentialPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)((?:https?|wss?)://[^\s\"]+/v2/)[^\s\"]+`),
	regexp.MustCompile(`(?i)([?&](?:api[_-]?key|apikey|key|token)=)[^&\s\"]+`),
}

func safeProviderError(err error) string {
	if err == nil {
		return ""
	}
	return redactProviderCredentials(err.Error())
}

func redactProviderCredentials(message string) string {
	message = strings.TrimSpace(message)
	for _, pattern := range providerCredentialPatterns {
		message = pattern.ReplaceAllString(message, `${1}[redacted]`)
	}
	if len(message) > 240 {
		message = message[:240]
	}
	return message
}
