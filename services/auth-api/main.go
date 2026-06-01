package main

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	memberCookieName = "koschei_member_session"
	memberSessionTTL = 7 * 24 * time.Hour
)

type server struct {
	client      *http.Client
	authBaseURL string
	jwksURL     string
	issuer      string
	databaseURL string
	secret      []byte
	secure      bool
}

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type identity struct {
	Subject string
	Email   string
}

type memberSession struct {
	Sub       string `json:"sub"`
	Email     string `json:"email"`
	ExpiresAt int64  `json:"expiresAt"`
}

type jwtHeader struct {
	Algorithm string `json:"alg"`
	KeyID     string `json:"kid"`
}

type jwtClaims struct {
	Subject   string      `json:"sub"`
	Email     string      `json:"email"`
	Issuer    string      `json:"iss"`
	ExpiresAt json.Number `json:"exp"`
	NotBefore json.Number `json:"nbf"`
}

type jwks struct {
	Keys []jwk `json:"keys"`
}

type jwk struct {
	KeyID     string   `json:"kid"`
	KeyType   string   `json:"kty"`
	Algorithm string   `json:"alg"`
	Use       string   `json:"use"`
	N         string   `json:"n"`
	E         string   `json:"e"`
	X         string   `json:"x"`
	Curve     string   `json:"crv"`
	X509      []string `json:"x5c"`
}

func main() {
	s, err := newServer()
	if err != nil {
		log.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("POST /auth/signup", s.signup)
	mux.HandleFunc("POST /auth/login", s.login)
	mux.HandleFunc("GET /auth/me", s.me)
	mux.HandleFunc("POST /auth/logout", s.logout)
	addr := envOr("AUTH_API_ADDR", ":8080")
	log.Printf("auth-api listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func newServer() (*server, error) {
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("NEON_AUTH_BASE_URL")), "/")
	jwksURL := strings.TrimSpace(os.Getenv("NEON_AUTH_JWKS_URL"))
	secret := strings.TrimSpace(os.Getenv("USER_SESSION_SECRET"))
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if base == "" || jwksURL == "" || secret == "" || databaseURL == "" {
		return nil, errors.New("NEON_AUTH_BASE_URL, NEON_AUTH_JWKS_URL, DATABASE_URL, and USER_SESSION_SECRET are required")
	}
	issuer, err := normalizedIssuer(strings.TrimSpace(os.Getenv("NEON_AUTH_ISSUER")), base, jwksURL)
	if err != nil {
		return nil, err
	}
	return &server{
		client: &http.Client{Timeout: 12 * time.Second}, authBaseURL: base, jwksURL: jwksURL,
		issuer: issuer, databaseURL: databaseURL, secret: []byte(secret), secure: envOr("APP_ENV", "development") == "production",
	}, nil
}

func (s *server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "service": "auth-api"})
}

func (s *server) signup(w http.ResponseWriter, r *http.Request) {
	s.authenticate(w, r, "sign-up/email")
}
func (s *server) login(w http.ResponseWriter, r *http.Request) { s.authenticate(w, r, "sign-in/email") }

func (s *server) authenticate(w http.ResponseWriter, r *http.Request, authPath string) {
	var input credentials
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON body.")
		return
	}
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	if !strings.Contains(input.Email, "@") || len(input.Password) < 8 || len(input.Password) > 128 {
		writeError(w, http.StatusBadRequest, "A valid email and password are required.")
		return
	}
	token, status, err := s.requestNeonAuth(r.Context(), authPath, input)
	if err != nil {
		log.Printf("neon auth %s failed: %v", authPath, err)
		if status >= 400 && status < 500 {
			writeError(w, http.StatusUnauthorized, "Invalid email or password.")
			return
		}
		writeError(w, http.StatusBadGateway, "Auth provider request failed.")
		return
	}
	member, err := s.verifyJWT(r.Context(), token)
	if err != nil {
		log.Printf("JWT verification failed: %v", err)
		writeError(w, http.StatusBadGateway, "Auth token verification failed.")
		return
	}
	if err := s.upsertProfile(r.Context(), member); err != nil {
		log.Printf("profile upsert failed: %v", err)
		writeError(w, http.StatusServiceUnavailable, "Could not create user profile.")
		return
	}
	s.setMemberCookie(w, member)
	writeJSON(w, http.StatusOK, map[string]any{"email": member.Email})
}

func (s *server) me(w http.ResponseWriter, r *http.Request) {
	member, err := s.readMemberCookie(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"loggedIn": true, "email": member.Email})
}

