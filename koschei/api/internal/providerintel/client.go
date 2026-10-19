// Package providerintel exposes bounded, read-only third-party evidence.
// Provider results are OBSERVATIONS, never ARVIS signed canonical verdicts.
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

var safeIdentifier = regexp.MustCompile(`^[a-zA-Z0-9:_-]{2,128}$`)

var ErrUnavailable = errors.New("external intelligence provider unavailable")
var ErrInvalid = errors.New("invalid external intelligence request")

// Input deliberately has no endpoint/base_url field; a user cannot turn the
// customer API into an arbitrary-address proxy or SSRF gadget.
type Input struct {
	Provider string          `json:"provider"`
	Operation string         `json:"operation"`
	Chain string             `json:"chain,omitempty"`
	Address string           `json:"address,omitempty"`
	Addresses []string       `json:"addresses,omitempty"`
	URL string               `json:"url,omitempty"`
	Payload json.RawMessage  `json:"payload,omitempty"`
	Limit int                `json:"limit,omitempty"`
}

type Result struct {
	Provider string          `json:"provider"`
	Operation string         `json:"operation"`
	Source string            `json:"source"`
	Verification string      `json:"verification"`
	RiskLevel string         `json:"risk_level"`
	Signed bool              `json:"signed"`
	Data json.RawMessage     `json:"data"`
}

type Client struct {
	HTTP *http.Client
	BlockaidKey string
	GoPlusKey string
	BubblemapsKey string
	// These addresses are constants in production. Private package tests can
	// replace origins with local httptest servers without exposing override APIs.
	blockaidOrigin string
	goplusOrigin string
	bubblemapsOrigin string
}

func NewFromEnv() *Client {
	return &Client{
		HTTP: &http.Client{Timeout: 9*time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }},
		BlockaidKey: strings.TrimSpace(os.Getenv("KOSCHEI_BLOCKAID_API_KEY")),
		GoPlusKey: strings.TrimSpace(os.Getenv("KOSCHEI_GOPLUS_ACCESS_TOKEN")),
		BubblemapsKey: strings.TrimSpace(os.Getenv("KOSCHEI_BUBBLEMAPS_API_KEY")),
		blockaidOrigin: "https://api.blockaid.io",
		goplusOrigin: "https://api.gopluslabs.io",
		bubblemapsOrigin: "https://api.bubblemaps.io",
	}
}

func validAddress(address string) bool {
	return safeIdentifier.MatchString(strings.TrimSpace(address))
}

func validChain(chain string) bool {
	switch chain {
	case "solana", "sol", "ethereum", "eth", "base", "bsc", "polygon", "arbitrum", "optimism", "avalanche":
		return true
	}
	return false
}
func goPlusChainID(chain string) string {
	switch chain {
	case "ethereum", "eth": return "1"
	case "bsc": return "56"
	case "polygon": return "137"
	case "arbitrum": return "42161"
	case "base": return "8453"
	case "optimism": return "10"
	case "avalanche": return "43114"
	}
	return ""
}
func bubbleChain(chain string) string {
	switch chain {
	case "solana", "sol": return "sol"
	case "ethereum", "eth": return "eth"
	case "base": return "base"
	case "bsc": return "bsc"
	case "polygon": return "poly"
	}
	return ""
}

