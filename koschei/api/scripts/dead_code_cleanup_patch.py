from pathlib import Path


def replace_once(text: str, old: str, new: str, label: str) -> str:
    if old not in text:
        raise SystemExit(f"missing replacement target: {label}")
    return text.replace(old, new, 1)

path = Path("internal/handlers/neon_auth.go")
text = path.read_text()
text = replace_once(
    text,
    '''func parseAndVerifyNeonJWT(token string) (neonJWTClaims, error) {
	if claims, ok, err := tryLocalJWT(token); ok {
		return claims, err
	}
	return neonClaimsFromToken(token)
}''',
    '''func parseAndVerifyNeonJWT(token string) (neonJWTClaims, error) {
	// HARD RULE: production customer authentication is Neon Auth only.
	// Do not reintroduce locally signed fallback JWTs or password storage.
	return neonClaimsFromToken(token)
}''',
    "Neon-only JWT verification",
)
path.write_text(text)
