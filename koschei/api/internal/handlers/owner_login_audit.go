package handlers

import (
	"net/http"
	"os"
	"strings"
	"time"

	"koschei/api/internal/services"
)

func (h *Handler) OwnerLoginAudited(w http.ResponseWriter, r *http.Request) {
	ownerWallet := normalizeWallet(firstEnv("OWNER_WALLET", "KOSCHEI_OWNER_WALLET"))
	ownerSecret := strings.TrimSpace(firstEnv("OWNER_SECRET", "KOSCHEI_OWNER_SECRET"))
	if ownerSecret == "" {
		services.WriteSecurityAuditEvent(r.Context(), h.DB, securityAuditFromRequest(r, "owner_login_failed", "owner", "error", map[string]any{"reason": "owner_secret_missing"}))
		http.NotFound(w, r)
		return
	}
	var req ownerLoginInput
	if err := decodeJSON(r, &req); err != nil {
		services.WriteSecurityAuditEvent(r.Context(), h.DB, securityAuditFromRequest(r, "owner_login_failed", "owner", "warning", map[string]any{"reason": "invalid_body"}))
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}
	if !constantTimeStringEqual(strings.TrimSpace(req.Secret), ownerSecret) {
		services.WriteSecurityAuditEvent(r.Context(), h.DB, securityAuditFromRequest(r, "owner_login_failed", "owner", "critical", map[string]any{"reason": "failed_secret"}))
		http.NotFound(w, r)
		return
	}
	loginWallet := normalizeWallet(req.Wallet)
	if ownerWallet != "" && loginWallet != "" && loginWallet != ownerWallet {
		services.WriteSecurityAuditEvent(r.Context(), h.DB, securityAuditFromRequest(r, "owner_login_failed", "owner", "critical", map[string]any{"reason": "wallet_mismatch"}))
		http.NotFound(w, r)
		return
	}
	secure := strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production")
	http.SetCookie(w, &http.Cookie{Name: "koschei_owner_secret", Value: ownerSecret, Path: "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteStrictMode, Expires: time.Now().Add(12 * time.Hour)})
	http.SetCookie(w, &http.Cookie{Name: "koschei_owner_wallet", Value: firstNonEmpty(ownerWallet, loginWallet), Path: "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteStrictMode, Expires: time.Now().Add(12 * time.Hour)})
	services.WriteSecurityAuditEvent(r.Context(), h.DB, securityAuditFromRequest(r, "owner_login_success", "owner", "info", map[string]any{"wallet_present": loginWallet != ""}))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "Owner oturumu açıldı."})
}

func securityAuditFromRequest(r *http.Request, eventType, actorType, severity string, metadata map[string]any) services.SecurityAuditEvent {
	return services.SecurityAuditEvent{EventType: eventType, ActorType: actorType, IP: requestAuditIP(r), UserAgent: r.UserAgent(), Path: r.URL.Path, Severity: severity, Metadata: metadata}
}

func requestAuditIP(r *http.Request) string {
	if r == nil {
		return "unknown"
	}
	if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" && len(xff) < 256 {
		first := strings.TrimSpace(strings.Split(xff, ",")[0])
		if first != "" {
			return first
		}
	}
	if strings.TrimSpace(r.RemoteAddr) != "" && len(r.RemoteAddr) < 128 {
		return strings.TrimSpace(r.RemoteAddr)
	}
	return "unknown"
}
