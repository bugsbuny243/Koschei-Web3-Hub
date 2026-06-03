package handlers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type web3Event struct {
	ID                 string    `json:"id"`
	SourceID           *string   `json:"source_id,omitempty"`
	SourceName         *string   `json:"source_name,omitempty"`
	UserID             *string   `json:"user_id,omitempty"`
	Email              *string   `json:"email,omitempty"`
	Provider           string    `json:"provider"`
	Chain              *string   `json:"chain,omitempty"`
	Network            *string   `json:"network,omitempty"`
	EventType          *string   `json:"event_type,omitempty"`
	Address            *string   `json:"address,omitempty"`
	TxHash             *string   `json:"tx_hash,omitempty"`
	BlockNumber        *string   `json:"block_number,omitempty"`
	Direction          *string   `json:"direction,omitempty"`
	AssetType          *string   `json:"asset_type,omitempty"`
	Amount             *string   `json:"amount,omitempty"`
	WalletAddress      *string   `json:"wallet_address,omitempty"`
	ContractAddress    *string   `json:"contract_address,omitempty"`
	TokenID            *string   `json:"token_id,omitempty"`
	AmountText         *string   `json:"amount_text,omitempty"`
	RawPayload         any       `json:"raw_payload,omitempty"`
	AISummary          *string   `json:"ai_summary,omitempty"`
	RiskLevel          string    `json:"risk_level"`
	Status             string    `json:"status"`
	VerificationStatus string    `json:"verification_status"`
	PayloadHash        *string   `json:"payload_hash,omitempty"`
	ErrorMessage       *string   `json:"error_message,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
}

type web3EventSource struct {
	ID                  string     `json:"id"`
	UserID              *string    `json:"user_id,omitempty"`
	Email               string     `json:"email"`
	Name                string     `json:"name"`
	Label               string     `json:"label"`
	Provider            string     `json:"provider"`
	Chain               string     `json:"chain"`
	Network             string     `json:"network"`
	Address             string     `json:"address"`
	SourceType          string     `json:"source_type"`
	Notes               *string    `json:"notes,omitempty"`
	Status              string     `json:"status"`
	ProviderSetupStatus string     `json:"provider_setup_status"`
	WebhookURL          *string    `json:"webhook_url,omitempty"`
	IsActive            bool       `json:"is_active"`
	VerificationMode    string     `json:"verification_mode"`
	LastEventAt         *time.Time `json:"last_event_at,omitempty"`
	DisabledReason      *string    `json:"disabled_reason,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

type webhookSource struct {
	ID               string
	UserID           sql.NullString
	Name             string
	Provider         string
	Network          string
	SecretHash       sql.NullString
	VerificationMode string
}

func (h *Handler) Web3AlchemyEvent(w http.ResponseWriter, r *http.Request) {
	// TODO: Add official Alchemy signature verification when a provider-specific
	// signature header is configured for this webhook source.
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}

	payloadHash := sha256Hex(raw)
	network := firstNonEmptyString(findStringByKeys(payload, "network"), findStringByKeys(payload, "chain", "blockchain"))
	sourceID := firstNonEmptyString(r.URL.Query().Get("source_id"), r.Header.Get("X-Koschei-Source-Id"))
	source, status, err := h.findWebhookSource(sourceID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}
	if status != "verified" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": status})
		return
	}

	providedSecret := webhookSecretFromRequest(r)
	if !source.SecretHash.Valid || strings.TrimSpace(source.SecretHash.String) == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unconfigured_source"})
		return
	}
	if providedSecret == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing_secret"})
		return
	}
	if !constantTimeEqualSHA256(providedSecret, strings.TrimSpace(source.SecretHash.String)) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid_secret"})
		return
	}

	eventID := newID()
	eventType := firstNonEmptyString(findStringByKeys(payload, "event_type", "eventType"), findStringByKeys(payload, "type", "category"))
	txHash := findStringByKeys(payload, "tx_hash", "txHash", "transactionHash", "hash")
	walletAddress := findStringByKeys(payload, "wallet_address", "walletAddress", "fromAddress", "toAddress", "from", "to", "address")
	contractAddress := findStringByKeys(payload, "contract_address", "contractAddress", "rawContract", "contract")
	receivedIP := requestIP(r)
	userAgent := r.UserAgent()

	if _, err := h.DB.Exec(`
		INSERT INTO web3_events (id, source_id, source_name, user_id, provider, network, event_type, tx_hash, wallet_address, contract_address, raw_payload, risk_level, status, verification_status, received_ip, received_user_agent, payload_hash)
		VALUES ($1,$2,$3,$4,'alchemy',$5,$6,$7,$8,$9,$10::jsonb,'unknown','received','verified',$11,$12,$13)`,
		eventID, source.ID, source.Name, nullStringToAny(source.UserID), nullIfEmpty(firstNonEmptyString(network, source.Network)), nullIfEmpty(eventType), nullIfEmpty(txHash), nullIfEmpty(walletAddress), nullIfEmpty(contractAddress), string(raw), nullIfEmpty(receivedIP), nullIfEmpty(userAgent), payloadHash); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}
	if _, err := h.DB.Exec(`UPDATE web3_event_sources SET last_event_at=NOW(), updated_at=NOW() WHERE id=$1`, source.ID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "event_id": eventID, "verification_status": "verified"})
}

