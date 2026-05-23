package handlers

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
)

func decodeBase64Raw(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}

func decodeBase64JSON(s string, out any) error {
	b, err := decodeBase64Raw(s)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

func verifyJWTSignatureRS256(token string, pub *rsa.PublicKey) error {
	parts := splitToken(token)
	h := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	sig, err := decodeBase64Raw(parts[2])
	if err != nil {
		return err
	}
	return rsa.VerifyPKCS1v15(pub, crypto.SHA256, h[:], sig)
}

func splitToken(token string) [3]string {
	var out [3]string
	idx := 0
	cur := ""
	for _, ch := range token {
		if ch == '.' {
			out[idx] = cur
			idx++
			cur = ""
			continue
		}
		cur += string(ch)
	}
	out[idx] = cur
	return out
}

var _ = rand.Reader