func (s *server) logout(w http.ResponseWriter, _ *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: memberCookieName, Value: "", Path: "/", HttpOnly: true, Secure: s.secure, SameSite: http.SameSiteLaxMode, MaxAge: -1, Expires: time.Unix(1, 0)})
	writeJSON(w, http.StatusOK, map[string]any{"loggedIn": false})
}

func (s *server) requestNeonAuth(ctx context.Context, authPath string, input credentials) (string, int, error) {
	body, _ := json.Marshal(input)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.authBaseURL+"/"+authPath, strings.NewReader(string(body)))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := s.client.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer res.Body.Close()
	var payload any
	if err := json.NewDecoder(io.LimitReader(res.Body, 2<<20)).Decode(&payload); err != nil && res.StatusCode >= 200 && res.StatusCode < 300 {
		return "", res.StatusCode, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return "", res.StatusCode, fmt.Errorf("provider returned %d", res.StatusCode)
	}
	if token := strings.TrimSpace(res.Header.Get("set-auth-jwt")); token != "" {
		return bearerToken(token), res.StatusCode, nil
	}
	if token := findToken(payload); token != "" {
		return bearerToken(token), res.StatusCode, nil
	}
	return "", res.StatusCode, errors.New("provider did not return a token")
}

func findToken(value any) string {
	switch typed := value.(type) {
	case map[string]any:
		for _, key := range []string{"token", "access_token", "accessToken", "jwt", "id_token", "idToken"} {
			if token, ok := typed[key].(string); ok && strings.TrimSpace(token) != "" {
				return token
			}
		}
		for _, nested := range typed {
			if token := findToken(nested); token != "" {
				return token
			}
		}
	case []any:
		for _, nested := range typed {
			if token := findToken(nested); token != "" {
				return token
			}
		}
	}
	return ""
}

func (s *server) verifyJWT(ctx context.Context, token string) (identity, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return identity{}, errors.New("malformed JWT")
	}
	var header jwtHeader
	if err := decodeJWTPart(parts[0], &header); err != nil {
		return identity{}, err
	}
	var claims jwtClaims
	if err := decodeJWTPart(parts[1], &claims); err != nil {
		return identity{}, err
	}
	if header.Algorithm != "RS256" && header.Algorithm != "EdDSA" {
		return identity{}, fmt.Errorf("unsupported JWT algorithm %q", header.Algorithm)
	}
	keys, err := s.fetchJWKS(ctx)
	if err != nil {
		return identity{}, err
	}
	message := []byte(parts[0] + "." + parts[1])
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return identity{}, err
	}
	verified := false
	for _, key := range keys.Keys {
		if header.KeyID != "" && key.KeyID != header.KeyID {
			continue
		}
		if key.Algorithm != "" && key.Algorithm != header.Algorithm {
			continue
		}
		if verifySignature(header.Algorithm, key, message, signature) == nil {
			verified = true
			break
		}
	}
	if !verified {
		return identity{}, errors.New("JWT signature is invalid")
	}
	now := time.Now().Unix()
	expiresAt, err := claims.ExpiresAt.Int64()
	if err != nil || expiresAt <= now {
		return identity{}, errors.New("JWT is expired or missing exp")
	}
	if claims.NotBefore != "" {
		if notBefore, err := claims.NotBefore.Int64(); err != nil || notBefore > now {
			return identity{}, errors.New("JWT is not active")
		}
	}
	if normalizeURL(claims.Issuer) != s.issuer {
		return identity{}, fmt.Errorf("unexpected JWT issuer %q", claims.Issuer)
	}
	claims.Email = strings.ToLower(strings.TrimSpace(claims.Email))
	if claims.Subject == "" || !strings.Contains(claims.Email, "@") {
		return identity{}, errors.New("JWT is missing sub or email")
	}
	return identity{Subject: claims.Subject, Email: claims.Email}, nil
}

func (s *server) fetchJWKS(ctx context.Context) (jwks, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.jwksURL, nil)
	if err != nil {
		return jwks{}, err
	}
	res, err := s.client.Do(req)
	if err != nil {
		return jwks{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return jwks{}, fmt.Errorf("JWKS endpoint returned %d", res.StatusCode)
	}
	var keys jwks
	if err := json.NewDecoder(io.LimitReader(res.Body, 2<<20)).Decode(&keys); err != nil {
		return jwks{}, err
	}
	return keys, nil
}

