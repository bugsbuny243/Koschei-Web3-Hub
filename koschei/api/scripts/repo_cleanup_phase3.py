from __future__ import annotations

from pathlib import Path

PATH = Path('internal/handlers/owner.go')


def remove_go_function(text: str, signature: str) -> tuple[str, int]:
    start = text.find(signature)
    if start < 0:
        return text, -1
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
                    return text[:start] + text[end:], start
        i += 1
    raise SystemExit(f'unclosed function: {signature}')


def replace_function(text: str, signature: str, replacement: str) -> str:
    text, start = remove_go_function(text, signature)
    if start < 0:
        raise SystemExit(f'missing function: {signature}')
    return text[:start] + replacement.rstrip() + '\n\n' + text[start:]


text = PATH.read_text()

text = replace_function(text, 'func (h *Handler) OwnerHealth(', '''func (h *Handler) OwnerHealth(w http.ResponseWriter, r *http.Request) {
\th.OwnerOperationsStatus(w, r)
}''')

text = replace_function(text, 'func (h *Handler) routeOwnerBrainCommand(', '''func (h *Handler) routeOwnerBrainCommand(ctx context.Context, message string) (string, map[string]any, string, bool) {
\tcommand := strings.ToLower(strings.TrimSpace(message))
\tcommand = strings.Join(strings.Fields(command), " ")
\tswitch {
\tcase strings.Contains(command, "son 24 saat") && strings.Contains(command, "hata"):
\t\treturn "recent_errors_24h", h.ownerRecentErrors(ctx), "Son 24 saat hata özeti hazır.", true
\tcase strings.HasPrefix(command, "kullanıcı ara") || strings.HasPrefix(command, "kullanici ara"):
\t\temail := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(command, "kullanıcı ara"), "kullanici ara"))
\t\treturn "user_search", h.ownerSearchUser(ctx, email), "Kullanıcı arama sonucu hazır.", true
\tcase (strings.Contains(command, "claude") || strings.Contains(command, "anthropic")) && strings.Contains(command, "durum"):
\t\treturn "owner_ai_status", envConfiguredResult([]string{"ANTHROPIC_OWNER_API_KEY", "ANTHROPIC_API_KEY"}, true), "Owner Claude yapılandırma durumu hazır.", true
\tcase (strings.Contains(command, "qwen") || strings.Contains(command, "together")) && strings.Contains(command, "durum"):
\t\treturn "customer_ai_status", envConfiguredResult([]string{"TOGETHER_API_KEY"}, true), "Müşteri Qwen yapılandırma durumu hazır.", true
\tcase strings.Contains(command, "alchemy") && strings.Contains(command, "durum"):
\t\treturn "solana_rpc_status", envConfiguredResult([]string{"SOLANA_RPC_URL", "ALCHEMY_SOLANA_RPC_URL", "HELIUS_SOLANA_RPC_URL", "QUICKNODE_SOLANA_RPC_URL", "ALCHEMY_API_KEY"}, true), "Solana RPC yapılandırma durumu hazır.", true
\tcase strings.Contains(command, "github") && strings.Contains(command, "durum"):
\t\treturn "github_status", envConfiguredResult([]string{"GITHUB_TOKEN", "GITHUB_REPO"}, false), "GitHub yapılandırma durumu hazır.", true
\tcase strings.Contains(command, "neon") && strings.Contains(command, "durum"):
\t\treturn "neon_status", envConfiguredResult([]string{"DATABASE_URL", "NEON_AUTH_JWKS_URL"}, false), "Neon veritabanı ve Auth durumu hazır.", true
\tdefault:
\t\treturn "unsupported", nil, "", false
\t}
}''')

text = replace_function(text, 'func (h *Handler) ownerRecentErrors(', '''func (h *Handler) ownerRecentErrors(ctx context.Context) map[string]any {
\tresult := map[string]any{}
\tif ownerTableExists(ctx, h.DB, "runtime_logs") {
\t\tresult["runtime_logs"] = ownerQueryRows(ctx, h.DB, `SELECT created_at, level, message FROM runtime_logs WHERE created_at >= now() - interval '24 hours' AND lower(level) IN ('error','fatal','warn','warning') ORDER BY created_at DESC LIMIT 50`, []string{"created_at", "level", "message"})
\t}
\tif ownerTableExists(ctx, h.DB, "model_route_logs") {
\t\tresult["ai_logs"] = ownerQueryRows(ctx, h.DB, `SELECT created_at, provider, model, status, tool FROM model_route_logs WHERE created_at >= now() - interval '24 hours' AND lower(COALESCE(status,'')) NOT IN ('ok','success','completed') ORDER BY created_at DESC LIMIT 50`, []string{"created_at", "provider", "model", "status", "tool"})
\t}
\tif ownerTableExists(ctx, h.DB, "security_audit_events") {
\t\tresult["security_events"] = ownerQueryRows(ctx, h.DB, `SELECT created_at, event_type, severity, path FROM security_audit_events WHERE created_at >= now() - interval '24 hours' AND lower(COALESCE(severity,'')) IN ('warning','error','critical') ORDER BY created_at DESC LIMIT 50`, []string{"created_at", "event_type", "severity", "path"})
\t}
\treturn result
}''')

