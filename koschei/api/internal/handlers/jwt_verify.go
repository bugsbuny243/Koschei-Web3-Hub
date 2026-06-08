package handlers

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type neonJWTClaims struct {
	Sub   string `json:"sub"`
	Email string `json:"email"`
	Name  string `json:"name"`
	jwt.RegisteredClaims
}

type JWKKey struct {
	KID       string `json:"kid"`
	KeyType   string `json:"kty"`
	Use       string `json:"use"`
	Algorithm string `json:"alg"`
	N         string `json:"n"`
	E         string `json:"e"`
	PublicKey *rsa.PublicKey `json:"-"`
}

type JWKSet struct {
	Keys []JWKKey `json:"keys"`
	keys map[string]*rsa.PublicKey
}

func (s *JWKSet) Key(kid string) *rsa.PublicKey {
	if s == nil || s.keys == nil {
		return nil
	}
	return s.keys[kid]
}

var neonJWKS *JWKSet

func loadNeonJWKS(jwksURL string) error {
	resp, err := http.Get(jwksURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	jwks := &JWKSet{}
	if err := json.Unmarshal(body, jwks); err != nil {
		return err
	}

	jwks.keys = make(map[string]*rsa.PublicKey)
	for i, key := range jwks.Keys {
		publicKey, err := parseRSAPublicKey(key.N, key.E)
		if err != nil {
			continue
		}
		jwks.keys[key.KID] = publicKey
		jwks.Keys[i].PublicKey = publicKey
	}

	neonJWKS = jwks
	return nil
}

func parseRSAPublicKey(n, e string) (*rsa.PublicKey, error) {
	nBytes, err := base64URLDecode(n)
	if err != nil {
		return nil, err
	}
	eBytes, err := base64URLDecode(e)
	if err != nil {
		return nil, err
	}

	var modulusInt, exponentInt big.Int
	modulusInt.SetBytes(nBytes)
	exponentInt.SetBytes(eBytes)

	return &rsa.PublicKey{
		N: &modulusInt,
		E: int(exponentInt.Int64()),
	}, nil
}

func base64URLDecode(s string) ([]byte, error) {
	padding := (4 - len(s)%4) % 4
	s = s + strings.Repeat("=", padding)
	s = strings.NewReplacer("-", "+", "_", "/").Replace(s)
	return base64.StdEncoding.DecodeString(s)
}

func parseAndVerifyNeonJWT(tokenString string) (neonJWTClaims, error) {
	if neonJWKS == nil {
		jwksURL := strings.TrimSpace(os.Getenv("NEON_AUTH_JWKS_URL"))
		if jwksURL == "" {
			return neonJWTClaims{}, errors.New("NEON_AUTH_JWKS_URL is not set")
		}
		if err := loadNeonJWKS(jwksURL); err != nil {
			return neonJWTClaims{}, fmt.Errorf("failed to load JWKS: %w", err)
		}
	}

	token, err := jwt.ParseWithClaims(tokenString, &neonJWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		kid, _ := token.Header["kid"].(string)
		key := neonJWKS.Key(kid)
		if key == nil {
			return nil, newNeonJWTVerificationError(neonJWTFailureJWKSKeyNotFound, fmt.Sprintf("key not found: %s", kid))
		}
		return key, nil
	})

	if err != nil {
		return neonJWTClaims{}, err
	}

	if claims, ok := token.Claims.(*neonJWTClaims); ok && token.Valid {
		return *claims, nil
	}

	return neonJWTClaims{}, errors.New("invalid token claims")
}

type neonJWTFailureCategory string

const (
	neonJWTFailureIssuerMismatch    neonJWTFailureCategory = "issuer_mismatch"
	neonJWTFailureJWKSKeyNotFound   neonJWTFailureCategory = "jwks_key_not_found"
	neonJWTFailureVerificationError neonJWTFailureCategory = "verification_error"
)

type neonJWTVerificationError struct {
	Category neonJWTFailureCategory
	Message  string
}

func (e neonJWTVerificationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Category, e.Message)
}

func newNeonJWTVerificationError(category neonJWTFailureCategory, message string) error {
	return neonJWTVerificationError{
		Category: category,
		Message:  message,
	}
}