func (c *Client) Query(ctx context.Context, in Input) (Result,error) {
	in.Provider = strings.ToLower(strings.TrimSpace(in.Provider))
	in.Operation = strings.ToLower(strings.TrimSpace(in.Operation))
	in.Chain = strings.ToLower(strings.TrimSpace(in.Chain))
	in.Address = strings.TrimSpace(in.Address)
	if ctx == nil { return Result{}, fmt.Errorf("%w: context required", ErrInvalid) }
	if c == nil { return Result{}, ErrUnavailable }
	result := Result{Provider:in.Provider, Operation:in.Operation, Verification:"external_observation_only", RiskLevel:"unknown",Signed:false}
	if in.Provider == "slither" {
		if in.Operation != "findings" { return Result{}, ErrInvalid }
		findings,err:=ParseSlitherFindings(in.Payload)
		if err != nil {return Result{},err}
		payload,err:=json.Marshal(findings)
		if err != nil{return Result{},err}
		result.Source="trail_of_bits_slither_json"
		result.Data=payload
		return result,nil
	}
	if !validChain(in.Chain) {return Result{},fmt.Errorf("%w: chain is unsupported",ErrInvalid)}
	var endpoint, key, kind string
	var data any
	method:=http.MethodGet
	switch in.Provider {
	case "blockaid":
		key=c.BlockaidKey;kind="X-API-Key";result.Source="blockaid_api"
		switch in.Operation {
		case "token", "address":
			if !validAddress(in.Address){return Result{},ErrInvalid}
			method=http.MethodPost
			endpoint="/v1/"+in.Operation+"/scan"
			data=map[string]any{"chain":in.Chain,"address":in.Address}
		case "transaction", "transaction_bulk":
			if goPlusChainID(in.Chain)==""{return Result{},fmt.Errorf("%w: EVM chain required",ErrInvalid)}
			if len(in.Payload)==0||len(in.Payload)>32768||!json.Valid(in.Payload){return Result{},ErrInvalid}
			var request map[string]json.RawMessage
			if err:=json.Unmarshal(in.Payload,&request);err!=nil{return Result{},ErrInvalid}
			if len(request["data"])==0||len(request["chain"])==0 {return Result{},fmt.Errorf("%w: transaction requires data and chain",ErrInvalid)}
			if in.Operation=="transaction"&&len(request["account_address"])==0{return Result{},fmt.Errorf("%w: transaction requires account_address",ErrInvalid)}
			if in.Operation=="transaction_bulk"{
				var items []json.RawMessage
				if err:=json.Unmarshal(request["data"],&items);err!=nil||len(items)==0||len(items)>20{return Result{},fmt.Errorf("%w: bulk size must be 1 to 20",ErrInvalid)}
			}
			method=http.MethodPost
			if in.Operation=="transaction_bulk"{endpoint="/v0/evm/transaction-bulk/scan"}else{endpoint="/v0/evm/transaction/scan"}
			data=request
		default: return Result{},ErrInvalid
		}
	case "goplus":
		key=c.GoPlusKey;kind="Authorization";result.Source="goplus_security_api"
		switch in.Operation {
		case "token":
			if !validAddress(in.Address){return Result{},ErrInvalid}
			if in.Chain=="solana"||in.Chain=="sol"{
				endpoint="/api/v1/solana/token_security?contract_addresses="+url.QueryEscape(in.Address)
			}else{
				id:=goPlusChainID(in.Chain);if id==""{return Result{},ErrInvalid}
				endpoint="/api/v1/token_security/"+id+"?contract_addresses="+url.QueryEscape(in.Address)
			}
		case "address":
			if !validAddress(in.Address){return Result{},ErrInvalid}
			id:=goPlusChainID(in.Chain);if id==""{return Result{},ErrInvalid}
			endpoint="/api/v1/address_security/"+url.PathEscape(in.Address)+"?chain_id="+id
		case "approval":
			if !validAddress(in.Address){return Result{},ErrInvalid}
			id:=goPlusChainID(in.Chain);if id==""{return Result{},ErrInvalid}
			endpoint="/api/v1/approval_security/"+id+"?contract_addresses="+url.QueryEscape(in.Address)
		case "phishing":
			if len(in.URL)<10||len(in.URL)>2048{return Result{},ErrInvalid}
			u,err:=url.Parse(in.URL)
			if err!=nil||(u.Scheme!="https"&&u.Scheme!="http")||u.Hostname()==""||u.User!=nil{return Result{},ErrInvalid}
			endpoint="/api/v1/phishing_site?url="+url.QueryEscape(in.URL)
		default:return Result{},ErrInvalid
		}
	case "bubblemaps":
		key=c.BubblemapsKey;kind="X-API-Key";result.Source="bubblemaps_data_api"
		chain:=bubbleChain(in.Chain)
		if chain==""{return Result{},ErrInvalid}
		switch in.Operation {
		case "holders","map","metrics":
			if !validAddress(in.Address){return Result{},ErrInvalid}
			endpoint="/v0/tokens/"+in.Operation+"/"+chain+"/"+url.PathEscape(in.Address)
			if in.Operation=="holders"{
				limit:=in.Limit;if limit==0{limit=80}
				if limit<1||limit>80{return Result{},ErrInvalid}
				endpoint+="?limit="+strconv.Itoa(limit)
			}
			if in.Operation=="map"{endpoint+="?return_nodes=true&return_relationships=true&return_clusters=true"}
		case "subgraph":
			if len(in.Addresses)==0||len(in.Addresses)>20||!validAddress(in.Address){return Result{},ErrInvalid}
			for _,a:=range in.Addresses{if !validAddress(a){return Result{},ErrInvalid}}
			method=http.MethodPost
			endpoint="/v0/relationships/subgraph"
			data=map[string]any{"addresses":in.Addresses,"context_token_key":map[string]string{"chain":chain,"address":in.Address}}
		default:return Result{},ErrInvalid
		}
	default:return Result{},fmt.Errorf("%w: provider is unsupported",ErrInvalid)
	}
	if key=="" {return Result{},fmt.Errorf("%w: %s credential not configured",ErrUnavailable,in.Provider)}
	origin:=""
	switch in.Provider {
	case "blockaid":origin=c.blockaidOrigin
	case "goplus":origin=c.goplusOrigin
	case "bubblemaps":origin=c.bubblemapsOrigin
	}
	payload,err:=c.request(ctx,method,origin+endpoint,kind,key,data)
	if err!=nil{return Result{},err}
	result.Data=payload
	return result,nil
}

