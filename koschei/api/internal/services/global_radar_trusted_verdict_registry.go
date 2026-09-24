package services

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

const GlobalRadarVerifiedSignatureState = "verified_ed25519_trusted_registry"

type GlobalRadarTrustedVerdictKey struct {
	KeyID              string    `json:"key_id"`
	PublicKeyBase64URL string    `json:"public_key_base64url"`
	ValidFrom          time.Time `json:"valid_from,omitempty"`
	ValidUntil         time.Time `json:"valid_until,omitempty"`
	RevokedAt          time.Time `json:"revoked_at,omitempty"`
}

type globalRadarTrustedVerdictKeyMaterial struct {
	config    GlobalRadarTrustedVerdictKey
	publicKey ed25519.PublicKey
}

type GlobalRadarTrustedVerdictRegistry struct {
	keys map[string]globalRadarTrustedVerdictKeyMaterial
}

func NewGlobalRadarTrustedVerdictRegistry(keys []GlobalRadarTrustedVerdictKey) (GlobalRadarTrustedVerdictRegistry, error) {
	registry := GlobalRadarTrustedVerdictRegistry{keys: make(map[string]globalRadarTrustedVerdictKeyMaterial, len(keys))}
	for _, key := range keys {
		keyID := strings.TrimSpace(key.KeyID)
		if keyID == "" {
			return GlobalRadarTrustedVerdictRegistry{}, errors.New("trusted verdict key_id is required")
		}
		if _, exists := registry.keys[keyID]; exists {
			return GlobalRadarTrustedVerdictRegistry{}, fmt.Errorf("duplicate trusted verdict key_id %q", keyID)
		}
		publicKey, err := decodeCanonicalGlobalRadarEd25519(key.PublicKeyBase64URL, ed25519.PublicKeySize)
		if err != nil {
			return GlobalRadarTrustedVerdictRegistry{}, fmt.Errorf("trusted verdict key %q: %w", keyID, err)
		}
		key.KeyID = keyID
		key.PublicKeyBase64URL = strings.TrimSpace(key.PublicKeyBase64URL)
		key.ValidFrom = key.ValidFrom.UTC()
		key.ValidUntil = key.ValidUntil.UTC()
		key.RevokedAt = key.RevokedAt.UTC()
		if !key.ValidFrom.IsZero() && !key.ValidUntil.IsZero() && !key.ValidUntil.After(key.ValidFrom) {
			return GlobalRadarTrustedVerdictRegistry{}, fmt.Errorf("trusted verdict key %q valid_until must be after valid_from", keyID)
		}
		if !key.ValidFrom.IsZero() && !key.RevokedAt.IsZero() && key.RevokedAt.Before(key.ValidFrom) {
			return GlobalRadarTrustedVerdictRegistry{}, fmt.Errorf("trusted verdict key %q revoked_at cannot be before valid_from", keyID)
		}
		registry.keys[keyID] = globalRadarTrustedVerdictKeyMaterial{
			config:    key,
			publicKey: publicKey,
		}
	}
	return registry, nil
}

func (r GlobalRadarTrustedVerdictRegistry) Resolve(keyID string, signedAt time.Time) (ed25519.PublicKey, error) {
	keyID = strings.TrimSpace(keyID)
	if keyID == "" {
		return nil, errors.New("verdict key_id is required")
	}
	if signedAt.IsZero() {
		return nil, errors.New("verdict generated_at is required")
	}
	material, ok := r.keys[keyID]
	if !ok {
		return nil, fmt.Errorf("no trusted Ed25519 public key is configured for key_id %q", keyID)
	}
	at := signedAt.UTC()
	if !material.config.ValidFrom.IsZero() && at.Before(material.config.ValidFrom) {
		return nil, fmt.Errorf("trusted verdict key %q was not yet valid at generated_at", keyID)
	}
	if !material.config.ValidUntil.IsZero() && !at.Before(material.config.ValidUntil) {
		return nil, fmt.Errorf("trusted verdict key %q was expired at generated_at", keyID)
	}
	if !material.config.RevokedAt.IsZero() && !at.Before(material.config.RevokedAt) {
		return nil, fmt.Errorf("trusted verdict key %q was revoked at generated_at", keyID)
	}
	return ed25519.PublicKey(append([]byte(nil), material.publicKey...)), nil
}

func VerifyUnifiedRadarVerdictWithTrustedRegistry(verdict UnifiedRadarVerdict, registry GlobalRadarTrustedVerdictRegistry) error {
	if !verdict.Signed {
		return errors.New("verdict signed flag is required")
	}
	if !strings.EqualFold(strings.TrimSpace(verdict.SignatureAlgorithm), UnifiedVerdictSignatureAlgorithmV1) {
		return errors.New("verdict signature_algorithm must be ed25519")
	}
	publicKey, err := registry.Resolve(verdict.KeyID, verdict.GeneratedAt)
	if err != nil {
		return err
	}
	payload, err := unifiedVerdictSigningBytesV1(verdict)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(payload)
	expectedHash := "sha256:" + hex.EncodeToString(sum[:])
	actualHash := strings.TrimSpace(verdict.PayloadHash)
	if subtle.ConstantTimeCompare([]byte(actualHash), []byte(expectedHash)) != 1 {
		return errors.New("verdict payload_hash does not match canonical authenticated payload")
	}
	signature, err := decodeCanonicalGlobalRadarEd25519(verdict.Signature, ed25519.SignatureSize)
	if err != nil {
		return fmt.Errorf("verdict signature: %w", err)
	}
	if !ed25519.Verify(publicKey, payload, signature) {
		return errors.New("verdict signature did not verify against trusted Ed25519 key")
	}
	return nil
}

func decodeCanonicalGlobalRadarEd25519(raw string, expectedSize int) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("canonical unpadded base64url value is required")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(decoded) != expectedSize || base64.RawURLEncoding.EncodeToString(decoded) != raw {
		return nil, fmt.Errorf("must be canonical unpadded base64url for %d bytes", expectedSize)
	}
	return decoded, nil
}
