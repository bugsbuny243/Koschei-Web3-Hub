package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
)

func (handler *Handler) Web3Config(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	WriteJSON(w, http.StatusOK, map[string]string{
		"neonAuthUrl": os.Getenv("EXPO_PUBLIC_NEON_AUTH_URL"),
		"version":     "2.0.0",
	})
}

func (handler *Handler) Web3Provision(w http.ResponseWriter, r *http.Request) {
	claims, err := handler.verifyJWT(r.Context(), extractBearerToken(r))
	if err != nil {
		WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	if handler.database != nil {
		_ = handler.provisionFreeEntitlement(r, claims.Subject)
	}
	WriteJSON(w, http.StatusOK, map[string]string{
		"ok":    "true",
		"sub":   claims.Subject,
		"email": claims.Email,
		"plan":  "free",
	})
}

func (handler *Handler) provisionFreeEntitlement(r *http.Request, subject string) error {
	database, err := url.Parse(os.Getenv("DATABASE_URL"))
	if err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]any{
		"query": `INSERT INTO entitlements (member_sub, plan_id, credits_remaining)
			VALUES ($1, 'free', 100) ON CONFLICT (member_sub) DO NOTHING`,
		"params": []string{subject},
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, "https://"+database.Hostname()+"/sql", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Neon-Connection-String", database.String())
	res, err := handler.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	return nil
}

func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if len(auth) > 7 && auth[:7] == "Bearer " {
		return auth[7:]
	}
	return ""
}
