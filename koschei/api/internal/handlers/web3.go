package handlers

import (
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

type web3Event struct {
	ID                 string    `json:"id"`
	SourceID           *string   `json:"source_id,omitempty"`
	SourceName         *string   `json:"source_name,omitempty"`
	UserID             *string   `json:"user_id,omitempty"`
	Provider           string    `json:"provider"`
	Network            *string   `json:"network,omitempty"`
	EventType          *string   `json:"event_type,omitempty"`
	TxHash             *string   `json:"tx_hash,omitempty"`
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
	ID               string     `json:"id"`
	UserID           *string    `json:"user_id,omitempty"`
	Name             string     `json:"name"`
	Provider         string     `json:"provider"`
	Network          string     `json:"network"`
	WebhookURL       *string    `json:"webhook_url,omitempty"`
	IsActive         bool       `json:"is_active"`
	VerificationMode string     `json:"verification_mode"`
	LastEventAt      *time.Time `json:"last_event_at,omitempty"`
	DisabledReason   *string    `json:"disabled_reason,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
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
	source, status, err := h.findWebhookSource(sourceID, network)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}
	if status != "verified" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": status})
		return
	}

	providedSecret := strings.TrimSpace(r.Header.Get("X-Koschei-Webhook-Secret"))
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

func (h *Handler) findWebhookSource(sourceID, network string) (webhookSource, string, error) {
	var source webhookSource
	if strings.TrimSpace(sourceID) != "" {
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
	if strings.TrimSpace(network) == "" {
		return source, "unconfigured_source", nil
	}
	err := h.DB.QueryRow(`
		SELECT id, user_id, name, provider, network, secret_hash, verification_mode
		FROM web3_event_sources
		WHERE provider='alchemy' AND network=$1 AND is_active=true
		ORDER BY updated_at DESC
		LIMIT 1`, strings.TrimSpace(network)).Scan(&source.ID, &source.UserID, &source.Name, &source.Provider, &source.Network, &source.SecretHash, &source.VerificationMode)
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
	userID := currentUserID(claims)
	rows, err := h.DB.Query(`
		SELECT id, source_id, source_name, user_id, provider, network, event_type, tx_hash, wallet_address, contract_address, token_id, amount_text, raw_payload, ai_summary, risk_level, status, verification_status, payload_hash, error_message, created_at
		FROM web3_events
		WHERE user_id=$1
		ORDER BY created_at DESC
		LIMIT 100`, userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}
	defer rows.Close()

	events := make([]web3Event, 0, 100)
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
	rows, err := h.DB.Query(`
		SELECT id, user_id, name, provider, network, webhook_url, COALESCE(is_active, true), verification_mode, last_event_at, disabled_reason, created_at, updated_at
		FROM web3_event_sources
		WHERE user_id=$1
		ORDER BY created_at DESC`, currentUserID(claims))
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
	writeJSON(w, http.StatusOK, map[string]any{"sources": sources})
}

type web3SourceRequest struct {
	Name       string `json:"name"`
	Provider   string `json:"provider"`
	Network    string `json:"network"`
	WebhookURL string `json:"webhook_url"`
	Secret     string `json:"secret"`
}

func (h *Handler) createWeb3Source(w http.ResponseWriter, r *http.Request) {
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	var req web3SourceRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}
	name := strings.TrimSpace(req.Name)
	network := strings.TrimSpace(req.Network)
	provider := firstNonEmptyString(req.Provider, "alchemy")
	secret := strings.TrimSpace(req.Secret)
	if name == "" || network == "" || secret == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name_network_and_secret_required"})
		return
	}
	if provider != "alchemy" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unsupported_provider"})
		return
	}

	sourceID := newID()
	_, err := h.DB.Exec(`
		INSERT INTO web3_event_sources (id, user_id, name, provider, network, webhook_url, is_active, secret_hash, verification_mode)
		VALUES ($1,$2,$3,$4,$5,$6,true,$7,'shared_secret')`, sourceID, currentUserID(claims), name, provider, network, nullIfEmpty(req.WebhookURL), sha256Hex([]byte(secret)))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}
	source, err := h.getWeb3SourceForUser(sourceID, currentUserID(claims))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"source":           source,
		"setup_hint":       "Configure your webhook provider to send X-Koschei-Source-Id and X-Koschei-Webhook-Secret headers. Store the secret securely; Koschei will not show it again.",
		"required_headers": []string{"X-Koschei-Source-Id", "X-Koschei-Webhook-Secret"},
	})
}

type web3SourcePatchRequest struct {
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
	userID := currentUserID(claims)
	if _, err := h.getWeb3SourceForUser(id, userID); err == sql.ErrNoRows {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
		return
	} else if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_name"})
			return
		}
		if _, err := h.DB.Exec(`UPDATE web3_event_sources SET name=$3, updated_at=NOW() WHERE id=$1 AND user_id=$2`, id, userID, name); err != nil {
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
		if _, err := h.DB.Exec(`UPDATE web3_event_sources SET network=$3, updated_at=NOW() WHERE id=$1 AND user_id=$2`, id, userID, network); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
			return
		}
	}
	if req.IsActive != nil {
		if _, err := h.DB.Exec(`UPDATE web3_event_sources SET is_active=$3, updated_at=NOW() WHERE id=$1 AND user_id=$2`, id, userID, *req.IsActive); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
			return
		}
	}
	if req.WebhookURL != nil {
		if _, err := h.DB.Exec(`UPDATE web3_event_sources SET webhook_url=$3, updated_at=NOW() WHERE id=$1 AND user_id=$2`, id, userID, nullIfEmpty(*req.WebhookURL)); err != nil {
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
		if _, err := h.DB.Exec(`UPDATE web3_event_sources SET secret_hash=$3, updated_at=NOW() WHERE id=$1 AND user_id=$2`, id, userID, sha256Hex([]byte(secret))); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
			return
		}
	}
	source, err := h.getWeb3SourceForUser(id, userID)
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
		INSERT INTO web3_events (id, user_id, provider, network, event_type, tx_hash, wallet_address, contract_address, raw_payload, risk_level, status, verification_status, payload_hash)
		VALUES ($1,$2,'alchemy','solana-devnet','test_event',$3,$4,$5,$6::jsonb,'low','received','verified',$7)`,
		eventID, userID, "demo-tx-"+eventID[:8], "demo-wallet-read-only", "demo-contract-read-only", string(raw), payloadHash); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}

	event, err := h.getWeb3EventForUser(eventID, userID)
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

	userID := currentUserID(claims)
	event, err := h.getWeb3EventForUser(id, userID)
	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}

	summary := deterministicWeb3Summary(event)
	if _, err := h.DB.Exec(`UPDATE web3_events SET ai_summary=$2 WHERE id=$1 AND user_id=$3`, id, summary, userID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}
	event.AISummary = &summary
	writeJSON(w, http.StatusOK, map[string]any{"event": event, "summary": summary})
}

