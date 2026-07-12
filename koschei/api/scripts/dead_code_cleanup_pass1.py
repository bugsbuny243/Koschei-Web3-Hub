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

# Confirmed disconnected handler files: no registered production route and no
# required symbol after Neon-only auth isolation. Build verification is the gate.
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
