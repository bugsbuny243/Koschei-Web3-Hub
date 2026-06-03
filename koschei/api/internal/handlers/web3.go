package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
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
	Email            *string    `json:"email,omitempty"`
	UserID           *string    `json:"user_id,omitempty"`
	Name             string     `json:"name"`
	Label            string     `json:"label"`
	Provider         string     `json:"provider"`
	Chain            string     `json:"chain"`
	Network          string     `json:"network"`
	Address          string     `json:"address"`
	SourceType       string     `json:"source_type"`
	Status           string     `json:"status"`
	Notes            *string    `json:"notes,omitempty"`
	WebhookURL       *string    `json:"webhook_url,omitempty"`
	IsActive         bool       `json:"is_active"`
	VerificationMode string     `json:"verification_mode"`
	LastEventAt      *time.Time `json:"last_event_at,omitempty"`
	DisabledReason   *string    `json:"disabled_reason,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type webhookSource struct {
	ID         string
	Email      sql.NullString
	Label      string
	Chain      string
	Network    string
	Address    string
	SourceType string
	Status     string
}

func (h *Handler) Web3AlchemyEvent(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil || len(strings.TrimSpace(string(raw))) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}

	verificationStatus := "unverified_no_signing_key"
	if signingKey := strings.TrimSpace(os.Getenv("ALCHEMY_WEBHOOK_SIGNING_KEY")); signingKey != "" {
		if !verifyAlchemySignature(raw, signingKey, r.Header.Get("X-Alchemy-Signature")) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid_signature"})
			return
		}
		verificationStatus = "verified"
	} else {
		log.Printf("warning: ALCHEMY_WEBHOOK_SIGNING_KEY is not set; accepting Alchemy webhook without signature verification")
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}

	normalized := normalizeAlchemyActivities(payload)
	if len(normalized) == 0 {
		normalized = []normalizedAlchemyEvent{{
			Chain:     chainFromNetwork(firstNonEmptyString(findStringByKeys(payload, "network"), findStringByKeys(payload, "chain", "blockchain"))),
			Network:   firstNonEmptyString(findStringByKeys(payload, "network"), findStringByKeys(payload, "chain", "blockchain")),
			EventType: "alchemy_webhook_unknown",
			Address:   findStringByKeys(payload, "fromAddress", "toAddress", "from", "to", "address"),
			TxHash:    findStringByKeys(payload, "hash", "tx_hash", "txHash", "transactionHash"),
			Raw:       payload,
		}}
	}

	created := 0
	ids := make([]string, 0, len(normalized))
	payloadHash := sha256Hex(raw)
	for _, event := range normalized {
		source, _ := h.matchWebhookSource(event.FromAddress, event.ToAddress, event.Address)
		eventID := newID()
		ids = append(ids, eventID)
		var sourceID any
		var email any
		var sourceName any
		if strings.TrimSpace(source.ID) != "" {
			sourceID = source.ID
			email = nullStringToAny(source.Email)
			sourceName = source.Label
		}
		rawEvent, _ := json.Marshal(event.Raw)
		chain := firstNonEmptyString(event.Chain, chainFromNetwork(event.Network))
		address := firstNonEmptyString(event.Address, event.FromAddress, event.ToAddress)
		if _, err := h.DB.Exec(`
			INSERT INTO web3_events (id, source_id, email, chain, network, event_type, address, tx_hash, block_number, direction, asset_type, amount, raw_payload, source_name, user_id, provider, wallet_address, contract_address, amount_text, risk_level, status, verification_status, received_ip, received_user_agent, payload_hash)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13::jsonb,$14,$15,'alchemy',$16,$17,$18,'unknown','received',$19,$20,$21,$22)`,
			eventID, sourceID, email, nullIfEmpty(chain), nullIfEmpty(event.Network), nullIfEmpty(event.EventType), nullIfEmpty(address), nullIfEmpty(event.TxHash), nullIfEmpty(event.BlockNumber), nullIfEmpty(event.Direction), nullIfEmpty(event.AssetType), nullIfEmpty(event.Amount), string(rawEvent), sourceName, email, nullIfEmpty(address), nullIfEmpty(firstNonEmptyString(event.ContractAddress, event.ToAddress)), nullIfEmpty(event.Amount), verificationStatus, nullIfEmpty(requestIP(r)), nullIfEmpty(r.UserAgent()), payloadHash); err != nil {
			log.Printf("web3 alchemy event insert failed: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
			return
		}
		if strings.TrimSpace(source.ID) != "" {
			if _, err := h.DB.Exec(`UPDATE web3_event_sources SET last_event_at=NOW(), updated_at=NOW() WHERE id=$1`, source.ID); err != nil {
				log.Printf("web3 source last_event_at update failed: %v", err)
			}
		}
		created++
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "event_ids": ids, "events_created": created, "verification_status": verificationStatus})
}

func (h *Handler) matchWebhookSource(addresses ...string) (webhookSource, error) {
	var source webhookSource
	for _, address := range addresses {
		address = strings.ToLower(strings.TrimSpace(address))
		if address == "" {
			continue
		}
		err := h.DB.QueryRow(`
			SELECT id, email, COALESCE(label, name, ''), COALESCE(chain, ''), COALESCE(network, ''), COALESCE(address, ''), COALESCE(source_type, 'wallet'), COALESCE(status, 'active')
			FROM web3_event_sources
			WHERE lower(address)=lower($1)
			  AND COALESCE(status, CASE WHEN COALESCE(is_active, true) THEN 'active' ELSE 'inactive' END) = 'active'
			ORDER BY created_at DESC
			LIMIT 1`, address).Scan(&source.ID, &source.Email, &source.Label, &source.Chain, &source.Network, &source.Address, &source.SourceType, &source.Status)
		if err == nil {
			return source, nil
		}
		if err != sql.ErrNoRows {
			return source, err
		}
	}
	return source, nil
}

func (h *Handler) Web3Events(w http.ResponseWriter, r *http.Request) {
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	email := strings.ToLower(strings.TrimSpace(claims.Email))
	if email == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	limit := parseLimit(r.URL.Query().Get("limit"), 50, 100)
	sourceID := strings.TrimSpace(r.URL.Query().Get("source_id"))
	args := []any{email, limit}
	where := "WHERE lower(email)=lower($1)"
	if sourceID != "" {
		where += " AND source_id=$3"
		args = append(args, sourceID)
	}
	rows, err := h.DB.Query(`
		SELECT id::text, source_id::text, email, chain, network, event_type, address, tx_hash, block_number, direction, asset_type, amount, raw_payload, created_at
		FROM web3_events
		`+where+`
		ORDER BY created_at DESC
		LIMIT $2`, args...)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}
	defer rows.Close()

	events := make([]map[string]any, 0, limit)
	for rows.Next() {
		var id string
		var sourceID, emailVal, chain, network, eventType, address, txHash, blockNumber, direction, assetType, amount sql.NullString
		var raw []byte
		var createdAt time.Time
		if err := rows.Scan(&id, &sourceID, &emailVal, &chain, &network, &eventType, &address, &txHash, &blockNumber, &direction, &assetType, &amount, &raw, &createdAt); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
			return
		}
		var rawPayload any
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &rawPayload)
		}
		events = append(events, map[string]any{
			"id": string(id), "source_id": stringPtrFromNull(sourceID), "email": stringPtrFromNull(emailVal), "chain": stringPtrFromNull(chain), "network": stringPtrFromNull(network), "event_type": stringPtrFromNull(eventType), "address": stringPtrFromNull(address), "tx_hash": stringPtrFromNull(txHash), "block_number": stringPtrFromNull(blockNumber), "direction": stringPtrFromNull(direction), "asset_type": stringPtrFromNull(assetType), "amount": stringPtrFromNull(amount), "raw_payload": rawPayload, "created_at": createdAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "events": events})
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
	switch r.Method {
	case http.MethodPatch:
		h.patchWeb3Source(w, r)
	case http.MethodDelete:
		h.deactivateWeb3Source(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (h *Handler) listWeb3Sources(w http.ResponseWriter, r *http.Request) {
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	email := strings.ToLower(strings.TrimSpace(claims.Email))
	rows, err := h.DB.Query(`
		SELECT id::text, email, COALESCE(label, name, ''), COALESCE(name, label, ''), COALESCE(provider, 'alchemy'), COALESCE(chain, ''), COALESCE(network, ''), COALESCE(address, ''), COALESCE(source_type, 'wallet'), COALESCE(status, CASE WHEN COALESCE(is_active, true) THEN 'active' ELSE 'inactive' END), notes, webhook_url, COALESCE(is_active, status = 'active'), COALESCE(verification_mode, 'alchemy_signature'), last_event_at, disabled_reason, created_at, updated_at
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
		source, scanErr := scanWatchlistSource(rows)
		if scanErr != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
			return
		}
		sources = append(sources, source)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "sources": sources})
}

