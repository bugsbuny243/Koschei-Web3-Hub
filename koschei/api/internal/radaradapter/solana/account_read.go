package solana

import (
 "context"
 "encoding/json"
 "errors"
 "fmt"
 radar "koschei/api/internal/radaradapter"
)

// ReadAccountAtSlot requires a provider context at or beyond the requested
// finalized slot. minContextSlot is a lower bound, not atomic snapshot proof.
func ReadAccountAtSlot(ctx context.Context, t radar.Transport, network, address string, slot uint64) (radar.Response, Account, error) {
 response, raw, err := radar.RPC(ctx, t, network, "primary", "getAccountInfo", []any{address, map[string]any{"encoding":"base64","commitment":"finalized","minContextSlot":slot}})
 if err != nil { return response, Account{}, err }
 var envelope struct { Context struct { Slot uint64 `json:"slot"` } `json:"context"` }
 if err := json.Unmarshal(raw, &envelope); err != nil || envelope.Context.Slot < slot { return response, Account{}, errors.New("missing or stale account context") }
 account, err := DecodeAccountInfo(raw)
 if err != nil { return response, Account{}, fmt.Errorf("decode Solana account: %w", err) }
 return response, account, nil
}