func (c *Client) request(ctx context.Context,method,endpoint,kind,key string,data any)(json.RawMessage,error){
	var body io.Reader
	if data!=nil {
		buf,err:=json.Marshal(data)
		if err!=nil{return nil,err}
		if len(buf)>32768{return nil,ErrInvalid}
		body=bytes.NewReader(buf)
	}
	req,err:=http.NewRequestWithContext(ctx,method,endpoint,body)
	if err!=nil{return nil,fmt.Errorf("%w: build request",ErrUnavailable)}
	req.Header.Set(kind,key)
	if data!=nil{req.Header.Set("Content-Type","application/json")}
	req.Header.Set("Accept","application/json")
	client:=c.HTTP
	if client==nil{client=&http.Client{Timeout:9*time.Second}}
	resp,err:=client.Do(req)
	if err!=nil{return nil,fmt.Errorf("%w: request failed",ErrUnavailable)}
	defer resp.Body.Close()
	if resp.StatusCode<200||resp.StatusCode>=300{
		return nil,fmt.Errorf("%w: provider returned HTTP %d",ErrUnavailable,resp.StatusCode)
	}
	raw,err:=io.ReadAll(io.LimitReader(resp.Body,maxProviderResponse+1))
	if err!=nil||len(raw)>maxProviderResponse{return nil,fmt.Errorf("%w: oversized or unreadable provider response",ErrUnavailable)}
	if !json.Valid(raw){return nil,fmt.Errorf("%w: provider returned invalid JSON",ErrUnavailable)}
	if len(raw)==0||bytes.Equal(bytes.TrimSpace(raw),[]byte("null")){return nil,fmt.Errorf("%w: provider response was empty",ErrUnavailable)}
	if strings.Contains(endpoint,"gopluslabs.io"){
		var status struct {Code json.Number `json:"code"`}
		if json.Unmarshal(raw,&status)==nil && status.Code!=""&&status.Code!="1" {
			return nil,fmt.Errorf("%w: GoPlus status %s",ErrUnavailable,status.Code)
		}
	}
	return json.RawMessage(raw),nil
}
