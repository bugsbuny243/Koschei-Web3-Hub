#!/usr/bin/env bash
set -euo pipefail

legacy_neon_auth="src/lib/server/neon-auth.ts"
dashboard="src/app/dashboard/page.tsx"
go_main="services/auth-api/main.go"
admin_auth="src/lib/server/admin-auth.ts"
admin_proxy="src/proxy.ts"

require_file() {
  if [[ ! -f "$1" ]]; then
    echo "Missing required architecture file: $1" >&2
    exit 1
  fi
}

require_pattern() {
  local pattern="$1"
  local file="$2"
  local message="$3"
  if ! rg -q "$pattern" "$file"; then
    echo "$message" >&2
    exit 1
  fi
}

require_file "src/app/api/auth/signup/route.ts"
require_file "src/app/api/auth/login/route.ts"
require_file "src/app/api/auth/me/route.ts"
require_file "src/app/api/auth/logout/route.ts"
require_file "$go_main"
require_file "$dashboard"
require_file "$admin_auth"
require_file "$admin_proxy"

require_pattern 'proxyAuthRequest\(request, "/auth/signup"\)' "src/app/api/auth/signup/route.ts" "Next.js signup must proxy to the Go auth-api /auth/signup endpoint."
require_pattern 'proxyAuthRequest\(request, "/auth/login"\)' "src/app/api/auth/login/route.ts" "Next.js login must proxy to the Go auth-api /auth/login endpoint."
require_pattern 'proxyAuthRequest\(request, "/auth/me"\)' "src/app/api/auth/me/route.ts" "Next.js me must proxy to the Go auth-api /auth/me endpoint."
require_pattern 'proxyAuthRequest\(request, "/auth/logout"\)' "src/app/api/auth/logout/route.ts" "Next.js logout must proxy to the Go auth-api /auth/logout endpoint."

if [[ -e "$legacy_neon_auth" ]]; then
  echo "Legacy Next.js Neon Auth helper must stay removed: $legacy_neon_auth" >&2
  exit 1
fi

if rg -n 'neon-auth|authenticateWithNeonAuth|NEON_AUTH_|password_hash|member_accounts' src; then
  echo "Next.js must not contain legacy member Neon Auth or password-table logic." >&2
  exit 1
fi

require_pattern 'HandleFunc\("GET /health"' "$go_main" "Go auth-api must expose GET /health."
require_pattern 'HandleFunc\("POST /auth/signup"' "$go_main" "Go auth-api must expose POST /auth/signup."
require_pattern 'HandleFunc\("POST /auth/login"' "$go_main" "Go auth-api must expose POST /auth/login."
require_pattern 'HandleFunc\("GET /auth/me"' "$go_main" "Go auth-api must expose GET /auth/me."
require_pattern 'HandleFunc\("POST /auth/logout"' "$go_main" "Go auth-api must expose POST /auth/logout."

if rg -n 'password_hash|member_accounts' services/auth-api; then
  echo "Go member auth must not use password_hash or member_accounts." >&2
  exit 1
fi

if rg -n 'Owner Console|isOwnerUser|href="/admin"' "$dashboard"; then
  echo "Normal dashboard must not expose owner or admin controls." >&2
  exit 1
fi

require_pattern 'ADMIN_EMAIL' "$admin_auth" "Admin auth must use ADMIN_EMAIL."
require_pattern 'ADMIN_PASSWORD' "$admin_auth" "Admin auth must use ADMIN_PASSWORD."
require_pattern 'matcher: "/api/admin/:path\*"' "$admin_proxy" "Admin API proxy must protect /api/admin routes."
require_pattern 'hasValidAdminCookie\(request\)' "$admin_proxy" "Admin API proxy must require the admin cookie."

for route in src/app/api/admin/*/route.ts; do
  if [[ "$route" == "src/app/api/admin/login/route.ts" ]]; then
    continue
  fi
  require_pattern 'isAdminRequest' "$route" "Admin API route must validate the admin cookie: $route"
done

echo "Member auth architecture guard passed."
