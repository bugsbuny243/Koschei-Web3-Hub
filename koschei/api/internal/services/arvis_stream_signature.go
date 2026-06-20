package services

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

func arvisStreamScopedVerdictSignature(baseSignature, moduleID, streamEventID string) string {
	baseSignature = strings.TrimSpace(baseSignature)
	moduleID = strings.TrimSpace(moduleID)
	streamEventID = strings.TrimSpace(streamEventID)
	if streamEventID == "" {
		return baseSignature
	}
	sum := sha256.Sum256([]byte(baseSignature + "|" + moduleID + "|" + streamEventID))
	return "arvis-stream-" + hex.EncodeToString(sum[:])
}
