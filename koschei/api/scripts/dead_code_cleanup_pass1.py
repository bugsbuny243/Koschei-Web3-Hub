from pathlib import Path

handlers = Path("internal/handlers")

# Neon is the only supported auth path. Remove the local-JWT fallback before
# deleting local_auth.go so production auth cannot silently accept legacy tokens.
neon = handlers / "neon_auth.go"
text = neon.read_text()
old = '''func parseAndVerifyNeonJWT(token string) (neonJWTClaims, error) {
	if claims, ok, err := tryLocalJWT(token); ok {
		return claims, err
	}
	return neonClaimsFromToken(token)
}'''
new = '''func parseAndVerifyNeonJWT(token string) (neonJWTClaims, error) {
	return neonClaimsFromToken(token)
}'''
if old not in text:
    raise SystemExit("legacy local JWT fallback not found")
neon.write_text(text.replace(old, new, 1))

# Remove an unreachable Shopier webhook left inside the owner monolith. The
# production route map exposes no Shopier webhook and the current product has no
# external payment-provider route.
owner = handlers / "owner.go"
owner_text = owner.read_text()
start_marker = "func (h *Handler) ShopierWebhook(w http.ResponseWriter, r *http.Request) {"
end_marker = "func (h *Handler) executeOwnerBrainCommand"
start = owner_text.find(start_marker)
end = owner_text.find(end_marker, start)
if start < 0 or end < 0:
    raise SystemExit("stale Shopier webhook block not found")
owner.write_text(owner_text[:start] + owner_text[end:])

# web3.go, package_status.go, impact_metrics.go and mev_shield.go accumulated
# generic helpers used by live handlers. Keep only those primitives here.
(handlers / "request_identity_helpers.go").write_text('''package handlers

import (
	"database/sql"
	"strings"
	"time"
)

func normalizedClaimEmail(claims neonJWTClaims) string {
	return strings.ToLower(strings.TrimSpace(claims.Email))
}

func currentUserID(claims neonJWTClaims) string {
	if strings.TrimSpace(claims.Sub) != "" {
		return strings.TrimSpace(claims.Sub)
	}
	return strings.TrimSpace(claims.Email)
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func nullTimePtr(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	return &value.Time
}

func isMissingRelation(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "does not exist") || strings.Contains(message, "undefined_table")
}
''')

# Confirmed disconnected handler files: no registered production route. The
# production and full Linux builds remain the final deletion gate.
files = [
    "rug_radar.go",
    "web3.go",
    "local_auth.go",
    "mev_shield.go",
    "liquidity_radar.go",
    "impact_metrics.go",
    "metadata.go",
    "owner_payment_health.go",
    "web3_jobs.go",
    "package_status.go",
    "jobs.go",
    "dao_guardian.go",
    "smart_money.go",
]
for name in files:
    path = handlers / name
    if path.exists():
        path.unlink()