func (h *Handler) findWebhookSource(sourceID string) (webhookSource, string, error) {
	var source webhookSource
	if strings.TrimSpace(sourceID) == "" {
		return source, "unconfigured_source", nil
	}
	err := h.DB.QueryRow(`
		SELECT id, user_id, name, provider, network, secret_hash, verification_mode
		FROM web3_event_sources
		WHERE id=$1 AND provider='alchemy' AND is_active=true`, strings.TrimSpace(sourceID)).Scan(&source.ID, &source.UserID, &source.Name, &source.Provider, &source.Network, &source.SecretHash, &source.VerificationMode)
	if err == sql.ErrNoRows {
		return source, "unconfigured_source", nil
	}
	if err != nil {
		return source, "", err
	}
	return source, "verified", nil
}

func (h *Handler) Web3Events(w http.ResponseWriter, r *http.Request) {
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	email := normalizedClaimEmail(claims)
	if email == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	limit := 50
	if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
		if parsed, err := strconv.Atoi(rawLimit); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}
	rows, err := h.DB.Query(`
		SELECT id, source_id, source_name, user_id, email, provider, chain, network, event_type, address, tx_hash, block_number, direction, asset_type, amount, wallet_address, contract_address, token_id, amount_text, raw_payload, ai_summary, risk_level, status, verification_status, payload_hash, error_message, created_at
		FROM web3_events
		WHERE lower(email)=lower($1)
		ORDER BY created_at DESC
		LIMIT $2`, email, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}
	defer rows.Close()

	events := make([]web3Event, 0, limit)
	for rows.Next() {
		event, scanErr := scanWeb3Event(rows)
		if scanErr != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
			return
		}
		events = append(events, event)
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": events})
}

func (h *Handler) Web3Sources(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		h.listWeb3Sources(w, r)
		return
	}
	if r.Method == http.MethodPost {
		h.createWeb3Source(w, r)
		return
	}
	w.WriteHeader(http.StatusMethodNotAllowed)
}

func (h *Handler) Web3Source(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost && strings.HasSuffix(strings.TrimRight(r.URL.Path, "/"), "/sync") {
		h.syncWeb3Source(w, r)
		return
	}
	if r.Method == http.MethodPatch {
		h.patchWeb3Source(w, r)
		return
	}
	w.WriteHeader(http.StatusMethodNotAllowed)
}

