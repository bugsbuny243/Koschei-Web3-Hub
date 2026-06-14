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
	"log"
	"math/big"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

type neonJWTClaims struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	Wallet        string `json:"wallet"`
	WalletAddress string `json:"wallet_address"`
	PublicAddress string `json:"public_address"`
	Iss           string `json:"iss"`
	Aud           any    `json:"aud"`
	Exp           int64  `json:"exp"`
}

type userProfile struct {
	ID, AuthSubject, Email, Role, PlanID string
	Credits                              int
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

type neonJWTFailureCategory string

const (
	neonJWTFailureNonJWTToken       neonJWTFailureCategory = "non_jwt_token"
	neonJWTFailureMissingKID        neonJWTFailureCategory = "missing_kid"
	neonJWTFailureJWKSKeyNotFound   neonJWTFailureCategory = "jwks_key_not_found"
	neonJWTFailureIssuerMismatch    neonJWTFailureCategory = "issuer_mismatch"
	neonJWTFailureExpiredToken      neonJWTFailureCategory = "expired_token"
	neonJWTFailureMissingEmailClaim neonJWTFailureCategory = "missing_email_claim"
	neonJWTFailureInvalidSignature  neonJWTFailureCategory = "invalid_signature"
	neonJWTFailureUnknown           neonJWTFailureCategory = "unknown"
)

type neonJWTVerificationError struct {
	Category neonJWTFailureCategory
	Cause    error
}

func (e neonJWTVerificationError) Error() string {
	if e.Cause == nil {
		return string(e.Category)
	}
	return string(e.Category) + ": " + e.Cause.Error()
}

func (e neonJWTVerificationError) Unwrap() error { return e.Cause }

var (
	jwksCache map[string]neonJWK
	jwksMu    sync.RWMutex
)

func extractAuthToken(resp *http.Response, body []byte) (string, bool) {
	if resp != nil {
		for _, header := range []string{"set-auth-jwt", "authorization", "x-auth-token"} {
			if token := authTokenFromString(resp.Header.Get(header)); token != "" {
				return token, true
			}
		}
	}
	var data map[string]any
	if len(body) == 0 || json.Unmarshal(body, &data) != nil {
		return "", false
	}
	paths := [][]string{{"token"}, {"jwt"}, {"access_token"}, {"id_token"}, {"auth_token"}, {"data", "token"}, {"data", "jwt"}, {"data", "access_token"}, {"data", "id_token"}, {"session", "token"}, {"session", "jwt"}, {"session", "access_token"}, {"session", "id_token"}}
	for _, path := range paths {
		if token := authTokenAtPath(data, path); token != "" {
			return token, true
		}
	}
	return "", false
}

func authTokenFromString(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(value), "bearer ") {
		value = strings.TrimSpace(value[7:])
	}
	if tokenLooksLikeJWT(value) {
		return value
	}
	return ""
}

func authTokenAtPath(data map[string]any, path []string) string {
	var current any = data
	for _, key := range path {
		object, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current = object[key]
	}
	if value, ok := current.(string); ok {
		return authTokenFromString(value)
	}
	return ""
}

func safeAuthDebugLog(endpoint string, status int, body []byte, cookies []*http.Cookie, tokenFound bool, tokenVerified bool) {
	log.Printf("neon_auth endpoint=%s status=%d json_keys=%v token_found=%t token_verified=%t cookie_names=%v", endpoint, status, topLevelJSONKeys(body), tokenFound, tokenVerified, cookieNames(cookies))
}