func verifySignature(algorithm string, key jwk, message, signature []byte) error {
	switch algorithm {
	case "RS256":
		publicKey, err := rsaPublicKey(key)
		if err != nil {
			return err
		}
		digest := sha256.Sum256(message)
		return rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, digest[:], signature)
	case "EdDSA":
		if key.KeyType != "OKP" || key.Curve != "Ed25519" {
			return errors.New("invalid EdDSA JWK")
		}
		x, err := base64.RawURLEncoding.DecodeString(key.X)
		if err != nil || len(x) != ed25519.PublicKeySize {
			return errors.New("invalid Ed25519 public key")
		}
		if !ed25519.Verify(ed25519.PublicKey(x), message, signature) {
			return errors.New("invalid EdDSA signature")
		}
		return nil
	}
	return errors.New("unsupported JWT algorithm")
}

func rsaPublicKey(key jwk) (*rsa.PublicKey, error) {
	if len(key.X509) > 0 {
		der, err := base64.StdEncoding.DecodeString(key.X509[0])
		if err != nil {
			return nil, err
		}
		cert, err := x509.ParseCertificate(der)
		if err != nil {
			return nil, err
		}
		publicKey, ok := cert.PublicKey.(*rsa.PublicKey)
		if !ok {
			return nil, errors.New("x5c key is not RSA")
		}
		return publicKey, nil
	}
	if key.KeyType != "RSA" {
		return nil, errors.New("invalid RSA JWK")
	}
	n, err := base64.RawURLEncoding.DecodeString(key.N)
	if err != nil {
		return nil, err
	}
	e, err := base64.RawURLEncoding.DecodeString(key.E)
	if err != nil {
		return nil, err
	}
	exponent := 0
	for _, part := range e {
		exponent = exponent<<8 + int(part)
	}
	if exponent == 0 {
		return nil, errors.New("invalid RSA exponent")
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: exponent}, nil
}

func (s *server) upsertProfile(ctx context.Context, member identity) error {
	database, err := url.Parse(s.databaseURL)
	if err != nil || database.Hostname() == "" {
		return errors.New("DATABASE_URL is invalid")
	}
	payload, _ := json.Marshal(map[string]any{"query": `INSERT INTO app_user_profiles (auth_subject, email) VALUES ($1, lower($2)) ON CONFLICT (auth_subject) DO UPDATE SET email = EXCLUDED.email, updated_at = now()`, "params": []string{member.Subject, member.Email}})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://"+database.Hostname()+"/sql", strings.NewReader(string(payload)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Neon-Connection-String", s.databaseURL)
	res, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("Neon SQL endpoint returned %d", res.StatusCode)
	}
	return nil
}

func (s *server) setMemberCookie(w http.ResponseWriter, member identity) {
	session := memberSession{Sub: member.Subject, Email: member.Email, ExpiresAt: time.Now().Add(memberSessionTTL).UnixMilli()}
	payload, _ := json.Marshal(session)
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	http.SetCookie(w, &http.Cookie{Name: memberCookieName, Value: encoded + "." + s.sign(encoded), Path: "/", HttpOnly: true, Secure: s.secure, SameSite: http.SameSiteLaxMode, MaxAge: int(memberSessionTTL.Seconds())})
}

func (s *server) readMemberCookie(r *http.Request) (memberSession, error) {
	cookie, err := r.Cookie(memberCookieName)
	if err != nil {
		return memberSession{}, err
	}
	parts := strings.Split(cookie.Value, ".")
	if len(parts) != 2 || !hmac.Equal([]byte(s.sign(parts[0])), []byte(parts[1])) {
		return memberSession{}, errors.New("invalid cookie signature")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return memberSession{}, err
	}
	var session memberSession
	if err := json.Unmarshal(payload, &session); err != nil {
		return memberSession{}, err
	}
	if session.Sub == "" || !strings.Contains(session.Email, "@") || session.ExpiresAt <= time.Now().UnixMilli() {
		return memberSession{}, errors.New("member session is invalid")
	}
	return session, nil
}

func (s *server) sign(value string) string {
	digest := hmac.New(sha256.New, s.secret)
	digest.Write([]byte(value))
	return hex.EncodeToString(digest.Sum(nil))
}
func decodeJWTPart(value string, target any) error {
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(decoded, target)
}
func bearerToken(value string) string {
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(value), "Bearer "))
}
func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
func normalizeURL(value string) string { return strings.TrimRight(strings.TrimSpace(value), "/") }
func normalizedIssuer(configured, base, jwksURL string) (string, error) {
	if configured != "" {
		return normalizeURL(configured), nil
	}
	if base != "" {
		return normalizeURL(base), nil
	}
	parsed, err := url.Parse(jwksURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("NEON_AUTH_JWKS_URL is invalid")
	}
	return parsed.Scheme + "://" + parsed.Host, nil
}
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"error": message})
}
func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
