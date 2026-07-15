package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type tokenAccessError struct {
	Status int
	Code   string
}

func (e tokenAccessError) Error() string { return e.Code }

type tokenAccessEvaluation struct {
	GateEnabled     bool              `json:"gate_enabled"`
	Configured      bool              `json:"configured"`
	WalletVerified  bool              `json:"wallet_verified"`
	WalletAddress   string            `json:"wallet_address,omitempty"`
	Network         string            `json:"network"`
	MintAddress     string            `json:"mint_address,omitempty"`
	AmountRaw       string            `json:"amount_raw"`
	Amount          string            `json:"amount"`
	Decimals        int               `json:"decimals"`
	Tier            string            `json:"tier"`
	Thresholds      map[string]string `json:"thresholds,omitempty"`
	CheckedAt       *time.Time        `json:"checked_at,omitempty"`
	SnapshotExpires *time.Time        `json:"snapshot_expires_at,omitempty"`
}

type tokenAccessContextKey struct{}

func tokenAccessFromContext(ctx context.Context) (tokenAccessEvaluation, bool) {
	evaluation, ok := ctx.Value(tokenAccessContextKey{}).(tokenAccessEvaluation)
	return evaluation, ok
}

type tokenAccessSupplyResult struct {
	Value struct {
		Amount         string `json:"amount"`
		Decimals       int    `json:"decimals"`
		UIAmountString string `json:"uiAmountString"`
	} `json:"value"`
}

type tokenAccessAccountsResult struct {
	Value []struct {
		Pubkey  string `json:"pubkey"`
		Account struct {
			Data struct {
				Parsed struct {
					Info struct {
						Mint        string `json:"mint"`
						TokenAmount struct {
							Amount         string `json:"amount"`
							Decimals       int    `json:"decimals"`
							UIAmountString string `json:"uiAmountString"`
						} `json:"tokenAmount"`
					} `json:"info"`
				} `json:"parsed"`
			} `json:"data"`
		} `json:"account"`
	} `json:"value"`
}

func (h *Handler) TokenAccessStatus(w http.ResponseWriter, r *http.Request) {
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	evaluation, err := h.evaluateTokenAccess(r.Context(), claims.Sub)
	if err != nil {
		if accessErr, ok := err.(tokenAccessError); ok {
			writeJSON(w, accessErr.Status, map[string]string{"error": accessErr.Code})
			return
		}
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "token_access_unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "access": evaluation})
}

func (h *Handler) RequireTokenTier(required string, next http.HandlerFunc) http.HandlerFunc {
	required = strings.ToLower(strings.TrimSpace(required))
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := userFromContext(r.Context())
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		if tokenTierRank(required) == 0 {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "invalid_token_tier_configuration"})
			return
		}
		evaluation, err := h.evaluateTokenAccess(r.Context(), claims.Sub)
		if err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "token_access_unavailable"})
			return
		}
		if !evaluation.GateEnabled {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "token_gate_disabled"})
			return
		}
		if !evaluation.WalletVerified {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "verified_wallet_required"})
			return
		}
		if tokenTierRank(evaluation.Tier) < tokenTierRank(required) {
			writeJSON(w, http.StatusForbidden, map[string]any{
				"error":         "token_tier_required",
				"required_tier": required,
				"current_tier":  evaluation.Tier,
			})
			return
		}
		ctx := context.WithValue(r.Context(), tokenAccessContextKey{}, evaluation)
		next(w, r.WithContext(ctx))
	}
}

