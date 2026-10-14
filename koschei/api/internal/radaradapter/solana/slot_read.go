package solana

import (
 "context"
 "encoding/json"
 "errors"
 radar "koschei/api/internal/radaradapter"
)

// ReadFinalizedSlot reads a finalized slot from a read-only RPC provider.
func ReadFinalizedSlot(ctx context.Context, t radar.Transport, network string) (uint64, error) {
 _, raw, err := radar.RPC(ctx, t, network, "primary", "getSlot", []any{map[string]any{"commitment": "finalized"}})
 if err != nil { return 0, err }
 var slot uint64
 if err := json.Unmarshal(raw, &slot); err != nil || slot == 0 { return 0, errors.New("invalid finalized slot") }
 return slot, nil
}
