package handlers

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type jwtClaims struct {
	Sub   string `json:"sub"`
	Email string `json:"email"`
	Iss   string `json:"iss"`
	Aud   any    `json:"aud"`
	Exp   int64  `json:"exp"`
}

type userProfile struct {
	ID, Email, Role, PlanID string
	Credits                 int
}
type jwksDoc struct {
	Keys []jwk `json:"keys"`
}
type jwk struct {
	Kid, Kty, N, E string `json:"kid","kty","n","e"`
}

var (
	jwksCache map[string]*rsa.PublicKey
	jwksMu    sync.RWMutex
)

func neonClaimsFromToken(token string) (jwtClaims, error) {
	var out jwtClaims
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return out, errors.New("invalid token")
	}
	headerRaw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return out, err
	}
	var header map[string]any
	if err := json.Unmarshal(headerRaw, &header); err != nil {
		return out, err
	}
	kid, _ := header["kid"].(string)
	alg, _ := header["alg"].(string)
	if kid == "" || alg != "RS256" {
		return out, errors.New("invalid token")
	}
	pub, err := loadJWKSPublicKey(kid)
	if err != nil {
		return out, err
	}
	h := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return out, err
	}
	if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, h[:], sig); err != nil {
		return out, errors.New("invalid token")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(payload, &out); err != nil {
		return out, err
	}
	if out.Exp < time.Now().Unix() || out.Sub == "" || out.Email == "" {
		return out, errors.New("invalid token")
	}
	if iss := strings.TrimSpace(os.Getenv("NEON_AUTH_ISSUER")); iss != "" && out.Iss != iss {
		return out, errors.New("invalid token")
	}
	if aud := strings.TrimSpace(os.Getenv("NEON_AUTH_AUDIENCE")); aud != "" && !matchesAudience(out.Aud, aud) {
		return out, errors.New("invalid token")
	}
	return out, nil
}

func loadJWKSPublicKey(kid string) (*rsa.PublicKey, error) {
	jwksMu.RLock()
	if k, ok := jwksCache[kid]; ok {
		jwksMu.RUnlock()
		return k, nil
	}
	jwksMu.RUnlock()
	jwksURL := strings.TrimSpace(os.Getenv("NEON_AUTH_JWKS_URL"))
	if jwksURL == "" {
		return nil, errors.New("jwks missing")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, jwksURL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, errors.New("jwks unavailable")
	}
	var doc jwksDoc
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return nil, err
	}
	cache := map[string]*rsa.PublicKey{}
	for _, key := range doc.Keys {
		if key.Kty != "RSA" || key.Kid == "" {
			continue
		}
		nBytes, errN := base64.RawURLEncoding.DecodeString(key.N)
		eBytes, errE := base64.RawURLEncoding.DecodeString(key.E)
		if errN != nil || errE != nil {
			continue
		}
		eInt := 0
		for _, b := range eBytes {
			eInt = eInt<<8 + int(b)
		}
		cache[key.Kid] = &rsa.PublicKey{N: new(big.Int).SetBytes(nBytes), E: eInt}
	}
	jwksMu.Lock()
	jwksCache = cache
	k := jwksCache[kid]
	jwksMu.Unlock()
	if k == nil {
		return nil, errors.New("unknown key")
	}
	return k, nil
}

func matchesAudience(v any, target string) bool {
	if s, ok := v.(string); ok {
		return s == target
	}
	if arr, ok := v.([]any); ok {
		for _, it := range arr {
			if s, ok := it.(string); ok && s == target {
				return true
			}
		}
	}
	return false
}

func (h *Handler) upsertProfile(ctx context.Context, subject, email string) (userProfile, error) {
	var out userProfile
	q := `INSERT INTO app_user_profiles (auth_subject, email) VALUES ($1, $2)
ON CONFLICT (auth_subject) DO UPDATE SET email=EXCLUDED.email, updated_at=now()
RETURNING auth_subject, email, role, plan_id, credits`
	err := h.runWithRetry(ctx, func(c context.Context) error {
		return h.DB.QueryRowContext(c, q, subject, strings.ToLower(email)).Scan(&out.ID, &out.Email, &out.Role, &out.PlanID, &out.Credits)
	})
	return out, err
}
