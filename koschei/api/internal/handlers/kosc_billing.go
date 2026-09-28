package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"koschei/api/internal/services"
)

const (
	koscProfessionalPriceUSD   = "199"
	koscProfessionalAccessDays = 30
)

var (
	errKOSCCheckoutDisabled = errors.New("kosc checkout disabled")
	errKOSCPriceUnavailable = errors.New("kosc price unavailable")
)

type koscCheckoutConfig struct {
	Mint       string
	Treasury   string
	JupiterKey string
	AccessDays int
	QuoteTTL   time.Duration
}

type koscJupiterPrice struct {
	USDPrice string
	BlockID  int64
	Decimals int
}

func (h *Handler) KOSCQuote(w http.ResponseWriter, r *http.Request) {
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	cfg, err := loadKOSCCheckoutConfig()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "kosc_checkout_paused"})
		return
	}
	if h == nil || h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "wallet_identity_store_unavailable"})
		return
	}
	store := h.entitlementStore()
	if store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "entitlement_store_unavailable"})
		return
	}

	email := strings.ToLower(strings.TrimSpace(claims.Email))
	subject := strings.TrimSpace(claims.Sub)
	if email == "" || subject == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "verified_identity_required"})
		return
	}

	wallet, err := h.koscVerifiedMainnetWallet(r.Context(), subject)
	if errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "verified_solana_wallet_required"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "wallet_identity_unavailable"})
		return
	}
	if wallet == cfg.Treasury {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "payment_wallet_conflicts_with_treasury"})
		return
	}
	if err := verifyKOSCCommercialIdentity(r.Context(), store, subject, email); err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "commercial_identity_not_provisioned"})
		return
	}
	if h.SolanaRPC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "solana_rpc_unavailable"})
		return
	}

	rpcURL := strings.TrimSpace(h.SolanaRPC.URL("solana-mainnet"))
	supply, err := services.SolanaGetTokenSupply(r.Context(), rpcURL, cfg.Mint)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "kosc_mint_verification_unavailable"})
		return
	}
	chainDecimals := supply.Value.Decimals
	if chainDecimals < 0 || chainDecimals > 18 {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "kosc_mint_decimals_unsupported"})
		return
	}

	price, err := fetchKOSCJupiterPrice(r.Context(), cfg.JupiterKey, cfg.Mint)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "kosc_price_unavailable"})
		return
	}
	if price.Decimals != chainDecimals {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "kosc_price_chain_mismatch"})
		return
	}
	rawAmount, err := koscRawAmountForUSD(koscProfessionalPriceUSD, price.USDPrice, chainDecimals)
	if err != nil || rawAmount.Sign() <= 0 || len(rawAmount.String()) > 78 {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "kosc_quote_amount_unavailable"})
		return
	}

	quoteID, err := newUUID()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "quote_id_unavailable"})
		return
	}
	now := time.Now().UTC()
	expiresAt := now.Add(cfg.QuoteTTL)
	if _, err := store.ExecContext(r.Context(), `
		INSERT INTO kosc_payment_quotes
			(id,auth_subject,email,wallet_address,mint,treasury,usd_amount,price_usd,token_decimals,raw_amount,price_block_id,access_days,status,expires_at,created_at,updated_at)
		VALUES
			($1::uuid,$2,lower($3),$4,$5,$6,$7::numeric,$8::numeric,$9,$10::numeric,$11,$12,'open',$13,$14,$14)
	`, quoteID, subject, email, wallet, cfg.Mint, cfg.Treasury, koscProfessionalPriceUSD, price.USDPrice, chainDecimals, rawAmount.String(), price.BlockID, cfg.AccessDays, expiresAt, now); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "kosc_quote_ledger_unavailable"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":                true,
		"schema_version":    "koschei-kosc-professional-quote-v1",
		"quote_id":          quoteID,
		"plan":              "professional",
		"usd_price":         koscProfessionalPriceUSD,
		"mint":              cfg.Mint,
		"treasury":          cfg.Treasury,
		"verified_wallet":   wallet,
		"token_price_usd":   price.USDPrice,
		"price_block_id":    price.BlockID,
		"token_decimals":    chainDecimals,
		"raw_amount":        rawAmount.String(),
		"token_amount":      formatKOSCRawAmount(rawAmount, chainDecimals),
		"access_days":       cfg.AccessDays,
		"expires_at":        expiresAt,
		"settlement_status": "not_submitted",
	})
}

