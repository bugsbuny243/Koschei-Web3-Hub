package tron

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// TronGrid evidence collection for consumers that need normalized Tron
// observations (watchlist monitoring, the Tron Flow Radar). Provider requests,
// protocol schemas and decoding stay in this adapter package; consumers only
// see Transfer, BlockTransfer and freeze/balance observations.

// USDTContract is Tether USD (TRC-20) on Tron mainnet, validated at runtime by
// Base58Check through Address.
const USDTContract = "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"

// USDTDecimals is the decimals value of the Tether USD TRC-20 contract.
const USDTDecimals = 6

const (
	FreezeFrozen  = "frozen"
	FreezeClear   = "not_frozen"
	FreezeUnknown = "unknown"

	transferTopic    = "ddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"
	accountPageLimit = 50
	maxSmallResponse = 2 << 20
	maxBlockResponse = 16 << 20
)

// Transfer is one confirmed outbound transfer from an account. EventID is
// unique per transfer event: a transaction can emit several TRC-20 transfers.
type Transfer struct {
	EventID          string `json:"event_id"`
	TransactionID    string `json:"transaction_id"`
	Asset            string `json:"asset"` // "trc20" or "trx"
	TokenContract    string `json:"token_contract,omitempty"`
	TokenSymbol      string `json:"token_symbol,omitempty"`
	From             string `json:"from"`
	To               string `json:"to"`
	ValueRaw         string `json:"value_raw"`
	Decimals         *int   `json:"decimals,omitempty"` // indexer-reported for TRC-20, protocol 6 for TRX
	BlockTimestampMS int64  `json:"block_timestamp_ms"`
}

// BlockTransfer is one successful USDT Transfer event in a solidified block.
type BlockTransfer struct {
	TransactionID    string
	EventIndex       int
	BlockNumber      uint64
	BlockTimestampMS int64
	From             string // 41-hex
	To               string // 41-hex
	Value            *big.Int
}

// EventID identifies the event within its transaction.
func (t BlockTransfer) EventID() string { return fmt.Sprintf("%s:%d", t.TransactionID, t.EventIndex) }

// GridClient reads TronGrid. The API key is sent only as the TRON-PRO-API-KEY
// header; errors carry categories only, never the URL, key or upstream body.
type GridClient struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client
}

func (c GridClient) client() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

