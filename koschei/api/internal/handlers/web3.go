package handlers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
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

const alchemyWebhookPath = "/api/web3/events/alchemy"
const alchemyWebhookURLFallback = "https://tradepigloball.co" + alchemyWebhookPath

var evmAddressRe = regexp.MustCompile(`^0x[0-9a-fA-F]{40}$`)
var solanaAddressRe = regexp.MustCompile(`^[1-9A-HJ-NP-Za-km-z]{32,44}$`)

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
	AmountText         *string   `json:"amount_text,omitempty"`
	RawPayload         any       `json:"raw_payload,omitempty"`
	Status             string    `json:"status"`
	VerificationStatus string    `json:"verification_status"`
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
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil || len(strings.TrimSpace(string(raw))) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}

	verificationStatus, ok := verifyAlchemySignature(raw, r.Header.Get("X-Alchemy-Signature"))
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid_signature"})
		return
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}

	network := normalizeNetwork(firstNonEmptyString(findStringByKeys(payload, "network", "networkName"), findStringByKeys(payload, "chain", "blockchain")))
	activities := normalizeAlchemyActivities(payload, raw)
	if len(activities) == 0 {
		activities = []normalizedAlchemyActivity{{
			EventType: "alchemy_webhook_unknown",
			Network:   network,
			RawJSON:   string(raw),
		}}
	}

	inserted := 0
	matched := 0
	for _, activity := range activities {
		if strings.TrimSpace(activity.Network) == "" {
			activity.Network = network
		}
		extracted := activity.extractedAddresses()
		source, matchErr := h.findSourceByAnyAddress(extracted)
		if matchErr != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
			return
		}
		if source.ID != "" {
			matched++
			if strings.TrimSpace(activity.Network) == "" {
				activity.Network = source.Network
			}
			if _, err := h.DB.Exec(`
				UPDATE web3_event_sources
				SET status='active', provider_setup_status='connected', last_event_at=now(), updated_at=now()
				WHERE id=$1`, source.ID); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
				return
			}
		}
		if err := h.insertWeb3Event(activity, source, verificationStatus); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
			return
		}
		inserted++
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "inserted": inserted, "matched": matched, "verification_status": verificationStatus})
}

func verifyAlchemySignature(raw []byte, signatureHeader string) (string, bool) {
	key := strings.TrimSpace(os.Getenv("ALCHEMY_WEBHOOK_SIGNING_KEY"))
	if key == "" {
		return "unverified", true
	}
	signatureHeader = strings.TrimSpace(signatureHeader)
	if signatureHeader == "" {
		return "verified", false
	}
	mac := hmac.New(sha256.New, []byte(key))
	_, _ = mac.Write(raw)
	digest := mac.Sum(nil)
	hexDigest := strings.ToLower(hex.EncodeToString(digest))
	base64Digest := base64.StdEncoding.EncodeToString(digest)
	candidates := []string{signatureHeader}
	for _, part := range strings.Split(signatureHeader, ",") {
		trimmed := strings.TrimSpace(part)
		candidates = append(candidates, trimmed)
		if idx := strings.Index(trimmed, "="); idx >= 0 && idx+1 < len(trimmed) {
			candidates = append(candidates, strings.TrimSpace(trimmed[idx+1:]))
		}
	}
	for _, candidate := range candidates {
		candidate = strings.Trim(candidate, " \t\r\n\"")
		if constantTimeStringEqual(strings.ToLower(candidate), hexDigest) || constantTimeStringEqual(candidate, base64Digest) {
			return "verified", true
		}
	}
	return "verified", false
}

type normalizedAlchemyActivity struct {
	EventType       string
	Network         string
	Address         string
	FromAddress     string
	ToAddress       string
	WalletAddress   string
	ContractAddress string
	TxHash          string
	BlockNumber     string
	Direction       string
	AssetType       string
	Amount          string
	RawJSON         string
}

