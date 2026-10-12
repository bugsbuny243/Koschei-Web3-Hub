package radaradapter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ReadTronMainnetHeight reads the current head height from the fixed TronGrid
// endpoint. It does not prove a separate getaccount response belongs to this block.
func ReadTronMainnetHeight(ctx context.Context, client *http.Client) (int64, error) {
	const endpoint = "https://api.trongrid.io/wallet/getnowblock"
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}
	bounded := *client
	bounded.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	if bounded.Timeout == 0 || bounded.Timeout > 8*time.Second {
		bounded.Timeout = 8 * time.Second
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader([]byte("{}")))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := bounded.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("tron getnowblock HTTP %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20+1))
	if err != nil {
		return 0, err
	}
	if len(raw) > 1<<20 {
		return 0, errors.New("tron block response exceeds 1 MiB")
	}
	var block struct {
		BlockHeader struct {
			RawData struct {
				Number int64 `json:"number"`
			} `json:"raw_data"`
		} `json:"block_header"`
	}
	if err := json.Unmarshal(raw, &block); err != nil {
		return 0, err
	}
	if block.BlockHeader.RawData.Number <= 0 {
		return 0, errors.New("tron block height missing or invalid")
	}
	return block.BlockHeader.RawData.Number, nil
}

// NewTronMainnetWalletAdapter wires a live height reader and account collector.
// This constructor does not register a universal executable route.
func NewTronMainnetWalletAdapter(client *http.Client, address string) (*TronWalletAdapter, error) {
	return NewTronWalletAdapter(client, func(ctx context.Context) (int64, error) {
		return ReadTronMainnetHeight(ctx, client)
	}, address)
}