func (c GridClient) do(ctx context.Context, method, path string, body any, limit int64) ([]byte, error) {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, errors.New("tron_request_encoding_failed")
		}
		reader = bytes.NewReader(encoded)
	}
	base := strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	if base == "" {
		base = "https://api.trongrid.io"
	}
	req, err := http.NewRequestWithContext(ctx, method, base+path, reader)
	if err != nil {
		return nil, errors.New("tron_request_invalid")
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if key := strings.TrimSpace(c.APIKey); key != "" {
		req.Header.Set("TRON-PRO-API-KEY", key)
	}
	res, err := c.client().Do(req)
	if err != nil {
		return nil, errors.New("tron_provider_unavailable")
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tron_provider_http_%d", res.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(res.Body, limit+1))
	if err != nil || int64(len(raw)) > limit {
		return nil, errors.New("tron_provider_response_too_large")
	}
	return raw, nil
}

// OutboundTRC20 returns up to 50 newest confirmed outbound TRC-20 transfers.
func (c GridClient) OutboundTRC20(ctx context.Context, base58, hexAddress string) ([]Transfer, error) {
	raw, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/v1/accounts/%s/transactions/trc20?only_confirmed=true&only_from=true&limit=%d", url.PathEscape(base58), accountPageLimit), nil, maxSmallResponse)
	if err != nil {
		return nil, err
	}
	return ParseOutboundTRC20(raw, hexAddress)
}

// OutboundTRX returns up to 50 newest confirmed, successful native TRX transfers.
func (c GridClient) OutboundTRX(ctx context.Context, base58, hexAddress string) ([]Transfer, error) {
	raw, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/v1/accounts/%s/transactions?only_confirmed=true&only_from=true&limit=%d", url.PathEscape(base58), accountPageLimit), nil, maxSmallResponse)
	if err != nil {
		return nil, err
	}
	return ParseOutboundTRX(raw, hexAddress)
}

func (c GridClient) constantCall(ctx context.Context, hexAddress, selector string) ([]byte, error) {
	contract, err := Address(USDTContract)
	if err != nil || len(hexAddress) != 42 {
		return nil, errors.New("tron_constant_call_invalid")
	}
	return c.do(ctx, http.MethodPost, "/wallet/triggerconstantcontract", map[string]any{
		"owner_address": hexAddress, "contract_address": contract, "function_selector": selector,
		"parameter": strings.Repeat("0", 24) + hexAddress[2:], "visible": false,
	}, maxSmallResponse)
}

// USDTFreezeState reads isBlackListed(address); any failure is unknown.
func (c GridClient) USDTFreezeState(ctx context.Context, hexAddress string) string {
	raw, err := c.constantCall(ctx, hexAddress, "isBlackListed(address)")
	if err != nil {
		return FreezeUnknown
	}
	return ParseUSDTFreezeWord(raw)
}

// USDTBalance reads balanceOf(address) in raw units at the node's latest state.
func (c GridClient) USDTBalance(ctx context.Context, hexAddress string) (*big.Int, error) {
	raw, err := c.constantCall(ctx, hexAddress, "balanceOf(address)")
	if err != nil {
		return nil, err
	}
	return ParseUSDTBalanceWord(raw)
}

// SolidHead returns the latest solidified (irreversible) block number.
func (c GridClient) SolidHead(ctx context.Context) (uint64, error) {
	raw, err := c.do(ctx, http.MethodPost, "/walletsolidity/getnowblock", map[string]any{}, maxSmallResponse)
	if err != nil {
		return 0, err
	}
	var block struct {
		Header struct {
			Raw struct {
				Number uint64 `json:"number"`
			} `json:"raw_data"`
		} `json:"block_header"`
	}
	if json.Unmarshal(raw, &block) != nil || block.Header.Raw.Number == 0 {
		return 0, errors.New("tron_head_invalid")
	}
	return block.Header.Raw.Number, nil
}

// SolidBlockUSDTTransfers returns the USDT Transfer events of a solidified block.
func (c GridClient) SolidBlockUSDTTransfers(ctx context.Context, number uint64) ([]BlockTransfer, error) {
	raw, err := c.do(ctx, http.MethodPost, "/walletsolidity/gettransactioninfobyblocknum", map[string]any{"num": number}, maxBlockResponse)
	if err != nil {
		return nil, err
	}
	return ParseBlockUSDTTransfers(raw)
}

func validValueRaw(value string) bool { return len(value) <= 78 && unsignedDecimal.MatchString(value) }

// ParseOutboundTRC20 keeps valid confirmed Transfer events sent by hexAddress.
func ParseOutboundTRC20(raw []byte, hexAddress string) ([]Transfer, error) {
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
				Symbol   string `json:"symbol"`
				Decimals *int   `json:"decimals"`
			} `json:"token_info"`
		} `json:"data"`
	}
	if json.Unmarshal(raw, &payload) != nil || !payload.Success {
		return nil, errors.New("tron_trc20_response_invalid")
	}
	out := []Transfer{}
	for i, item := range payload.Data {
		if i >= accountPageLimit {
			break
		}
		from, fe := Address(item.From)
		to, te := Address(item.To)
		token, ce := Address(item.Token.Address)
		id, ie := hex.DecodeString(item.ID)
		if fe != nil || te != nil || ce != nil || ie != nil || len(id) != 32 || from != hexAddress || item.Type != "Transfer" ||
			item.Timestamp <= 0 || !validValueRaw(item.Value) {
			continue
		}
		txID := strings.ToLower(item.ID)
		transfer := Transfer{
			EventID:       strings.Join([]string{txID, token, to, item.Value}, ":"),
			TransactionID: txID, Asset: "trc20", TokenContract: token, From: from, To: to,
			ValueRaw: item.Value, BlockTimestampMS: item.Timestamp,
		}
		if symbol := strings.TrimSpace(item.Token.Symbol); symbol != "" && len(symbol) <= 16 {
			transfer.TokenSymbol = symbol
		}
		if item.Token.Decimals != nil && *item.Token.Decimals >= 0 && *item.Token.Decimals <= 36 {
			decimals := *item.Token.Decimals
			transfer.Decimals = &decimals
		}
		out = append(out, transfer)
	}
	return out, nil
}