func loadKOSCCheckoutConfig() (koscCheckoutConfig, error) {
	if !strings.EqualFold(strings.TrimSpace(os.Getenv("KOSCHEI_KOSC_CHECKOUT_ENABLED")), "true") {
		return koscCheckoutConfig{}, errKOSCCheckoutDisabled
	}
	cfg := koscCheckoutConfig{
		Mint:       strings.TrimSpace(os.Getenv("KOSCHEI_TOKEN_MINT")),
		Treasury:   strings.TrimSpace(os.Getenv("KOSCHEI_TOKEN_TREASURY")),
		JupiterKey: strings.TrimSpace(os.Getenv("JUPITER_API_KEY")),
		QuoteTTL:   5 * time.Minute,
	}
	network, networkOK := normalizeWalletNetwork(os.Getenv("KOSCHEI_TOKEN_NETWORK"))
	if !networkOK || network != "solana-mainnet" || cfg.Mint == "" || cfg.Treasury == "" || cfg.JupiterKey == "" {
		return koscCheckoutConfig{}, errors.New("kosc checkout configuration incomplete")
	}
	if _, err := decodeSolanaPublicKey(cfg.Mint); err != nil {
		return koscCheckoutConfig{}, errors.New("kosc mint is invalid")
	}
	if cfg.Mint != canonicalKOSCMint {
		return koscCheckoutConfig{}, errors.New("configured kosc mint does not match canonical mint")
	}
	if _, err := decodeSolanaPublicKey(cfg.Treasury); err != nil {
		return koscCheckoutConfig{}, errors.New("kosc treasury is invalid")
	}
	days, err := strconv.Atoi(strings.TrimSpace(os.Getenv("KOSCHEI_KOSC_ACCESS_DAYS")))
	if err != nil || days != koscProfessionalAccessDays {
		return koscCheckoutConfig{}, errors.New("kosc access term must match canonical 30-day Professional contract")
	}
	cfg.AccessDays = koscProfessionalAccessDays
	if raw := strings.TrimSpace(os.Getenv("KOSCHEI_KOSC_QUOTE_TTL_SECONDS")); raw != "" {
		seconds, err := strconv.Atoi(raw)
		if err != nil || seconds < 60 || seconds > 900 {
			return koscCheckoutConfig{}, errors.New("kosc quote ttl is invalid")
		}
		cfg.QuoteTTL = time.Duration(seconds) * time.Second
	}
	return cfg, nil
}

func (h *Handler) koscVerifiedMainnetWallet(ctx context.Context, subject string) (string, error) {
	var wallet string
	err := h.DB.QueryRowContext(ctx, `
		SELECT wallet_address
		FROM verified_wallet_links
		WHERE auth_subject=$1 AND status='active' AND network='solana-mainnet'
		ORDER BY verified_at DESC
		LIMIT 1
	`, strings.TrimSpace(subject)).Scan(&wallet)
	if err != nil {
		return "", err
	}
	wallet = strings.TrimSpace(wallet)
	if _, err := decodeSolanaPublicKey(wallet); err != nil {
		return "", errors.New("verified wallet is invalid")
	}
	return wallet, nil
}