func (h *Handler) getWeb3EventForUser(id, userID string) (web3Event, error) {
	row := h.DB.QueryRow(`
		SELECT id, source_id, source_name, user_id, provider, network, event_type, tx_hash, wallet_address, contract_address, token_id, amount_text, raw_payload, ai_summary, risk_level, status, verification_status, payload_hash, error_message, created_at
		FROM web3_events
		WHERE id=$1 AND user_id=$2`, id, userID)
	return scanWeb3Event(row)
}

func (h *Handler) getWeb3SourceForUser(id, userID string) (web3EventSource, error) {
	row := h.DB.QueryRow(`
		SELECT id, user_id, name, provider, network, webhook_url, COALESCE(is_active, true), verification_mode, last_event_at, disabled_reason, created_at, updated_at
		FROM web3_event_sources
		WHERE id=$1 AND user_id=$2`, id, userID)
	return scanWeb3Source(row)
}

type web3EventScanner interface {
	Scan(dest ...any) error
}

func scanWeb3Event(scanner web3EventScanner) (web3Event, error) {
	var event web3Event
	var sourceID, sourceName, userID, network, eventType, txHash, walletAddress, contractAddress, tokenID, amountText, aiSummary, payloadHash, errorMessage sql.NullString
	var raw []byte
	if err := scanner.Scan(&event.ID, &sourceID, &sourceName, &userID, &event.Provider, &network, &eventType, &txHash, &walletAddress, &contractAddress, &tokenID, &amountText, &raw, &aiSummary, &event.RiskLevel, &event.Status, &event.VerificationStatus, &payloadHash, &errorMessage, &event.CreatedAt); err != nil {
		return event, err
	}
	event.SourceID = stringPtrFromNull(sourceID)
	event.SourceName = stringPtrFromNull(sourceName)
	event.UserID = stringPtrFromNull(userID)
	event.Network = stringPtrFromNull(network)
	event.EventType = stringPtrFromNull(eventType)
	event.TxHash = stringPtrFromNull(txHash)
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
	var userID, webhookURL, disabledReason sql.NullString
	var lastEventAt sql.NullTime
	if err := scanner.Scan(&source.ID, &userID, &source.Name, &source.Provider, &source.Network, &webhookURL, &source.IsActive, &source.VerificationMode, &lastEventAt, &disabledReason, &source.CreatedAt, &source.UpdatedAt); err != nil {
		return source, err
	}
	source.UserID = stringPtrFromNull(userID)
	source.WebhookURL = stringPtrFromNull(webhookURL)
	if lastEventAt.Valid {
		source.LastEventAt = &lastEventAt.Time
	}
	source.DisabledReason = stringPtrFromNull(disabledReason)
	return source, nil
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