// ParseOutboundTRX keeps successful single-contract TransferContract
// transactions sent by hexAddress.
func ParseOutboundTRX(raw []byte, hexAddress string) ([]Transfer, error) {
	var payload struct {
		Success bool `json:"success"`
		Data    []struct {
			ID        string `json:"txID"`
			Timestamp int64  `json:"block_timestamp"`
			Ret       []struct {
				ContractRet string `json:"contractRet"`
			} `json:"ret"`
			RawData struct {
				Contract []struct {
					Type      string `json:"type"`
					Parameter struct {
						Value struct {
							Amount       json.Number `json:"amount"`
							OwnerAddress string      `json:"owner_address"`
							ToAddress    string      `json:"to_address"`
						} `json:"value"`
					} `json:"parameter"`
				} `json:"contract"`
			} `json:"raw_data"`
		} `json:"data"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if decoder.Decode(&payload) != nil || !payload.Success {
		return nil, errors.New("tron_trx_response_invalid")
	}
	out := []Transfer{}
	for i, item := range payload.Data {
		if i >= accountPageLimit {
			break
		}
		if len(item.RawData.Contract) != 1 || item.RawData.Contract[0].Type != "TransferContract" || len(item.Ret) == 0 || item.Ret[0].ContractRet != "SUCCESS" {
			continue
		}
		value := item.RawData.Contract[0].Parameter.Value
		from, fe := Address(value.OwnerAddress)
		to, te := Address(value.ToAddress)
		id, ie := hex.DecodeString(item.ID)
		amount := value.Amount.String()
		if fe != nil || te != nil || ie != nil || len(id) != 32 || from != hexAddress || item.Timestamp <= 0 || !validValueRaw(amount) {
			continue
		}
		decimals := 6
		txID := strings.ToLower(item.ID)
		out = append(out, Transfer{
			EventID: txID, TransactionID: txID, Asset: "trx", TokenSymbol: "TRX", From: from, To: to,
			ValueRaw: amount, Decimals: &decimals, BlockTimestampMS: item.Timestamp,
		})
	}
	return out, nil
}

func parseConstantWord(raw []byte) (string, bool) {
	var payload struct {
		Result struct {
			Result bool `json:"result"`
		} `json:"result"`
		ConstantResult []string `json:"constant_result"`
	}
	if json.Unmarshal(raw, &payload) != nil || !payload.Result.Result || len(payload.ConstantResult) != 1 {
		return "", false
	}
	word := strings.ToLower(payload.ConstantResult[0])
	if len(word) != 64 {
		return "", false
	}
	if _, err := hex.DecodeString(word); err != nil {
		return "", false
	}
	return word, true
}

// ParseUSDTFreezeWord maps an isBlackListed(address) response to frozen,
// not_frozen or unknown. It never guesses not_frozen.
func ParseUSDTFreezeWord(raw []byte) string {
	word, ok := parseConstantWord(raw)
	switch {
	case !ok:
		return FreezeUnknown
	case word == strings.Repeat("0", 64):
		return FreezeClear
	case word == strings.Repeat("0", 63)+"1":
		return FreezeFrozen
	default:
		return FreezeUnknown
	}
}

// ParseUSDTBalanceWord decodes a balanceOf(address) response.
func ParseUSDTBalanceWord(raw []byte) (*big.Int, error) {
	word, ok := parseConstantWord(raw)
	if !ok {
		return nil, errors.New("tron_balance_response_invalid")
	}
	value, _ := new(big.Int).SetString(word, 16)
	return value, nil
}

// ParseBlockUSDTTransfers extracts successful USDT Transfer events from a
// gettransactioninfobyblocknum response. Malformed entries are skipped.
func ParseBlockUSDTTransfers(raw []byte) ([]BlockTransfer, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || string(trimmed) == "{}" {
		return []BlockTransfer{}, nil // empty block
	}
	var infos []struct {
		ID          string `json:"id"`
		BlockNumber uint64 `json:"blockNumber"`
		Timestamp   int64  `json:"blockTimeStamp"`
		Result      string `json:"result"`
		Receipt     struct {
			Result string `json:"result"`
		} `json:"receipt"`
		Log []struct {
			Address string   `json:"address"`
			Topics  []string `json:"topics"`
			Data    string   `json:"data"`
		} `json:"log"`
	}
	if err := json.Unmarshal(trimmed, &infos); err != nil {
		return nil, errors.New("tron_block_info_invalid")
	}
	usdt, _ := Address(USDTContract)
	out := []BlockTransfer{}
	for _, info := range infos {
		if strings.EqualFold(info.Result, "FAILED") || (info.Receipt.Result != "" && info.Receipt.Result != "SUCCESS") {
			continue
		}
		id, err := hex.DecodeString(info.ID)
		if err != nil || len(id) != 32 {
			continue
		}
		for index, entry := range info.Log {
			if len(entry.Topics) != 3 || strings.ToLower(entry.Topics[0]) != transferTopic || "41"+strings.ToLower(entry.Address) != usdt {
				continue
			}
			from, okFrom := topicAddress(entry.Topics[1])
			to, okTo := topicAddress(entry.Topics[2])
			data, err := hex.DecodeString(entry.Data)
			if !okFrom || !okTo || err != nil || len(data) != 32 {
				continue
			}
			out = append(out, BlockTransfer{
				TransactionID: strings.ToLower(info.ID), EventIndex: index, BlockNumber: info.BlockNumber, BlockTimestampMS: info.Timestamp,
				From: from, To: to, Value: new(big.Int).SetBytes(data),
			})
		}
	}
	return out, nil
}

func topicAddress(topic string) (string, bool) {
	topic = strings.ToLower(topic)
	if len(topic) != 64 || topic[:24] != strings.Repeat("0", 24) {
		return "", false
	}
	if _, err := hex.DecodeString(topic[24:]); err != nil {
		return "", false
	}
	return "41" + topic[24:], true
}

// Base58 renders a 41-prefixed hex address in wallet Base58Check form, or ""
// when the input is not a valid Tron address.
func Base58(hexAddress string) string {
	payload, err := hex.DecodeString(hexAddress)
	if err != nil || len(payload) != 21 || payload[0] != 0x41 {
		return ""
	}
	first := sha256.Sum256(payload)
	second := sha256.Sum256(first[:])
	full := append(append([]byte{}, payload...), second[:4]...)
	n := new(big.Int).SetBytes(full)
	mod := new(big.Int)
	out := []byte{}
	for n.Sign() > 0 {
		n.DivMod(n, big.NewInt(58), mod)
		out = append([]byte{alphabet[mod.Int64()]}, out...)
	}
	encoded := string(out)
	if roundTrip, err := Address(encoded); err != nil || roundTrip != strings.ToLower(hexAddress) {
		return ""
	}
	return encoded
}

// FormatUnits renders an integer amount with decimals without rounding.
func FormatUnits(value *big.Int, decimals int) string {
	if value == nil {
		return ""
	}
	if decimals <= 0 {
		return value.String()
	}
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
	whole, fraction := new(big.Int).QuoRem(value, scale, new(big.Int))
	frac := strings.TrimRight(fmt.Sprintf("%0*s", decimals, fraction.String()), "0")
	if frac == "" {
		return whole.String()
	}
	return whole.String() + "." + frac
}
