package auth

import (
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestVerifySupportsRS256AndNormalizedIssuer(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	key := jwk{Kty: "RSA", Kid: "rsa-key", Alg: "RS256", N: encode(privateKey.N.Bytes()), E: encode(big.NewInt(int64(privateKey.E)).Bytes())}
	client := testClient(t, key, "https://issuer.example/")
	token := signToken(t, "RS256", "rsa-key", "https://issuer.example", func(signed []byte) []byte {
		digest := sha256.Sum256(signed)
		signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, digest[:])
		if err != nil {
			t.Fatal(err)
		}
		return signature
	})
	identity, err := client.Verify(t.Context(), token)
	if err != nil {
		t.Fatal(err)
	}
	if identity.Subject != "member-1" || identity.Email != "member@example.com" {
		t.Fatalf("unexpected identity: %#v", identity)
	}
}
func TestVerifySupportsEdDSA(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	client := testClient(t, jwk{Kty: "OKP", Kid: "ed-key", Alg: "EdDSA", Crv: "Ed25519", X: encode(publicKey)}, "https://issuer.example")
	token := signToken(t, "EdDSA", "ed-key", "https://issuer.example/", func(signed []byte) []byte { return ed25519.Sign(privateKey, signed) })
	if _, err := client.Verify(t.Context(), token); err != nil {
		t.Fatal(err)
	}
}
func testClient(t *testing.T, key jwk, issuer string) *Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _ = json.NewEncoder(w).Encode(jwks{Keys: []jwk{key}}) }))
	t.Cleanup(server.Close)
	client, err := New("https://auth.example", issuer, server.URL)
	if err != nil {
		t.Fatal(err)
	}
	return client
}
func signToken(t *testing.T, alg, kid, issuer string, sign func([]byte) []byte) string {
	t.Helper()
	header, _ := json.Marshal(jwtHeader{Alg: alg, Kid: kid})
	claims, _ := json.Marshal(jwtClaims{Subject: "member-1", Email: "member@example.com", Issuer: issuer, ExpiresAt: float64(time.Now().Add(time.Minute).Unix())})
	unsigned := encode(header) + "." + encode(claims)
	return unsigned + "." + encode(sign([]byte(unsigned)))
}
func encode(value []byte) string { return base64.RawURLEncoding.EncodeToString(value) }
