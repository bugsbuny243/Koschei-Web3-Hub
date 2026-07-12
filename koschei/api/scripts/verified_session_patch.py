from pathlib import Path


def replace_once(text: str, old: str, new: str, label: str) -> str:
    if old not in text:
        raise SystemExit(f"missing replacement target: {label}")
    return text.replace(old, new, 1)


auth_path = Path("public/js/koschei-auth.js")
auth = auth_path.read_text()
auth = replace_once(
    auth,
    "  const LEGACY_KEY = 'koschei_token';\n",
    "  const LEGACY_KEY = 'koschei_token';\n  // A syntactically valid JWT is not an authenticated session until /api/me confirms it.\n  let sessionVerified = false;\n",
    "session verification state",
)
auth = replace_once(
    auth,
    "  function clearJwt() {\n    try {",
    "  function clearJwt() {\n    sessionVerified = false;\n    try {",
    "clearJwt verified state",
)
auth = replace_once(
    auth,
    "    saveJwt(jwt);\n    const me = await verifyMe(jwt);\n    return { ...result.data, me, access_token: jwt, token_type: 'Bearer' };",
    "    sessionVerified = false;\n    saveJwt(jwt);\n    const me = await verifyMe(jwt);\n    sessionVerified = true;\n    return { ...result.data, me, access_token: jwt, token_type: 'Bearer' };",
    "finishAuth verified state",
)
auth = replace_once(
    auth,
    "  async function init() {\n    consumeAccessTokenFromHash();",
    "  async function init() {\n    sessionVerified = false;\n    consumeAccessTokenFromHash();",
    "init reset",
)
auth = replace_once(
    auth,
    "        await verifyMe(jwt);\n        return true;",
    "        await verifyMe(jwt);\n        sessionVerified = true;\n        return true;",
    "init verification success",
)
auth = replace_once(
    auth,
    "  function isLoggedIn() { return jwtIsUsable(getJwt()); }",
    "  function isLoggedIn() { return sessionVerified && jwtIsUsable(getJwt()); }",
    "verified isLoggedIn",
)
auth_path.write_text(auth)

changed = []
for html_path in sorted(Path("public").rglob("*.html")):
    if html_path.name == "dashboard.html":
        continue
    text = html_path.read_text()
    updated = text.replace("/js/koschei-auth.js?v=32", "/js/koschei-auth.js?v=33")
    if updated != text:
        html_path.write_text(updated)
        changed.append(str(html_path))

if not changed:
    raise SystemExit("no koschei-auth.js?v=32 HTML references were updated")

print(f"updated auth cache version in {len(changed)} HTML files")
for path in changed:
    print(path)
