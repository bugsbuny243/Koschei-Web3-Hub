package handlers

import (
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
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

var neonJWKS *jwt.JWKSet

func loadNeonJWKS(jwksURL string) error {
	resp, err := http.Get(jwksURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	return json.Unmarshal(body, &neonJWKS)
}

func parseAndVerifyNeonJWT(tokenString string) (neonJWTClaims, error) {
	if neonJWKS == nil {
		jwksURL := strings.TrimSpace(getenv("NEON_AUTH_JWKS_URL"))
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
			return nil, fmt.Errorf("key not found: %s", kid)
		}
		return key.PublicKey, nil
	})

	if err != nil {
		return neonJWTClaims{}, err
	}

	if claims, ok := token.Claims.(*neonJWTClaims); ok && token.Valid {
		return *claims, nil
	}

	return neonJWTClaims{}, errors.New("invalid token claims")
}
