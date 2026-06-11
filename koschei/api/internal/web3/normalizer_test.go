package web3

import "testing"

func TestNormalizer_TokenSupplyAndAuthorities(t *testing.T) {
	var supply TokenSupplyRPC
	supply.Value.Amount = "1000"
	supply.Value.Decimals = 6
	mintAuthority := "mint-auth"
	account := TokenAccountInfoRPC{Value: &struct {
		Data struct {
			Parsed struct {
				Info struct {
					MintAuthority   *string `json:"mintAuthority"`
					FreezeAuthority *string `json:"freezeAuthority"`
				} `json:"info"`
			} `json:"parsed"`
		} `json:"data"`
	}{}}
	account.Value.Data.Parsed.Info.MintAuthority = &mintAuthority
	var largest TokenLargestAccountsRPC
	largest.Value = append(largest.Value, struct {
		Amount string `json:"amount"`
	}{"600"}, struct {
		Amount string `json:"amount"`
	}{"200"})
	out := NormalizeTokenData("solana-mainnet", "mint", "mock", supply, account, largest)
	if out.Decimals != 6 || out.MintAuthority == nil || out.LargestHolderPercent != 60 || out.TopTenPercent != 80 {
		t.Fatalf("unexpected normalized data: %+v", out)
	}
}
