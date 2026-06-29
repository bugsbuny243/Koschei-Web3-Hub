package handlers

import (
	"context"
	"math/big"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

const (
	legacyTokenProgramID = "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"
	token2022ProgramID    = "TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb"
)

var solanaPublicKeyPattern = regexp.MustCompile(`^[1-9A-HJ-NP-Za-km-z]{32,44}$`)

type publicTokenMintAccount struct {
	Owner string `json:"owner"`
	Data  struct {
		Parsed struct {
			Info struct {
				Decimals        int     `json:"decimals"`
				Supply          string  `json:"supply"`
				MintAuthority   *string `json:"mintAuthority"`
				FreezeAuthority *string `json:"freezeAuthority"`
				Extensions      []any   `json:"extensions"`
			} `json:"info"`
			Type string `json:"type"`
		} `json:"parsed"`
	} `json:"data"`
	Executable bool `json:"executable"`
}

type publicTokenSupply struct {
	Value struct {
		Amount         string `json:"amount"`
		Decimals       int    `json:"decimals"`
		UIAmountString string `json:"uiAmountString"`
	} `json:"value"`
}

type publicTokenLargestAccounts struct {
	Value []struct {
		Address        string `json:"address"`
		Amount         string `json:"amount"`
		Decimals       int    `json:"decimals"`
		UIAmountString string `json:"uiAmountString"`
	} `json:"value"`
}

// PublicTokenStatus is the source of truth for the future Koschei ecosystem
// token. Before a mint exists it reports a planning state. After a mint is
// configured it reads authorities, supply and concentration directly from
// Solana RPC; marketing copy is never accepted as proof.
func (h *Handler) PublicTokenStatus(w http.ResponseWriter, r *http.Request) {
	mint := strings.TrimSpace(os.Getenv("KOSCHEI_TOKEN_MINT"))
	network := strings.TrimSpace(os.Getenv("KOSCHEI_TOKEN_NETWORK"))
	if network == "" {
		network = "solana-mainnet"
	}

	base := map[string]any{
		"ok":         true,
		"configured": mint != "",
		"phase":      "planning",
		"network":    network,
		"mint":       mint,
		"identity": map[string]any{
			"name":   strings.TrimSpace(os.Getenv("KOSCHEI_TOKEN_NAME")),
			"symbol": strings.TrimSpace(os.Getenv("KOSCHEI_TOKEN_SYMBOL")),
		},
		"public_links": map[string]string{
			"disclosure":     strings.TrimSpace(os.Getenv("KOSCHEI_TOKEN_DISCLOSURE_URL")),
			"vesting":        strings.TrimSpace(os.Getenv("KOSCHEI_TOKEN_VESTING_URL")),
			"liquidity_lock": strings.TrimSpace(os.Getenv("KOSCHEI_TOKEN_LIQUIDITY_LOCK_URL")),
		},
		"treasury": strings.TrimSpace(os.Getenv("KOSCHEI_TOKEN_TREASURY")),
		"policy": map[string]any{
			"investment_return_promised": false,
			"wash_trading_allowed":       false,
			"hidden_minting_allowed":     false,
			"price_guarantee":            false,
			"utility_first":              true,
		},
		"generated_at": time.Now().UTC(),
	}

	if mint == "" {
		base["message"] = "Token mint has not been configured. No token is represented as live."
		writeJSON(w, http.StatusOK, base)
		return
	}
	if !solanaPublicKeyPattern.MatchString(mint) {
		base["ok"] = false
		base["phase"] = "configuration_error"
		base["error"] = "invalid_token_mint"
		writeJSON(w, http.StatusServiceUnavailable, base)
		return
	}
	if h == nil || h.SolanaRPC == nil {
		base["ok"] = false
		base["phase"] = "verification_unavailable"
		base["error"] = "solana_rpc_unavailable"
		writeJSON(w, http.StatusServiceUnavailable, base)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	var account publicTokenMintAccount
	if err := h.SolanaRPC.Call(ctx, network, "getAccountInfo", []any{mint, map[string]any{"encoding": "jsonParsed", "commitment": "confirmed"}}, &account, 30*time.Second); err != nil {
		base["ok"] = false
		base["phase"] = "verification_unavailable"
		base["error"] = "mint_account_unavailable"
		writeJSON(w, http.StatusServiceUnavailable, base)
		return
	}

	var supply publicTokenSupply
	if err := h.SolanaRPC.Call(ctx, network, "getTokenSupply", []any{mint, map[string]any{"commitment": "confirmed"}}, &supply, time.Minute); err != nil {
		base["ok"] = false
		base["phase"] = "verification_unavailable"
		base["error"] = "token_supply_unavailable"
		writeJSON(w, http.StatusServiceUnavailable, base)
		return
	}

	var largest publicTokenLargestAccounts
	if err := h.SolanaRPC.Call(ctx, network, "getTokenLargestAccounts", []any{mint, map[string]any{"commitment": "confirmed"}}, &largest, 5*time.Minute); err != nil {
		base["ok"] = false
		base["phase"] = "verification_unavailable"
		base["error"] = "holder_concentration_unavailable"
		writeJSON(w, http.StatusServiceUnavailable, base)
		return
	}

	standard := "unknown"
	switch account.Owner {
	case legacyTokenProgramID:
		standard = "spl-token"
	case token2022ProgramID:
		standard = "token-2022"
	}

	topAccounts := make([]map[string]any, 0, len(largest.Value))
	for index, item := range largest.Value {
		if index >= 20 {
			break
		}
		topAccounts = append(topAccounts, map[string]any{
			"token_account":   item.Address,
			"amount_raw":      item.Amount,
			"ui_amount_string": item.UIAmountString,
		})
	}

	base["phase"] = "live"
	base["onchain_verified"] = true
	base["standard"] = standard
	base["program_id"] = account.Owner
	base["supply"] = map[string]any{
		"amount_raw":       supply.Value.Amount,
		"ui_amount_string": supply.Value.UIAmountString,
		"decimals":         supply.Value.Decimals,
	}
	base["authorities"] = map[string]any{
		"mint_authority":          account.Data.Parsed.Info.MintAuthority,
		"freeze_authority":        account.Data.Parsed.Info.FreezeAuthority,
		"mint_authority_revoked":  account.Data.Parsed.Info.MintAuthority == nil,
		"freeze_authority_revoked": account.Data.Parsed.Info.FreezeAuthority == nil,
	}
	base["extensions"] = account.Data.Parsed.Info.Extensions
	base["concentration"] = map[string]any{
		"top_10_percentage": tokenAccountConcentration(largest.Value, supply.Value.Amount, 10),
		"top_20_percentage": tokenAccountConcentration(largest.Value, supply.Value.Amount, 20),
		"largest_token_accounts": topAccounts,
		"note": "RPC returns token accounts, not guaranteed unique beneficial owners.",
	}
	writeJSON(w, http.StatusOK, base)
}

func tokenAccountConcentration(accounts []struct {
	Address        string `json:"address"`
	Amount         string `json:"amount"`
	Decimals       int    `json:"decimals"`
	UIAmountString string `json:"uiAmountString"`
}, totalRaw string, limit int) float64 {
	total := new(big.Int)
	if _, ok := total.SetString(totalRaw, 10); !ok || total.Sign() <= 0 {
		return 0
	}
	if limit > len(accounts) {
		limit = len(accounts)
	}
	sum := new(big.Int)
	for _, account := range accounts[:limit] {
		value := new(big.Int)
		if _, ok := value.SetString(account.Amount, 10); ok {
			sum.Add(sum, value)
		}
	}
	ratio := new(big.Rat).SetFrac(sum, total)
	percentage, _ := new(big.Float).Mul(new(big.Float).SetRat(ratio), big.NewFloat(100)).Float64()
	return percentage
}
