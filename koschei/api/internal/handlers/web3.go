package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type web3Event struct {
	ID              string    `json:"id"`
	SourceID        *string   `json:"source_id,omitempty"`
	UserID          *string   `json:"user_id,omitempty"`
	Provider        string    `json:"provider"`
	Network         *string   `json:"network,omitempty"`
	EventType       *string   `json:"event_type,omitempty"`
	TxHash          *string   `json:"tx_hash,omitempty"`
	WalletAddress   *string   `json:"wallet_address,omitempty"`
	ContractAddress *string   `json:"contract_address,omitempty"`
	TokenID         *string   `json:"token_id,omitempty"`
	AmountText      *string   `json:"amount_text,omitempty"`
	RawPayload      any       `json:"raw_payload,omitempty"`
	AISummary       *string   `json:"ai_summary,omitempty"`
	RiskLevel       string    `json:"risk_level"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"created_at"`
}

func (h *Handler) Web3AlchemyEvent(w http.ResponseWriter, r *http.Request) {
	// TODO: Verify Alchemy webhook signatures before production use.
	var payload map[string]any
	if err := decodeJSON(r, &payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}

	raw, _ := json.Marshal(payload)
	eventID := newID()
	network := firstNonEmptyString(findStringByKeys(payload, "network"), findStringByKeys(payload, "chain", "blockchain"))
	eventType := firstNonEmptyString(findStringByKeys(payload, "event_type", "eventType"), findStringByKeys(payload, "type", "category"))
	txHash := findStringByKeys(payload, "tx_hash", "txHash", "transactionHash", "hash")
	walletAddress := findStringByKeys(payload, "wallet_address", "walletAddress", "fromAddress", "toAddress", "from", "to", "address")
	contractAddress := findStringByKeys(payload, "contract_address", "contractAddress", "rawContract", "contract")
	userID := findStringByKeys(payload, "user_id", "userId")

	if _, err := h.DB.Exec(`
		INSERT INTO web3_events (id, user_id, provider, network, event_type, tx_hash, wallet_address, contract_address, raw_payload, risk_level, status)
		VALUES ($1,$2,'alchemy',$3,$4,$5,$6,$7,$8::jsonb,'unknown','received')`,
		eventID, nullIfEmpty(userID), nullIfEmpty(network), nullIfEmpty(eventType), nullIfEmpty(txHash), nullIfEmpty(walletAddress), nullIfEmpty(contractAddress), string(raw)); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "event_id": eventID})
}

func (h *Handler) Web3Events(w http.ResponseWriter, r *http.Request) {
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	userID := currentUserID(claims)
	rows, err := h.DB.Query(`
		SELECT id, source_id, user_id, provider, network, event_type, tx_hash, wallet_address, contract_address, token_id, amount_text, raw_payload, ai_summary, risk_level, status, created_at
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
	if _, err := h.DB.Exec(`
		INSERT INTO web3_events (id, user_id, provider, network, event_type, tx_hash, wallet_address, contract_address, raw_payload, risk_level, status)
		VALUES ($1,$2,'alchemy','solana-devnet','test_event',$3,$4,$5,$6::jsonb,'low','received')`,
		eventID, userID, "demo-tx-"+eventID[:8], "demo-wallet-read-only", "demo-contract-read-only", string(raw)); err != nil {
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
		SELECT id, source_id, user_id, provider, network, event_type, tx_hash, wallet_address, contract_address, token_id, amount_text, raw_payload, ai_summary, risk_level, status, created_at
		FROM web3_events
		WHERE id=$1 AND user_id=$2`, id, userID)
	return scanWeb3Event(row)
}

type web3EventScanner interface {
	Scan(dest ...any) error
}

func scanWeb3Event(scanner web3EventScanner) (web3Event, error) {
	var event web3Event
	var sourceID, userID, network, eventType, txHash, walletAddress, contractAddress, tokenID, amountText, aiSummary sql.NullString
	var raw []byte
	if err := scanner.Scan(&event.ID, &sourceID, &userID, &event.Provider, &network, &eventType, &txHash, &walletAddress, &contractAddress, &tokenID, &amountText, &raw, &aiSummary, &event.RiskLevel, &event.Status, &event.CreatedAt); err != nil {
		return event, err
	}
	event.SourceID = stringPtrFromNull(sourceID)
	event.UserID = stringPtrFromNull(userID)
	event.Network = stringPtrFromNull(network)
	event.EventType = stringPtrFromNull(eventType)
	event.TxHash = stringPtrFromNull(txHash)
	event.WalletAddress = stringPtrFromNull(walletAddress)
	event.ContractAddress = stringPtrFromNull(contractAddress)
	event.TokenID = stringPtrFromNull(tokenID)
	event.AmountText = stringPtrFromNull(amountText)
	event.AISummary = stringPtrFromNull(aiSummary)
	if len(raw) > 0 {
		var payload any
		if err := json.Unmarshal(raw, &payload); err == nil {
			event.RawPayload = payload
		}
	}
	return event, nil
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