func verifyKOSCCommercialIdentity(ctx context.Context, store *sql.DB, subject, email string) error {
	var exists bool
	if err := store.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM app_user_profiles
			WHERE auth_subject=$1 AND lower(email)=lower($2) AND status='active'
		)
	`, strings.TrimSpace(subject), strings.TrimSpace(email)).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return errors.New("commercial identity missing")
	}
	return nil
}

func fetchKOSCJupiterPrice(ctx context.Context, apiKey, mint string) (koscJupiterPrice, error) {
	endpoint := "https://api.jup.ag/price/v3?ids=" + url.QueryEscape(strings.TrimSpace(mint))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return koscJupiterPrice{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("x-api-key", strings.TrimSpace(apiKey))
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return koscJupiterPrice{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return koscJupiterPrice{}, fmt.Errorf("jupiter price status %d", resp.StatusCode)
	}
	limited := &io.LimitedReader{R: resp.Body, N: 64*1024 + 1}
	body, err := io.ReadAll(limited)
	if err != nil {
		return koscJupiterPrice{}, err
	}
	if limited.N <= 0 {
		return koscJupiterPrice{}, errors.New("jupiter price response too large")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var payload map[string]struct {
		USDPrice json.Number `json:"usdPrice"`
		BlockID  int64       `json:"blockId"`
		Decimals int         `json:"decimals"`
	}
	if err := decoder.Decode(&payload); err != nil {
		return koscJupiterPrice{}, err
	}
	entry, ok := payload[strings.TrimSpace(mint)]
	if !ok || strings.TrimSpace(entry.USDPrice.String()) == "" || entry.BlockID <= 0 || entry.Decimals < 0 {
		return koscJupiterPrice{}, errKOSCPriceUnavailable
	}
	price, err := koscDecimalRat(entry.USDPrice.String())
	if err != nil || price.Sign() <= 0 {
		return koscJupiterPrice{}, errKOSCPriceUnavailable
	}
	return koscJupiterPrice{USDPrice: entry.USDPrice.String(), BlockID: entry.BlockID, Decimals: entry.Decimals}, nil
}

func koscRawAmountForUSD(usd, tokenPrice string, decimals int) (*big.Int, error) {
	if decimals < 0 || decimals > 18 {
		return nil, errors.New("token decimals out of range")
	}
	usdRat, err := koscDecimalRat(usd)
	if err != nil || usdRat.Sign() <= 0 {
		return nil, errors.New("usd amount invalid")
	}
	priceRat, err := koscDecimalRat(tokenPrice)
	if err != nil || priceRat.Sign() <= 0 {
		return nil, errors.New("token price invalid")
	}
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
	required := new(big.Rat).Mul(usdRat, new(big.Rat).SetInt(scale))
	required.Quo(required, priceRat)
	quotient, remainder := new(big.Int).QuoRem(new(big.Int).Set(required.Num()), new(big.Int).Set(required.Denom()), new(big.Int))
	if remainder.Sign() > 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	return quotient, nil
}

func koscDecimalRat(raw string) (*big.Rat, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, errors.New("decimal is empty")
	}
	sign := 1
	if value[0] == '+' || value[0] == '-' {
		if value[0] == '-' {
			sign = -1
		}
		value = value[1:]
	}
	exponent := 0
	if index := strings.IndexAny(value, "eE"); index >= 0 {
		parsed, err := strconv.Atoi(value[index+1:])
		if err != nil {
			return nil, errors.New("decimal exponent invalid")
		}
		exponent = parsed
		value = value[:index]
	}
	parts := strings.Split(value, ".")
	if len(parts) > 2 {
		return nil, errors.New("decimal point invalid")
	}
	whole := parts[0]
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
	}
	if whole == "" {
		whole = "0"
	}
	digits := whole + fraction
	if digits == "" {
		return nil, errors.New("decimal digits missing")
	}
	for _, char := range digits {
		if char < '0' || char > '9' {
			return nil, errors.New("decimal contains non-digit")
		}
	}
	numerator := new(big.Int)
	if _, ok := numerator.SetString(digits, 10); !ok {
		return nil, errors.New("decimal numerator invalid")
	}
	if sign < 0 {
		numerator.Neg(numerator)
	}
	denominator := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(len(fraction))), nil)
	if exponent > 0 {
		numerator.Mul(numerator, new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(exponent)), nil))
	} else if exponent < 0 {
		denominator.Mul(denominator, new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(-exponent)), nil))
	}
	return new(big.Rat).SetFrac(numerator, denominator), nil
}

func formatKOSCRawAmount(raw *big.Int, decimals int) string {
	if raw == nil {
		return ""
	}
	if decimals <= 0 {
		return raw.String()
	}
	value := raw.String()
	for len(value) <= decimals {
		value = "0" + value
	}
	split := len(value) - decimals
	whole, fraction := value[:split], value[split:]
	fraction = strings.TrimRight(fraction, "0")
	if fraction == "" {
		return whole
	}
	return whole + "." + fraction
}
