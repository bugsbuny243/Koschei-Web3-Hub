package services

// Helius Wallet Identity labeling for holder wallets.
//
// A resolved holder wallet may still be an exchange, market maker, or protocol
// treasury that on-chain program metadata alone cannot reveal (System Program
// ownership just means "a normal wallet", not "an unlabeled personal wallet").
// This module asks the Helius Wallet Identity API whether an address maps to a
// known entity, so "ROL ÇÖZÜLMEDİ / Normal cüzdan" can become "Binance",
// "OKX", "Wintermute", etc.
//
// Design rules honored:
//   - Reuses heliusEnhancedAPIKey; no new credentials. If no key resolves, the
//     labeler is a no-op and existing role behavior is untouched.
//   - Never fabricates: only labels the API positively returns are surfaced.
//     An empty/unknown response yields no label, not a guess.
//   - Process-level cache: the same exchange wallet appears across many scans;
//     each address is queried at most once per process lifetime.
//   - Identity lookups are archival calls (1 credit each) and only run for the
//     bounded Top-N holder set, so credit cost stays negligible.

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const heliusIdentityBaseURL = "https://api.helius.xyz/v1/wallet"

// WalletLabel is a positively-resolved entity label for an address.
type WalletLabel struct {
	Address  string   `json:"address"`
	Name     string   `json:"name,omitempty"`     // e.g. "Binance Hot Wallet 1"
	Entity   string   `json:"entity,omitempty"`   // e.g. "Binance"
	Category string   `json:"category,omitempty"` // e.g. "CEX", "MARKET_MAKER", "PROGRAM"
	Labels   []string `json:"labels,omitempty"`   // explicit third-party taxonomy labels
	Tags     []string `json:"tags,omitempty"`
	Source   string   `json:"source"` // always "helius_identity" for provenance
}

type heliusIdentityResponse struct {
	Name     string   `json:"name"`
	Entity   string   `json:"entity"`
	Category string   `json:"category"`
	Labels   []string `json:"labels"`
	Tags     []string `json:"tags"`
}

var (
	walletLabelCache   = map[string]*WalletLabel{}
	walletLabelCacheMu sync.RWMutex
)

// labelCacheGet returns a cached label. The bool distinguishes "cached as
// unlabeled" (present, nil value) from "never queried".
func labelCacheGet(address string) (*WalletLabel, bool) {
	walletLabelCacheMu.RLock()
	defer walletLabelCacheMu.RUnlock()
	label, ok := walletLabelCache[address]
	return label, ok
}

func labelCacheSet(address string, label *WalletLabel) {
	walletLabelCacheMu.Lock()
	defer walletLabelCacheMu.Unlock()
	walletLabelCache[address] = label
}

// ResolveWalletLabel returns a positively-resolved entity label for a holder
// wallet, or nil when the address is unknown, unlabeled, or unresolvable. It
// never returns an error to callers: labeling is best-effort enrichment and
// must never break a scan.
func ResolveWalletLabel(ctx context.Context, rpcURL, address string) *WalletLabel {
	address = strings.TrimSpace(address)
	if address == "" {
		return nil
	}
	if cached, ok := labelCacheGet(address); ok {
		return cached
	}

	apiKey := heliusEnhancedAPIKey(rpcURL)
	if apiKey == "" {
		labelCacheSet(address, nil)
		return nil
	}

	endpoint := heliusIdentityBaseURL + "/" + url.PathEscape(address) + "/identity"
	query := url.Values{}
	query.Set("api-key", apiKey)

	reqCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, endpoint+"?"+query.Encode(), nil)
	if err != nil {
		labelCacheSet(address, nil)
		return nil
	}
	req.Header.Set("Accept", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		// Transient failure: do NOT cache, so a later scan can retry.
		return nil
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusNotFound {
		// Definitively unlabeled: cache the negative to avoid re-querying.
		labelCacheSet(address, nil)
		return nil
	}
	if res.StatusCode != http.StatusOK {
		return nil // transient; don't cache
	}

	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return nil
	}
	var decoded heliusIdentityResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		labelCacheSet(address, nil)
		return nil
	}

	name := strings.TrimSpace(decoded.Name)
	entity := strings.TrimSpace(decoded.Entity)
	category := strings.TrimSpace(decoded.Category)
	if category == "" && len(decoded.Labels) > 0 {
		category = strings.TrimSpace(decoded.Labels[0])
	}
	if name == "" && entity == "" && category == "" && len(decoded.Labels) == 0 && len(decoded.Tags) == 0 {
		labelCacheSet(address, nil) // resolved but genuinely unlabeled
		return nil
	}

	label := &WalletLabel{
		Address:  address,
		Name:     name,
		Entity:   entity,
		Category: category,
		Labels:   append([]string{}, decoded.Labels...),
		Tags:     append([]string{}, decoded.Tags...),
		Source:   "helius_identity",
	}
	labelCacheSet(address, label)
	return label
}

// walletLabelDisplay renders a short human label for a holder row, or "" when
// unlabeled. Prefers entity ("Binance") over the full deposit-address name.
func walletLabelDisplay(label *WalletLabel) string {
	if label == nil {
		return ""
	}
	switch {
	case label.Entity != "":
		if label.Category != "" {
			return label.Entity + " · " + label.Category
		}
		return label.Entity
	case label.Name != "":
		return label.Name
	default:
		return label.Category
	}
}
