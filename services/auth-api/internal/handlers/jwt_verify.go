package handlers

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"

	"koschei-bridge/services/auth-api/internal/db"
)

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
	X         string   `json:"x"`
	Curve     string   `json:"crv"`
	N         string   `json:"n"`
	E         string   `json:"e"`
	X509      []string `json:"x5c"`
}

func (handler *Handler) verifyJWT(ctx context.Context, token string) (db.Profile, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return db.Profile{}, errors.New("malformed JWT")
	}
	var header jwtHeader
	if err := decodeJWTPart(parts[0], &header); err != nil {
		return db.Profile{}, err
	}
	var claims jwtClaims
	if err := decodeJWTPart(parts[1], &claims); err != nil {
		return db.Profile{}, err
	}
	if header.Algorithm != "RS256" && header.Algorithm != "EdDSA" {
		return db.Profile{}, fmt.Errorf("unsupported JWT algorithm %q", header.Algorithm)
	}
	keys, err := handler.fetchJWKS(ctx)
	if err != nil {
		return db.Profile{}, err
	}
	message := []byte(parts[0] + "." + parts[1])
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return db.Profile{}, err
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
		return db.Profile{}, errors.New("JWT signature is invalid")
	}
	now := time.Now().Unix()
	expiresAt, err := claims.ExpiresAt.Int64()
	if err != nil || expiresAt <= now {
		return db.Profile{}, errors.New("JWT is expired or missing exp")
	}
	if claims.NotBefore != "" {
		if notBefore, err := claims.NotBefore.Int64(); err != nil || notBefore > now {
			return db.Profile{}, errors.New("JWT is not active")
		}
	}
	if normalizeURL(claims.Issuer) != handler.issuer {
		return db.Profile{}, fmt.Errorf("unexpected JWT issuer %q", claims.Issuer)
	}
	claims.Email = strings.ToLower(strings.TrimSpace(claims.Email))
	if claims.Subject == "" || !strings.Contains(claims.Email, "@") {
		return db.Profile{}, errors.New("JWT is missing sub or email")
	}
	return db.Profile{Subject: claims.Subject, Email: claims.Email}, nil
}

func (handler *Handler) fetchJWKS(ctx context.Context) (jwks, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, handler.jwksURL, nil)
	if err != nil {
		return jwks{}, err
	}
	res, err := handler.client.Do(req)
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

func decodeJWTPart(value string, target any) error {
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(decoded, target)
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
