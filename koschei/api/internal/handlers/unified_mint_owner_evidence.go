package handlers

import (
	"context"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

// collectUnifiedMintOwnerEvidence reads the mint owner directly from RPC.
// Unknown owners and failed calls never imply classic SPL Token.
func (h *Handler) collectUnifiedMintOwnerEvidence(parent context.Context, target, network string) map[string]any {
	unverified := map[string]any{"mint_owner_program_evidence_status": "unverified"}
	if h == nil || strings.TrimSpace(target) == "" || !strings.HasPrefix(strings.ToLower(network), "solana") {
		return unverified
	}
	ctx, cancel := context.WithTimeout(parent, 6500*time.Millisecond)
	defer cancel()
	var account struct {
		Value *struct {
			Owner string `json:"owner"`
			Data  struct {
				Parsed struct {
					Type string         `json:"type"`
					Info map[string]any `json:"info"`
				} `json:"parsed"`
				Space int `json:"space"`
			} `json:"data"`
		} `json:"value"`
	}
	rpcURL := solanaRPCURL(network, os.Getenv("ALCHEMY_API_KEY"))
	if err := h.callSolanaRPC(ctx, &http.Client{Timeout: 7 * time.Second}, rpcURL, network, "getAccountInfo", []interface{}{target, map[string]string{"encoding": "jsonParsed"}}, &account); err != nil || account.Value == nil {
		return unverified
	}
	if !strings.EqualFold(strings.TrimSpace(account.Value.Data.Parsed.Type), "mint") {
		return unverified
	}
	ownerEvidence := mintOwnerProgramEvidence(account.Value.Owner)
	if ownerEvidence["token_2022"] != true {
		return ownerEvidence
	}
	info := account.Value.Data.Parsed.Info
	extensions := parseToken2022Extensions(info)
	status, complete := token2022ExtensionResolution(true, account.Value.Data.Space, extensions)
	// A partial/invalid jsonParsed extensions array cannot certify that the
	// entire Token-2022 TLV extension set was decoded.
	rawExtensions, present := info["extensions"]
	items, validArray := rawExtensions.([]any)
	if account.Value.Data.Space < 82 || (account.Value.Data.Space == 82 && len(extensions) > 0) ||
		(account.Value.Data.Space > 82 && (!present || !validArray)) ||
		(validArray && len(items) != len(extensions)) {
		status, complete = "token_2022_extensions_unresolved", false
	}
	names := make([]string, 0, len(extensions))
	for _, extension := range extensions {
		names = append(names, extension.Name)
	}
	sort.Strings(names)
	evidence := map[string]any{
		"status": status, "evidence_complete": complete,
		"extension_count": len(extensions), "extension_names": names,
	}
	if !complete {
		evidence["limitation_code"] = "TOKEN_2022_EXTENSIONS_UNRESOLVED"
	}
	ownerEvidence["token_2022_extension_evidence"] = evidence
	return ownerEvidence
}
