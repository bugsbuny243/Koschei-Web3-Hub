package web3

import (
	"strconv"
	"time"
)

type TokenSupplyRPC struct {
	Value struct {
		Amount   string `json:"amount"`
		Decimals int    `json:"decimals"`
	} `json:"value"`
}

type TokenAccountInfoRPC struct {
	Value *struct {
		Data struct {
			Parsed struct {
				Info struct {
					MintAuthority   *string `json:"mintAuthority"`
					FreezeAuthority *string `json:"freezeAuthority"`
				} `json:"info"`
			} `json:"parsed"`
		} `json:"data"`
	} `json:"value"`
}

type TokenLargestAccountsRPC struct {
	Value []struct {
		Amount string `json:"amount"`
	} `json:"value"`
}

func NormalizeTokenData(network, mint, provider string, supply TokenSupplyRPC, account TokenAccountInfoRPC, largest TokenLargestAccountsRPC) NormalizedTokenData {
	total, _ := strconv.ParseFloat(supply.Value.Amount, 64)
	topOne, topTen := 0.0, 0.0
	for i, holder := range largest.Value {
		amount, _ := strconv.ParseFloat(holder.Amount, 64)
		if total > 0 && i < 10 {
			topTen += amount / total * 100
			if i == 0 {
				topOne = amount / total * 100
			}
		}
	}
	out := NormalizedTokenData{Mint: mint, Network: network, SupplyRaw: supply.Value.Amount, Decimals: supply.Value.Decimals, LargestHolderPercent: round(topOne), TopTenPercent: round(topTen), SourceProvider: provider, FetchedAt: time.Now().UTC()}
	if account.Value != nil {
		out.MintAuthority = account.Value.Data.Parsed.Info.MintAuthority
		out.FreezeAuthority = account.Value.Data.Parsed.Info.FreezeAuthority
	}
	return out
}

func ScoreTokenRisk(token NormalizedTokenData) TokenRiskResult {
	score := 100
	findings := []string{}
	if token.MintAuthority != nil {
		score -= 25
		findings = append(findings, "Mint authority aktif; ek arz oluşturulabilir.")
	} else {
		findings = append(findings, "Mint authority devre dışı.")
	}
	if token.FreezeAuthority != nil {
		score -= 20
		findings = append(findings, "Freeze authority aktif; token hesapları dondurulabilir.")
	} else {
		findings = append(findings, "Freeze authority devre dışı.")
	}
	if token.LargestHolderPercent >= 50 {
		score -= 35
		findings = append(findings, "En büyük holder arzın en az yarısını kontrol ediyor.")
	} else if token.LargestHolderPercent >= 20 {
		score -= 20
		findings = append(findings, "En büyük holder tarafında anlamlı yoğunlaşma var.")
	}
	if token.TopTenPercent >= 80 {
		score -= 20
		findings = append(findings, "İlk on holder arzın büyük bölümünü kontrol ediyor.")
	}
	if score < 0 {
		score = 0
	}
	risk := "low"
	if score < 40 {
		risk = "high"
	} else if score < 70 {
		risk = "medium"
	}
	return TokenRiskResult{Token: token, Score: score, RiskLevel: risk, Findings: findings, Disclaimer: "Ön zincir üstü sinyaller; güvenlik denetimi veya finansal tavsiye değildir."}
}

func round(value float64) float64 { return float64(int(value*100+0.5)) / 100 }