func normalizeAlchemyActivities(payload map[string]any, raw []byte) []normalizedAlchemyActivity {
	activitiesAny := findValueByKeys(payload, "activity", "activities")
	activities, ok := activitiesAny.([]any)
	if !ok || len(activities) == 0 {
		return nil
	}
	out := make([]normalizedAlchemyActivity, 0, len(activities))
	for _, item := range activities {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		b, _ := json.Marshal(m)
		activity := normalizeAlchemyActivity(m, string(b))
		activity.Network = firstNonEmptyString(activity.Network, findStringByKeys(payload, "network", "networkName"))
		out = append(out, activity)
	}
	return out
}

func normalizeAlchemyActivity(m map[string]any, raw string) normalizedAlchemyActivity {
	eventType := firstNonEmptyString(findStringByKeys(m, "event_type", "eventType", "category", "type"), "alchemy_address_activity")
	contractAddress := firstNonEmptyString(findStringByKeys(m, "contractAddress", "contract_address"), findNestedString(m, "rawContract", "address"), findNestedString(m, "asset", "contractAddress"))
	amount := firstNonEmptyString(findStringByKeys(m, "amount", "amount_text", "amountText", "value"), findNestedString(m, "erc721Token", "amount"), findNestedString(m, "erc1155Metadata", "amount"))
	assetType := firstNonEmptyString(findStringByKeys(m, "asset_type", "assetType", "category"), findNestedString(m, "rawContract", "assetType"))
	return normalizedAlchemyActivity{
		EventType:       eventType,
		Network:         normalizeNetwork(findStringByKeys(m, "network", "networkName", "chain")),
		Address:         normalizeAddressForStorage(findStringByKeys(m, "address")),
		FromAddress:     normalizeAddressForStorage(findStringByKeys(m, "fromAddress", "from", "from_address")),
		ToAddress:       normalizeAddressForStorage(findStringByKeys(m, "toAddress", "to", "to_address")),
		WalletAddress:   normalizeAddressForStorage(findStringByKeys(m, "walletAddress", "wallet_address")),
		ContractAddress: normalizeAddressForStorage(contractAddress),
		TxHash:          findStringByKeys(m, "hash", "txHash", "tx_hash", "transactionHash"),
		BlockNumber:     firstNonEmptyString(findStringByKeys(m, "blockNum", "blockNumber", "block_number"), stringifyWeb3JSONValue(findValueByKeys(m, "block"))),
		Direction:       findStringByKeys(m, "direction"),
		AssetType:       assetType,
		Amount:          amount,
		RawJSON:         raw,
	}
}

func (a normalizedAlchemyActivity) extractedAddresses() []string {
	return uniqueNonEmptyStrings(a.Address, a.FromAddress, a.ToAddress, a.WalletAddress, a.ContractAddress)
}

func (h *Handler) findSourceByAnyAddress(addresses []string) (matchedSource, error) {
	var source matchedSource
	for _, addr := range addresses {
		normalized := normalizeAddressForStorage(addr)
		if normalized == "" {
			continue
		}
		err := h.DB.QueryRow(`
			SELECT id, email, COALESCE(chain,''), COALESCE(network,''), address
			FROM web3_event_sources
			WHERE lower(address)=lower($1)
			  AND provider='alchemy'
			  AND COALESCE(status, '') <> 'paused'
			ORDER BY created_at DESC
			LIMIT 1`, normalized).Scan(&source.ID, &source.Email, &source.Chain, &source.Network, &source.Address)
		if err == nil {
			return source, nil
		}
		if err != sql.ErrNoRows {
			return source, err
		}
	}
	return source, nil
}

