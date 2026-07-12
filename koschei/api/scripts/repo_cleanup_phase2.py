from __future__ import annotations

from pathlib import Path

ROOT = Path('.')


def replace_once(text: str, old: str, new: str, label: str) -> str:
    if old not in text:
        raise SystemExit(f'missing replacement target: {label}')
    return text.replace(old, new, 1)


def remove_go_function(text: str, signature: str) -> tuple[str, bool]:
    start = text.find(signature)
    if start < 0:
        return text, False
    brace = text.find('{', start)
    if brace < 0:
        raise SystemExit(f'malformed function: {signature}')
    depth = 0
    in_string = False
    in_raw = False
    in_line_comment = False
    in_block_comment = False
    escape = False
    i = brace
    while i < len(text):
        ch = text[i]
        nxt = text[i + 1] if i + 1 < len(text) else ''
        if in_line_comment:
            if ch == '\n':
                in_line_comment = False
        elif in_block_comment:
            if ch == '*' and nxt == '/':
                in_block_comment = False
                i += 1
        elif in_raw:
            if ch == '`':
                in_raw = False
        elif in_string:
            if escape:
                escape = False
            elif ch == '\\':
                escape = True
            elif ch == '"':
                in_string = False
        else:
            if ch == '/' and nxt == '/':
                in_line_comment = True
                i += 1
            elif ch == '/' and nxt == '*':
                in_block_comment = True
                i += 1
            elif ch == '`':
                in_raw = True
            elif ch == '"':
                in_string = True
            elif ch == '{':
                depth += 1
            elif ch == '}':
                depth -= 1
                if depth == 0:
                    end = i + 1
                    while end < len(text) and text[end] in '\r\n':
                        end += 1
                    return text[:start] + text[end:], True
        i += 1
    raise SystemExit(f'unclosed function: {signature}')


def remove_functions(path: Path, signatures: list[str]) -> None:
    text = path.read_text()
    for signature in signatures:
        text, _ = remove_go_function(text, signature)
    path.write_text(text)


# Shared helpers were trapped inside obsolete feature files. Move only the
# genuinely shared pieces into the active helper surface before deleting them.
shared = ROOT / 'internal/handlers/shared_helpers.go'
shared.write_text('''package handlers

import (
    "crypto/rand"
    "database/sql"
    "fmt"
    "os"
    "strings"
    "time"
)

func solanaRPCURL(network string, apiKey string) string {
    if apiKey != "" {
        switch strings.ToLower(network) {
        case "solana-mainnet", "mainnet", "mainnet-beta", "solana-mainnet-beta":
            return "https://solana-mainnet.g.alchemy.com/v2/" + apiKey
        case "solana-devnet", "devnet":
            return "https://solana-devnet.g.alchemy.com/v2/" + apiKey
        case "solana-testnet", "testnet":
            return "https://solana-testnet.g.alchemy.com/v2/" + apiKey
        }
    }
    switch strings.ToLower(network) {
    case "solana-mainnet", "mainnet", "mainnet-beta", "solana-mainnet-beta":
        return "https://api.mainnet-beta.solana.com"
    case "solana-testnet", "testnet":
        return "https://api.testnet.solana.com"
    default:
        return "https://api.devnet.solana.com"
    }
}

func aiProviderConfigured() bool {
    return strings.TrimSpace(os.Getenv("TOGETHER_API_KEY")) != "" ||
        strings.TrimSpace(os.Getenv("ANTHROPIC_OWNER_API_KEY")) != "" ||
        strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY")) != ""
}

func newID() string {
    var b [16]byte
    _, _ = rand.Read(b[:])
    b[6] = (b[6] & 0x0f) | 0x40
    b[8] = (b[8] & 0x3f) | 0x80
    return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func maxInt(a, b int) int {
    if a > b {
        return a
    }
    return b
}

func firstNonEmpty(values ...string) string {
    for _, value := range values {
        if strings.TrimSpace(value) != "" {
            return strings.TrimSpace(value)
        }
    }
    return ""
}

func firstNonEmptyString(values ...string) string { return firstNonEmpty(values...) }

func normalizedClaimEmail(claims neonJWTClaims) string {
    return strings.ToLower(strings.TrimSpace(claims.Email))
}

func isMissingRelation(err error) bool {
    if err == nil {
        return false
    }
    message := strings.ToLower(err.Error())
    return strings.Contains(message, "does not exist") || strings.Contains(message, "undefined_table")
}

func nullTimePtr(value sql.NullTime) *time.Time {
    if !value.Valid {
        return nil
    }
    return &value.Time
}
''')