type web3SourceRequest struct {
	Name       string `json:"name"`
	Label      string `json:"label"`
	Provider   string `json:"provider"`
	Chain      string `json:"chain"`
	Network    string `json:"network"`
	Address    string `json:"address"`
	SourceType string `json:"source_type"`
	Notes      string `json:"notes"`
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
	email := strings.ToLower(strings.TrimSpace(claims.Email))
	label := firstNonEmptyString(req.Label, req.Name)
	chain := strings.TrimSpace(req.Chain)
	network := strings.TrimSpace(req.Network)
	address := strings.TrimSpace(req.Address)
	sourceType := strings.TrimSpace(req.SourceType)
	if label == "" || address == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "label_and_address_required"})
		return
	}
	if !validWatchlistNetwork(chain, network) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unsupported_chain_network"})
		return
	}
	if !validWatchlistSourceType(sourceType) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unsupported_source_type"})
		return
	}
	freeOnly, err := h.isFreeOnlyUser(email)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}
	if freeOnly {
		var activeCount int
		if err := h.DB.QueryRow(`SELECT COUNT(*) FROM web3_event_sources WHERE lower(email)=lower($1) AND COALESCE(status, CASE WHEN COALESCE(is_active, true) THEN 'active' ELSE 'inactive' END)='active'`, email).Scan(&activeCount); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
			return
		}
		if activeCount >= 1 {
			writeJSON(w, http.StatusPaymentRequired, map[string]string{"error": "watchlist_limit_reached", "message": "Free plan includes 1 watchlist source. Upgrade to monitor more addresses."})
			return
		}
	}

	sourceID := newID()
	_, err = h.DB.Exec(`
		INSERT INTO web3_event_sources (id, email, user_id, label, name, provider, chain, network, address, source_type, status, notes, webhook_url, is_active, verification_mode)
		VALUES ($1,$2,$2,$3,$3,'alchemy',$4,$5,$6,$7,'active',$8,$9,true,'alchemy_signature')`, sourceID, email, label, chain, network, address, sourceType, nullIfEmpty(req.Notes), nullIfEmpty(req.WebhookURL))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}
	source, err := h.getWatchlistSourceForUser(sourceID, email)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "source": source})
}

