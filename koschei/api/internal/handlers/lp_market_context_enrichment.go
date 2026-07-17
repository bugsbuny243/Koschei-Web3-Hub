package handlers

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"time"

	"koschei/api/internal/services"
)

func (h *Handler) collectCompleteLPControlEvidence(ctx context.Context, network, mint, creator string, market services.TokenMarketSnapshot, source map[string]any) services.LPControlEvidence {
	rpc := h.lpRPC()
	lp := collectLPControlEvidence(ctx, rpc, network, mint, creator, market, source)
	lp.TokenMint = strings.TrimSpace(mint)
	if lp.Available && lp.TokenReserve > 0 && market.PriceUSD > 0 {
		// Constant-product pools are approximately balanced by value at the read
		// slot. This is a context value derived from the direct token-vault reserve
		// and the separately timestamped market price; raw reserves remain primary.
		lp.ReserveLiquidityUSD = math.Round(lp.TokenReserve*market.PriceUSD*2*100) / 100
		lp.ReserveValueSource = "direct_token_vault_reserve_x_market_price_x2"
		lp.EvidenceKeys = append(lp.EvidenceKeys, fmt.Sprintf("reserve_value:%s@%d", lp.TokenVault, lp.ReadSlot))
	}
	if lp.LockerAccount == "" || lp.LockerProgram != streamflowProgram || rpc == nil {
		return lp
	}
	var account rpcAccountInfoResponse
	if err := rpc(ctx, network, "getAccountInfo", []any{lp.LockerAccount, map[string]any{"encoding":"base64","commitment":"confirmed"}}, &account); err != nil || account.Value == nil {
		lp.Limitations = append(lp.Limitations, "The Streamflow-owned account was observed, but its schedule account could not be read.")
		return lp
	}
	data, err := accountDataBytes(account.Value.Data)
	if err != nil {
		lp.Limitations = append(lp.Limitations, "The Streamflow-owned account was observed, but its schedule payload could not be decoded.")
		return lp
	}
	if unlock, ok := conservativeStreamflowUnlock(data, time.Now().UTC()); ok {
		lp.LockedUntil = &unlock
		lp.Status = services.LPControlVerifiedLocked
		lp.ReasonCode = "streamflow_schedule_observed"
		if account.Context.Slot > lp.ReadSlot { lp.ReadSlot = account.Context.Slot }
		lp.EvidenceKeys = append(lp.EvidenceKeys, fmt.Sprintf("locker:%s@%d", lp.LockerAccount, account.Context.Slot))
	}
	return lp
}

// conservativeStreamflowUnlock intentionally avoids guessing a lock timestamp.
// Streamflow schedules contain multiple Unix-second fields (start/cliff/end).
// A value is accepted only when at least two aligned plausible schedule times
// are present; the latest future time is then the bounded unlock reference.
func conservativeStreamflowUnlock(data []byte, now time.Time) (time.Time, bool) {
	if len(data) < 24 { return time.Time{}, false }
	lower := now.Add(-10 * 365 * 24 * time.Hour).Unix()
	upper := now.Add(20 * 365 * 24 * time.Hour).Unix()
	future := now.Add(time.Minute).Unix()
	plausible := make([]int64, 0, 4)
	for offset := 0; offset+8 <= len(data); offset += 8 {
		value := int64(binary.LittleEndian.Uint64(data[offset:offset+8]))
		if value >= lower && value <= upper { plausible = append(plausible, value) }
	}
	if len(plausible) < 2 { return time.Time{}, false }
	latest := int64(0)
	for _, value := range plausible { if value > latest { latest = value } }
	if latest < future { return time.Time{}, false }
	return time.Unix(latest, 0).UTC(), true
}
