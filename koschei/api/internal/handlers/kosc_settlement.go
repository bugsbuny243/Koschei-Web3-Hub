package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"strings"
	"time"

	"koschei/api/internal/services"
)

const canonicalKOSCMint = "7X9V77axASFAV8hKqqn2EfyAz4Qz3tceN8iikfukLqy1"

type koscSettlementRequest struct {
	QuoteID   string `json:"quote_id"`
	Signature string `json:"signature"`
}

type koscQuoteLedgerRow struct {
	Email      string
	Wallet     string
	Mint       string
	Treasury   string
	RawAmount  string
	AccessDays int
	Status     string
	ExpiresAt  time.Time
}

type koscSettlementEvidence struct {
	Slot              int64
	BlockTime         time.Time
	TreasuryRawDelta  *big.Int
	WalletRawDecrease *big.Int
}

type koscSignatureStatus struct {
	Slot               int64  `json:"slot"`
	Err                any    `json:"err"`
	ConfirmationStatus string `json:"confirmationStatus"`
}

type koscSignatureStatusesResult struct {
	Value []*koscSignatureStatus `json:"value"`
}

func (h *Handler) KOSCSettle(w http.ResponseWriter, r *http.Request) {
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
	if h == nil || h.DB == nil || h.SolanaRPC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "kosc_settlement_dependencies_unavailable"})
		return
	}
	store := h.entitlementStore()
	if store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "entitlement_store_unavailable"})
		return
	}

	var request koscSettlementRequest
	decoder := json.NewDecoder(io.LimitReader(r.Body, 8*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_settlement_request"})
		return
	}
	request.QuoteID = strings.TrimSpace(request.QuoteID)
	request.Signature = strings.TrimSpace(request.Signature)
	if !validKOSCQuoteID(request.QuoteID) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_quote_id"})
		return
	}
	if _, err := decodeBase58Exact(request.Signature, 64); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_transaction_signature"})
		return
	}

	email := strings.ToLower(strings.TrimSpace(claims.Email))
	subject := strings.TrimSpace(claims.Sub)
	if email == "" || subject == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "verified_identity_required"})
		return
	}
	wallet, err := h.koscVerifiedMainnetWallet(r.Context(), subject)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "verified_solana_wallet_required"})
		return
	}
	if err := verifyKOSCCommercialIdentity(r.Context(), store, subject, email); err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "commercial_identity_not_provisioned"})
		return
	}

	quote, err := loadKOSCQuote(r.Context(), store, request.QuoteID, subject)
	if errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "kosc_quote_not_found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "kosc_quote_ledger_unavailable"})
		return
	}
	if quote.Status != "open" {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "kosc_quote_not_open"})
		return
	}
	if time.Now().UTC().After(quote.ExpiresAt) {
		_, _ = store.ExecContext(r.Context(), "UPDATE kosc_payment_quotes SET status='expired', updated_at=now() WHERE id=$1::uuid AND status='open'", request.QuoteID)
		writeJSON(w, http.StatusConflict, map[string]string{"error": "kosc_quote_expired"})
		return
	}
	if !strings.EqualFold(quote.Email, email) || quote.Wallet != wallet || quote.Mint != cfg.Mint || quote.Treasury != cfg.Treasury || quote.AccessDays != cfg.AccessDays {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "kosc_quote_binding_mismatch"})
		return
	}
	requiredRaw := new(big.Int)
	if _, ok := requiredRaw.SetString(strings.TrimSpace(quote.RawAmount), 10); !ok || requiredRaw.Sign() <= 0 {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "kosc_quote_amount_invalid"})
		return
	}

	var signatureStatuses koscSignatureStatusesResult
	if err := h.SolanaRPC.Call(
		r.Context(),
		"solana-mainnet",
		"getSignatureStatuses",
		[]any{[]string{request.Signature}, map[string]any{"searchTransactionHistory": true}},
		&signatureStatuses,
		5*time.Second,
	); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "kosc_finality_unavailable"})
		return
	}
	finalizedSlot, err := validateKOSCFinalizedStatus(signatureStatuses)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "kosc_payment_not_finalized"})
		return
	}

	var txResult services.SolanaTransactionResult
	if err := h.SolanaRPC.Call(
		r.Context(),
		"solana-mainnet",
		"getTransaction",
		[]any{request.Signature, map[string]any{
			"encoding":                       "jsonParsed",
			"commitment":                     "finalized",
			"maxSupportedTransactionVersion": 1,
		}},
		&txResult,
		24*time.Hour,
	); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "kosc_transaction_unavailable"})
		return
	}
	if txResult == nil || creatorIntelInt64(txResult["slot"]) != finalizedSlot {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "kosc_finalized_slot_mismatch"})
		return
	}
	evidence, err := verifyKOSCSettlementTransaction(txResult, wallet, cfg.Mint, cfg.Treasury, requiredRaw)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "kosc_payment_not_verified"})
		return
	}

	dbtx, err := store.BeginTx(r.Context(), nil)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "kosc_settlement_ledger_unavailable"})
		return
	}
	defer dbtx.Rollback()

	var lockedStatus string
	var lockedExpires time.Time
	if err := dbtx.QueryRowContext(r.Context(), "SELECT status, expires_at FROM kosc_payment_quotes WHERE id=$1::uuid AND auth_subject=$2 FOR UPDATE", request.QuoteID, subject).Scan(&lockedStatus, &lockedExpires); err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "kosc_quote_lock_failed"})
		return
	}
	if lockedStatus != "open" || time.Now().UTC().After(lockedExpires) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "kosc_quote_not_open"})
		return
	}

	var blockTime any
	if !evidence.BlockTime.IsZero() {
		blockTime = evidence.BlockTime
	}
	if _, err := dbtx.ExecContext(r.Context(), "INSERT INTO kosc_payment_settlements (signature,quote_id,auth_subject,email,wallet_address,mint,treasury,raw_amount,slot,block_time,observed_at,created_at) VALUES ($1,$2::uuid,$3,lower($4),$5,$6,$7,$8::numeric,$9,$10,now(),now())", request.Signature, request.QuoteID, subject, email, wallet, cfg.Mint, cfg.Treasury, evidence.TreasuryRawDelta.String(), evidence.Slot, blockTime); err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "kosc_settlement_replay_or_conflict"})
		return
	}

	entitlementExpires := time.Now().UTC().Add(time.Duration(cfg.AccessDays) * 24 * time.Hour)
	activation, err := activatePackageEntitlementDetailedTx(r.Context(), dbtx, email, "professional", "kosc", request.Signature, "", "", "", entitlementExpires)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "kosc_entitlement_activation_failed"})
		return
	}
	if _, err := dbtx.ExecContext(r.Context(), "UPDATE kosc_payment_quotes SET status='settled', settled_at=now(), updated_at=now() WHERE id=$1::uuid AND status='open'", request.QuoteID); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "kosc_quote_finalize_failed"})
		return
	}
	if err := dbtx.Commit(); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "kosc_settlement_commit_failed"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":                  true,
		"schema_version":      "koschei-kosc-professional-settlement-v1",
		"quote_id":            request.QuoteID,
		"signature":           request.Signature,
		"plan":                activation.PackageID,
		"outputs_total":       activation.OutputsTotal,
		"outputs_remaining":   activation.OutputsRemaining,
		"access_expires_at":   entitlementExpires,
		"observed_slot":       evidence.Slot,
		"observed_raw_amount": evidence.TreasuryRawDelta.String(),
		"finalized":           true,
		"settlement_status":   "verified",
	})
}

