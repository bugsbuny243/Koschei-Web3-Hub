package handlers

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"koschei/api/internal/services"
)

type addressPoisoningRequest struct {
	Wallet        string   `json:"wallet"`
	Candidate     string   `json:"candidate"`
	Recipient     string   `json:"recipient"`
	Address       string   `json:"address"`
	Network       string   `json:"network"`
	KnownContacts []string `json:"known_contacts"`
	Limit         int      `json:"limit"`
}

type addressPoisoningMatch struct {
	KnownAddress string `json:"known_address"`
	Signal       string `json:"signal"`
	Prefix       int    `json:"prefix"`
	Suffix       int    `json:"suffix"`
	RiskBonus    int    `json:"risk_bonus"`
}

func (h *Handler) AddressPoisoningCheck(w http.ResponseWriter, r *http.Request) {
	var req addressPoisoningRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid_body"})
		return
	}
	wallet := strings.TrimSpace(req.Wallet)
	candidate := strings.TrimSpace(firstNonEmptyString(req.Candidate, req.Recipient, req.Address))
	if wallet == "" || candidate == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "wallet_and_candidate_required"})
		return
	}
	network := strings.TrimSpace(req.Network)
	if network == "" {
		network = "solana-mainnet"
	}
	// The contact collector below is Solana-only. Never treat another chain
	// as though a Solana signature query had validated its addresses.
	if network != "solana-mainnet" && network != "solana-devnet" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "address_poisoning_unsupported_network"})
		return
	}
	if len(req.KnownContacts) > 40 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "too_many_known_contacts"})
		return
	}
	limit := req.Limit
	if limit <= 0 || limit > 20 {
		limit = 12
	}

	contacts := normalizeAddressList(req.KnownContacts)
	userSuppliedCount := len(contacts)
	// Even an explicit user-provided contact list is not verified identity;
	// retain provenance separately from chain-derived observations.
	observed, rpcEvidence := h.collectAddressPoisoningContacts(r.Context(), wallet, network, limit)
	contacts = appendUniqueAddresses(contacts, observed...)
	rpcEvidence["user_supplied_contact_count"] = userSuppliedCount
	result := evaluateAddressPoisoning(wallet, candidate, contacts, rpcEvidence)
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) collectAddressPoisoningContacts(ctx context.Context, wallet, network string, limit int) ([]string, map[string]any) {
	ctx, cancel := context.WithTimeout(ctx, 7500*time.Millisecond)
	defer cancel()
	rpcURL := strings.TrimSpace(os.Getenv("SOLANA_RPC_URL"))
	if rpcURL == "" {
		rpcURL = solanaRPCURL(network, os.Getenv("ALCHEMY_API_KEY"))
	}
	evidence := map[string]any{"collected": false, "provider": "solana_rpc", "signature_limit": limit}
	signatures, err := services.SolanaGetSignaturesForAddress(ctx, rpcURL, wallet, limit)
	if err != nil {
		evidence["error"] = compactAddressPoisoningError(err)
		return nil, evidence
	}
	evidence["collected"] = true
	evidence["signature_count"] = len(signatures)
	evidence["coverage_status"] = "bounded_recent_history"
	if len(signatures) >= limit {
		evidence["history_may_be_truncated"] = true
	}
	contacts := []string{}
	maxTx := len(signatures)
	if maxTx > 8 {
		maxTx = 8
	}
	evidence["transaction_attempted"] = maxTx
	if len(signatures) > maxTx {
		evidence["history_may_be_truncated"] = true
	}
	decoded := 0
	failed := 0
	for i := 0; i < maxTx; i++ {
		if strings.TrimSpace(signatures[i].Signature) == "" {
			failed++
			continue
		}
		tx, err := services.SolanaGetTransactionJSONParsed(ctx, rpcURL, signatures[i].Signature)
		if err != nil {
			failed++
			continue
		}
		decoded++
		contacts = appendUniqueAddresses(contacts, extractSolanaTransactionAddresses(tx)...)
	}
	evidence["transaction_decoded"] = decoded
	evidence["transaction_failed"] = failed
	if failed > 0 {
		evidence["coverage_status"] = "partial_decode_failure"
	}
	contacts = removeAddress(contacts, wallet)
	evidence["contact_count"] = len(contacts)
	return contacts, evidence
}

