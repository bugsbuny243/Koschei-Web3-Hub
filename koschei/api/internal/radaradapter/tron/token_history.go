package tron

import (
	"context"
	"encoding/hex"
	"encoding/json"
	r "koschei/api/internal/radaradapter"
	"net/url"
	"regexp"
)

var unsignedDecimal = regexp.MustCompile(`^(0|[1-9][0-9]*)$`)

// The V1 endpoint also returns TRC-721 and approval events. Preserve the event
// type; a historical Approval is never represented as a current allowance.
func (a *Adapter) tokenHistory(ctx context.Context, height string) ([]r.EvidenceRow, []r.Limitation) {
	res, err := a.transport.Do(ctx, r.Request{Network: a.target.Network, Provider: "indexed", Method: "GET", Path: "/v1/accounts/" + url.PathEscape(a.hexAddress) + "/transactions/trc20?only_confirmed=true&limit=50"})
	if err != nil {
		return nil, []r.Limitation{{Code: "indexed_token_history_unavailable", Reason: "Confirmed token-event history could not be read."}}
	}
	return a.tokenHistoryRows(res, height)
}
func (a *Adapter) tokenHistoryRows(res r.Response, height string) ([]r.EvidenceRow, []r.Limitation) {
	var payload struct {
		Success bool `json:"success"`
		Data    []struct {
			ID        string `json:"transaction_id"`
			From      string `json:"from"`
			To        string `json:"to"`
			Type      string `json:"type"`
			Value     string `json:"value"`
			Timestamp int64  `json:"block_timestamp"`
			Token     struct {
				Address  string `json:"address"`
				Decimals *int   `json:"decimals"`
			} `json:"token_info"`
		} `json:"data"`
	}
	limits := []r.Limitation{{Code: "bounded_indexed_token_history", Reason: "At most 50 indexed token events are inspected. Event timestamps are not block heights. Token standard, current allowance, balances and holder concentration are not established by this endpoint."}}
	rows := []r.EvidenceRow{}
	if json.Unmarshal(res.Raw, &payload) != nil || !payload.Success {
		return rows, append(limits, r.Limitation{Code: "indexed_token_history_invalid", Reason: "Token-event response schema or success flag was missing."})
	}
	rejected := false
	for i, item := range payload.Data {
		if i >= 50 {
			break
		}
		from, fe := Address(item.From)
		to, te := Address(item.To)
		token, ce := Address(item.Token.Address)
		id, ie := hex.DecodeString(item.ID)
		if fe != nil || te != nil || ce != nil || ie != nil || len(id) != 32 || (from != a.hexAddress && to != a.hexAddress) || item.Timestamp <= 0 || !unsignedDecimal.MatchString(item.Value) || (item.Type != "Transfer" && item.Type != "Approval") {
			rejected = true
			continue
		}
		facts := map[string]any{"transaction_hash": item.ID, "from": from, "to": to, "token_contract": token, "event_type": item.Type, "value_raw": item.Value, "block_timestamp_ms": item.Timestamp, "height_kind": "confirmed_head_context_not_event_height", "event_height_verified": false, "token_standard": "unverified"}
		if item.Token.Decimals != nil && *item.Token.Decimals >= 0 && *item.Token.Decimals <= 255 {
			facts["indexed_decimals"] = *item.Token.Decimals
		}
		rows = append(rows, r.NewRow(a.subject, "tron-indexed-token-event", res, height, facts))
	}
	if rejected {
		limits = append(limits, r.Limitation{Code: "indexed_token_events_rejected", Reason: "Malformed or unrelated indexed events were excluded."})
	}
	return rows, limits
}
