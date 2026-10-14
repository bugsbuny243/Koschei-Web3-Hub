package handlers

import (
	"context"
	"encoding/base64"
	"errors"
	"math"
	"testing"

	"koschei/api/internal/services"
)

// Snapshot returned by Solana getAccountInfo (finalized slot 456946064)
// for Orca PYUSD/USDC Whirlpool 9tXiuRRw7kbejLhZXtxDxYs2REe43uH2e7k1kocgdM9B.
// Owner: whirLbMiicVdio4qvUfM5KAg6Ct8VwpYzGff3uctyCc, space: 653.
// Vault readings were separately checked with getTokenAccountBalance.
const orcaPYUSDUSDCWhirlpoolRPCFixture = "P5XRDOGAYwkT5EH4ORPKaLBjT7Al/eqohzfoQRDRJV41ezN33e4czf8BAAEAZAAUBevzaBKILS8AAAAAAAAAAAC3zWJEt5QCAAEAAAAAAAAAAAAAABc+AgAAAAAAJHwUAAAAAAAXkkg7bIoqh7dHHYFPlZH5OVyECpzj2fTVun06S4p0nsqyXOJQokFo/NIz4KGnJohGd81f6eq6l6BjjIwLUZ+nQ+t6uoBBAgAAAAAAAAAAAMb6evO+2606PWXzaqvJdDGxu+TC0vbg5HymAgNFL11hBVu9tYk+VhpyfHYC4VK5YZdlNda5HFvU4kdV+KcipRMZe+AruzYCAAAAAAAAAAAAV9LPagAAAAAXkkg7bIoqh7dHHYFPlZH5OVyECpzj2fTVun06S4p0nv2c/kXycevMpaE75U8vr7oK6kIb5ZvXWnjtDHAxc1dvui6/LwKaI7GKR1R798LZ7CubYjLuw+NoR9dh+omDPGYAAAAAAAAAAAAAAAAAAAAA6LkkWr3HAAAAAAAAAAAAAAwA0K/rhhTafxmroC1A8YxpJYX2UCDfztPV5fmpwMTh+l31H+YC5Jblq6uF7Nol3Z5xgz0h5vC17r5W6ZKVBxcAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABdQP9ftBYAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="

func TestOrcaWhirlpoolPYUSDRealRPCFixture(t *testing.T) {
	const pool = "9tXiuRRw7kbejLhZXtxDxYs2REe43uH2e7k1kocgdM9B"
	const pyusd = "2b1kV6DkPAnxd5ixfnxCpjxmKwqjjaYmCZfHsFu24GXo"
	const usdc = "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"
	const pyusdVault = "EeF6oBy6AQiBJoRx5xiRNxa6cmpQE3ayVagj28QFZuyg"
	const usdcVault = "MvB8poDgpDPbRgx8MXeb7EPEsawGuiBTqpkpM9exeLi"
	balances := map[string]string{pyusdVault: "8772025.407911", usdcVault: "7831027.785407"}
	for _, tc := range []struct {
		mint, tokenVault, quoteVault, quoteMint string
	}{
		{pyusd, pyusdVault, usdcVault, usdc},
		{usdc, usdcVault, pyusdVault, pyusd},
	} {
		t.Run(tc.mint, func(t *testing.T) {
			reads := 0
			rpc := func(_ context.Context, _ string, method string, params any, out any) error {
				switch method {
				case "getAccountInfo":
					reads++
					response := out.(*rpcAccountInfoResponse)
					response.Context.Slot = 456946064
					response.Value = &struct {
						Owner string `json:"owner"`
						Data  any    `json:"data"`
					}{Owner: orcaWhirlpoolProgram, Data: []any{orcaPYUSDUSDCWhirlpoolRPCFixture, "base64"}}
				case "getTokenAccountBalance":
					reads++
					address := params.([]any)[0].(string)
					balance, ok := balances[address]
					if !ok {
						return errors.New("unexpected vault " + address)
					}
					response := out.(*rpcTokenBalanceResponse)
					response.Context.Slot = 456946093
					response.Value.UIAmountString = balance
					response.Value.Decimals = 6
				default:
					return errors.New("Orca position pool must not request fungible LP supply or owner estimates: " + method)
				}
				return nil
			}
			got := collectProtocolLPControlEvidence(context.Background(), rpc, "solana-mainnet", tc.mint, "", pool)
			if !got.Available || got.PoolProgram != orcaWhirlpoolProgram || got.PoolType != "orca_whirlpool" || got.ControlModel != "position_nft" {
				t.Fatalf("Whirlpool owner/model evidence=%#v", got)
			}
			if got.TokenVault != tc.tokenVault || got.QuoteVault != tc.quoteVault || got.QuoteMint != tc.quoteMint {
				t.Fatalf("decoded onchain mints/vaults=%#v", got)
			}
			if got.Status != services.LPControlUnverified || got.ReasonCode != "ORCA_POSITION_OWNERSHIP_UNRESOLVED" || got.LPMint != "" || got.BurnedSharePct != 0 {
				t.Fatalf("Orca position owner must remain unresolved: %#v", got)
			}
			if got.PoolLiquidityRaw == "" || got.TokenReserve == 0 || got.QuoteReserve == 0 || reads != 3 {
				t.Fatalf("reserve/liquidity evidence incomplete: %#v reads=%d", got, reads)
			}
			if math.Abs(got.TokenReserve-8772025.407911) < 0.001 && tc.mint != pyusd {
				t.Fatalf("incorrect reserve side: %#v", got)
			}
		})
	}
}

func TestOrcaWhirlpoolRejectsCorruptDiscriminatorAndUnsupportedOwner(t *testing.T) {
	raw, err := base64.StdEncoding.DecodeString(orcaPYUSDUSDCWhirlpoolRPCFixture)
	if err != nil || len(raw) != orcaWhirlpoolAccountLen {
		t.Fatalf("fixture corrupted: length=%d err=%v", len(raw), err)
	}
	raw[0] ^= 0xff
	invalid := decodeOrcaWhirlpoolLPControl(context.Background(), nil, "solana-mainnet", services.LPControlEvidence{TokenMint: "target"}, raw)
	if invalid.ReasonCode != "orca_whirlpool_account_invalid" || invalid.Available {
		t.Fatalf("invalid discriminator accepted: %#v", invalid)
	}
	rpc := func(_ context.Context, _ string, method string, _ any, out any) error {
		if method != "getAccountInfo" {
			return errors.New("unsupported program must not cause reserve calls")
		}
		response := out.(*rpcAccountInfoResponse)
		response.Context.Slot = 456946064
		response.Value = &struct {
			Owner string `json:"owner"`
			Data  any    `json:"data"`
		}{Owner: "UnsupportedProgram11111111111111111111111111", Data: []any{orcaPYUSDUSDCWhirlpoolRPCFixture, "base64"}}
		return nil
	}
	unknown := collectProtocolLPControlEvidence(context.Background(), rpc, "solana-mainnet", "target", "", "pool")
	if unknown.ReasonCode != "unsupported_pool_program" || unknown.Available {
		t.Fatalf("unsupported owner was upgraded to Orca: %#v", unknown)
	}
}