func (h *Handler) listWeb3Sources(w http.ResponseWriter, r *http.Request) {
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	email := normalizedClaimEmail(claims)
	if email == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	rows, err := h.DB.Query(`
		SELECT id, user_id, COALESCE(email,''), name, COALESCE(label, name), provider, COALESCE(chain,''), network, COALESCE(address,''), COALESCE(source_type,'wallet'), notes, COALESCE(status, CASE WHEN COALESCE(is_active,true) THEN 'active' ELSE 'inactive' END), COALESCE(provider_setup_status, verification_mode), webhook_url, COALESCE(is_active, true), verification_mode, last_event_at, disabled_reason, created_at, updated_at
		FROM web3_event_sources
		WHERE lower(email)=lower($1)
		ORDER BY created_at DESC`, email)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}
	defer rows.Close()

	sources := []web3EventSource{}
	for rows.Next() {
		source, scanErr := scanWeb3Source(rows)
		if scanErr != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
			return
		}
		sources = append(sources, source)
	}
	writeJSON(w, http.StatusOK, map[string]any{"sources": sources, "limits": map[string]int{"free": 1, "starter": 3, "builder": 10, "studio": 50}})
}

type web3SourceRequest struct {
	Label      string `json:"label"`
	Name       string `json:"name"`
	Chain      string `json:"chain"`
	Network    string `json:"network"`
	Address    string `json:"address"`
	SourceType string `json:"source_type"`
	Notes      string `json:"notes"`
	Provider   string `json:"provider"`
	WebhookURL string `json:"webhook_url"`
	Secret     string `json:"secret"`
}

func (h *Handler) createWeb3Source(w http.ResponseWriter, r *http.Request) {
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	email := normalizedClaimEmail(claims)
	if email == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	var req web3SourceRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}
	label := firstNonEmptyString(req.Label, req.Name)
	chain := strings.ToLower(firstNonEmptyString(req.Chain, "base"))
	network := strings.ToLower(firstNonEmptyString(req.Network, "base-mainnet"))
	address := strings.ToLower(strings.TrimSpace(req.Address))
	sourceType := strings.ToLower(firstNonEmptyString(req.SourceType, "wallet"))
	provider := strings.ToLower(firstNonEmptyString(req.Provider, "alchemy"))
	if label == "" || address == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "label_and_address_required"})
		return
	}
	if sourceType != "wallet" && sourceType != "contract" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unsupported_source_type"})
		return
	}
	if provider != "alchemy" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unsupported_provider"})
		return
	}
	if !isSupportedAlchemyNetwork(chain, network) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unsupported_network"})
		return
	}
	limit, err := h.watchlistSourceLimit(r.Context(), email)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}
	var current int
	if err := h.DB.QueryRow(`SELECT COUNT(*) FROM web3_event_sources WHERE lower(email)=lower($1) AND COALESCE(status,'active')='active'`, email).Scan(&current); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}
	if current >= limit {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "watchlist_source_limit_reached", "limit": limit})
		return
	}

	sourceID := newID()
	userID := currentUserID(claims)
	_, err = h.DB.Exec(`
		INSERT INTO web3_event_sources (id, user_id, email, name, label, provider, chain, network, address, source_type, notes, status, provider_setup_status, webhook_url, is_active, verification_mode, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$4,$5,$6,$7,$8,$9,$10,'active','api_polling',NULL,true,'api_polling',NOW(),NOW())`,
		sourceID, userID, email, label, provider, chain, network, address, sourceType, nullIfEmpty(req.Notes))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}
	source, err := h.getWeb3SourceForUser(sourceID, email)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"source":     source,
		"ok":         true,
		"setup_mode": "api_polling",
		"message":    "Webhook setup is not required. Koschei checks public activity through Alchemy API.",
	})
}

type web3SourcePatchRequest struct {
	Label      *string `json:"label"`
	Name       *string `json:"name"`
	Network    *string `json:"network"`
	IsActive   *bool   `json:"is_active"`
	WebhookURL *string `json:"webhook_url"`
	Secret     *string `json:"secret"`
}

