package utils

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"
)

func HashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	sum := sha256.Sum256(append(salt, []byte(password)...))
	return fmt.Sprintf("$koschei-sha256$%s$%s", base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(sum[:])), nil
}

func ComparePassword(hash, password string) bool {
	parts := strings.Split(hash, "$")
	if len(parts) == 4 && parts[1] == "koschei-sha256" {
		salt, err := base64.RawStdEncoding.DecodeString(parts[2])
		if err != nil {
			return false
		}
		expected, err := base64.RawStdEncoding.DecodeString(parts[3])
		if err != nil {
			return false
		}
		sum := sha256.Sum256(append(salt, []byte(password)...))
		return subtle.ConstantTimeCompare(sum[:], expected) == 1
	}
	return false
}

func IsArgon2Hash(s string) bool {
	return strings.HasPrefix(s, "$argon2id$") || strings.HasPrefix(s, "$koschei-sha256$")
}
