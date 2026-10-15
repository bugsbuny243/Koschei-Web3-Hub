package handlers

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"

	"koschei/api/internal/services"
)

func TestOrcaMarketDEXDispatchesToVerifiedWhirlpoolCollector(t *testing.T) {
	const pyusd = "2b1kV6DkPAnxd5ixfnxCpjxmKwqjjaYmCZfHsFu24GXo"
	const pool = "9tXiuRRw7kbejLhZXtxDxYs2REe43uH2e7k1kocgdM9B"
	const pyusdVault = "EeF6oBy6AQiBJoRx5xiRNxa6cmpQE3ayVagj28QFZuyg"
	const usdcVault = "MvB8poDgpDPbRgx8MXeb7EPEsawGuiBTqpkpM9exeLi"

	calls := 0
	rpc := func(_ context.Context, _ string, method string, params any, out any) error {
		calls++
		switch method {
		case "getAccountInfo":
			got := out.(*rpcAccountInfoResponse)
			got.Context.Slot = 456948159
			got.Value = &struct {
				Owner string `json:"owner"`
				Data  any    `json:"data"`
			}{
				Owner: orcaWhirlpoolProgram,
				Data:  []any{orcaPYUSDUSDCWhirlpoolRPCFixture, "base64"},
			}
			return nil
		case "getTokenAccountBalance":
			got := out.(*rpcTokenBalanceResponse)
			got.Context.Slot = 456948160
			switch params.([]any)[0] {
			case pyusdVault:
				got.Value.UIAmountString = "8772025.407911"
			case usdcVault:
				got.Value.UIAmountString = "7831027.785407"
			default:
				return errors.New("unexpected token vault")
			}
			return nil
		default:
			return errors.New("position pool must not request fungible LP ownership evidence")
		}
	}
	market := services.TokenMarketSnapshot{
		Available:            true,
		BestPairAddress:      pool,
		BestPairDEX:          "orca",
		BestPairLiquidityUSD: 16600000,
	}
	result := collectPrimaryPoolLPControlEvidence(context.Background(), rpc, "solana-mainnet", pyusd, "", market, map[string]any{})
	if calls != 3 {
		t.Fatalf("expected pool and two vault RPC calls, got %d", calls)
	}
	if !result.Available || result.PoolProgram != orcaWhirlpoolProgram || result.PoolType != "orca_whirlpool" {
		t.Fatalf("Orca market pair did not reach verified pool decoder: %#v", result)
	}
	if result.ReasonCode != "ORCA_POSITION_OWNERSHIP_UNRESOLVED" || result.Status != services.LPControlUnverified {
		t.Fatalf("unobserved position owners cannot become verified controls: %#v", result)
	}
	if result.ControlModel != "position_nft" || result.LPMint != "" || result.TokenVault != pyusdVault || result.QuoteVault != usdcVault {
		t.Fatalf("Orca pool incorrectly treated as fungible LP: %#v", result)
	}
	if result.TokenReserve <= 0 || result.QuoteReserve <= 0 {
		t.Fatalf("real vault reserve fixture was not propagated: %#v", result)
	}
}

func TestOrcaMarketDEXRejectsUnknownPoolOwner(t *testing.T) {
	market := services.TokenMarketSnapshot{
		BestPairDEX:     "orca",
		BestPairAddress: "9tXiuRRw7kbejLhZXtxDxYs2REe43uH2e7k1kocgdM9B",
	}
	rpc := func(_ context.Context, _ string, method string, _ any, out any) error {
		if method != "getAccountInfo" {
			return errors.New("untrusted pool owner must not yield vault reads")
		}
		got := out.(*rpcAccountInfoResponse)
		got.Value = &struct {
			Owner string `json:"owner"`
			Data  any    `json:"data"`
		}{
			Owner: "UnrecognizedOwner1111111111111111111111111",
			Data:  []any{base64.StdEncoding.EncodeToString([]byte("not a trusted pool")), "base64"},
		}
		return nil
	}
	result := collectPrimaryPoolLPControlEvidence(context.Background(), rpc, "solana-mainnet", "mint", "", market, nil)
	if result.Available || result.ReasonCode != "unsupported_pool_program" {
		t.Fatalf("DEX market label must not imply verified Orca owner: %#v", result)
	}
}