func loadKOSCQuote(ctx context.Context, store *sql.DB, quoteID, subject string) (koscQuoteLedgerRow, error) {
	var row koscQuoteLedgerRow
	err := store.QueryRowContext(ctx, "SELECT lower(email),wallet_address,mint,treasury,raw_amount::text,access_days,status,expires_at FROM kosc_payment_quotes WHERE id=$1::uuid AND auth_subject=$2", quoteID, subject).Scan(&row.Email, &row.Wallet, &row.Mint, &row.Treasury, &row.RawAmount, &row.AccessDays, &row.Status, &row.ExpiresAt)
	return row, err
}

func validateKOSCFinalizedStatus(result koscSignatureStatusesResult) (int64, error) {
	if len(result.Value) != 1 || result.Value[0] == nil {
		return 0, errors.New("signature status unavailable")
	}
	status := result.Value[0]
	if status.Err != nil {
		return 0, errors.New("payment transaction failed")
	}
	if status.Slot <= 0 {
		return 0, errors.New("finalized slot unavailable")
	}
	if !strings.EqualFold(strings.TrimSpace(status.ConfirmationStatus), "finalized") {
		return 0, errors.New("payment transaction is not finalized")
	}
	return status.Slot, nil
}

func verifyKOSCSettlementTransaction(tx services.SolanaTransactionResult, wallet, mint, treasury string, requiredRaw *big.Int) (koscSettlementEvidence, error) {
	if requiredRaw == nil || requiredRaw.Sign() <= 0 {
		return koscSettlementEvidence{}, errors.New("required amount invalid")
	}
	slot := creatorIntelInt64(tx["slot"])
	if slot <= 0 {
		return koscSettlementEvidence{}, errors.New("transaction slot missing")
	}
	meta := creatorIntelMap(tx["meta"])
	if meta == nil || meta["err"] != nil {
		return koscSettlementEvidence{}, errors.New("transaction failed")
	}
	transaction := creatorIntelMap(tx["transaction"])
	message := creatorIntelMap(transaction["message"])
	_, signers := transactionInvestigationAccountKeys(message, meta)
	if !containsExactString(signers, wallet) {
		return koscSettlementEvidence{}, errors.New("verified wallet is not a signer")
	}
	treasuryPre, err := koscRawOwnerBalance(meta["preTokenBalances"], mint, treasury)
	if err != nil {
		return koscSettlementEvidence{}, err
	}
	treasuryPost, err := koscRawOwnerBalance(meta["postTokenBalances"], mint, treasury)
	if err != nil {
		return koscSettlementEvidence{}, err
	}
	walletPre, err := koscRawOwnerBalance(meta["preTokenBalances"], mint, wallet)
	if err != nil {
		return koscSettlementEvidence{}, err
	}
	walletPost, err := koscRawOwnerBalance(meta["postTokenBalances"], mint, wallet)
	if err != nil {
		return koscSettlementEvidence{}, err
	}
	treasuryDelta := new(big.Int).Sub(treasuryPost, treasuryPre)
	walletDecrease := new(big.Int).Sub(walletPre, walletPost)
	if treasuryDelta.Cmp(requiredRaw) < 0 || walletDecrease.Cmp(requiredRaw) < 0 {
		return koscSettlementEvidence{}, errors.New("verified payment amount is insufficient")
	}
	blockTime := time.Time{}
	if unix := creatorIntelInt64(tx["blockTime"]); unix > 0 {
		blockTime = time.Unix(unix, 0).UTC()
	}
	return koscSettlementEvidence{Slot: slot, BlockTime: blockTime, TreasuryRawDelta: treasuryDelta, WalletRawDecrease: walletDecrease}, nil
}

func koscRawOwnerBalance(raw any, mint, owner string) (*big.Int, error) {
	total := big.NewInt(0)
	items, _ := raw.([]any)
	for _, rawItem := range items {
		item := creatorIntelMap(rawItem)
		if creatorIntelCleanString(item["mint"]) != mint || creatorIntelCleanString(item["owner"]) != owner {
			continue
		}
		amount := strings.TrimSpace(creatorIntelCleanString(creatorIntelMap(item["uiTokenAmount"])["amount"]))
		if amount == "" {
			return nil, errors.New("token balance raw amount missing")
		}
		value := new(big.Int)
		if _, ok := value.SetString(amount, 10); !ok || value.Sign() < 0 {
			return nil, errors.New("token balance raw amount invalid")
		}
		total.Add(total, value)
	}
	return total, nil
}

func containsExactString(values []string, target string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == strings.TrimSpace(target) {
			return true
		}
	}
	return false
}

func validKOSCQuoteID(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 36 {
		return false
	}
	for index, char := range value {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			if char != '-' {
				return false
			}
			continue
		}
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F')) {
			return false
		}
	}
	return true
}