func evaluateAddressPoisoning(wallet, candidate string, contacts []string, rpcEvidence map[string]any) map[string]any {
	wallet = strings.TrimSpace(wallet)
	candidate = strings.TrimSpace(candidate)
	contacts = removeAddress(normalizeAddressList(contacts), wallet)
	contacts = removeAddress(contacts, candidate)
	matches := make([]addressPoisoningMatch, 0)
	for _, known := range contacts {
		if known == "" || known == candidate {
			continue
		}
		prefix := commonPrefixLen(candidate, known)
		suffix := commonSuffixLen(candidate, known)
		bonus := 0
		signal := ""
		switch {
		case prefix >= 8 && suffix >= 8:
			bonus, signal = 90, "near-identical visible prefix and suffix"
		case prefix >= 6 && suffix >= 6:
			bonus, signal = 78, "strong visible-address collision"
		case prefix >= 5 && suffix >= 5:
			bonus, signal = 65, "high lookalike risk"
		case prefix >= 4 && suffix >= 4:
			bonus, signal = 52, "possible address poisoning lookalike"
		}
		if bonus > 0 {
			matches = append(matches, addressPoisoningMatch{KnownAddress: known, Signal: signal, Prefix: prefix, Suffix: suffix, RiskBonus: bonus})
		}
	}
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].RiskBonus > matches[j].RiskBonus })
	// A bounded history comparison cannot authenticate a transfer recipient.
	// In particular, no-match is not an allowance to sign or send funds.
	risk := 25
	policy := "warn"
	verdict := "No visible lookalike match was observed in the bounded contact sample. This is not proof that the recipient is trusted; verify the full address independently."
	if len(matches) > 0 {
		risk = matches[0].RiskBonus
		policy = "warn"
		verdict = "Candidate address visually collides with a known or recently observed address. Verify the full address before signing."
		if risk >= 78 {
			policy = "block"
			verdict = "Strong address-poisoning pattern detected. Do not rely on truncated address display."
		}
	} else if len(contacts) == 0 {
		verdict = "No usable contact graph was available; verify the full recipient address out-of-band."
	}
	coverageStatus, _ := rpcEvidence["coverage_status"].(string)
	if coverageStatus == "" {
		coverageStatus = "unavailable"
	}
	findingState := "none_observed_in_bounded_scope"
	if len(matches) > 0 {
		findingState = "suspected_lookalike"
	} else if len(contacts) == 0 {
		findingState = "unavailable"
	}
	return map[string]any{
		"ok":                     true,
		"module":                 "Address Poisoning Shield",
		"module_id":              "address_poisoning_shield",
		"wallet":                 wallet,
		"candidate":              candidate,
		"risk_index":             risk,
		"risk_level":             exposureRiskLevelFromScore(risk),
		"policy":                 policy,
		"verdict":                verdict,
		"matches":                matches,
		"observed_contact_count": len(contacts),
		"rpc_evidence":           rpcEvidence,
		"coverage_status":        coverageStatus,
		"finding_state":          findingState,
		"enforcement_status":     "not_connected",
		"analysis_scope":         "visible_prefix_suffix_comparison_against_user_supplied_and_bounded_recent_solana_contacts",
		"limitation":             "UNKNOWN_OR_HISTORICAL_RECIPIENTS_NOT_RULED_OUT",
		"evidence_policy":        map[string]any{"no_evidence_no_claim": true, "safe_terms": []string{"lookalike risk", "possible address poisoning", "recipient verification required"}, "blocked_terms_without_proof": []string{"the wallet is hacked", "confirmed thief", "fraud"}},
		"disclaimer":             "Read-only recipient-risk analysis. This is not an accusation or financial advice.",
	}
}

func extractSolanaTransactionAddresses(tx services.SolanaTransactionResult) []string {
	out := []string{}
	transaction, _ := tx["transaction"].(map[string]any)
	message, _ := transaction["message"].(map[string]any)
	if keys, ok := message["accountKeys"].([]any); ok {
		for _, item := range keys {
			switch value := item.(type) {
			case string:
				out = append(out, value)
			case map[string]any:
				out = append(out, strings.TrimSpace(fmt.Sprint(value["pubkey"])))
			}
		}
	}
	meta, _ := tx["meta"].(map[string]any)
	for _, key := range []string{"preTokenBalances", "postTokenBalances"} {
		if balances, ok := meta[key].([]any); ok {
			for _, item := range balances {
				if balance, ok := item.(map[string]any); ok {
					out = append(out, strings.TrimSpace(fmt.Sprint(balance["owner"])))
					out = append(out, strings.TrimSpace(fmt.Sprint(balance["mint"])))
				}
			}
		}
	}
	return normalizeAddressList(out)
}

func normalizeAddressList(values []string) []string {
	seen := map[string]struct{}{}
	out := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if len(value) < 32 || len(value) > 64 {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func appendUniqueAddresses(values []string, extra ...string) []string {
	return normalizeAddressList(append(values, extra...))
}

func removeAddress(values []string, address string) []string {
	address = strings.TrimSpace(address)
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" && value != address {
			out = append(out, value)
		}
	}
	return out
}

func commonPrefixLen(a, b string) int {
	limit := len(a)
	if len(b) < limit {
		limit = len(b)
	}
	count := 0
	for count < limit && a[count] == b[count] {
		count++
	}
	return count
}

func commonSuffixLen(a, b string) int {
	ia, ib, count := len(a)-1, len(b)-1, 0
	for ia >= 0 && ib >= 0 && a[ia] == b[ib] {
		count++
		ia--
		ib--
	}
	return count
}

func compactAddressPoisoningError(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.TrimSpace(err.Error())
	if len(msg) > 180 {
		msg = msg[:180]
	}
	return msg
}
