package http

import (
	"encoding/hex"
	"os"
	"strings"
)

func currentDeploymentRevision() string {
	return deploymentRevisionFrom(os.Getenv)
}

func deploymentRevisionFrom(getenv func(string) string) string {
	if getenv == nil {
		return ""
	}
	value := strings.ToLower(strings.TrimSpace(getenv("RAILWAY_GIT_COMMIT_SHA")))
	if len(value) != 40 {
		return ""
	}
	if _, err := hex.DecodeString(value); err != nil {
		return ""
	}
	return value
}