# Keep the active telemetry hooks that the Radar uses; discard the old grant OS.
(ROOT / 'internal/handlers/telemetry.go').write_text('''package handlers

func (h *Handler) logTool(email, tool, status string) {
    if h == nil || h.DB == nil {
        return
    }
    _, _ = h.DB.Exec(`INSERT INTO tool_usage_logs(email,tool_key,status) VALUES(NULLIF($1,''),$2,$3)`, email, tool, status)
}

func (h *Handler) trackEvent(email, name, path string) {
    if h == nil || h.DB == nil {
        return
    }
    _, _ = h.DB.Exec(`INSERT INTO analytics_events(event_name,email,path,metadata) VALUES($1,NULLIF($2,''),$3,'{}'::jsonb)`, name, email, path)
}
''')

# Replace the legacy package/output counter with a direct KOSCH access guard.
(ROOT / 'internal/handlers/kosch_access_guard.go').write_text('''package handlers

import (
    "context"
    "errors"
)

func (h *Handler) requireKOSCHAccess(ctx context.Context, authSubject string) error {
    if h == nil || h.DB == nil {
        return errors.New("database unavailable")
    }
    active, err := h.hasTokenTierAccess(ctx, authSubject, "basic")
    if err != nil {
        return err
    }
    if !active {
        return errors.New("verified KOSCH holder access required")
    }
    return nil
}
''')

# route_stubs.go is live auth/session code, not a stub. Rename it truthfully.
route_stub = ROOT / 'internal/handlers/route_stubs.go'
if route_stub.exists():
    (ROOT / 'internal/handlers/auth_session.go').write_text(route_stub.read_text())
    route_stub.unlink()

# Neon is the only customer authentication authority. Remove local JWT fallback.
neon = ROOT / 'internal/handlers/neon_auth.go'
text = neon.read_text()
text = replace_once(
    text,
    '''func parseAndVerifyNeonJWT(token string) (neonJWTClaims, error) {
\tif claims, ok, err := tryLocalJWT(token); ok {
\t\treturn claims, err
\t}
\treturn neonClaimsFromToken(token)
}''',
    '''func parseAndVerifyNeonJWT(token string) (neonJWTClaims, error) {
\treturn neonClaimsFromToken(token)
}''',
    'Neon-only JWT verification',
)
neon.write_text(text)

# Active Radar routes now state their real access model and no longer mention or
# consume package outputs.
radar = ROOT / 'internal/handlers/security_radar.go'
text = radar.read_text()
text = replace_once(
    text,
    '''\tif _, err := h.requirePremiumOutput(claims.Sub, claimEmail); err != nil {
\t\twriteJSON(w, http.StatusPaymentRequired, insufficientOutputsResponse())
\t\treturn
\t}''',
    '''\tif err := h.requireKOSCHAccess(r.Context(), claims.Sub); err != nil {
\t\twriteJSON(w, http.StatusForbidden, map[string]any{"error": "kosch_holder_required", "message": "Verified KOSCH holder access is required."})
\t\treturn
\t}''',
    'Radar KOSCH guard',
)
text = replace_once(
    text,
    '''\tif err := h.consumePremiumOutput(claims.Sub, claimEmail, "security_radar_check"); err != nil {
\t\twriteJSON(w, http.StatusPaymentRequired, insufficientOutputsResponse())
\t\treturn
\t}
''',
    '',
    'remove no-op output consumption',
)
radar.write_text(text)

graph = ROOT / 'internal/handlers/security_radar_graph.go'
text = graph.read_text()
text = replace_once(
    text,
    '''\tif _, err := h.requirePremiumOutput(claims.Sub, normalizedClaimEmail(claims)); err != nil {''',
    '''\tif err := h.requireKOSCHAccess(r.Context(), claims.Sub); err != nil {''',
    'Graph KOSCH guard',
)
graph.write_text(text)

