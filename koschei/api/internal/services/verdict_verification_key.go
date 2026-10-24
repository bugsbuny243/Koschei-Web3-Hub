package services

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
)

// PublishedVerdictVerificationKey is the public half of the configured verdict
// signer. It never carries private key material.
type PublishedVerdictVerificationKey struct {
	KeyID              string `json:"key_id"`
	Algorithm          string `json:"algorithm"`
	SignatureDomain    string `json:"signature_domain"`
	PublicKeyBase64URL string `json:"public_key_base64url"`
}

var errVerdictSignerConfigInvalid = errors.New("verdict signer configuration is invalid")

// ActiveVerdictVerificationKey derives the public key of the configured verdict
// signer so anyone can verify signed ARVIS verdicts and operators can register
// it as trusted. configured is false when no signer is set. A configuration
// error is reported without echoing the cause, which may describe the key.
func ActiveVerdictVerificationKey() (PublishedVerdictVerificationKey, bool, error) {
	keyID, privateKey, configured, err := unifiedVerdictSignerFromEnv()
	if err != nil {
		return PublishedVerdictVerificationKey{}, configured, errVerdictSignerConfigInvalid
	}
	if !configured {
		return PublishedVerdictVerificationKey{}, false, nil
	}
	public, ok := privateKey.Public().(ed25519.PublicKey)
	if !ok || len(public) != ed25519.PublicKeySize {
		return PublishedVerdictVerificationKey{}, true, errVerdictSignerConfigInvalid
	}
	return PublishedVerdictVerificationKey{
		KeyID:              keyID,
		Algorithm:          UnifiedVerdictSignatureAlgorithmV1,
		SignatureDomain:    UnifiedVerdictSignatureDomainV1,
		PublicKeyBase64URL: base64.RawURLEncoding.EncodeToString(public),
	}, true, nil
}
