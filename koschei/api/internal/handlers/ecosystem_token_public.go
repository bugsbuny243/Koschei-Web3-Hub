package handlers

import (
	"context"
	"math/big"
	"net/http"
	"os"
	"strings"
	"time"
)

const legacySPLTokenProgramID = "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"

type publicTokenMintResult struct {
	Value *struct {
		Owner string `json:"owner"`
		Data  struct {
			Parsed struct {
				Type string `json:"type"`
				Info struct {
					MintAuthority   *string `json:"mintAuthority"`
					FreezeAuthority *string `json:"freezeAuthority"`
					Extensions      []any   `json:"extensions"`
				} `json:"info"`
			} `json:"parsed"`
		} `json:"data"`
	} `json:"value"`
}

type publicTokenSupplyResult struct {
	Value struct {
		Amount         string `json:"amount"`
		Decimals       int    `json:"decimals"`
		UIAmountString string `json:"uiAmountString"`
	} `json:"value"`
}

type publicTokenLargestAccount struct {
	Address        string `json:"address"`
	Amount         string `json:"amount"`
	UIAmountString string `json:"uiAmountString"`
}

type publicTokenLargestResult struct {
	Value []publicTokenLargestAccount `json:"value"`
}

func (h *Handler) PublicTokenStatus(w http.ResponseWriter, r *http.Request) {
	mint := strings.TrimSpace(os.Getenv("KOSCHEI_TOKEN_MINT"))
	network, networkOK := normalizeWalletNetwork(os.Getenv("KOSCHEI_TOKEN_NETWORK"))
	if !networkOK {
		network = strings.TrimSpace(os.Getenv("KOSCHEI_TOKEN_NETWORK"))
	}
	response := map[string]any{
		"ok":         true,
		"configured": mint != "",
		"phase":      "planning",
		"network":    network,
		"mint":       mint,
		"identity": map[string]string{
			"name":   strings.TrimSpace(os.Getenv("KOSCHEI_TOKEN_NAME")),
			"symbol": strings.TrimSpace(os.Getenv("KOSCHEI_TOKEN_SYMBOL")),
		},
		"treasury": strings.TrimSpace(os.Getenv("KOSCHEI_TOKEN_TREASURY")),
		"public_links": map[string]string{
			"disclosure":     strings.TrimSpace(os.Getenv("KOSCHEI_TOKEN_DISCLOSURE_URL")),
			"vesting":        strings.TrimSpace(os.Getenv("KOSCHEI_TOKEN_VESTING_URL")),
			"liquidity_lock": strings.TrimSpace(os.Getenv("KOSCHEI_TOKEN_LIQUIDITY_LOCK_URL")),
		},
		"policy": map[string]bool{
			"investment_return_promised": false,
			"price_guarantee":            false,
			"hidden_minting_allowed":     false,
			"utility_first":              true,
		},
		"generated_at": time.Now().UTC(),
	}
	if mint == "" {
		response["message"] = "Token mint has not been configured. No token is represented as live."
		writeJSON(w, http.StatusOK, response)
		return
	}
	if !networkOK {
		writePublicTokenFailure(w, response, "configuration_error", "invalid_token_network")
		return
	}
	if _, err := decodeSolanaPublicKey(mint); err != nil {
		writePublicTokenFailure(w, response, "configuration_error", "invalid_token_mint")
		return
	}
	if h == nil || h.SolanaRPC == nil {
		writePublicTokenFailure(w, response, "verification_unavailable", "solana_rpc_unavailable")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	var mintAccount publicTokenMintResult
	if err := h.SolanaRPC.Call(ctx, network, "getAccountInfo", []any{mint, map[string]any{"encoding": "jsonParsed", "commitment": "confirmed"}}, &mintAccount, 30*time.Second); err != nil || mintAccount.Value == nil {
		writePublicTokenFailure(w, response, "verification_unavailable", "mint_account_unavailable")
		return
	}
	if mintAccount.Value.Data.Parsed.Type != "mint" {
		writePublicTokenFailure(w, response, "configuration_error", "configured_address_is_not_a_mint")
		return
	}
	standard := ""
	switch mintAccount.Value.Owner {
	case legacySPLTokenProgramID:
		standard = "spl-token"
	case token2022ProgramID:
		standard = "token-2022"
	default:
		response["program_id"] = mintAccount.Value.Owner
		writePublicTokenFailure(w, response, "configuration_error", "unsupported_token_program")
		return
	}

	var supply publicTokenSupplyResult
	if err := h.SolanaRPC.Call(ctx, network, "getTokenSupply", []any{mint, map[string]any{"commitment": "confirmed"}}, &supply, time.Minute); err != nil {
		writePublicTokenFailure(w, response, "verification_unavailable", "token_supply_unavailable")
		return
	}
	var largest publicTokenLargestResult
	if err := h.SolanaRPC.Call(ctx, network, "getTokenLargestAccounts", []any{mint, map[string]any{"commitment": "confirmed"}}, &largest, 5*time.Minute); err != nil {
		writePublicTokenFailure(w, response, "verification_unavailable", "holder_concentration_unavailable")
		return
	}

	accounts := make([]map[string]string, 0, len(largest.Value))
	for index, account := range largest.Value {
		if index >= 20 {
			break
		}
		accounts = append(accounts, map[string]string{
			"token_account":    account.Address,
			"amount_raw":       account.Amount,
			"ui_amount_string": account.UIAmountString,
		})
	}
	mintAuthorityRevoked := mintAccount.Value.Data.Parsed.Info.MintAuthority == nil
	freezeAuthorityRevoked := mintAccount.Value.Data.Parsed.Info.FreezeAuthority == nil
	response["phase"] = "live"
	response["onchain_verified"] = true
	response["standard"] = standard
	response["program_id"] = mintAccount.Value.Owner
	response["supply"] = map[string]any{
		"amount_raw":       supply.Value.Amount,
		"ui_amount_string": supply.Value.UIAmountString,
		"decimals":         supply.Value.Decimals,
	}
	response["authorities"] = map[string]any{
		"mint_authority":           mintAccount.Value.Data.Parsed.Info.MintAuthority,
		"freeze_authority":         mintAccount.Value.Data.Parsed.Info.FreezeAuthority,
		"mint_authority_revoked":   mintAuthorityRevoked,
		"freeze_authority_revoked": freezeAuthorityRevoked,
	}
	response["extensions"] = mintAccount.Value.Data.Parsed.Info.Extensions
	response["concentration"] = map[string]any{
		"top_10_percentage":      publicTokenConcentration(largest.Value, supply.Value.Amount, 10),
		"top_20_percentage":      publicTokenConcentration(largest.Value, supply.Value.Amount, 20),
		"largest_token_accounts": accounts,
		"note":                   "Solana RPC reports token accounts, not guaranteed unique beneficial owners.",
	}
	response["risk_flags"] = map[string]bool{
		"mint_authority_active":   !mintAuthorityRevoked,
		"freeze_authority_active": !freezeAuthorityRevoked,
	}
	writeJSON(w, http.StatusOK, response)
}

func writePublicTokenFailure(w http.ResponseWriter, response map[string]any, phase, code string) {
	response["ok"] = false
	response["phase"] = phase
	response["error"] = code
	writeJSON(w, http.StatusServiceUnavailable, response)
}

func publicTokenConcentration(accounts []publicTokenLargestAccount, totalRaw string, limit int) float64 {
	total := new(big.Int)
	if _, ok := total.SetString(strings.TrimSpace(totalRaw), 10); !ok || total.Sign() <= 0 {
		return 0
	}
	if limit > len(accounts) {
		limit = len(accounts)
	}
	sum := new(big.Int)
	for _, account := range accounts[:limit] {
		amount := new(big.Int)
		if _, ok := amount.SetString(strings.TrimSpace(account.Amount), 10); ok && amount.Sign() >= 0 {
			sum.Add(sum, amount)
		}
	}
	ratio := new(big.Rat).SetFrac(sum, total)
	percentage, _ := new(big.Float).Mul(new(big.Float).SetRat(ratio), big.NewFloat(100)).Float64()
	return percentage
}