func (h *Handler) evaluateTokenAccess(ctx context.Context, authSubject string) (tokenAccessEvaluation, error) {
	mint := configuredKoscheiTokenMint()
	network, validNetwork := normalizeWalletNetwork(firstNonEmptyString(os.Getenv("KOSCHEI_TOKEN_NETWORK"), os.Getenv("KOSCH_TOKEN_NETWORK"), "solana-mainnet"))
	if !validNetwork {
		return tokenAccessEvaluation{}, tokenAccessError{Status: http.StatusServiceUnavailable, Code: "invalid_token_network_configuration"}
	}
	gateEnabled := configuredKoscheiTokenGateEnabled()
	evaluation := tokenAccessEvaluation{
		GateEnabled: gateEnabled,
		Configured:  mint != "",
		Network:     network,
		MintAddress: mint,
		AmountRaw:   "0",
		Amount:      "0",
		Tier:        "none",
	}

	var verifiedAt time.Time
	err := h.DB.QueryRowContext(ctx, `
		SELECT wallet_address,verified_at
		FROM verified_wallet_links
		WHERE auth_subject=$1 AND network=$2 AND status='active'
		ORDER BY verified_at DESC
		LIMIT 1`, authSubject, network).Scan(&evaluation.WalletAddress, &verifiedAt)
	if err == sql.ErrNoRows {
		return evaluation, nil
	}
	if err != nil {
		return tokenAccessEvaluation{}, err
	}
	evaluation.WalletVerified = true
	if !gateEnabled {
		return evaluation, nil
	}
	if mint == "" {
		return tokenAccessEvaluation{}, tokenAccessError{Status: http.StatusServiceUnavailable, Code: "token_mint_not_configured"}
	}
	if _, err := decodeSolanaPublicKey(mint); err != nil {
		return tokenAccessEvaluation{}, tokenAccessError{Status: http.StatusServiceUnavailable, Code: "invalid_token_mint_configuration"}
	}
	if h.SolanaRPC == nil {
		return tokenAccessEvaluation{}, tokenAccessError{Status: http.StatusServiceUnavailable, Code: "solana_rpc_unavailable"}
	}

	var supply tokenAccessSupplyResult
	if err := h.SolanaRPC.Call(ctx, network, "getTokenSupply", []any{mint, map[string]any{"commitment": "confirmed"}}, &supply, time.Minute); err != nil {
		return tokenAccessEvaluation{}, tokenAccessError{Status: http.StatusServiceUnavailable, Code: "token_supply_unavailable"}
	}
	if supply.Value.Decimals < 0 || supply.Value.Decimals > 18 {
		return tokenAccessEvaluation{}, tokenAccessError{Status: http.StatusServiceUnavailable, Code: "unsupported_token_decimals"}
	}
	evaluation.Decimals = supply.Value.Decimals

	thresholds, rawThresholds, err := configuredTokenThresholds(evaluation.Decimals)
	if err != nil {
		return tokenAccessEvaluation{}, tokenAccessError{Status: http.StatusServiceUnavailable, Code: "invalid_token_tier_configuration"}
	}
	evaluation.Thresholds = thresholds

	var accounts tokenAccessAccountsResult
	params := []any{
		evaluation.WalletAddress,
		map[string]any{"mint": mint},
		map[string]any{"encoding": "jsonParsed", "commitment": "confirmed"},
	}
	if err := h.SolanaRPC.Call(ctx, network, "getTokenAccountsByOwner", params, &accounts, 30*time.Second); err != nil {
		return tokenAccessEvaluation{}, tokenAccessError{Status: http.StatusServiceUnavailable, Code: "token_balance_unavailable"}
	}

	total := new(big.Int)
	for _, account := range accounts.Value {
		if account.Account.Data.Parsed.Info.Mint != "" && account.Account.Data.Parsed.Info.Mint != mint {
			continue
		}
		amount := new(big.Int)
		if _, ok := amount.SetString(strings.TrimSpace(account.Account.Data.Parsed.Info.TokenAmount.Amount), 10); ok && amount.Sign() >= 0 {
			total.Add(total, amount)
		}
	}
	evaluation.AmountRaw = total.String()
	evaluation.Amount = formatTokenAmount(total, evaluation.Decimals)
	evaluation.Tier = evaluateTokenTier(total, rawThresholds)
	checkedAt := time.Now().UTC()
	expiresAt := checkedAt.Add(2 * time.Minute)
	evaluation.CheckedAt = &checkedAt
	evaluation.SnapshotExpires = &expiresAt

	_, _ = h.DB.ExecContext(ctx, `
		INSERT INTO token_access_snapshots
		(auth_subject,wallet_address,network,mint_address,amount_raw,decimals,tier,gate_enabled,checked_at,expires_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		authSubject,
		evaluation.WalletAddress,
		network,
		mint,
		evaluation.AmountRaw,
		evaluation.Decimals,
		evaluation.Tier,
		gateEnabled,
		checkedAt,
		expiresAt,
	)
	return evaluation, nil
}

func configuredKoscheiTokenMint() string {
	return strings.TrimSpace(firstNonEmptyString(os.Getenv("KOSCHEI_TOKEN_MINT"), os.Getenv("KOSCH_TOKEN_MINT"), officialKOSCHMint))
}

func configuredKoscheiTokenGateEnabled() bool {
	value := strings.TrimSpace(os.Getenv("KOSCHEI_TOKEN_GATE_ENABLED"))
	if value == "" {
		return true
	}
	enabled, err := strconv.ParseBool(value)
	if err != nil {
		return false
	}
	return enabled
}

func configuredTokenThresholds(decimals int) (map[string]string, map[string]*big.Int, error) {
	values := map[string]string{
		"basic":      tokenTierThresholdEnv("KOSCHEI_TOKEN_TIER_BASIC", "25000"),
		"pro":        tokenTierThresholdEnv("KOSCHEI_TOKEN_TIER_PRO", "250000"),
		"enterprise": tokenTierThresholdEnv("KOSCHEI_TOKEN_TIER_ENTERPRISE", "2000000"),
	}
	raw := map[string]*big.Int{}
	for _, tier := range []string{"basic", "pro", "enterprise"} {
		if values[tier] == "" {
			return nil, nil, fmt.Errorf("missing %s threshold", tier)
		}
		amount, err := parseTokenAmount(values[tier], decimals)
		if err != nil || amount.Sign() <= 0 {
			return nil, nil, fmt.Errorf("invalid %s threshold", tier)
		}
		raw[tier] = amount
	}
	if raw["basic"].Cmp(raw["pro"]) > 0 || raw["pro"].Cmp(raw["enterprise"]) > 0 {
		return nil, nil, fmt.Errorf("threshold order is invalid")
	}
	return values, raw, nil
}

func tokenTierThresholdEnv(name, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value != "" {
		return value
	}
	return fallback
}

func parseTokenAmount(value string, decimals int) (*big.Int, error) {
	value = strings.TrimSpace(value)
	if value == "" || strings.HasPrefix(value, "-") || strings.HasPrefix(value, "+") {
		return nil, fmt.Errorf("invalid token amount")
	}
	parts := strings.Split(value, ".")
	if len(parts) > 2 {
		return nil, fmt.Errorf("invalid token amount")
	}
	whole := parts[0]
	if whole == "" {
		whole = "0"
	}
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
	}
	if len(fraction) > decimals {
		return nil, fmt.Errorf("too many decimal places")
	}
	for _, char := range whole + fraction {
		if char < '0' || char > '9' {
			return nil, fmt.Errorf("invalid token amount")
		}
	}
	fraction += strings.Repeat("0", decimals-len(fraction))
	rawText := strings.TrimLeft(whole+fraction, "0")
	if rawText == "" {
		rawText = "0"
	}
	raw := new(big.Int)
	if _, ok := raw.SetString(rawText, 10); !ok {
		return nil, fmt.Errorf("invalid token amount")
	}
	return raw, nil
}

func formatTokenAmount(raw *big.Int, decimals int) string {
	if raw == nil {
		return "0"
	}
	text := raw.String()
	if decimals == 0 {
		return text
	}
	if len(text) <= decimals {
		text = strings.Repeat("0", decimals-len(text)+1) + text
	}
	position := len(text) - decimals
	formatted := text[:position] + "." + text[position:]
	formatted = strings.TrimRight(formatted, "0")
	formatted = strings.TrimRight(formatted, ".")
	if formatted == "" {
		return "0"
	}
	return formatted
}

func evaluateTokenTier(amount *big.Int, thresholds map[string]*big.Int) string {
	if amount == nil {
		return "none"
	}
	if threshold := thresholds["enterprise"]; threshold != nil && amount.Cmp(threshold) >= 0 {
		return "enterprise"
	}
	if threshold := thresholds["pro"]; threshold != nil && amount.Cmp(threshold) >= 0 {
		return "pro"
	}
	if threshold := thresholds["basic"]; threshold != nil && amount.Cmp(threshold) >= 0 {
		return "basic"
	}
	return "none"
}

func tokenTierRank(tier string) int {
	switch strings.ToLower(strings.TrimSpace(tier)) {
	case "enterprise":
		return 3
	case "pro":
		return 2
	case "basic":
		return 1
	default:
		return 0
	}
}
