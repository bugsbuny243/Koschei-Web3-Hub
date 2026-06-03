package handlers

import (
	"crypto/hmac"
	"crypto/rand"
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
	"regexp"
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
	Email               string     `json:"email"`
	Label               string     `json:"label"`
	Chain               string     `json:"chain"`
	Network             string     `json:"network"`
	Address             string     `json:"address"`
	SourceType          string     `json:"source_type"`
	Status              string     `json:"status"`
	Provider            string     `json:"provider"`
	WebhookURL          string     `json:"webhook_url"`
	ProviderSetupStatus string     `json:"provider_setup_status"`
	LastEventAt         *time.Time `json:"last_event_at,omitempty"`
	Notes               *string    `json:"notes,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

type matchedSource struct {
	ID      string
	Email   string
	Chain   string
	Network string
	Address string
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
	email := strings.ToLower(strings.TrimSpace(claims.Email))
	limit := parseLimit(r.URL.Query().Get("limit"), 50, 100)
	rows, err := h.DB.Query(`
		SELECT id, source_id, email, provider, chain, network, event_type, address, tx_hash, block_number, direction, asset_type, amount, wallet_address, contract_address, amount_text, raw_payload, status, verification_status, created_at
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
	w.WriteHeader(http.StatusMethodNotAllowed)
}

func (h *Handler) listWeb3Sources(w http.ResponseWriter, r *http.Request) {
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	email := strings.ToLower(strings.TrimSpace(claims.Email))
	rows, err := h.DB.Query(`
		SELECT id, email, label, chain, network, address, source_type, status, provider, webhook_url, provider_setup_status, last_event_at, notes, created_at, updated_at
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
	Label      string `json:"label"`
	Chain      string `json:"chain"`
	Network    string `json:"network"`
	Address    string `json:"address"`
	SourceType string `json:"source_type"`
	Notes      string `json:"notes"`
}

func (h *Handler) createWeb3Source(w http.ResponseWriter, r *http.Request) {
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	email := strings.ToLower(strings.TrimSpace(claims.Email))
	if email == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing_email"})
		return
	}
	var req web3SourceRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}
	label := strings.TrimSpace(req.Label)
	chain := strings.ToLower(strings.TrimSpace(req.Chain))
	network := strings.ToLower(strings.TrimSpace(req.Network))
	sourceType := strings.ToLower(strings.TrimSpace(firstNonEmptyString(req.SourceType, "wallet")))
	address, err := normalizeAndValidateSourceAddress(chain, req.Address)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
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

	sourceID := newID()
	setupToken := secureRandomToken()
	webhookURL := publicAlchemyWebhookURL()
	_, err = h.DB.Exec(`
		INSERT INTO web3_event_sources (
			id, email, label, chain, network, address, source_type, status, provider, setup_token, webhook_url, provider_setup_status, notes,
			user_id, name, is_active, verification_mode
		) VALUES ($1,$2,$3,$4,$5,$6,'wallet','waiting_for_setup','alchemy',$7,$8,'manual_required',$9,$10,$3,true,'alchemy_signature')`,
		sourceID, email, label, chain, network, address, setupToken, webhookURL, nullIfEmpty(req.Notes), currentUserID(claims))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}
	source, err := h.getWeb3SourceForEmail(sourceID, email)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"ok":     true,
		"source": source,
		"setup": map[string]any{
			"provider":       "Alchemy",
			"webhook_type":   "Address Activity",
			"network":        displayNetwork(network),
			"address_to_add": address,
			"webhook_url":    webhookURL,
			"message":        "Add this address to your Alchemy Address Activity webhook. Events will appear after real on-chain activity.",
		},
	})
}

func (h *Handler) enforceFreeWatchlistLimit(email string) error {
	planID := "free"
	_ = h.DB.QueryRow(`
		SELECT COALESCE(plan_id, 'free')
		FROM entitlements
		WHERE lower(email)=lower($1) AND status='active'
		ORDER BY CASE WHEN COALESCE(plan_id,'free') <> 'free' THEN 0 ELSE 1 END, created_at DESC
		LIMIT 1`, email).Scan(&planID)
	if planID != "" && strings.ToLower(planID) != "free" {
		return nil
	}
	var count int
	if err := h.DB.QueryRow(`SELECT COUNT(*) FROM web3_event_sources WHERE lower(email)=lower($1)`, email).Scan(&count); err != nil {
		return fmt.Errorf("db_failed")
	}
	if count >= 1 {
		return fmt.Errorf("free_plan_source_limit_reached")
	}
	return nil
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
	if lastEventAt.Valid {
		source.LastEventAt = &lastEventAt.Time
	}
	source.Notes = stringPtrFromNull(notes)
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