# Owner users are Neon/app-profile users. Remove local-auth, credits, package and
# entitlement joins from the production owner contract.
(ROOT / 'internal/handlers/owner_users_v2.go').write_text('''package handlers

import (
    "net/http"
    "strings"
)

func (h *Handler) OwnerUsersV2(w http.ResponseWriter, r *http.Request) {
    if !h.ownerAuth(w, r) {
        return
    }
    if err := ensureOwnerSchema(r.Context(), h.DB); err != nil {
        writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "owner schema unavailable"})
        return
    }
    query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
    rows, err := h.DB.QueryContext(r.Context(), `
        SELECT id::text, COALESCE(auth_subject,''), lower(email), COALESCE(wallet_address,''),
               COALESCE(status,'active'), created_at, updated_at, banned_at
        FROM app_user_profiles
        WHERE ($1='' OR lower(email) LIKE $2 OR lower(COALESCE(wallet_address,'')) LIKE $2 OR lower(COALESCE(auth_subject,'')) LIKE $2)
        ORDER BY created_at DESC
        LIMIT 500`, query, "%"+query+"%")
    if err != nil {
        writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db query failed", "message": err.Error()})
        return
    }
    defer rows.Close()
    users := []ownerUserRecord{}
    for rows.Next() {
        var user ownerUserRecord
        if err := rows.Scan(&user.ID, &user.AuthSubject, &user.Email, &user.WalletAddress, &user.Status, &user.CreatedAt, &user.UpdatedAt, &user.BannedAt); err != nil {
            writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db scan failed", "message": err.Error()})
            return
        }
        users = append(users, user)
    }
    writeJSON(w, http.StatusOK, map[string]any{"ok": true, "users": users, "source": "app_user_profiles"})
}
''')

owner = ROOT / 'internal/handlers/owner.go'
text = owner.read_text()
# Production owner user record: identity, wallet and account state only.
record_start = text.index('type ownerUserRecord struct {')
record_end = text.index('\n}\n', record_start) + 3
text = text[:record_start] + '''type ownerUserRecord struct {
\tID            string     `json:"id"`
\tAuthSubject   string     `json:"auth_subject"`
\tEmail         string     `json:"email"`
\tWalletAddress string     `json:"wallet_address,omitempty"`
\tStatus        string     `json:"status"`
\tCreatedAt     time.Time  `json:"created_at"`
\tUpdatedAt     time.Time  `json:"updated_at"`
\tBannedAt      *time.Time `json:"banned_at,omitempty"`
}
''' + text[record_end:]
# Remove legacy input type.
credit_start = text.find('type ownerCreditInput struct {')
if credit_start >= 0:
    credit_end = text.index('\n}\n', credit_start) + 3
    text = text[:credit_start] + text[credit_end:]

for signature in [
    'func (h *Handler) OwnerLogin(',
    'func (h *Handler) OwnerUsers(',
    'func (h *Handler) OwnerAddCredits(',
    'func (h *Handler) OwnerPaymentRequests(',
    'func (h *Handler) OwnerApprovePayment(',
    'func (h *Handler) OwnerRejectPayment(',
    'func (h *Handler) OwnerEmergencyControl(',
    'func (h *Handler) OwnerGrants(',
    'func (h *Handler) ownerAddCreditsCommand(',
    'func (h *Handler) ownerChangePackage(',
    'func (h *Handler) ownerRevenue(',
    'func (h *Handler) ownerPaddleSummary(',
    'func (h *Handler) ownerOpenAISummary(',
    'func intArg(',
    'func lowerStringFromAny(',
]:
    text, _ = remove_go_function(text, signature)

# OwnerStatus is retained as a compatibility endpoint but delegates to the
# current KOSCH-era operations contract.
status_sig = 'func (h *Handler) OwnerStatus('
start = text.find(status_sig)
if start >= 0:
    brace = text.find('{', start)
    old_text, _ = remove_go_function(text, status_sig)
    # remove_go_function removed the whole function; insert the new one at the same location.
    text = old_text[:start] + '''func (h *Handler) OwnerStatus(w http.ResponseWriter, r *http.Request) {
\th.OwnerOperationsStatus(w, r)
}

''' + old_text[start:]

# Remove obsolete owner brain commands.
for old in [
    '''\tcase strings.Contains(lc, "kredi ekle"):
\t\temail := stringArg(args, "email")
\t\tcredits := intArg(args, "credits")
\t\treturn h.ownerAddCreditsCommand(ctx, email, credits)
''',
    '''\tcase strings.Contains(lc, "paketi değiştir") || strings.Contains(lc, "paketi degistir"):
\t\treturn h.ownerChangePackage(ctx, stringArg(args, "email"), stringArg(args, "package"))
''',
    '''\tcase strings.Contains(lc, "bugünkü gelir") || strings.Contains(lc, "bugunku gelir"):
\t\treturn h.ownerRevenue(ctx, "today")
''',
    '''\tcase strings.Contains(lc, "aylık gelir") || strings.Contains(lc, "aylik gelir"):
\t\treturn h.ownerRevenue(ctx, "month")
''',
    '''\tcase strings.Contains(lc, "paddle") && strings.Contains(lc, "webhook"):
\t\treturn h.ownerPaddleSummary(ctx)
''',
    '''\tcase strings.Contains(lc, "openai") && strings.Contains(lc, "maliyet"):
\t\treturn h.ownerOpenAISummary(ctx)
''',
]:
    text = text.replace(old, '')