func (h *Handler) patchWeb3Source(w http.ResponseWriter, r *http.Request) {
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/web3/sources/"), "/")
	if id == "" || strings.Contains(id, "/") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_source_id"})
		return
	}
	var req web3SourcePatchRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}
	email := normalizedClaimEmail(claims)
	if email == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	if _, err := h.getWeb3SourceForUser(id, email); err == sql.ErrNoRows {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
		return
	} else if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}

	if req.Label != nil || req.Name != nil {
		name := ""
		if req.Label != nil {
			name = strings.TrimSpace(*req.Label)
		} else {
			name = strings.TrimSpace(*req.Name)
		}
		if name == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_name"})
			return
		}
		if _, err := h.DB.Exec(`UPDATE web3_event_sources SET name=$3, label=$3, updated_at=NOW() WHERE id=$1 AND lower(email)=lower($2)`, id, email, name); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
			return
		}
	}
	if req.Network != nil {
		network := strings.TrimSpace(*req.Network)
		if network == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_network"})
			return
		}
		if _, err := h.DB.Exec(`UPDATE web3_event_sources SET network=$3, updated_at=NOW() WHERE id=$1 AND lower(email)=lower($2)`, id, email, network); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
			return
		}
	}
	if req.IsActive != nil {
		status := "inactive"
		if *req.IsActive {
			status = "active"
		}
		if _, err := h.DB.Exec(`UPDATE web3_event_sources SET is_active=$3, status=$4, updated_at=NOW() WHERE id=$1 AND lower(email)=lower($2)`, id, email, *req.IsActive, status); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
			return
		}
	}
	if req.WebhookURL != nil {
		if _, err := h.DB.Exec(`UPDATE web3_event_sources SET webhook_url=$3, updated_at=NOW() WHERE id=$1 AND lower(email)=lower($2)`, id, email, nullIfEmpty(*req.WebhookURL)); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
			return
		}
	}
	if req.Secret != nil {
		secret := strings.TrimSpace(*req.Secret)
		if secret == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_secret"})
			return
		}
		if _, err := h.DB.Exec(`UPDATE web3_event_sources SET secret_hash=$3, updated_at=NOW() WHERE id=$1 AND lower(email)=lower($2)`, id, email, sha256Hex([]byte(secret))); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
			return
		}
	}
	source, err := h.getWeb3SourceForUser(id, email)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"source": source})
}

func (h *Handler) Web3TestEvent(w http.ResponseWriter, r *http.Request) {
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	userID := currentUserID(claims)
	eventID := newID()
	rawPayload := map[string]any{
		"demo":            true,
		"safety_boundary": "read_only_monitoring_no_custody",
		"network":         "solana-devnet",
		"event_type":      "test_event",
	}
	raw, _ := json.Marshal(rawPayload)
	payloadHash := sha256Hex(raw)
	if _, err := h.DB.Exec(`
		INSERT INTO web3_events (id, user_id, email, provider, network, event_type, tx_hash, wallet_address, contract_address, raw_payload, risk_level, status, verification_status, payload_hash)
		VALUES ($1,$2,$3,'alchemy','solana-devnet','test_event',$4,$5,$6,$7::jsonb,'low','received','verified',$8)`,
		eventID, userID, normalizedClaimEmail(claims), "demo-tx-"+eventID[:8], "demo-wallet-read-only", "demo-contract-read-only", string(raw), payloadHash); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}

	event, err := h.getWeb3EventForUser(eventID, normalizedClaimEmail(claims))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"event": event})
}

func (h *Handler) Web3ExplainEvent(w http.ResponseWriter, r *http.Request) {
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/web3/events/"), "/")
	if !strings.HasSuffix(path, "/explain") {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
		return
	}
	id := strings.TrimSuffix(path, "/explain")
	id = strings.Trim(id, "/")
	if id == "" || strings.Contains(id, "/") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_event_id"})
		return
	}

	email := normalizedClaimEmail(claims)
	event, err := h.getWeb3EventForUser(id, email)
	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}

	summary := deterministicWeb3Summary(event)
	if _, err := h.DB.Exec(`UPDATE web3_events SET ai_summary=$2 WHERE id=$1 AND lower(email)=lower($3)`, id, summary, email); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}
	event.AISummary = &summary
	writeJSON(w, http.StatusOK, map[string]any{"event": event, "summary": summary})
}

