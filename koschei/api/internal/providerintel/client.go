// Package providerintel implements bounded external security observations.
// Vendor results are never signed ARVIS verdicts or claims of safety.
package providerintel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const maxProviderResponse = 2 << 20

var safeIdentifier = regexp.MustCompile("^[a-zA-Z0-9:_-]{2,128}$")
var ErrUnavailable = errors.New("external intelligence provider unavailable")
var ErrInvalid = errors.New("invalid external intelligence request")

type Input struct {
	Provider  string          `json:"provider"`
	Operation string          `json:"operation"`
	Chain     string          `json:"chain,omitempty"`
	Address   string          `json:"address,omitempty"`
	Addresses []string        `json:"addresses,omitempty"`
	URL       string          `json:"url,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	Limit     int             `json:"limit,omitempty"`
}

type Result struct {
	Provider     string          `json:"provider"`
	Operation    string          `json:"operation"`
	Source       string          `json:"source"`
	Verification string          `json:"verification"`
	RiskLevel    string          `json:"risk_level"`
	Signed       bool            `json:"signed"`
	Data         json.RawMessage `json:"data"`
}

type Client struct {
	HTTP             *http.Client
	BlockaidKey      string
	GoPlusKey        string
	BubblemapsKey    string
	blockaidOrigin   string
	goplusOrigin     string
	bubblemapsOrigin string
}

func NewFromEnv() *Client {
	return &Client{
		HTTP: &http.Client{
			Timeout: 9 * time.Second,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		BlockaidKey:      strings.TrimSpace(os.Getenv("KOSCHEI_BLOCKAID_API_KEY")),
		GoPlusKey:        strings.TrimSpace(os.Getenv("KOSCHEI_GOPLUS_ACCESS_TOKEN")),
		BubblemapsKey:    strings.TrimSpace(os.Getenv("KOSCHEI_BUBBLEMAPS_API_KEY")),
		blockaidOrigin:   "https://api.blockaid.io",
		goplusOrigin:     "https://api.gopluslabs.io",
		bubblemapsOrigin: "https://api.bubblemaps.io",
	}
}

func validAddress(a string) bool {
	return safeIdentifier.MatchString(strings.TrimSpace(a))
}

func validChain(c string) bool {
	switch c {
	case "solana", "sol", "ethereum", "eth", "base", "bsc", "polygon", "arbitrum", "optimism", "avalanche":
		return true
	default:
		return false
	}
}

func goPlusChainID(chain string) string {
	switch chain {
	case "ethereum", "eth":
		return "1"
	case "bsc":
		return "56"
	case "polygon":
		return "137"
	case "arbitrum":
		return "42161"
	case "base":
		return "8453"
	case "optimism":
		return "10"
	case "avalanche":
		return "43114"
	}
	return ""
}

func bubbleChain(chain string) string {
	switch chain {
	case "solana", "sol":
		return "sol"
	case "ethereum", "eth":
		return "eth"
	case "base":
		return "base"
	case "bsc":
		return "bsc"
	case "polygon":
		return "poly"
	}
	return ""
}

func (c *Client) Query(ctx context.Context, in Input) (Result, error) {
	if ctx == nil {
		return Result{}, ErrInvalid
	}
	if c == nil {
		return Result{}, ErrUnavailable
	}
	in.Provider = strings.ToLower(strings.TrimSpace(in.Provider))
	in.Operation = strings.ToLower(strings.TrimSpace(in.Operation))
	in.Chain = strings.ToLower(strings.TrimSpace(in.Chain))
	in.Address = strings.TrimSpace(in.Address)
	result := Result{
		Provider: in.Provider, Operation: in.Operation, Verification: "external_observation_only",
		RiskLevel: "unknown", Signed: false,
	}
	if in.Provider == "slither" {
		if in.Operation != "findings" {
			return Result{}, ErrInvalid
		}
		evidence, err := ParseSlitherFindings(in.Payload)
		if err != nil {
			return Result{}, err
		}
		result.Source = "trail_of_bits_slither_json"
		result.Data, err = json.Marshal(evidence)
		return result, err
	}
	if !validChain(in.Chain) {
		return Result{}, fmt.Errorf("%w: unsupported chain", ErrInvalid)
	}

	var endpoint, origin, key, headerName string
	var data any
	method := http.MethodGet
	switch in.Provider {
	case "blockaid":
		key, headerName, origin = c.BlockaidKey, "X-API-Key", c.blockaidOrigin
		result.Source = "blockaid_api"
		switch in.Operation {
		case "token", "address":
			if !validAddress(in.Address) {
				return Result{}, ErrInvalid
			}
			method = http.MethodPost
			endpoint = "/v1/" + in.Operation + "/scan"
			data = map[string]any{"chain": in.Chain, "address": in.Address}
		case "transaction", "transaction_bulk":
			if goPlusChainID(in.Chain) == "" || len(in.Payload) == 0 || len(in.Payload) > 32768 || !json.Valid(in.Payload) {
				return Result{}, ErrInvalid
			}
			var request map[string]json.RawMessage
			if err := json.Unmarshal(in.Payload, &request); err != nil {
				return Result{}, ErrInvalid
			}
			if len(request["chain"]) == 0 || len(request["data"]) == 0 {
				return Result{}, ErrInvalid
			}
			if in.Operation == "transaction" && len(request["account_address"]) == 0 {
				return Result{}, ErrInvalid
			}
			if in.Operation == "transaction_bulk" {
				var items []json.RawMessage
				if err := json.Unmarshal(request["data"], &items); err != nil || len(items) == 0 || len(items) > 20 {
					return Result{}, ErrInvalid
				}
				endpoint = "/v0/evm/transaction-bulk/scan"
			} else {
				endpoint = "/v0/evm/transaction/scan"
			}
			method, data = http.MethodPost, request
		default:
			return Result{}, ErrInvalid
		}
	case "goplus":
		key, headerName, origin = c.GoPlusKey, "Authorization", c.goplusOrigin
		result.Source = "goplus_security_api"
		switch in.Operation {
		case "token":
			if !validAddress(in.Address) {
				return Result{}, ErrInvalid
			}
			if in.Chain == "solana" || in.Chain == "sol" {
				endpoint = "/api/v1/solana/token_security?contract_addresses=" + url.QueryEscape(in.Address)
			} else {
				id := goPlusChainID(in.Chain)
				if id == "" {
					return Result{}, ErrInvalid
				}
				endpoint = "/api/v1/token_security/" + id + "?contract_addresses=" + url.QueryEscape(in.Address)
			}
		case "address", "approval":
			if !validAddress(in.Address) {
				return Result{}, ErrInvalid
			}
			id := goPlusChainID(in.Chain)
			if id == "" {
				return Result{}, ErrInvalid
			}
			if in.Operation == "approval" {
				endpoint = "/api/v1/approval_security/" + id + "?contract_addresses=" + url.QueryEscape(in.Address)
			} else {
				endpoint = "/api/v1/address_security/" + url.PathEscape(in.Address) + "?chain_id=" + id
			}
		case "phishing":
			u, err := url.Parse(in.URL)
			if err != nil || len(in.URL) < 10 || len(in.URL) > 2048 || u.User != nil || u.Hostname() == "" || (u.Scheme != "https" && u.Scheme != "http") {
				return Result{}, ErrInvalid
			}
			endpoint = "/api/v1/phishing_site?url=" + url.QueryEscape(in.URL)
		default:
			return Result{}, ErrInvalid
		}
	case "bubblemaps":
		key, headerName, origin = c.BubblemapsKey, "X-API-Key", c.bubblemapsOrigin
		result.Source = "bubblemaps_data_api"
		chain := bubbleChain(in.Chain)
		if chain == "" {
			return Result{}, ErrInvalid
		}
		switch in.Operation {
		case "holders", "map", "metrics":
			if !validAddress(in.Address) {
				return Result{}, ErrInvalid
			}
			endpoint = "/v0/tokens/" + in.Operation + "/" + chain + "/" + url.PathEscape(in.Address)
			if in.Operation == "holders" {
				limit := in.Limit
				if limit == 0 {
					limit = 80
				}
				if limit < 1 || limit > 80 {
					return Result{}, ErrInvalid
				}
				endpoint += "?limit=" + strconv.Itoa(limit)
			}
			if in.Operation == "map" {
				endpoint += "?return_nodes=true&return_relationships=true&return_clusters=true"
			}
		case "subgraph":
			if !validAddress(in.Address) || len(in.Addresses) == 0 || len(in.Addresses) > 20 {
				return Result{}, ErrInvalid
			}
			for _, addr := range in.Addresses {
				if !validAddress(addr) {
					return Result{}, ErrInvalid
				}
			}
			endpoint, method = "/v0/relationships/subgraph", http.MethodPost
			data = map[string]any{
				"addresses":         in.Addresses,
				"context_token_key": map[string]string{"chain": chain, "address": in.Address},
			}
		default:
			return Result{}, ErrInvalid
		}
	default:
		return Result{}, ErrInvalid
	}
	if key == "" {
		return Result{}, fmt.Errorf("%w: %s credential not configured", ErrUnavailable, in.Provider)
	}
	if in.Provider == "goplus" {
		key = "Bearer " + key
	}
	payload, err := c.request(ctx, method, origin+endpoint, headerName, key, data)
	if err != nil {
		return Result{}, err
	}
	result.Data = payload
	return result, nil
}

func (c *Client) request(ctx context.Context, method, endpoint, headerName, key string, data any) (json.RawMessage, error) {
	var body io.Reader
	if data != nil {
		encoded, err := json.Marshal(data)
		if err != nil || len(encoded) > 32768 {
			return nil, ErrInvalid
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, ErrUnavailable
	}
	req.Header.Set(headerName, key)
	req.Header.Set("Accept", "application/json")
	if data != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 9 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: request failed", ErrUnavailable)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%w: HTTP %d", ErrUnavailable, resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxProviderResponse+1))
	if err != nil || len(raw) > maxProviderResponse || !json.Valid(raw) || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, fmt.Errorf("%w: invalid or excessive provider response", ErrUnavailable)
	}
	if strings.Contains(endpoint, "gopluslabs.io") {
		var status struct {
			Code json.Number `json:"code"`
		}
		if json.Unmarshal(raw, &status) == nil && status.Code != "" && status.Code != "1" {
			return nil, fmt.Errorf("%w: GoPlus status %s", ErrUnavailable, status.Code)
		}
	}
	return json.RawMessage(raw), nil
}
