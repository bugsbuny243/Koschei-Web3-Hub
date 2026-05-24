package handlers

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

type neonJWTClaims struct {
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
type neonJWKSDoc struct {
	Keys []neonJWK `json:"keys"`
}
type neonJWK struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
	Crv string `json:"crv"`
	X   string `json:"x"`
}

var (
	jwksCache map[string]neonJWK
	jwksMu    sync.RWMutex
)

func parseAndVerifyNeonJWT(token string) (neonJWTClaims, error) {
	return neonClaimsFromToken(token)
}

func originOnly(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return raw
	}
	return u.Scheme + "://" + u.Host
}

func trimSlash(s string) string {
	return strings.TrimRight(strings.TrimSpace(s), "/")
}

func addIssuerCandidate(set map[string]bool, raw string) {
	raw = trimSlash(raw)
	if raw == "" {
		return
	}
	set[raw] = true

	origin := originOnly(raw)
	if origin != "" {
		set[trimSlash(origin)] = true
	}
}

func allowedIssuers() map[string]bool {
	allowed := map[string]bool{}
	addIssuerCandidate(allowed, os.Getenv("NEON_AUTH_ISSUER"))
	addIssuerCandidate(allowed, os.Getenv("NEON_AUTH_BASE_URL"))
	return allowed
}

func neonClaimsFromToken(token string) (neonJWTClaims, error) {
	var out neonJWTClaims
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
	if kid == "" || alg == "" {
		return out, errors.New("invalid token")
	}
	jwk, err := loadJWKSKey(kid)
	if err != nil {
		return out, err
	}
	signingInput := []byte(parts[0] + "." + parts[1])
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return out, err
	}
	switch alg {
	case "RS256":
		pub, err := rsaPublicKeyFromJWK(jwk)
		if err != nil {
			return out, errors.New("invalid token")
		}
		h := sha256.Sum256(signingInput)
		if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, h[:], sig); err != nil {
			return out, errors.New("invalid token")
		}
	case "EdDSA":
		pub, err := ed25519PublicKeyFromJWK(jwk)
		if err != nil {
			return out, errors.New("invalid token")
		}
		if !ed25519.Verify(pub, signingInput, sig) {
			return out, errors.New("invalid token")
		}
	default:
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
	allowed := allowedIssuers()
	actualIssuer := trimSlash(out.Iss)
	if len(allowed) > 0 && !allowed[actualIssuer] {
		return out, errors.New("invalid issuer: " + actualIssuer)
	}
	aud := trimSlash(os.Getenv("NEON_AUTH_AUDIENCE"))
	if aud != "" && !matchesAudience(out.Aud, aud) {
		return out, errors.New("invalid audience")
	}
	return out, nil
}

func loadJWKSKey(kid string) (neonJWK, error) {
	jwksMu.RLock()
	if k, ok := jwksCache[kid]; ok {
		jwksMu.RUnlock()
		return k, nil
	}
	jwksMu.RUnlock()
	jwksURL := strings.TrimSpace(os.Getenv("NEON_AUTH_JWKS_URL"))
	if jwksURL == "" {
		baseURL := strings.TrimSpace(os.Getenv("NEON_AUTH_BASE_URL"))
		if baseURL != "" {
			jwksURL = strings.TrimRight(baseURL, "/") + "/.well-known/jwks.json"
		}
	}
	if jwksURL == "" {
		return neonJWK{}, errors.New("jwks missing")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, jwksURL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return neonJWK{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return neonJWK{}, errors.New("jwks unavailable")
	}
	var doc neonJWKSDoc
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return neonJWK{}, err
	}
	cache := map[string]neonJWK{}
	for _, key := range doc.Keys {
		if key.Kid == "" {
			continue
		}
		cache[key.Kid] = key
	}
	jwksMu.Lock()
	jwksCache = cache
	k := jwksCache[kid]
	jwksMu.Unlock()
	if k.Kid == "" {
		return neonJWK{}, errors.New("unknown key")
	}
	return k, nil
}

func rsaPublicKeyFromJWK(key neonJWK) (*rsa.PublicKey, error) {
	if key.Kty != "RSA" || key.N == "" || key.E == "" {
		return nil, errors.New("invalid rsa jwk")
	}
	nBytes, errN := base64.RawURLEncoding.DecodeString(key.N)
	eBytes, errE := base64.RawURLEncoding.DecodeString(key.E)
	if errN != nil || errE != nil {
		return nil, errors.New("invalid rsa jwk")
	}
	eInt := 0
	for _, b := range eBytes {
		eInt = eInt<<8 + int(b)
	}
	if eInt <= 0 {
		return nil, errors.New("invalid rsa jwk")
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(nBytes), E: eInt}, nil
}

func ed25519PublicKeyFromJWK(key neonJWK) (ed25519.PublicKey, error) {
	if key.Kty != "OKP" || key.Crv != "Ed25519" || key.X == "" {
		return nil, errors.New("invalid eddsa jwk")
	}
	xBytes, err := base64.RawURLEncoding.DecodeString(key.X)
	if err != nil || len(xBytes) != ed25519.PublicKeySize {
		return nil, errors.New("invalid eddsa jwk")
	}
	return ed25519.PublicKey(xBytes), nil
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
