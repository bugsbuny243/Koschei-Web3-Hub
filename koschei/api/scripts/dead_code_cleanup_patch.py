from pathlib import Path
import re

root = Path('.')

# Move the handful of generic helpers that leaked out of legacy modules into
# the shared helper file before deleting those modules.
shared = root / 'internal/handlers/shared_helpers.go'
text = shared.read_text()
text = text.replace('import (\n\t"crypto/rand"', 'import (\n\t"crypto/rand"\n\t"database/sql"')
text = text.replace('\t"strings"\n)', '\t"strings"\n\t"time"\n)')
append = r'''

func normalizedClaimEmail(claims neonJWTClaims) string {
	return strings.ToLower(strings.TrimSpace(claims.Email))
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

func isMissingRelation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "does not exist") || strings.Contains(msg, "undefined_table")
}

func nullTimePtr(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	return &value.Time
}
'''
if 'func normalizedClaimEmail(' not in text:
    text += append
shared.write_text(text)

# Neon is the only supported user authentication path. Remove the local JWT
# fallback instead of preserving an undocumented second issuer.
neon = root / 'internal/handlers/neon_auth.go'
text = neon.read_text()
text = text.replace('''func parseAndVerifyNeonJWT(token string) (neonJWTClaims, error) {
	if claims, ok, err := tryLocalJWT(token); ok {
		return claims, err
	}
	return neonClaimsFromToken(token)
}''', '''func parseAndVerifyNeonJWT(token string) (neonJWTClaims, error) {
	return neonClaimsFromToken(token)
}''')
neon.write_text(text)

# Remove the orphaned Shopier webhook implementation. It is not registered and
# referenced a package map that no longer exists, breaking the production build.
owner = root / 'internal/handlers/owner.go'
text = owner.read_text()
pattern = re.compile(r'\nfunc \(h \*Handler\) ShopierWebhook\(w http\.ResponseWriter, r \*http\.Request\) \{.*?\n\}\n\nfunc \(h \*Handler\) executeOwnerBrainCommand', re.S)
text, count = pattern.subn('\nfunc (h *Handler) executeOwnerBrainCommand', text, count=1)
if count != 1:
    raise SystemExit('ShopierWebhook block not found')
owner.write_text(text)

# Files proven to have no live route or production reference. Helpers used by
# live code were moved above. Test-only enterprise modules are removed together
# with their obsolete feature test.
remove = [
    'internal/handlers/rug_radar.go',
    'internal/handlers/web3.go',
    'internal/handlers/local_auth.go',
    'internal/handlers/mev_shield.go',
    'internal/handlers/liquidity_radar.go',
    'internal/handlers/impact_metrics.go',
    'internal/handlers/metadata.go',
    'internal/handlers/owner_payment_health.go',
    'internal/handlers/web3_jobs.go',
    'internal/handlers/package_status.go',
    'internal/handlers/jobs.go',
    'internal/handlers/dao_guardian.go',
    'internal/handlers/plans.go',
    'internal/handlers/smart_money.go',
    'internal/handlers/credits.go',
    'internal/handlers/credits_reservation.go',
    'internal/handlers/enterprise_modules_test.go',
]
for name in remove:
    path = root / name
    if path.exists():
        path.unlink()
