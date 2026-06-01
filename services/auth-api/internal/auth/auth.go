package auth

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/mail"
	"strings"
	"time"
)

type Identity struct {
	Subject string `json:"sub"`
	Email   string `json:"email"`
}
type Client struct {
	baseURL, issuer, jwksURL string
	http                     *http.Client
}
type jwks struct {
	Keys []jwk `json:"keys"`
}
type jwk struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
	Crv string `json:"crv"`
	X   string `json:"x"`
}
type jwtHeader struct {
	Alg string `json:"alg"`
	Kid string `json:"kid"`
}
type jwtClaims struct {
	Subject   string  `json:"sub"`
	Email     string  `json:"email"`
	Issuer    string  `json:"iss"`
	ExpiresAt float64 `json:"exp"`
	NotBefore float64 `json:"nbf"`
}
type ProviderError struct {
	Status  int
	Message string
}

func (e *ProviderError) Error() string { return e.Message }
func New(baseURL, issuer, jwksURL string) (*Client, error) {
	baseURL, issuer, jwksURL = normalizeURL(baseURL), normalizeURL(issuer), strings.TrimSpace(jwksURL)
	if baseURL == "" || issuer == "" || jwksURL == "" {
		return nil, errors.New("NEON_AUTH_BASE_URL, NEON_AUTH_ISSUER and NEON_AUTH_JWKS_URL are required")
	}
	return &Client{baseURL: baseURL, issuer: issuer, jwksURL: jwksURL, http: &http.Client{Timeout: 12 * time.Second}}, nil
}
func normalizeURL(value string) string { return strings.TrimRight(strings.TrimSpace(value), "/") }
func (c *Client) Authenticate(ctx context.Context, mode, email, password string) (*Identity, error) {
	endpoint := "sign-in/email"
	if mode == "signup" {
		endpoint = "sign-up/email"
	}
	body, _ := json.Marshal(map[string]string{"email": strings.ToLower(strings.TrimSpace(email)), "password": password})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/"+endpoint, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &ProviderError{Status: resp.StatusCode, Message: providerMessage(payload)}
	}
	token := findToken(payload)
	if token == "" {
		return nil, errors.New("Neon Auth did not return an access token")
	}
	return c.Verify(ctx, token)
}
func providerMessage(payload []byte) string {
	var value map[string]any
	if json.Unmarshal(payload, &value) == nil {
		for _, key := range []string{"message", "error", "detail"} {
			if text, ok := value[key].(string); ok && text != "" {
				return text
			}
		}
	}
	return "Neon Auth request failed"
}
func findToken(payload []byte) string {
	var value any
	if json.Unmarshal(payload, &value) != nil {
		return ""
	}
	paths := [][]string{{"data", "session", "access_token"}, {"session", "access_token"}, {"data", "access_token"}, {"access_token"}, {"data", "token"}, {"token"}, {"data", "session", "token"}, {"session", "token"}}
	for _, path := range paths {
		current := value
		for _, part := range path {
			object, ok := current.(map[string]any)
			if !ok {
				current = nil
				break
			}
			current = object[part]
		}
		if text, ok := current.(string); ok && len(strings.Split(text, ".")) == 3 {
			return text
		}
	}
	return ""
}
func (c *Client) Verify(ctx context.Context, raw string) (*Identity, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return nil, errors.New("JWT is malformed")
	}
	var header jwtHeader
	if err := decodeJSON(parts[0], &header); err != nil {
		return nil, err
	}
	if header.Alg != "RS256" && header.Alg != "EdDSA" {
		return nil, fmt.Errorf("unsupported JWT algorithm %q", header.Alg)
	}
	keys, err := c.fetchKeys(ctx)
	if err != nil {
		return nil, err
	}
	key, err := matchingKey(keys, header.Kid, header.Alg)
	if err != nil {
		return nil, err
	}
	signature, err := decode(parts[2])
	if err != nil {
		return nil, err
	}
	signed := []byte(parts[0] + "." + parts[1])
	if err := verifySignature(header.Alg, key, signed, signature); err != nil {
		return nil, err
	}
	var claims jwtClaims
	if err := decodeJSON(parts[1], &claims); err != nil {
		return nil, err
	}
	now := float64(time.Now().Unix())
	if claims.ExpiresAt == 0 || claims.ExpiresAt <= now {
		return nil, errors.New("JWT is expired")
	}
	if claims.NotBefore > now {
		return nil, errors.New("JWT is not active")
	}
	if normalizeURL(claims.Issuer) != c.issuer {
		return nil, errors.New("JWT issuer is invalid")
	}
	claims.Subject = strings.TrimSpace(claims.Subject)
	claims.Email = strings.ToLower(strings.TrimSpace(claims.Email))
	if claims.Subject == "" || claims.Email == "" {
		return nil, errors.New("JWT identity is incomplete")
	}
	return &Identity{Subject: claims.Subject, Email: claims.Email}, nil
}
func verifySignature(alg string, key any, signed, signature []byte) error {
	if alg == "RS256" {
		publicKey, ok := key.(*rsa.PublicKey)
		if !ok {
			return errors.New("RSA key is invalid")
		}
		digest := sha256.Sum256(signed)
		return rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, digest[:], signature)
	}
	publicKey, ok := key.(ed25519.PublicKey)
	if !ok || !ed25519.Verify(publicKey, signed, signature) {
		return errors.New("EdDSA signature is invalid")
	}
	return nil
}
func (c *Client) fetchKeys(ctx context.Context) ([]jwk, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.jwksURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JWKS request failed (%d)", resp.StatusCode)
	}
	var value jwks
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&value); err != nil {
		return nil, err
	}
	return value.Keys, nil
}
func matchingKey(keys []jwk, kid, alg string) (any, error) {
	for _, key := range keys {
		if kid != "" && key.Kid != kid {
			continue
		}
		if key.Alg != "" && key.Alg != alg {
			continue
		}
		parsed, err := parseKey(key, alg)
		if err == nil {
			return parsed, nil
		}
	}
	return nil, errors.New("matching JWKS key not found")
}
func parseKey(key jwk, alg string) (any, error) {
	if alg == "RS256" && key.Kty == "RSA" {
		n, err := decode(key.N)
		if err != nil {
			return nil, err
		}
		e, err := decode(key.E)
		if err != nil {
			return nil, err
		}
		exponent := 0
		for _, part := range e {
			exponent = exponent<<8 + int(part)
		}
		return &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: exponent}, nil
	}
	if alg == "EdDSA" && key.Kty == "OKP" && key.Crv == "Ed25519" {
		x, err := decode(key.X)
		if err != nil {
			return nil, err
		}
		if len(x) != ed25519.PublicKeySize {
			return nil, errors.New("invalid Ed25519 key")
		}
		return ed25519.PublicKey(x), nil
	}
	return nil, errors.New("unsupported JWKS key")
}
func decodeJSON(value string, target any) error {
	payload, err := decode(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(payload, target)
}
func decode(value string) ([]byte, error) { return base64.RawURLEncoding.DecodeString(value) }
func IsEmail(value string) bool {
	address, err := mail.ParseAddress(strings.TrimSpace(value))
	return err == nil && strings.EqualFold(address.Address, strings.TrimSpace(value))
}
