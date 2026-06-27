package webhooks

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

const secretPrefix = "whsec_"

func GenerateSecret() (string, error) {
	raw := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, raw); err != nil {
		return "", err
	}
	return secretPrefix + base64.RawURLEncoding.EncodeToString(raw), nil
}

func EncryptSecret(secret string) (string, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return "", errors.New("empty webhook secret")
	}
	block, err := aes.NewCipher(encryptionKey())
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nil, nonce, []byte(secret), []byte("koschei-webhook-secret-v1"))
	payload := append(nonce, sealed...)
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func DecryptSecret(ciphertext string) (string, error) {
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(ciphertext))
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(encryptionKey())
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(payload) <= gcm.NonceSize() {
		return "", errors.New("invalid webhook secret ciphertext")
	}
	nonce := payload[:gcm.NonceSize()]
	sealed := payload[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, sealed, []byte("koschei-webhook-secret-v1"))
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func Signature(secret, timestamp string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(timestamp))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write(payload)
	return "v1=" + hex.EncodeToString(mac.Sum(nil))
}

func VerifySignature(secret, timestamp string, payload []byte, signature string) bool {
	expected := Signature(secret, timestamp, payload)
	return hmac.Equal([]byte(expected), []byte(strings.TrimSpace(signature)))
}

func Last4(secret string) string {
	secret = strings.TrimSpace(secret)
	if len(secret) <= 4 {
		return secret
	}
	return secret[len(secret)-4:]
}

func encryptionKey() []byte {
	seed := strings.TrimSpace(os.Getenv("WEBHOOK_ENCRYPTION_KEY"))
	if seed == "" {
		seed = strings.TrimSpace(os.Getenv("USER_SESSION_SECRET"))
	}
	if seed == "" {
		seed = "koschei-development-webhook-key"
	}
	hash := sha256.Sum256([]byte(fmt.Sprintf("koschei-webhook-encryption-v1:%s", seed)))
	return hash[:]
}