text = replace_function(text, 'func (h *Handler) ownerSearchUser(', '''func (h *Handler) ownerSearchUser(ctx context.Context, email string) map[string]any {
\tresult := map[string]any{"email": email, "user": nil}
\tif strings.TrimSpace(email) == "" || !ownerTableExists(ctx, h.DB, "app_user_profiles") {
\t\treturn result
\t}
\trows := ownerQueryRows(ctx, h.DB, `SELECT email, COALESCE(auth_subject,''), COALESCE(status,'active'), COALESCE(wallet_address,''), created_at, updated_at FROM app_user_profiles WHERE lower(email)=lower($1) ORDER BY updated_at DESC LIMIT 1`, []string{"email", "auth_subject", "status", "wallet_address", "created_at", "updated_at"}, email)
\tif len(rows) > 0 {
\t\tresult["user"] = rows[0]
\t}
\treturn result
}''')

text, _ = remove_go_function(text, 'func (h *Handler) ownerPendingPayments(')

text = replace_function(text, 'func (h *Handler) ownerFindUser(', '''func (h *Handler) ownerFindUser(ctx context.Context, email string) (string, string, map[string]any) {
\temail = strings.TrimSpace(email)
\tif email == "" {
\t\treturn "error", "Kullanıcı aramak için mail gerekli.", nil
\t}
\trows, err := h.DB.QueryContext(ctx, `SELECT id::text, COALESCE(auth_subject,''), email, COALESCE(wallet_address,''), COALESCE(status,'active'), created_at, updated_at, banned_at FROM app_user_profiles WHERE lower(email) LIKE lower($1) ORDER BY created_at DESC LIMIT 20`, "%"+email+"%")
\tif err != nil {
\t\treturn "error", "Kullanıcı araması başarısız: " + err.Error(), nil
\t}
\tdefer rows.Close()
\tusers := []ownerUserRecord{}
\tfor rows.Next() {
\t\tvar user ownerUserRecord
\t\t_ = rows.Scan(&user.ID, &user.AuthSubject, &user.Email, &user.WalletAddress, &user.Status, &user.CreatedAt, &user.UpdatedAt, &user.BannedAt)
\t\tusers = append(users, user)
\t}
\treturn "completed", fmt.Sprintf("%d kullanıcı bulundu.", len(users)), map[string]any{"users": users}
}''')

text = replace_function(text, 'func (h *Handler) ownerListBannedUsers(', '''func (h *Handler) ownerListBannedUsers(ctx context.Context) (string, string, map[string]any) {
\trows, err := h.DB.QueryContext(ctx, `SELECT id::text, COALESCE(auth_subject,''), email, COALESCE(wallet_address,''), COALESCE(status,'active'), created_at, updated_at, banned_at FROM app_user_profiles WHERE status='banned' ORDER BY banned_at DESC NULLS LAST LIMIT 100`)
\tif err != nil {
\t\treturn "error", "Banlı kullanıcılar okunamadı: " + err.Error(), nil
\t}
\tdefer rows.Close()
\tusers := []ownerUserRecord{}
\tfor rows.Next() {
\t\tvar user ownerUserRecord
\t\t_ = rows.Scan(&user.ID, &user.AuthSubject, &user.Email, &user.WalletAddress, &user.Status, &user.CreatedAt, &user.UpdatedAt, &user.BannedAt)
\t\tusers = append(users, user)
\t}
\treturn "completed", fmt.Sprintf("%d banlı kullanıcı listelendi.", len(users)), map[string]any{"users": users}
}''')

text = replace_function(text, 'func (h *Handler) ownerServiceStatuses(', '''func (h *Handler) ownerServiceStatuses(ctx context.Context) []map[string]any {
\treturn []map[string]any{
\t\t{"name": "OWNER CLAUDE", "status": ownerAnyEnvHealth("ANTHROPIC_OWNER_API_KEY", "ANTHROPIC_API_KEY")},
\t\t{"name": "CUSTOMER QWEN", "status": ownerAnyEnvHealth("TOGETHER_API_KEY")},
\t\t{"name": "GITHUB", "status": ownerGitHubHealth(ctx)},
\t\t{"name": "NEON AUTH", "status": ownerAnyEnvHealth("NEON_AUTH_JWKS_URL")},
\t\t{"name": "DATABASE", "status": ownerDBHealth(ctx, h.DB)},
\t\t{"name": "SOLANA RPC", "status": ownerAnyEnvHealth("SOLANA_RPC_URL", "ALCHEMY_SOLANA_RPC_URL", "HELIUS_SOLANA_RPC_URL", "QUICKNODE_SOLANA_RPC_URL", "ALCHEMY_API_KEY")},
\t\t{"name": "KOSCH ACCESS", "status": ownerAnyEnvHealth("KOSCHEI_TOKEN_MINT", "KOSCH_TOKEN_MINT")},
\t\t{"name": "RENDER", "status": ownerAnyEnvHealth("RENDER_DEPLOY_HOOK_URL")},
\t}
}''')

for signature in [
    'func configuredStatus(',
    'func configuredStatusAny(',
]:
    text, _ = remove_go_function(text, signature)

text += '''
func ownerAnyEnvHealth(keys ...string) map[string]any {
\tfor _, key := range keys {
\t\tif strings.TrimSpace(os.Getenv(key)) != "" {
\t\t\treturn map[string]any{"state": "Connected", "detail": key + " configured"}
\t\t}
\t}
\treturn map[string]any{"state": "Disconnected", "detail": "configuration missing"}
}
'''

PATH.write_text(text)
