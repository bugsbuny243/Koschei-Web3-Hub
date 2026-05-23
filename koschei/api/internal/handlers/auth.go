package handlers

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net/http"
)

type authUser struct {
	ID, Email, Role, Plan string
	Credits               int
}

type jwtClaims struct {
	Sub, Email, Role, PlanID string
	Exp                      int64
}

type jwksDoc struct {
	Keys []struct {
		Kty string `json:"kty"`
		Kid string `json:"kid"`
		Alg string `json:"alg"`
		Use string `json:"use"`
		N   string `json:"n"`
		E   string `json:"e"`
	} `json:"keys"`
}

func (h *Handler) Register(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusGone, map[string]string{"error": "custom_auth_disabled", "message": "Use Neon Auth email sign-in"})
}

func (h *Handler) Login(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusGone, map[string]string{"error": "custom_auth_disabled", "message": "Use Neon Auth email sign-in"})
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	if err := h.dbAvailable(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database unavailable"})
		return
	}
	claims, ok := userFromContext(r.Context())
	if !ok || strings.TrimSpace(claims.Email) == "" {
		writeJSON(w, 401, map[string]string{"error": "unauthorized"})
		return
	}
	user, err := h.upsertAppProfile(r.Context(), claims.Sub, claims.Email)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "db failed"})
		return
	}
	writeJSON(w, 200, map[string]any{"user": user})
}

func (h *Handler) upsertAppProfile(ctx context.Context, subject, email string) (authUser, error) {
	out := authUser{}
	q := `INSERT INTO app_user_profiles (auth_subject, email)
VALUES ($1, $2)
ON CONFLICT (auth_subject) DO UPDATE SET email=EXCLUDED.email, updated_at=now()
RETURNING id::text, email, role, plan_id, credits`
	err := h.runWithRetry(ctx, func(inner context.Context) error {
		return h.DB.QueryRowContext(inner, q, subject, strings.ToLower(strings.TrimSpace(email))).Scan(&out.ID, &out.Email, &out.Role, &out.Plan, &out.Credits)
	})
	return out, err
}

func parseAndVerifyNeonJWT(ctx context.Context, token string) (jwtClaims, error) {
	var out jwtClaims
	jwksURL := strings.TrimSpace(os.Getenv("NEON_AUTH_JWKS_URL"))
	if jwksURL == "" {
		return out, errors.New("jwks missing")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return out, errors.New("invalid token")
	}
	var hdr map[string]any
	if err := decodeSegment(parts[0], &hdr); err != nil {
		return out, err
	}
	kid, _ := hdr["kid"].(string)
	if kid == "" {
		return out, errors.New("missing kid")
	}
	var claims map[string]any
	if err := decodeSegment(parts[1], &claims); err != nil {
		return out, err
	}
	issExpected := strings.TrimSpace(os.Getenv("NEON_AUTH_ISSUER"))
	if issExpected != "" {
		iss, _ := claims["iss"].(string)
		if iss != issExpected {
			return out, errors.New("invalid issuer")
		}
	}
	exp, ok := claims["exp"].(float64)
	if !ok || int64(exp) < time.Now().Unix() {
		return out, errors.New("expired")
	}
	if err := verifyRS256WithJWKS(ctx, token, kid, jwksURL); err != nil {
		log.Printf("auth verify failed")
		return out, err
	}
	out.Sub, _ = claims["sub"].(string)
	out.Email, _ = claims["email"].(string)
	out.Role = "user"
	out.PlanID = "free"
	out.Exp = int64(exp)
	if out.Sub == "" || out.Email == "" {
		return out, errors.New("missing claims")
	}
	return out, nil
}

func verifyRS256WithJWKS(ctx context.Context, token, kid, jwksURL string) error {
	// minimalist verification via stdlib jwt parsing
	pub, err := fetchJWKSKey(ctx, jwksURL, kid)
	if err != nil {
		return err
	}
	return verifyJWTSignatureRS256(token, pub)
}

func decodeSegment(seg string, out any) error { return decodeBase64JSON(seg, out) }

func fetchJWKSKey(ctx context.Context, url, kid string) (*rsa.PublicKey, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		return nil, fmt.Errorf("jwks unavailable")
	}
	var doc jwksDoc
	if err := json.NewDecoder(res.Body).Decode(&doc); err != nil {
		return nil, err
	}
	for _, k := range doc.Keys {
		if k.Kid != kid || k.Kty != "RSA" {
			continue
		}
		n := new(big.Int)
		nb, err := decodeBase64Raw(k.N)
		if err != nil {
			return nil, err
		}
		n.SetBytes(nb)
		eb, err := decodeBase64Raw(k.E)
		if err != nil {
			return nil, err
		}
		e := 0
		for _, b := range eb {
			e = e<<8 + int(b)
		}
		return &rsa.PublicKey{N: n, E: e}, nil
	}
	return nil, errors.New("kid not found")
}

func (h *Handler) runWithRetry(ctx context.Context, op func(context.Context) error) error {
	err := op(ctx)
	if !isTransientDBError(err) {
		return err
	}
	_ = h.dbAvailable(ctx)
	return op(ctx)
}