type web3SourcePatchRequest struct {
	Name       *string `json:"name"`
	Label      *string `json:"label"`
	Chain      *string `json:"chain"`
	Network    *string `json:"network"`
	Address    *string `json:"address"`
	SourceType *string `json:"source_type"`
	Status     *string `json:"status"`
	Notes      *string `json:"notes"`
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
	id := web3SourceIDFromPath(r.URL.Path)
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_source_id"})
		return
	}
	var req web3SourcePatchRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}
	email := strings.ToLower(strings.TrimSpace(claims.Email))
	existing, err := h.getWatchlistSourceForUser(id, email)
	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}
	label := existing.Label
	chain := existing.Chain
	network := existing.Network
	address := existing.Address
	sourceType := existing.SourceType
	status := existing.Status
	if req.Label != nil {
		label = strings.TrimSpace(*req.Label)
	}
	if req.Name != nil {
		label = strings.TrimSpace(*req.Name)
	}
	if req.Chain != nil {
		chain = strings.TrimSpace(*req.Chain)
	}
	if req.Network != nil {
		network = strings.TrimSpace(*req.Network)
	}
	if req.Address != nil {
		address = strings.TrimSpace(*req.Address)
	}
	if req.SourceType != nil {
		sourceType = strings.TrimSpace(*req.SourceType)
	}
	if req.Status != nil {
		status = strings.TrimSpace(*req.Status)
	}
	if req.IsActive != nil {
		if *req.IsActive {
			status = "active"
		} else {
			status = "inactive"
		}
	}
	if label == "" || address == "" || !validWatchlistNetwork(chain, network) || !validWatchlistSourceType(sourceType) || !validWatchlistStatus(status) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_source"})
		return
	}
	_, err = h.DB.Exec(`
		UPDATE web3_event_sources
		SET label=$3, name=$3, chain=$4, network=$5, address=$6, source_type=$7, status=$8, is_active=($8='active'), notes=COALESCE($9, notes), webhook_url=COALESCE($10, webhook_url), updated_at=NOW()
		WHERE id=$1 AND lower(email)=lower($2)`, id, email, label, chain, network, address, sourceType, status, nullablePatchString(req.Notes), nullablePatchString(req.WebhookURL))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}
	source, err := h.getWatchlistSourceForUser(id, email)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "source": source})
}