func (h *Handler) getWeb3EventForUser(id, email string) (web3Event, error) {
	row := h.DB.QueryRow(`
		SELECT id, source_id, source_name, user_id, email, provider, chain, network, event_type, address, tx_hash, block_number, direction, asset_type, amount, wallet_address, contract_address, token_id, amount_text, raw_payload, ai_summary, risk_level, status, verification_status, payload_hash, error_message, created_at
		FROM web3_events
		WHERE id=$1 AND lower(email)=lower($2)`, id, email)
	return scanWeb3Event(row)
}

func (h *Handler) getWeb3SourceForUser(id, email string) (web3EventSource, error) {
	row := h.DB.QueryRow(`
		SELECT id, user_id, COALESCE(email,''), name, COALESCE(label, name), provider, COALESCE(chain,''), network, COALESCE(address,''), COALESCE(source_type,'wallet'), notes, COALESCE(status, CASE WHEN COALESCE(is_active,true) THEN 'active' ELSE 'inactive' END), COALESCE(provider_setup_status, verification_mode), webhook_url, COALESCE(is_active, true), verification_mode, last_event_at, disabled_reason, created_at, updated_at
		FROM web3_event_sources
		WHERE id=$1 AND lower(email)=lower($2)`, id, email)
	return scanWeb3Source(row)
}

type web3EventScanner interface {
	Scan(dest ...any) error
}

func scanWeb3Event(scanner web3EventScanner) (web3Event, error) {
	var event web3Event
	var sourceID, sourceName, userID, email, chain, network, eventType, address, txHash, blockNumber, direction, assetType, amount, walletAddress, contractAddress, tokenID, amountText, aiSummary, payloadHash, errorMessage sql.NullString
	var raw []byte
	if err := scanner.Scan(&event.ID, &sourceID, &sourceName, &userID, &email, &event.Provider, &chain, &network, &eventType, &address, &txHash, &blockNumber, &direction, &assetType, &amount, &walletAddress, &contractAddress, &tokenID, &amountText, &raw, &aiSummary, &event.RiskLevel, &event.Status, &event.VerificationStatus, &payloadHash, &errorMessage, &event.CreatedAt); err != nil {
		return event, err
	}
	event.SourceID = stringPtrFromNull(sourceID)
	event.SourceName = stringPtrFromNull(sourceName)
	event.UserID = stringPtrFromNull(userID)
	event.Email = stringPtrFromNull(email)
	event.Chain = stringPtrFromNull(chain)
	event.Network = stringPtrFromNull(network)
	event.EventType = stringPtrFromNull(eventType)
	event.Address = stringPtrFromNull(address)
	event.TxHash = stringPtrFromNull(txHash)
	event.BlockNumber = stringPtrFromNull(blockNumber)
	event.Direction = stringPtrFromNull(direction)
	event.AssetType = stringPtrFromNull(assetType)
	event.Amount = stringPtrFromNull(amount)
	event.WalletAddress = stringPtrFromNull(walletAddress)
	event.ContractAddress = stringPtrFromNull(contractAddress)
	event.TokenID = stringPtrFromNull(tokenID)
	event.AmountText = stringPtrFromNull(amountText)
	event.AISummary = stringPtrFromNull(aiSummary)
	event.PayloadHash = stringPtrFromNull(payloadHash)
	event.ErrorMessage = stringPtrFromNull(errorMessage)
	if len(raw) > 0 {
		var payload any
		if err := json.Unmarshal(raw, &payload); err == nil {
			event.RawPayload = payload
		}
	}
	return event, nil
}

func scanWeb3Source(scanner web3EventScanner) (web3EventSource, error) {
	var source web3EventSource
	var userID, notes, webhookURL, disabledReason sql.NullString
	var lastEventAt sql.NullTime
	if err := scanner.Scan(&source.ID, &userID, &source.Email, &source.Name, &source.Label, &source.Provider, &source.Chain, &source.Network, &source.Address, &source.SourceType, &notes, &source.Status, &source.ProviderSetupStatus, &webhookURL, &source.IsActive, &source.VerificationMode, &lastEventAt, &disabledReason, &source.CreatedAt, &source.UpdatedAt); err != nil {
		return source, err
	}
	source.UserID = stringPtrFromNull(userID)
	source.Notes = stringPtrFromNull(notes)
	source.WebhookURL = stringPtrFromNull(webhookURL)
	if lastEventAt.Valid {
		source.LastEventAt = &lastEventAt.Time
	}
	source.DisabledReason = stringPtrFromNull(disabledReason)
	return source, nil
}

