package services

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

// A Tron Flow Radar finding is only useful for stopping funds if it reaches
// the party that can hold them (Tether for USDT, a bridge operator, a
// receiving exchange) quickly and in a form they can check. The evidence
// package is that form: the finding, its on-chain references, public links an
// analyst can open, plain statements of what it does not claim, and an
// Ed25519 signature by the platform signing key.
//
// Domain separation: the signed bytes start with the package schema line, so
// a package signature can never be presented as an ARVIS verdict signature
// made with the same key, and vice versa.

const TronFlowEvidencePackageSchemaV1 = "koschei.tron-flow-evidence-package.v1"

type TronFlowEvidenceSignature struct {
	Algorithm     string `json:"algorithm"`
	KeyID         string `json:"key_id"`
	PublicKey     string `json:"public_key_base64url"`
	PayloadSHA256 string `json:"payload_sha256"`
	Signature     string `json:"signature_base64url"`
	SignedBytes   string `json:"signed_bytes"`
}

// TronFlowEvidenceBody is the signed content.
type TronFlowEvidenceBody struct {
	Schema       string            `json:"schema"`
	GeneratedAt  string            `json:"generated_at"`
	Recipients   []string          `json:"intended_recipients"`
	Finding      TronFlowFinding   `json:"finding"`
	PublicLinks  map[string]string `json:"public_verification_links"`
	Statements   []string          `json:"statements"`
	NetworkScope string            `json:"network"`
}

type TronFlowEvidencePackage struct {
	TronFlowEvidenceBody
	SignatureStatus string                     `json:"signature_status"`
	Signature       *TronFlowEvidenceSignature `json:"signature,omitempty"`
}

var tronFlowTxID = regexp.MustCompile(`^[0-9a-f]{64}$`)
var tronFlowBase58Address = regexp.MustCompile(`^T[1-9A-HJ-NP-Za-km-z]{33}$`)

func tronFlowRecipients(options []string) []string {
	out := []string{}
	for _, option := range options {
		switch {
		case option == "issuer_freeze_request:tether_usdt":
			out = append(out, "Tether (USDT issuer): freeze request")
		case strings.HasPrefix(option, "bridge_operator_notice:"):
			out = append(out, strings.TrimPrefix(option, "bridge_operator_notice:")+" (bridge operator): hold or block request")
		case option == "receiving_party_notice":
			out = append(out, "Receiving exchange or service: hold request")
		}
	}
	return out
}

func tronFlowStringOptions(value any) []string {
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		out := []string{}
		for _, item := range typed {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func tronFlowPublicLinks(finding TronFlowFinding) map[string]string {
	links := map[string]string{}
	if tx, _ := finding.Evidence["transaction_id"].(string); tronFlowTxID.MatchString(tx) {
		links["transaction"] = "https://tronscan.org/#/transaction/" + tx
	}
	for _, field := range []string{"from", "to", "origin_seed"} {
		if address, _ := finding.Evidence[field].(string); tronFlowBase58Address.MatchString(address) {
			links[field+"_address"] = "https://tronscan.org/#/address/" + address
		}
	}
	if tronFlowBase58Address.MatchString(finding.Address) {
		links["subject_address"] = "https://tronscan.org/#/address/" + finding.Address
	}
	return links
}

// tronFlowEvidenceSignedBytes is the schema line, LF, then compact JSON without
// HTML escaping, so any JSON library can rebuild the exact bytes.
func tronFlowEvidenceSignedBytes(body TronFlowEvidenceBody) ([]byte, error) {
	var buffer bytes.Buffer
	buffer.WriteString(TronFlowEvidencePackageSchemaV1 + "\n")
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(body); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buffer.Bytes(), []byte("\n")), nil
}

// BuildTronFlowEvidencePackage packages one finding. It signs with the
// configured platform key when available; without one the package is still
// produced and says so, rather than inventing a signature.
func BuildTronFlowEvidencePackage(finding TronFlowFinding, now time.Time) (TronFlowEvidencePackage, error) {
	recipients := tronFlowRecipients(tronFlowStringOptions(finding.Evidence["response_options"]))
	sort.Strings(recipients)
	body := TronFlowEvidenceBody{
		Schema: TronFlowEvidencePackageSchemaV1, GeneratedAt: now.UTC().Format(time.RFC3339), Recipients: recipients,
		Finding: finding, PublicLinks: tronFlowPublicLinks(finding), NetworkScope: "tron-mainnet",
		Statements: []string{
			"These are observations of confirmed (solidified) Tron mainnet USDT transfer events read from TronGrid.",
			"\"Followed\" means the address received USDT descending from an operator seed wallet within the stated hop limit. It is not an attribution of control, identity or intent.",
			"Koschei cannot stop or reverse an on-chain transfer. Only the token issuer, a bridge operator or a receiving service can hold these funds.",
			"Every transaction and address can be checked independently through the public links; amounts are USDT with 6 decimals and the raw integer value is included.",
		},
	}
	pkg := TronFlowEvidencePackage{TronFlowEvidenceBody: body, SignatureStatus: "signer_not_configured"}
	keyID, privateKey, configured, err := unifiedVerdictSignerFromEnv()
	if err != nil {
		pkg.SignatureStatus = "signer_invalid"
		return pkg, nil
	}
	if !configured {
		return pkg, nil
	}
	signed, err := tronFlowEvidenceSignedBytes(body)
	if err != nil {
		return pkg, fmt.Errorf("encode evidence package: %w", err)
	}
	sum := sha256.Sum256(signed)
	pkg.SignatureStatus = "signed"
	pkg.Signature = &TronFlowEvidenceSignature{
		Algorithm: "Ed25519", KeyID: keyID,
		PublicKey:     base64.RawURLEncoding.EncodeToString(privateKey.Public().(ed25519.PublicKey)),
		PayloadSHA256: "sha256:" + hex.EncodeToString(sum[:]),
		Signature:     base64.RawURLEncoding.EncodeToString(ed25519.Sign(privateKey, signed)),
		SignedBytes:   "UTF-8 line \"" + TronFlowEvidencePackageSchemaV1 + "\" + LF + the compact JSON (no HTML escaping) of every package field except signature_status and signature, in the order shown",
	}
	return pkg, nil
}

// VerifyTronFlowEvidencePackage checks a package signature against the public
// key it carries. Callers must still decide whether they trust that key.
func VerifyTronFlowEvidencePackage(pkg TronFlowEvidencePackage) bool {
	if pkg.Signature == nil || pkg.Signature.Algorithm != "Ed25519" {
		return false
	}
	publicKey, err := base64.RawURLEncoding.DecodeString(pkg.Signature.PublicKey)
	if err != nil || len(publicKey) != ed25519.PublicKeySize {
		return false
	}
	signature, err := base64.RawURLEncoding.DecodeString(pkg.Signature.Signature)
	if err != nil {
		return false
	}
	signed, err := tronFlowEvidenceSignedBytes(pkg.TronFlowEvidenceBody)
	if err != nil {
		return false
	}
	sum := sha256.Sum256(signed)
	return "sha256:"+hex.EncodeToString(sum[:]) == pkg.Signature.PayloadSHA256 && ed25519.Verify(publicKey, signed, signature)
}

// FindingByDedupeKey returns the most recent in-memory finding with that key.
func (r *TronFlowRadar) FindingByDedupeKey(key string) (TronFlowFinding, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := len(r.state.findings) - 1; i >= 0; i-- {
		if r.state.findings[i].DedupeKey == key {
			return r.state.findings[i], true
		}
	}
	return TronFlowFinding{}, false
}
