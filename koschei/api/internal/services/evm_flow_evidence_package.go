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

// The EVM evidence package is the EVM counterpart of the Tron package: one
// finding, its on-chain references, public explorer links, plain statements
// of what it does not claim, and an Ed25519 signature by the platform signing
// key. The signed bytes start with this schema line (domain separation), so
// a package signature cannot be presented as a Tron package or an ARVIS
// verdict signature made with the same key.

const EVMFlowEvidencePackageSchemaV1 = "koschei.evm-flow-evidence-package.v1"

type EVMFlowEvidenceBody struct {
	Schema      string            `json:"schema"`
	GeneratedAt string            `json:"generated_at"`
	Recipients  []string          `json:"intended_recipients"`
	Finding     EVMFlowFinding    `json:"finding"`
	PublicLinks map[string]string `json:"public_verification_links"`
	Statements  []string          `json:"statements"`
	Network     string            `json:"network"`
}

type EVMFlowEvidencePackage struct {
	EVMFlowEvidenceBody
	SignatureStatus string                     `json:"signature_status"`
	Signature       *TronFlowEvidenceSignature `json:"signature,omitempty"`
}

var evmFlowTxHash = regexp.MustCompile(`^0x[0-9a-f]{64}$`)
var evmFlowHexAddress = regexp.MustCompile(`^0x[0-9a-f]{40}$`)

func evmFlowRecipients(options []string) []string {
	out := []string{}
	for _, option := range options {
		switch {
		case option == "issuer_freeze_request:tether_usdt":
			out = append(out, "Tether (USDT issuer): freeze request")
		case option == "issuer_freeze_request:circle_usdc":
			out = append(out, "Circle (USDC issuer): freeze request")
		case strings.HasPrefix(option, "bridge_operator_notice:"):
			out = append(out, strings.TrimPrefix(option, "bridge_operator_notice:")+" (bridge operator): hold or block request")
		case option == "receiving_party_notice":
			out = append(out, "Receiving exchange or service: hold request")
		}
	}
	return out
}

func evmFlowPublicLinks(finding EVMFlowFinding) map[string]string {
	links := map[string]string{}
	explorer := EVMFlowRadarExplorer(finding.Network)
	if explorer == "" {
		return links
	}
	if tx, _ := finding.Evidence["transaction_id"].(string); evmFlowTxHash.MatchString(tx) {
		links["transaction"] = explorer + "/tx/" + tx
	}
	for _, field := range []string{"from", "to", "origin_seed", "token_contract"} {
		if address, _ := finding.Evidence[field].(string); evmFlowHexAddress.MatchString(address) {
			links[field+"_address"] = explorer + "/address/" + address
		}
	}
	if evmFlowHexAddress.MatchString(finding.Address) {
		links["subject_address"] = explorer + "/address/" + finding.Address
	}
	return links
}

func evmFlowEvidenceSignedBytes(body EVMFlowEvidenceBody) ([]byte, error) {
	var buffer bytes.Buffer
	buffer.WriteString(EVMFlowEvidencePackageSchemaV1 + "\n")
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(body); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buffer.Bytes(), []byte("\n")), nil
}

// BuildEVMFlowEvidencePackage packages one finding and signs it with the
// configured platform key when available; without one the package says so.
func BuildEVMFlowEvidencePackage(finding EVMFlowFinding, now time.Time) (EVMFlowEvidencePackage, error) {
	recipients := evmFlowRecipients(tronFlowStringOptions(finding.Evidence["response_options"]))
	sort.Strings(recipients)
	body := EVMFlowEvidenceBody{
		Schema: EVMFlowEvidencePackageSchemaV1, GeneratedAt: now.UTC().Format(time.RFC3339), Recipients: recipients,
		Finding: finding, PublicLinks: evmFlowPublicLinks(finding), Network: finding.Network,
		Statements: []string{
			"These are observations of ERC-20 Transfer logs of issuer stablecoins read through eth_getLogs a stated number of blocks behind the chain head.",
			"\"Followed\" means the address received stablecoins descending from an operator seed wallet within the stated hop limit. It is not an attribution of control, identity or intent.",
			"Koschei cannot stop or reverse an on-chain transfer. Only the token issuer, a bridge operator or a receiving service can hold these funds.",
			"Every transaction and address can be checked independently through the public links; the raw integer amount and the token decimals are included.",
		},
	}
	pkg := EVMFlowEvidencePackage{EVMFlowEvidenceBody: body, SignatureStatus: "signer_not_configured"}
	keyID, privateKey, configured, err := unifiedVerdictSignerFromEnv()
	if err != nil {
		pkg.SignatureStatus = "signer_invalid"
		return pkg, nil
	}
	if !configured {
		return pkg, nil
	}
	signed, err := evmFlowEvidenceSignedBytes(body)
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
		SignedBytes:   "UTF-8 line \"" + EVMFlowEvidencePackageSchemaV1 + "\" + LF + the compact JSON (no HTML escaping) of every package field except signature_status and signature, in the order shown",
	}
	return pkg, nil
}

// VerifyEVMFlowEvidencePackage checks a package signature against the public
// key it carries. Callers must still decide whether they trust that key.
func VerifyEVMFlowEvidencePackage(pkg EVMFlowEvidencePackage) bool {
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
	signed, err := evmFlowEvidenceSignedBytes(pkg.EVMFlowEvidenceBody)
	if err != nil {
		return false
	}
	sum := sha256.Sum256(signed)
	return "sha256:"+hex.EncodeToString(sum[:]) == pkg.Signature.PayloadSHA256 && ed25519.Verify(publicKey, signed, signature)
}