# Runtime schema repair is limited to live owner data; migrations own all other tables.
old_schema = '''\tstmts := []string{
\t\t`ALTER TABLE app_user_profiles ADD COLUMN IF NOT EXISTS wallet_address text`,
\t\t`ALTER TABLE app_user_profiles ADD COLUMN IF NOT EXISTS status text NOT NULL DEFAULT 'active'`,
\t\t`ALTER TABLE app_user_profiles ADD COLUMN IF NOT EXISTS banned_at timestamptz`,
\t\t`ALTER TABLE app_user_profiles ADD COLUMN IF NOT EXISTS ban_reason text`,
\t\t`CREATE TABLE IF NOT EXISTS credit_events (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), email text, amount integer NOT NULL, reason text, event_type text, created_at timestamptz NOT NULL DEFAULT now())`,
\t\t`CREATE TABLE IF NOT EXISTS ai_command_logs (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), command text NOT NULL, output text NOT NULL DEFAULT '', status text NOT NULL DEFAULT 'queued', created_at timestamptz NOT NULL DEFAULT now())`,
\t\t`CREATE TABLE IF NOT EXISTS owner_system_controls (key text PRIMARY KEY, enabled boolean NOT NULL DEFAULT false, reason text NOT NULL DEFAULT '', updated_at timestamptz NOT NULL DEFAULT now())`,
\t\t`CREATE TABLE IF NOT EXISTS owner_central_brain_events (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), event_type text NOT NULL, message text NOT NULL DEFAULT '', status text NOT NULL DEFAULT 'info', payload jsonb NOT NULL DEFAULT '{}'::jsonb, created_at timestamptz NOT NULL DEFAULT now())`,
\t\t`CREATE TABLE IF NOT EXISTS owner_impersonation_tokens (token text PRIMARY KEY, email text NOT NULL, expires_at timestamptz NOT NULL, used_at timestamptz, created_at timestamptz NOT NULL DEFAULT now())`,
\t\t`CREATE TABLE IF NOT EXISTS system_analytics (day date PRIMARY KEY DEFAULT CURRENT_DATE, active_users integer NOT NULL DEFAULT 0, revenue_try numeric NOT NULL DEFAULT 0, credits_consumed integer NOT NULL DEFAULT 0, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now())`,
\t\t`CREATE TABLE IF NOT EXISTS mev_protection_events (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), user_wallet text NOT NULL DEFAULT '', tx_signature text NOT NULL DEFAULT '', estimated_loss_usd numeric NOT NULL DEFAULT 0, mev_saved_usd numeric NOT NULL DEFAULT 0, jito_tip_used boolean NOT NULL DEFAULT false, risk_score integer NOT NULL DEFAULT 0, risk_level text NOT NULL DEFAULT 'DÜŞÜK', route text NOT NULL DEFAULT '', raw_payload jsonb NOT NULL DEFAULT '{}'::jsonb, created_at timestamptz NOT NULL DEFAULT now())`,
\t\t`ALTER TABLE mev_protection_events ADD COLUMN IF NOT EXISTS mev_saved_usd numeric NOT NULL DEFAULT 0`,
\t\t`ALTER TABLE mev_protection_events ADD COLUMN IF NOT EXISTS raw_payload jsonb NOT NULL DEFAULT '{}'::jsonb`,
\t\t`CREATE TABLE IF NOT EXISTS liquidity_drain_alerts (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), pool_address text NOT NULL DEFAULT '', token_mint text NOT NULL DEFAULT '', severity text NOT NULL DEFAULT 'DÜŞÜK', risk_score integer NOT NULL DEFAULT 0, removed_liquidity_usd numeric NOT NULL DEFAULT 0, loss_prevented_usd numeric NOT NULL DEFAULT 0, telegram_queued boolean NOT NULL DEFAULT false, sms_queued boolean NOT NULL DEFAULT false, created_at timestamptz NOT NULL DEFAULT now())`,
\t\t`CREATE TABLE IF NOT EXISTS proposal_risks (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), dao_id text NOT NULL DEFAULT '', treasury_address text NOT NULL DEFAULT '', proposal_id text NOT NULL DEFAULT '', risk_score integer NOT NULL DEFAULT 0, risk_level text NOT NULL DEFAULT 'DÜŞÜK', estimated_outflow_usd numeric NOT NULL DEFAULT 0, instruction_count integer NOT NULL DEFAULT 0, created_at timestamptz NOT NULL DEFAULT now())`,
\t}'''
new_schema = '''\tstmts := []string{
\t\t`ALTER TABLE app_user_profiles ADD COLUMN IF NOT EXISTS wallet_address text`,
\t\t`ALTER TABLE app_user_profiles ADD COLUMN IF NOT EXISTS status text NOT NULL DEFAULT 'active'`,
\t\t`ALTER TABLE app_user_profiles ADD COLUMN IF NOT EXISTS banned_at timestamptz`,
\t\t`ALTER TABLE app_user_profiles ADD COLUMN IF NOT EXISTS ban_reason text`,
\t\t`CREATE TABLE IF NOT EXISTS ai_command_logs (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), command text NOT NULL, output text NOT NULL DEFAULT '', status text NOT NULL DEFAULT 'queued', created_at timestamptz NOT NULL DEFAULT now())`,
\t\t`CREATE TABLE IF NOT EXISTS owner_system_controls (key text PRIMARY KEY, enabled boolean NOT NULL DEFAULT false, reason text NOT NULL DEFAULT '', updated_at timestamptz NOT NULL DEFAULT now())`,
\t\t`CREATE TABLE IF NOT EXISTS owner_central_brain_events (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), event_type text NOT NULL, message text NOT NULL DEFAULT '', status text NOT NULL DEFAULT 'info', payload jsonb NOT NULL DEFAULT '{}'::jsonb, created_at timestamptz NOT NULL DEFAULT now())`,
\t\t`CREATE TABLE IF NOT EXISTS owner_impersonation_tokens (token text PRIMARY KEY, email text NOT NULL, expires_at timestamptz NOT NULL, used_at timestamptz, created_at timestamptz NOT NULL DEFAULT now())`,
\t}'''
text = replace_once(text, old_schema, new_schema, 'owner live schema')
text = replace_once(text, '\treturn ensurePaymentSchema(ctx, db)\n', '\treturn nil\n', 'remove payment schema chaining')
owner.write_text(text)

