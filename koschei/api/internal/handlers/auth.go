package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"
)

type authReq struct{ Email, Password string }
type authUser struct {
	ID, Email, Role, Plan string
	Credits               int
}
type jwtClaims struct {
	Sub, Email, Role, PlanID string
	Exp                      int64
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	if !h.requireJWTConfigured(w) {
		return
	}
	if err := h.dbAvailable(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database unavailable", "details": err.Error()})
		return
	}
	var req authReq
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid body"})
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if !validEmail(email) || len(req.Password) < 8 {
		writeJSON(w, 400, map[string]string{"error": "invalid body"})
		return
	}
	var id, role, planID string
	query := `INSERT INTO auth_accounts (email,password_hash,role,plan_id,is_active) VALUES ($1, crypt($2, gen_salt('bf')),'user','free',true) RETURNING id,role,plan_id`
	err := h.runWithRetry(r.Context(), func(ctx context.Context) error {
		return h.DB.QueryRowContext(ctx, query, email, req.Password).Scan(&id, &role, &planID)
	})
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") || strings.Contains(strings.ToLower(err.Error()), "unique") {
			writeJSON(w, 409, map[string]string{"error": "account exists"})
			return
		}
		writeJSON(w, 500, map[string]string{"error": "db failed"})
		return
	}
	token, err := issueJWT(jwtClaims{Sub: id, Email: email, Role: role, PlanID: planID})
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "config error", "details": err.Error()})
		return
	}
	writeJSON(w, 201, map[string]any{"token": token, "user": authUser{ID: id, Email: email, Role: role, Plan: planID, Credits: 0}})
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	if !h.requireJWTConfigured(w) {
		return
	}
	if err := h.dbAvailable(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database unavailable", "details": err.Error()})
		return
	}
	var req authReq
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid body"})
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if !validEmail(email) || req.Password == "" {
		writeJSON(w, 400, map[string]string{"error": "invalid body"})
		return
	}
	var id, role, plan string
	query := `SELECT id,role,plan_id FROM auth_accounts WHERE lower(email)=lower($1) AND is_active=true AND password_hash = crypt($2, password_hash)`
	err := h.runWithRetry(r.Context(), func(ctx context.Context) error {
		return h.DB.QueryRowContext(ctx, query, email, req.Password).Scan(&id, &role, &plan)
	})
	if err == sql.ErrNoRows {
		writeJSON(w, 401, map[string]string{"error": "invalid credentials"})
		return
	}
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "db failed"})
		return
	}
	token, err := issueJWT(jwtClaims{Sub: id, Email: email, Role: role, PlanID: plan})
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "config error", "details": err.Error()})
		return
	}
	credits, _ := h.userCredits(id)
	writeJSON(w, 200, map[string]any{"token": token, "user": authUser{ID: id, Email: email, Role: role, Plan: plan, Credits: credits}})
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	if err := h.dbAvailable(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database unavailable", "details": err.Error()})
		return
	}
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, 401, map[string]string{"error": "unauthorized"})
		return
	}
	credits, _ := h.userCredits(claims.Sub)
	writeJSON(w, 200, map[string]any{"user": authUser{ID: claims.Sub, Email: claims.Email, Role: claims.Role, Plan: claims.PlanID, Credits: credits}})
}

func (h *Handler) runWithRetry(ctx context.Context, op func(context.Context) error) error {
	err := op(ctx)
	if !isTransientDBError(err) {
		return err
	}
	_ = h.dbAvailable(ctx)
	return op(ctx)
}

func (h *Handler) userCredits(userID string) (int, error) {
	var credits int
	query := `SELECT COALESCE(SUM(cl.amount),0) FROM credits_ledger cl JOIN auth_accounts a ON lower(cl.email)=lower(a.email) WHERE a.id=$1`
	err := h.runWithRetry(context.Background(), func(ctx context.Context) error {
		return h.DB.QueryRowContext(ctx, query, userID).Scan(&credits)
	})
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return credits, err
}

// jwt helpers unchanged
func (h *Handler) requireJWTConfigured(w http.ResponseWriter) bool {
	if strings.TrimSpace(os.Getenv("JWT_SECRET")) == "" {
		writeJSON(w, 500, map[string]string{"error": "config error", "details": "JWT_SECRET is not set"})
		return false
	}
	return true
}
func issueJWT(c jwtClaims) (string, error) {
	secret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if secret == "" {
		return "", errors.New("JWT_SECRET is not set")
	}
	if c.Exp == 0 {
		c.Exp = time.Now().Add(7 * 24 * time.Hour).Unix()
	}
	head := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	pb, _ := json.Marshal(map[string]any{"sub": c.Sub, "email": c.Email, "role": c.Role, "plan_id": c.PlanID, "exp": c.Exp})
	payload := base64.RawURLEncoding.EncodeToString(pb)
	signing := head + "." + payload
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signing))
	return signing + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}
func parseJWT(token string) (jwtClaims, error) {
	var out jwtClaims
	secret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if secret == "" {
		return out, errors.New("JWT_SECRET is not set")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return out, errors.New("invalid token")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(parts[0] + "." + parts[1]))
	if !hmac.Equal([]byte(base64.RawURLEncoding.EncodeToString(mac.Sum(nil))), []byte(parts[2])) {
		return out, errors.New("invalid token")
	}
	pb, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return out, err
	}
	var claims map[string]any
	if err := json.Unmarshal(pb, &claims); err != nil {
		return out, err
	}
	out.Sub, _ = claims["sub"].(string)
	out.Email, _ = claims["email"].(string)
	out.Role, _ = claims["role"].(string)
	out.PlanID, _ = claims["plan_id"].(string)
	if exp, ok := claims["exp"].(float64); ok {
		out.Exp = int64(exp)
	}
	if out.Sub == "" || out.Exp < time.Now().Unix() {
		return out, errors.New("expired or invalid token")
	}
	return out, nil
}
