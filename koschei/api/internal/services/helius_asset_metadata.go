package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	heliusDASMainnetURL         = "https://mainnet.helius-rpc.com/"
	heliusAssetMetadataCacheTTL = 30 * time.Minute
)

type heliusAssetMetadata struct {
	TokenStandard string
	Decimals      *int
	Available     bool
}

type heliusDASAssetResponse struct {
	Result struct {
		Interface string `json:"interface"`
		Content   struct {
			Metadata struct {
				TokenStandard string `json:"token_standard"`
			} `json:"metadata"`
		} `json:"content"`
		TokenInfo struct {
			Decimals *int `json:"decimals"`
		} `json:"token_info"`
	} `json:"result"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type heliusAssetMetadataCacheEntry struct {
	Metadata  heliusAssetMetadata
	ExpiresAt time.Time
}

var heliusAssetMetadataCache = struct {
	sync.RWMutex
	Items map[string]heliusAssetMetadataCacheEntry
}{Items: map[string]heliusAssetMetadataCacheEntry{}}

func resolveHeliusAssetMetadata(ctx context.Context, apiKey, mint string, budget *holderScanRPCBudget) heliusAssetMetadata {
	mint = strings.TrimSpace(mint)
	if mint == "" || strings.TrimSpace(apiKey) == "" {
		return heliusAssetMetadata{}
	}
	if cached, ok := cachedHeliusAssetMetadata(mint); ok {
		return cached
	}
	if budget != nil && !budget.Reserve(1) {
		return heliusAssetMetadata{}
	}
	metadata, err := fetchHeliusAssetMetadata(ctx, apiKey, mint)
	if err != nil || !metadata.Available {
		return heliusAssetMetadata{}
	}
	cacheHeliusAssetMetadata(mint, metadata)
	return metadata
}

func fetchHeliusAssetMetadata(ctx context.Context, apiKey, mint string) (heliusAssetMetadata, error) {
	query := url.Values{}
	query.Set("api-key", strings.TrimSpace(apiKey))
	payload, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      "koschei-holder-flow",
		"method":  "getAsset",
		"params": map[string]any{
			"id": strings.TrimSpace(mint),
			"options": map[string]any{
				"showFungible": true,
			},
		},
	})
	if err != nil {
		return heliusAssetMetadata{}, err
	}
	reqCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, heliusDASMainnetURL+"?"+query.Encode(), bytes.NewReader(payload))
	if err != nil {
		return heliusAssetMetadata{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return heliusAssetMetadata{}, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if err != nil {
		return heliusAssetMetadata{}, err
	}
	if res.StatusCode != http.StatusOK {
		return heliusAssetMetadata{}, fmt.Errorf("helius DAS status %d: %s", res.StatusCode, compactClusterError(fmt.Errorf("%s", strings.TrimSpace(string(body)))))
	}
	var response heliusDASAssetResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return heliusAssetMetadata{}, fmt.Errorf("helius DAS decode: %w", err)
	}
	if response.Error != nil {
		return heliusAssetMetadata{}, fmt.Errorf("helius DAS error %d: %s", response.Error.Code, response.Error.Message)
	}
	standard := strings.TrimSpace(response.Result.Content.Metadata.TokenStandard)
	if standard == "" {
		standard = strings.TrimSpace(response.Result.Interface)
	}
	metadata := heliusAssetMetadata{
		TokenStandard: standard,
		Decimals:      firstHolderClusterDecimals(response.Result.TokenInfo.Decimals),
	}
	metadata.Available = metadata.TokenStandard != "" || metadata.Decimals != nil
	return metadata, nil
}

func heliusTransferDecimals(tx heliusEnhancedTransaction, transfer heliusTokenTransfer) *int {
	if transfer.Decimals != nil {
		return firstHolderClusterDecimals(transfer.Decimals)
	}
	mint := strings.TrimSpace(transfer.Mint)
	fromTokenAccount := strings.TrimSpace(transfer.FromTokenAccount)
	toTokenAccount := strings.TrimSpace(transfer.ToTokenAccount)
	fromUserAccount := strings.TrimSpace(transfer.FromUserAccount)
	toUserAccount := strings.TrimSpace(transfer.ToUserAccount)
	for _, account := range tx.AccountData {
		for _, change := range account.TokenBalanceChanges {
			if mint != "" && !strings.EqualFold(strings.TrimSpace(change.Mint), mint) {
				continue
			}
			tokenAccount := strings.TrimSpace(change.TokenAccount)
			userAccount := strings.TrimSpace(change.UserAccount)
			matchesEndpoint := (fromTokenAccount != "" && tokenAccount == fromTokenAccount) ||
				(toTokenAccount != "" && tokenAccount == toTokenAccount) ||
				(fromUserAccount != "" && userAccount == fromUserAccount) ||
				(toUserAccount != "" && userAccount == toUserAccount)
			if !matchesEndpoint || change.RawTokenAmount.Decimals == nil {
				continue
			}
			return firstHolderClusterDecimals(change.RawTokenAmount.Decimals)
		}
	}
	return nil
}

func cachedHeliusAssetMetadata(mint string) (heliusAssetMetadata, bool) {
	heliusAssetMetadataCache.RLock()
	entry, ok := heliusAssetMetadataCache.Items[mint]
	heliusAssetMetadataCache.RUnlock()
	if !ok {
		return heliusAssetMetadata{}, false
	}
	if time.Now().After(entry.ExpiresAt) {
		heliusAssetMetadataCache.Lock()
		delete(heliusAssetMetadataCache.Items, mint)
		heliusAssetMetadataCache.Unlock()
		return heliusAssetMetadata{}, false
	}
	return entry.Metadata, true
}

func cacheHeliusAssetMetadata(mint string, metadata heliusAssetMetadata) {
	heliusAssetMetadataCache.Lock()
	heliusAssetMetadataCache.Items[mint] = heliusAssetMetadataCacheEntry{Metadata: metadata, ExpiresAt: time.Now().Add(heliusAssetMetadataCacheTTL)}
	heliusAssetMetadataCache.Unlock()
}

func resetHeliusAssetMetadataCacheForTest() {
	heliusAssetMetadataCache.Lock()
	heliusAssetMetadataCache.Items = map[string]heliusAssetMetadataCacheEntry{}
	heliusAssetMetadataCache.Unlock()
}