func (h *Handler) insertWeb3Event(activity normalizedAlchemyActivity, source matchedSource, verificationStatus string) error {
	address := firstNonEmptyString(source.Address, activity.WalletAddress, activity.Address, activity.FromAddress, activity.ToAddress)
	walletAddress := firstNonEmptyString(activity.WalletAddress, activity.Address, activity.FromAddress, activity.ToAddress)
	_, err := h.DB.Exec(`
		INSERT INTO web3_events (
			id, source_id, email, provider, chain, network, event_type, address, tx_hash, block_number, direction, asset_type, amount, raw_payload, verification_status, status,
			wallet_address, contract_address, amount_text, user_id, source_name, risk_level, payload_hash
		) VALUES (
			$1,$2,$3,'alchemy',$4,$5,$6,$7,$8,$9,$10,$11,$12,$13::jsonb,$14,'received',
			$15,$16,$17,$18,$19,'unknown',$20
		)`,
		newID(), nullIfEmpty(source.ID), nullIfEmpty(source.Email), nullIfEmpty(source.Chain), nullIfEmpty(activity.Network), nullIfEmpty(activity.EventType), nullIfEmpty(address), nullIfEmpty(activity.TxHash), nullIfEmpty(activity.BlockNumber), nullIfEmpty(activity.Direction), nullIfEmpty(activity.AssetType), nullIfEmpty(activity.Amount), activity.RawJSON, verificationStatus, nullIfEmpty(walletAddress), nullIfEmpty(activity.ContractAddress), nullIfEmpty(activity.Amount), nullIfEmpty(source.Email), nullIfEmpty(source.ID), sha256Hex([]byte(activity.RawJSON)))
	return err
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
		source, scanErr := scanWatchlistSource(rows)
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
	if label == "" || chain == "" || network == "" || sourceType != "wallet" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_source"})
		return
	}
	if err := h.enforceFreeWatchlistLimit(email); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
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

func (h *Handler) getWeb3SourceForEmail(id, email string) (web3EventSource, error) {
	row := h.DB.QueryRow(`
		SELECT id, email, label, chain, network, address, source_type, status, provider, webhook_url, provider_setup_status, last_event_at, notes, created_at, updated_at
		FROM web3_event_sources
		WHERE id=$1 AND lower(email)=lower($2)`, id, email)
	return scanWeb3Source(row)
}

type web3EventScanner interface {
	Scan(dest ...any) error
}

func scanWeb3Event(scanner web3EventScanner) (web3Event, error) {
	var event web3Event
	var sourceID, email, chain, network, eventType, address, txHash, blockNumber, direction, assetType, amount, walletAddress, contractAddress, amountText sql.NullString
	var raw []byte
	if err := scanner.Scan(&event.ID, &sourceID, &email, &event.Provider, &chain, &network, &eventType, &address, &txHash, &blockNumber, &direction, &assetType, &amount, &walletAddress, &contractAddress, &amountText, &raw, &event.Status, &event.VerificationStatus, &event.CreatedAt); err != nil {
		return event, err
	}
	event.SourceID = stringPtrFromNull(sourceID)
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
	event.AmountText = stringPtrFromNull(amountText)
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
	var lastEventAt sql.NullTime
	var notes sql.NullString
	if err := scanner.Scan(&source.ID, &source.Email, &source.Label, &source.Chain, &source.Network, &source.Address, &source.SourceType, &source.Status, &source.Provider, &source.WebhookURL, &source.ProviderSetupStatus, &lastEventAt, &notes, &source.CreatedAt, &source.UpdatedAt); err != nil {
		return source, err
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
	source.Notes = stringPtrFromNull(notes)
	return source, nil
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
}

	event, err := h.getWeb3EventForUser(eventID, normalizedClaimEmail(claims))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
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
}

	summary := deterministicWeb3Summary(event)
	if _, err := h.DB.Exec(`UPDATE web3_events SET ai_summary=$2 WHERE id=$1 AND lower(email)=lower($3)`, id, summary, email); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}
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

func web3SourceIDFromPath(path string) string {
	id := strings.Trim(strings.TrimPrefix(path, "/api/web3/sources/"), "/")
	if id == "" || strings.Contains(id, "/") {
		return ""
	}
	return id
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
	address := valueOrUnknown(event.Address)
	contract := valueOrUnknown(event.ContractAddress)
	return fmt.Sprintf("Read-only Web3 Bridge monitor received a %s event on %s. Transaction: %s. Address: %s. Contract: %s. No private keys, custody, escrow, or automatic transfers are used.", eventType, network, txHash, address, contract)
}

