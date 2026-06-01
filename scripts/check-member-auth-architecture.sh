#!/usr/bin/env bash
set -euo pipefail

signup_route="src/app/api/auth/signup/route.ts"
dashboard="src/app/dashboard/page.tsx"
legacy_neon_auth="src/lib/server/neon-auth.ts"

if [[ ! -f "$signup_route" ]]; then
  echo "Missing Next.js signup proxy route: $signup_route" >&2
  exit 1
fi

if ! rg -q 'proxyAuthRequest\(request, "/auth/signup"\)' "$signup_route"; then
  echo "Next.js signup must proxy to the Go auth-api /auth/signup endpoint." >&2
  exit 1
fi

if [[ -e "$legacy_neon_auth" ]]; then
  echo "Legacy Next.js Neon Auth helper must stay removed: $legacy_neon_auth" >&2
  exit 1
fi

if rg -n 'password_hash|member_accounts|Owner Console|isOwnerUser|href="/admin"' src services/auth-api \
  --glob '!app/admin/**' --glob '!app/api/admin/**' --glob '!lib/server/admin-auth.ts'; then
  echo "Forbidden member-auth or dashboard architecture reference found." >&2
  exit 1
fi

if rg -n 'Owner Console|isOwnerUser|href="/admin"' "$dashboard"; then
  echo "Normal dashboard must not expose owner or admin controls." >&2
  exit 1
fi

echo "Member auth architecture guard passed."