# Keep only the current static-route test; remove the obsolete module catalogue.
server_test = ROOT / 'internal/http/server_test.go'
text = server_test.read_text()
text, _ = remove_go_function(text, 'func TestCleanRoutesExposeAllPublicModules(')
text = text.replace('\t"io"\n', '')
server_test.write_text(text)

# Remove the metadata branch from the still-live chains module shell.
premium_js = ROOT / 'public/js/koschei-premium-modules.js'
if premium_js.exists():
    text = premium_js.read_text()
    text = text.replace("  metadata:{title:'Metadata Stüdyosu',badge:'KOSCH HOLDER · PROJE İSTİHBARATI',desc:'Proje ve token metadata taslağını doğrulanabilir kanıtlarla hazırlayın.',endpoint:'/api/metadata/generate',method:'POST',button:'Metadata oluştur',fields:[['asset_name','Proje adı','input'],['description','Açıklama','textarea'],['website','Web sitesi','input']],build:f=>({asset_name:f.asset_name,description:f.description,traits:`website:${f.website}`})}\n", '')
    premium_js.write_text(text)

# Confirmed dead production surfaces and tests. Database migrations are retained
# as immutable history; this cleanup only removes executable/runtime code.
DELETE = [
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
    'internal/handlers/payments.go',
    'internal/handlers/smart_money.go',
    'internal/handlers/intelligence_os.go',
    'internal/handlers/credits.go',
    'internal/handlers/credits_atomic.go',
    'internal/handlers/credits_reservation.go',
    'internal/handlers/enterprise_modules_test.go',
    'internal/handlers/owner_game_studio_ownership_test.go',
    'internal/router/model_router.go',
    'internal/router/ai_router_test.go',
    'public/metadata.html',
    'public/mev-shield.html',
    'public/smart-money.html',
]
for relative in DELETE:
    path = ROOT / relative
    if path.exists():
        path.unlink()