func normalizeAndValidateSourceAddress(chain, raw string) (string, error) {
	address := strings.TrimSpace(raw)
	if address == "" {
		return "", fmt.Errorf("invalid_address")
	}
	chain = strings.ToLower(strings.TrimSpace(chain))
	if chain == "solana" || chain == "sol" {
		if !solanaAddressRe.MatchString(address) {
			return "", fmt.Errorf("invalid_solana_address")
		}
		return address, nil
	}
	if !evmAddressRe.MatchString(address) {
		return "", fmt.Errorf("invalid_evm_address")
	}
	return strings.ToLower(address), nil
}

func normalizeAddressForStorage(raw string) string {
	address := strings.TrimSpace(raw)
	if evmAddressRe.MatchString(address) {
		return strings.ToLower(address)
	}
	return address
}

func normalizeNetwork(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func displayNetwork(network string) string {
	switch strings.ToLower(strings.TrimSpace(network)) {
	case "base-mainnet", "base", "base mainnet":
		return "Base Mainnet"
	case "eth-mainnet", "ethereum-mainnet", "ethereum":
		return "Ethereum Mainnet"
	case "solana-mainnet", "solana":
		return "Solana Mainnet"
	default:
		parts := strings.FieldsFunc(network, func(r rune) bool { return r == '-' || r == '_' })
		for i, p := range parts {
			if p != "" {
				parts[i] = strings.ToUpper(p[:1]) + p[1:]
			}
		}
		return strings.Join(parts, " ")
	}
}

func publicAlchemyWebhookURL() string {
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("PUBLIC_APP_URL")), "/")
	if base == "" {
		base = strings.TrimRight(strings.TrimSpace(os.Getenv("APP_PUBLIC_URL")), "/")
	}
	if base == "" {
		base = strings.TrimRight(strings.TrimSpace(os.Getenv("CORS_ALLOWED_ORIGIN")), "/")
	}
	if base == "" {
		return alchemyWebhookURLFallback
	}
	return base + alchemyWebhookPath
}

func secureRandomToken() string {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return newID() + newID()
	}
	return hex.EncodeToString(b[:])
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

func findValueByKeys(v any, keys ...string) any {
	m, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	for _, key := range keys {
		for k, val := range m {
			if strings.EqualFold(k, key) {
				return val
			}
		}
	}
	for _, val := range m {
		if nested := findValueByKeys(val, keys...); nested != nil {
			return nested
		}
	}
	return nil
}

func findStringByKeys(v any, keys ...string) string {
	return stringifyWeb3JSONValue(findValueByKeys(v, keys...))
}

func findNestedString(m map[string]any, path ...string) string {
	var cur any = m
	for _, key := range path {
		obj, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		cur = nil
		for k, v := range obj {
			if strings.EqualFold(k, key) {
				cur = v
				break
			}
		}
		if cur == nil {
			return ""
		}
	}
	return stringifyWeb3JSONValue(cur)
}

func stringifyWeb3JSONValue(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(t)
	case float64:
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.8f", t), "0"), ".")
	case bool:
		if t {
			return "true"
		}
		return "false"
	case json.Number:
		return t.String()
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return ""
		}
		return strings.Trim(string(b), "\"")
	}
}

func uniqueNonEmptyStrings(values ...string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, value)
	}
	return out
}

func requestIP(r *http.Request) string {
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
		return strings.TrimSpace(strings.Split(forwarded, ",")[0])
	}
	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		return realIP
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func webhookSecretFromRequest(r *http.Request) string {
	if secret := strings.TrimSpace(r.Header.Get("X-Koschei-Webhook-Secret")); secret != "" {
		return secret
	}
	if auth := strings.TrimSpace(r.Header.Get("Authorization")); strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[7:])
	}
	return strings.TrimSpace(r.Header.Get("X-Alchemy-Token"))
}

func constantTimeEqualSHA256(raw, expectedHex string) bool {
	actual := sha256Hex([]byte(raw))
	return constantTimeStringEqual(strings.ToLower(actual), strings.ToLower(strings.TrimSpace(expectedHex)))
}

func constantTimeStringEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func nullIfEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return strings.TrimSpace(s)
}

func stringPtrFromNull(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	return &v.String
}

func valueOrUnknown(v *string) string {
	if v == nil || strings.TrimSpace(*v) == "" {
		return "unknown"
	}
	return strings.TrimSpace(*v)
}
