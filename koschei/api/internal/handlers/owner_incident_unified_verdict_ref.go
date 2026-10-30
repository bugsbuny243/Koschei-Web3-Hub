package handlers

import (
	"context"
	"database/sql"
	"regexp"
	"strings"
)

const unifiedSOCVerdictRefPrefix = "unified-verdict:"
const unifiedSOCVerdictRefSource = "security_unified_radar_verdict:stored_signature_match_only"

var unifiedSOCVerdictRefUUID = regexp.MustCompile("^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$")
var unifiedSOCVerdictRefSignature = regexp.MustCompile("^[A-Za-z0-9_-]{86}$")

// resolveUnifiedSOCVerdictReference is an owner-only evidence-reference lookup.
// It links a known customer receipt to a manually created SOC case; it neither
// creates alerts nor changes the ARVIS decision. The stored signature is matched
// byte-for-byte, NOT independently cryptographically reverified at read time.
// The time-varying unified history row must not be labelled an immutable proof.
func resolveUnifiedSOCVerdictReference(
	ctx context.Context, tx *sql.Tx, ref, network, target string,
) (string, error) {
	if tx == nil || strings.TrimSpace(network) == "" || strings.TrimSpace(target) == "" {
		return "", nil
	}
	raw := strings.TrimPrefix(ref, unifiedSOCVerdictRefPrefix)
	uuid, signature, ok := strings.Cut(raw, ":")
	if !ok || !unifiedSOCVerdictRefUUID.MatchString(uuid) ||
		!unifiedSOCVerdictRefSignature.MatchString(signature) {
		return "", nil
	}
	var matched bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM security_unified_radar_verdicts
			WHERE id=$1::uuid AND network=$2 AND target_kind='token' AND target_id=$3
			  AND signed=true AND signature=$4 AND signature_algorithm='ed25519'
			  AND key_id IS NOT NULL AND btrim(key_id)<>''
			  AND payload_hash ~ '^sha256:[0-9a-f]{64}$'
		)`, uuid, network, target, signature).Scan(&matched); err != nil {
		return "", err
	}
	if !matched {
		return "", nil
	}
	return unifiedSOCVerdictRefSource, nil
}
