package handlers

import (
	"net/http"
	"strings"
	"time"
)

func (h *Handler) OwnerLogout(w http.ResponseWriter, r *http.Request) {
	secure := strings.EqualFold(strings.TrimSpace(firstEnv("APP_ENV")), "production")
	for _, name := range []string{"koschei_owner_secret", "koschei_owner_wallet"} {
		http.SetCookie(w, &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			Secure:   secure,
			SameSite: http.SameSiteStrictMode,
			Expires:  time.Unix(0, 0),
			MaxAge:   -1,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "Owner oturumu kapatıldı."})
}