func topLevelJSONKeys(body []byte) []string {
	var data map[string]any
	if len(body) == 0 || json.Unmarshal(body, &data) != nil {
		return nil
	}
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func cookieNames(cookies []*http.Cookie) []string {
	if len(cookies) == 0 {
		return nil
	}
	names := make([]string, 0, len(cookies))
	for _, cookie := range cookies {
		if cookie != nil && strings.TrimSpace(cookie.Name) != "" {
			names = append(names, cookie.Name)
		}
	}
	sort.Strings(names)
	return names
}

func parseAndVerifyNeonJWT(token string) (neonJWTClaims, error) {
	if claims, ok, err := tryLocalJWT(token); ok {
		return claims, err
	}
	return neonClaimsFromToken(token)
}

func trimSlash(s string) string { return strings.TrimRight(strings.TrimSpace(s), "/") }

func configuredIssuer() string { return trimSlash(configuredNeonAuthIssuer()) }

func jwtVerifyError(category neonJWTFailureCategory, err error) error {
	return neonJWTVerificationError{Category: category, Cause: err}
}

func neonClaimsFromToken(token string) (neonJWTClaims, error) {
	var out neonJWTClaims
	if !tokenLooksLikeJWT(token) {
		return out, jwtVerifyError(neonJWTFailureNonJWTToken, errors.New("token is not three JWT segments"))
	}
	parts := strings.Split(token, ".")
	headerRaw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return out, jwtVerifyError(neonJWTFailureUnknown, err)
	}
	var header map[string]any
	if err := json.Unmarshal(headerRaw, &header); err != nil {
		return out, jwtVerifyError(neonJWTFailureUnknown, err)
	}
	kid, _ := header["kid"].(string)
	alg, _ := header["alg"].(string)
	if strings.TrimSpace(kid) == "" {
		return out, jwtVerifyError(neonJWTFailureMissingKID, errors.New("JWT header missing kid"))
	}
	if strings.TrimSpace(alg) == "" {
		return out, jwtVerifyError(neonJWTFailureUnknown, errors.New("JWT header missing alg"))
	}
	jwk, err := loadJWKSKey(kid)
	if err != nil {
		return out, jwtVerifyError(neonJWTFailureJWKSKeyNotFound, err)
	}
	signingInput := []byte(parts[0] + "." + parts[1])
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return out, jwtVerifyError(neonJWTFailureInvalidSignature, err)
	}
	switch alg {
	case "RS256":
		pub, err := rsaPublicKeyFromJWK(jwk)
		if err != nil {
			return out, jwtVerifyError(neonJWTFailureJWKSKeyNotFound, err)
		}
		h := sha256.Sum256(signingInput)
		if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, h[:], sig); err != nil {
			return out, jwtVerifyError(neonJWTFailureInvalidSignature, err)
		}
	case "EdDSA":
		pub, err := ed25519PublicKeyFromJWK(jwk)
		if err != nil {
			return out, jwtVerifyError(neonJWTFailureJWKSKeyNotFound, err)
		}
		if !ed25519.Verify(pub, signingInput, sig) {
			return out, jwtVerifyError(neonJWTFailureInvalidSignature, errors.New("signature verification failed"))
		}
	default:
		return out, jwtVerifyError(neonJWTFailureInvalidSignature, errors.New("unsupported JWT signing algorithm"))
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return out, jwtVerifyError(neonJWTFailureUnknown, err)
	}
	if err := json.Unmarshal(payload, &out); err != nil {
		return out, jwtVerifyError(neonJWTFailureUnknown, err)
	}
	if out.Exp < time.Now().Unix() {
		return out, jwtVerifyError(neonJWTFailureExpiredToken, errors.New("JWT is expired"))
	}
	if strings.TrimSpace(out.Email) == "" {
		return out, jwtVerifyError(neonJWTFailureMissingEmailClaim, errors.New("JWT missing email claim"))
	}
	if strings.TrimSpace(out.Sub) == "" {
		return out, jwtVerifyError(neonJWTFailureUnknown, errors.New("JWT missing sub claim"))
	}
	if issuer := configuredIssuer(); issuer != "" && trimSlash(out.Iss) != issuer {
		return out, jwtVerifyError(neonJWTFailureIssuerMismatch, errors.New("JWT issuer does not match NEON_AUTH_ISSUER"))
	}
	aud := trimSlash(os.Getenv("NEON_AUTH_AUDIENCE"))
	if aud != "" && !matchesAudience(out.Aud, aud) {
		return out, jwtVerifyError(neonJWTFailureUnknown, errors.New("invalid audience"))
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
	jwksURL := strings.TrimSpace(configuredNeonAuthJWKSURL())
	if jwksURL == "" && !isProduction() {
		baseURL := strings.TrimSpace(configuredNeonAuthBaseURL())
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
	profile, err := h.upsertAppProfile(ctx, subject, email)
	out := userProfile{ID: profile.ID, AuthSubject: profile.AuthSubject, Email: profile.Email, Role: profile.Role, PlanID: profile.Plan, Credits: profile.Credits}
	if err != nil {
		log.Printf("upsertProfile failed: %v", err)
	}
	return out, err
}
