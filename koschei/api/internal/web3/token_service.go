package web3

import (
	"context"
	"encoding/json"
	"fmt"
)

type TokenService struct {
	RPC   RPCCaller
	Cache *SmartCache
}

func NewTokenService(rpc RPCCaller, cache *SmartCache) *TokenService {
	return &TokenService{RPC: rpc, Cache: cache}
}

func (s *TokenService) ScanToken(ctx context.Context, network, mint string) (TokenRiskResult, error) {
	key := fmt.Sprintf("token:rugcheck:%s:%s", network, mint)
	load := func(ctx context.Context) ([]byte, error) {
		result, err := s.loadToken(ctx, network, mint)
		if err != nil {
			return nil, err
		}
		return json.Marshal(result)
	}
	b, err := s.Cache.GetOrLoad(ctx, key, RugCheckTTL, load)
	if err != nil {
		return TokenRiskResult{}, err
	}
	var result TokenRiskResult
	if err := json.Unmarshal(b, &result); err != nil {
		return TokenRiskResult{}, err
	}
	return result, nil
}

func (s *TokenService) loadToken(ctx context.Context, network, mint string) (TokenRiskResult, error) {
	var supply TokenSupplyRPC
	provider, err := s.RPC.Call(ctx, "getTokenSupply", []any{mint}, &supply)
	if err != nil {
		return TokenRiskResult{}, err
	}
	var account TokenAccountInfoRPC
	if _, err := s.RPC.Call(ctx, "getAccountInfo", []any{mint, map[string]string{"encoding": "jsonParsed"}}, &account); err != nil {
		return TokenRiskResult{}, err
	}
	var largest TokenLargestAccountsRPC
	if _, err := s.RPC.Call(ctx, "getTokenLargestAccounts", []any{mint}, &largest); err != nil {
		return TokenRiskResult{}, err
	}
	token := NormalizeTokenData(network, mint, provider, supply, account, largest)
	return ScoreTokenRisk(token), nil
}