type alchemyTransfer struct {
	BlockNum    string         `json:"blockNum"`
	Hash        string         `json:"hash"`
	From        string         `json:"from"`
	To          string         `json:"to"`
	Category    string         `json:"category"`
	Asset       string         `json:"asset"`
	Value       any            `json:"value"`
	TokenID     string         `json:"tokenId"`
	RawContract map[string]any `json:"rawContract"`
	Metadata    map[string]any `json:"metadata"`
}

type alchemyTransferResponse struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Result  struct {
		Transfers []alchemyTransfer `json:"transfers"`
	} `json:"result"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func (h *Handler) syncWeb3Source(w http.ResponseWriter, r *http.Request) {
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	email := normalizedClaimEmail(claims)
	if email == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	id := strings.TrimSuffix(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/web3/sources/"), "/"), "/sync")
	id = strings.Trim(id, "/")
	if id == "" || strings.Contains(id, "/") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_source_id"})
		return
	}
	source, err := h.getWeb3SourceForUser(id, email)
	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}
	transfers, err := fetchAlchemyTransfers(source)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"ok": false, "error": "alchemy_sync_failed", "message": "Could not fetch activity right now."})
		return
	}
	inserted := 0
	for _, transfer := range transfers {
		wasInserted, err := h.insertAlchemyTransfer(source, transfer, currentUserID(claims))
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
			return
		}
		if wasInserted {
			inserted++
		}
	}
	if inserted > 0 {
		_, err = h.DB.Exec(`UPDATE web3_event_sources SET last_event_at=NOW(), updated_at=NOW() WHERE id=$1 AND lower(email)=lower($2)`, source.ID, email)
	} else {
		_, err = h.DB.Exec(`UPDATE web3_event_sources SET updated_at=NOW() WHERE id=$1 AND lower(email)=lower($2)`, source.ID, email)
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "inserted": inserted, "fetched": len(transfers), "source_id": source.ID})
}

func fetchAlchemyTransfers(source web3EventSource) ([]alchemyTransfer, error) {
	apiKey := strings.TrimSpace(os.Getenv("ALCHEMY_API_KEY"))
	if apiKey == "" {
		return nil, fmt.Errorf("alchemy api key missing")
	}
	endpoint, ok := alchemyRPCURL(source.Chain, source.Network, apiKey)
	if !ok {
		return nil, fmt.Errorf("unsupported alchemy network")
	}
	address := strings.ToLower(strings.TrimSpace(source.Address))
	if address == "" {
		return nil, fmt.Errorf("source address missing")
	}
	params := []map[string]any{}
	base := map[string]any{
		"fromBlock":        "0x0",
		"toBlock":          "latest",
		"category":         []string{"external", "erc20", "erc721", "erc1155"},
		"withMetadata":     true,
		"excludeZeroValue": false,
		"maxCount":         "0x14",
		"order":            "desc",
	}
	if source.SourceType == "contract" {
		p := cloneAlchemyParams(base)
		p["contractAddresses"] = []string{address}
		params = append(params, p)
	} else {
		from := cloneAlchemyParams(base)
		from["fromAddress"] = address
		to := cloneAlchemyParams(base)
		to["toAddress"] = address
		params = append(params, from, to)
	}
	seen := map[string]bool{}
	out := []alchemyTransfer{}
	for _, param := range params {
		body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "alchemy_getAssetTransfers", "params": []any{param}})
		req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= http.StatusBadRequest {
			return nil, fmt.Errorf("alchemy returned HTTP %d", resp.StatusCode)
		}
		var parsed alchemyTransferResponse
		if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&parsed); err != nil {
			return nil, err
		}
		if parsed.Error != nil {
			return nil, fmt.Errorf("alchemy error: %s", parsed.Error.Message)
		}
		for _, transfer := range parsed.Result.Transfers {
			key := strings.ToLower(transfer.Hash + ":" + transfer.Category + ":" + transfer.From + ":" + transfer.To + ":" + transfer.TokenID)
			if key == "::::" || seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, transfer)
		}
	}
	return out, nil
}

func (h *Handler) insertAlchemyTransfer(source web3EventSource, transfer alchemyTransfer, userID string) (bool, error) {
	txHash := strings.TrimSpace(transfer.Hash)
	eventType := firstNonEmptyString(transfer.Category, "transfer")
	address := strings.ToLower(strings.TrimSpace(source.Address))
	if txHash == "" || address == "" {
		return false, nil
	}
	var exists bool
	if err := h.DB.QueryRow(`SELECT EXISTS (SELECT 1 FROM web3_events WHERE lower(email)=lower($1) AND lower(address)=lower($2) AND tx_hash=$3 AND event_type=$4)`, source.Email, address, txHash, eventType).Scan(&exists); err != nil {
		return false, err
	}
	if exists {
		return false, nil
	}
	raw, _ := json.Marshal(transfer)
	direction := transferDirection(address, transfer.From, transfer.To, source.SourceType)
	contractAddress := rawContractAddress(transfer.RawContract)
	amount := stringifyAlchemyValue(transfer.Value)
	assetType := firstNonEmptyString(transfer.Category, transfer.Asset)
	walletAddress := firstNonEmptyString(transfer.From, transfer.To)
	_, err := h.DB.Exec(`
		INSERT INTO web3_events (id, source_id, source_name, user_id, email, provider, chain, network, event_type, address, tx_hash, block_number, direction, asset_type, amount, wallet_address, contract_address, token_id, amount_text, raw_payload, risk_level, status, verification_status, payload_hash, created_at)
		VALUES ($1,$2,$3,$4,$5,'alchemy',$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$14,$18::jsonb,'unknown','received','api_polling',$19,NOW())`,
		newID(), source.ID, source.Label, userID, source.Email, source.Chain, source.Network, eventType, address, txHash, transfer.BlockNum, direction, assetType, amount, walletAddress, nullIfEmpty(contractAddress), nullIfEmpty(transfer.TokenID), string(raw), sha256Hex(raw))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func alchemyRPCURL(chain, network, apiKey string) (string, bool) {
	chain = strings.ToLower(strings.TrimSpace(chain))
	network = strings.ToLower(strings.TrimSpace(network))
	if chain == "base" && (network == "mainnet" || network == "base-mainnet" || network == "base") {
		return "https://base-mainnet.g.alchemy.com/v2/" + apiKey, true
	}
	return "", false
}

func isSupportedAlchemyNetwork(chain, network string) bool {
	_, ok := alchemyRPCURL(chain, network, "test")
	return ok
}

func cloneAlchemyParams(in map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range in {
		out[k] = v
	}
	return out
}

func transferDirection(address, from, to, sourceType string) string {
	address = strings.ToLower(strings.TrimSpace(address))
	if sourceType == "contract" {
		return "contract"
	}
	if strings.EqualFold(address, strings.TrimSpace(from)) {
		return "outgoing"
	}
	if strings.EqualFold(address, strings.TrimSpace(to)) {
		return "incoming"
	}
	return "related"
}

func rawContractAddress(raw map[string]any) string {
	if raw == nil {
		return ""
	}
	if v, ok := raw["address"].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func stringifyAlchemyValue(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

func (h *Handler) watchlistSourceLimit(ctx context.Context, email string) (int, error) {
	plan := "free"
	if err := h.DB.QueryRowContext(ctx, `
		SELECT COALESCE(plan_id,'free')
		FROM entitlements
		WHERE lower(email)=lower($1) AND status='active'
		ORDER BY CASE COALESCE(plan_id,'free') WHEN 'studio' THEN 4 WHEN 'builder' THEN 3 WHEN 'starter' THEN 2 WHEN 'pro' THEN 2 ELSE 1 END DESC, created_at DESC
		LIMIT 1`, email).Scan(&plan); err != nil && err != sql.ErrNoRows {
		return 0, err
	}
	switch strings.ToLower(plan) {
	case "starter", "pro":
		return 3, nil
	case "builder":
		return 10, nil
	case "studio":
		return 50, nil
	default:
		return 1, nil
	}
}

func normalizedClaimEmail(claims neonJWTClaims) string {
	return strings.ToLower(strings.TrimSpace(claims.Email))
}

func currentUserID(claims neonJWTClaims) string {
	if strings.TrimSpace(claims.Sub) != "" {
		return strings.TrimSpace(claims.Sub)
	}
	return strings.TrimSpace(claims.Email)
}

func deterministicWeb3Summary(event web3Event) string {
	network := valueOrUnknown(event.Network)
	eventType := valueOrUnknown(event.EventType)
	txHash := valueOrUnknown(event.TxHash)
	wallet := valueOrUnknown(event.WalletAddress)
	contract := valueOrUnknown(event.ContractAddress)
	return fmt.Sprintf("Read-only Web3 Bridge monitor received a %s event on %s. Transaction: %s. Wallet: %s. Contract: %s. Risk is currently %s; no private keys, custody, escrow, or automatic transfers are used.", eventType, network, txHash, wallet, contract, event.RiskLevel)
}

func valueOrUnknown(v *string) string {
	if v == nil || strings.TrimSpace(*v) == "" {
		return "unknown"
	}
	return strings.TrimSpace(*v)
}

func stringPtrFromNull(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	s := v.String
	return &s
}

func nullStringToAny(v sql.NullString) any {
	if !v.Valid || strings.TrimSpace(v.String) == "" {
		return nil
	}
	return strings.TrimSpace(v.String)
}

func nullIfEmpty(v string) any {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return strings.TrimSpace(v)
}

func firstNonEmptyString(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func webhookSecretFromRequest(r *http.Request) string {
	if secret := strings.TrimSpace(r.Header.Get("X-Koschei-Webhook-Secret")); secret != "" {
		return secret
	}

	authorization := strings.TrimSpace(r.Header.Get("Authorization"))
	if len(authorization) > len("Bearer ") && strings.EqualFold(authorization[:len("Bearer ")], "Bearer ") {
		if secret := strings.TrimSpace(authorization[len("Bearer "):]); secret != "" {
			return secret
		}
	}

	return strings.TrimSpace(r.Header.Get("X-Alchemy-Token"))
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func constantTimeEqualSHA256(secret, expectedHash string) bool {
	providedHash := sha256Hex([]byte(secret))
	return subtle.ConstantTimeCompare([]byte(providedHash), []byte(expectedHash)) == 1
}

func requestIP(r *http.Request) string {
	for _, header := range []string{"X-Forwarded-For", "X-Real-IP"} {
		value := strings.TrimSpace(r.Header.Get(header))
		if value == "" {
			continue
		}
		if header == "X-Forwarded-For" {
			parts := strings.Split(value, ",")
			if len(parts) > 0 {
				return strings.TrimSpace(parts[0])
			}
		}
		return value
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}

func findStringByKeys(v any, keys ...string) string {
	wanted := map[string]bool{}
	for _, key := range keys {
		wanted[strings.ToLower(key)] = true
	}
	return findStringByKeySet(v, wanted, 0)
}

func findStringByKeySet(v any, wanted map[string]bool, depth int) string {
	if depth > 8 {
		return ""
	}
	switch typed := v.(type) {
	case map[string]any:
		for key, value := range typed {
			if wanted[strings.ToLower(key)] {
				if s, ok := value.(string); ok && strings.TrimSpace(s) != "" {
					return strings.TrimSpace(s)
				}
				if f, ok := value.(float64); ok {
					return fmt.Sprintf("%.0f", f)
				}
			}
		}
		for _, value := range typed {
			if found := findStringByKeySet(value, wanted, depth+1); found != "" {
				return found
			}
		}
	case []any:
		for _, item := range typed {
			if found := findStringByKeySet(item, wanted, depth+1); found != "" {
				return found
			}
		}
	}
	return ""
}