func (h *Handler) deactivateWeb3Source(w http.ResponseWriter, r *http.Request) {
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	id := web3SourceIDFromPath(r.URL.Path)
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_source_id"})
		return
	}
	email := strings.ToLower(strings.TrimSpace(claims.Email))
	res, err := h.DB.Exec(`UPDATE web3_event_sources SET status='inactive', is_active=false, updated_at=NOW() WHERE id=$1 AND lower(email)=lower($2)`, id, email)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) Web3TestEvent(w http.ResponseWriter, r *http.Request) {
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	userID := currentUserID(claims)
	email := strings.ToLower(strings.TrimSpace(claims.Email))
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
		INSERT INTO web3_events (id, user_id, email, provider, chain, network, event_type, address, tx_hash, wallet_address, contract_address, amount, raw_payload, risk_level, status, verification_status, payload_hash)
		VALUES ($1,$2,$3,'alchemy','solana','solana-devnet','test_event',$4,$5,$4,$6,'0',$7::jsonb,'low','received','verified',$8)`,
		eventID, userID, email, "demo-wallet-read-only", "demo-tx-"+eventID[:8], "demo-contract-read-only", string(raw), payloadHash); err != nil {
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

type normalizedAlchemyEvent struct {
	Chain           string
	Network         string
	EventType       string
	Address         string
	FromAddress     string
	ToAddress       string
	ContractAddress string
	TxHash          string
	BlockNumber     string
	Direction       string
	AssetType       string
	Amount          string
	Raw             any
}

func verifyAlchemySignature(raw []byte, signingKey, signature string) bool {
	signature = strings.TrimSpace(strings.TrimPrefix(signature, "sha256="))
	if signature == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(signingKey))
	_, _ = mac.Write(raw)
	expected := hex.EncodeToString(mac.Sum(nil))
	return subtle.ConstantTimeCompare([]byte(strings.ToLower(signature)), []byte(expected)) == 1
}

func normalizeAlchemyActivities(payload map[string]any) []normalizedAlchemyEvent {
	envelopeType := firstNonEmptyString(asString(payload["type"]), asString(payload["webhookType"]), findStringByKeys(payload, "type"))
	envelopeNetwork := firstNonEmptyString(asString(payload["network"]), findStringByKeys(payload, "network"))
	activities := activityItems(payload)
	out := make([]normalizedAlchemyEvent, 0, len(activities))
	for _, activity := range activities {
		from := findStringByKeys(activity, "fromAddress", "from")
		to := findStringByKeys(activity, "toAddress", "to")
		network := firstNonEmptyString(findStringByKeys(activity, "network"), envelopeNetwork)
		eventType := firstNonEmptyString(findStringByKeys(activity, "category", "event_type", "eventType", "type"), envelopeType, "alchemy_address_activity")
		raw := map[string]any{
			"webhookId": payload["webhookId"],
			"id":        payload["id"],
			"createdAt": payload["createdAt"],
			"type":      envelopeType,
			"network":   network,
			"activity":  activity,
		}
		out = append(out, normalizedAlchemyEvent{
			Chain:           chainFromNetwork(network),
			Network:         network,
			EventType:       eventType,
			Address:         firstNonEmptyString(from, to, findStringByKeys(activity, "address")),
			FromAddress:     from,
			ToAddress:       to,
			ContractAddress: findStringByKeys(activity, "contractAddress", "contract", "rawContract"),
			TxHash:          findStringByKeys(activity, "hash", "tx_hash", "txHash", "transactionHash"),
			BlockNumber:     findStringByKeys(activity, "blockNum", "blockNumber", "block"),
			Direction:       inferDirection(from, to),
			AssetType:       firstNonEmptyString(findStringByKeys(activity, "asset_type", "assetType", "category"), findStringByKeys(activity, "asset")),
			Amount:          firstNonEmptyString(findStringByKeys(activity, "value", "amount"), findStringByKeys(activity, "rawValue")),
			Raw:             raw,
		})
	}
	return out
}

func activityItems(payload map[string]any) []map[string]any {
	if event, ok := payload["event"].(map[string]any); ok {
		if activity, ok := event["activity"].([]any); ok {
			return mapsFromAnySlice(activity)
		}
		if activity, ok := event["activities"].([]any); ok {
			return mapsFromAnySlice(activity)
		}
	}
	if activity, ok := payload["activity"].([]any); ok {
		return mapsFromAnySlice(activity)
	}
	if activity, ok := payload["activities"].([]any); ok {
		return mapsFromAnySlice(activity)
	}
	return nil
}

func mapsFromAnySlice(items []any) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func asString(v any) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case json.Number:
		return t.String()
	default:
		return ""
	}
}

func chainFromNetwork(network string) string {
	n := strings.ToLower(strings.TrimSpace(network))
	switch {
	case strings.Contains(n, "base"):
		return "base"
	case strings.Contains(n, "arbitrum"):
		return "arbitrum"
	case strings.Contains(n, "optimism"):
		return "optimism"
	case strings.Contains(n, "polygon") || strings.Contains(n, "amoy"):
		return "polygon"
	case strings.Contains(n, "solana"):
		return "solana"
	case strings.Contains(n, "eth") || strings.Contains(n, "sepolia"):
		return "ethereum"
	default:
		return n
	}
}

func inferDirection(from, to string) string {
	if strings.TrimSpace(from) != "" && strings.TrimSpace(to) != "" {
		return "transfer"
	}
	if strings.TrimSpace(from) != "" {
		return "out"
	}
	if strings.TrimSpace(to) != "" {
		return "in"
	}
	return ""
}

func validWatchlistNetwork(chain, network string) bool {
	combo := strings.ToLower(strings.TrimSpace(chain)) + ":" + strings.ToLower(strings.TrimSpace(network))
	switch combo {
	case "ethereum:sepolia", "base:sepolia", "arbitrum:sepolia", "optimism:sepolia", "polygon:amoy", "solana:devnet":
		return true
	default:
		return false
	}
}

func validWatchlistSourceType(sourceType string) bool {
	switch strings.TrimSpace(sourceType) {
	case "wallet", "contract", "nft_collection":
		return true
	default:
		return false
	}
}

func validWatchlistStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case "active", "inactive", "paused":
		return true
	default:
		return false
	}
}

func parseLimit(raw string, fallback, max int) int {
	limit, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || limit <= 0 {
		return fallback
	}
	if limit > max {
		return max
	}
	return limit
}

func web3SourceIDFromPath(path string) string {
	id := strings.Trim(strings.TrimPrefix(path, "/api/web3/sources/"), "/")
	if id == "" || strings.Contains(id, "/") {
		return ""
	}
	return id
}

func nullablePatchString(v *string) any {
	if v == nil {
		return nil
	}
	return nullIfEmpty(*v)
}

func (h *Handler) isFreeOnlyUser(email string) (bool, error) {
	var paidCount int
	if err := h.DB.QueryRow(`SELECT COUNT(*) FROM entitlements WHERE lower(email)=lower($1) AND status='active' AND COALESCE(plan_id, 'free') <> 'free'`, email).Scan(&paidCount); err != nil {
		return false, err
	}
	return paidCount == 0, nil
}

func (h *Handler) getWatchlistSourceForUser(id, email string) (web3EventSource, error) {
	row := h.DB.QueryRow(`
		SELECT id::text, email, COALESCE(label, name, ''), COALESCE(name, label, ''), COALESCE(provider, 'alchemy'), COALESCE(chain, ''), COALESCE(network, ''), COALESCE(address, ''), COALESCE(source_type, 'wallet'), COALESCE(status, CASE WHEN COALESCE(is_active, true) THEN 'active' ELSE 'inactive' END), notes, webhook_url, COALESCE(is_active, status = 'active'), COALESCE(verification_mode, 'alchemy_signature'), last_event_at, disabled_reason, created_at, updated_at
		FROM web3_event_sources
		WHERE id=$1 AND lower(email)=lower($2)`, id, email)
	return scanWatchlistSource(row)
}

func scanWatchlistSource(scanner web3EventScanner) (web3EventSource, error) {
	var source web3EventSource
	var email, notes, webhookURL, disabledReason sql.NullString
	var lastEventAt sql.NullTime
	if err := scanner.Scan(&source.ID, &email, &source.Label, &source.Name, &source.Provider, &source.Chain, &source.Network, &source.Address, &source.SourceType, &source.Status, &notes, &webhookURL, &source.IsActive, &source.VerificationMode, &lastEventAt, &disabledReason, &source.CreatedAt, &source.UpdatedAt); err != nil {
		return source, err
	}
	source.Email = stringPtrFromNull(email)
	source.UserID = stringPtrFromNull(email)
	source.Notes = stringPtrFromNull(notes)
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
