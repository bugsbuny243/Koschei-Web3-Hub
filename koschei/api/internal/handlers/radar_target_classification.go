package handlers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"koschei/api/internal/services"
)

const (
	radarTargetTokenMint            = "token_mint"
	radarTargetTokenAccount         = "token_account"
	radarTargetWallet               = "wallet"
	radarTargetProgram              = "program"
	radarTargetTransactionSignature = "transaction_signature"
	radarTargetUnknown              = "unknown"
)

type radarTargetClassification struct {
	Type             string `json:"type"`
	Status           string `json:"status"`
	AccountOwner     string `json:"account_owner,omitempty"`
	TokenOwnerWallet string `json:"token_owner_wallet,omitempty"`
	ParsedType       string `json:"parsed_type,omitempty"`
	Executable       bool   `json:"executable"`
	Evidence         string `json:"evidence"`
}

// classifyRadarTarget prevents an account, token account, program or signature
// from being scored as if it were an SPL token mint. Unavailable classification
// is never treated as low risk.
func classifyRadarTarget(parent context.Context, target string) radarTargetClassification {
	target = strings.TrimSpace(target)
	out := radarTargetClassification{Type: radarTargetUnknown, Status: "insufficient_evidence", Evidence: "Target type could not be verified."}
	if target == "" {
		out.Status = "empty_target"
		out.Evidence = "Target is empty."
		return out
	}
	if len(target) > 60 {
		out.Type = radarTargetTransactionSignature
		out.Status = "syntax_classified"
		out.Evidence = "The supplied value has transaction-signature length and cannot receive a token-mint verdict."
		return out
	}

	rpcURL := creatorIntelRPCURL()
	if rpcURL == "" {
		out.Status = "rpc_unavailable"
		out.Evidence = "Solana RPC is unavailable; target type was not verified."
		return out
	}
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	account, err := services.SolanaGetAccountInfoJSONParsed(ctx, rpcURL, target)
	if err != nil {
		out.Status = "lookup_failed"
		out.Evidence = "Solana account lookup failed: " + compactRadarTargetError(err)
		return out
	}
	if account.Value == nil {
		out.Status = "account_not_found"
		out.Evidence = "No Solana account was found for the supplied target."
		return out
	}

	out.AccountOwner = strings.TrimSpace(account.Value.Owner)
	out.Executable = account.Value.Executable
	data := radarTargetMap(account.Value.Data)
	parsed := radarTargetMap(data["parsed"])
	out.ParsedType = strings.ToLower(strings.TrimSpace(fmt.Sprint(parsed["type"])))
	if out.ParsedType == "<nil>" {
		out.ParsedType = ""
	}
	info := radarTargetMap(parsed["info"])

	switch {
	case out.ParsedType == "mint":
		out.Type = radarTargetTokenMint
		out.Status = "verified_rpc_observation"
		out.Evidence = "Parsed SPL token mint account verified through Solana RPC."
	case out.ParsedType == "account":
		out.Type = radarTargetTokenAccount
		out.Status = "verified_rpc_observation"
		out.TokenOwnerWallet = radarTargetString(info["owner"])
		out.Evidence = "Parsed SPL token account verified; this is a holder account, not a token mint."
	case out.Executable:
		out.Type = radarTargetProgram
		out.Status = "verified_rpc_observation"
		out.Evidence = "Executable Solana program account verified; token-mint scoring is not applicable."
	default:
		out.Type = radarTargetWallet
		out.Status = "verified_rpc_observation"
		out.Evidence = "Non-executable Solana account verified; wallet intelligence is required instead of token-mint scoring."
	}
	return out
}

func radarTargetTokenVerdictAllowed(classification radarTargetClassification) bool {
	return classification.Type == radarTargetTokenMint && classification.Status == "verified_rpc_observation"
}

func radarTargetRejectionMessage(classification radarTargetClassification) string {
	switch classification.Type {
	case radarTargetTokenAccount:
		if classification.TokenOwnerWallet != "" {
			return "Bu adres bir SPL token hesabıdır; token mint değildir. Holder owner cüzdanı " + classification.TokenOwnerWallet + " için Wallet/Holder Intelligence çalıştırılmalıdır."
		}
		return "Bu adres bir SPL token hesabıdır; token mint değildir. Holder Intelligence çalıştırılmalıdır."
	case radarTargetWallet:
		return "Bu hedef bir cüzdan adresidir. Token risk skoru uygulanamaz; Wallet Intelligence, funding cluster ve işlem geçmişi analizi çalıştırılmalıdır."
	case radarTargetProgram:
		return "Bu hedef yürütülebilir bir Solana programıdır. Token risk skoru uygulanamaz; Program Risk analizi çalıştırılmalıdır."
	case radarTargetTransactionSignature:
		return "Bu hedef bir işlem imzasına benziyor. Token risk skoru uygulanamaz; Transaction/MEV analizi çalıştırılmalıdır."
	default:
		return "Hedefin token mint olduğu doğrulanamadı. Risk skoru verilmedi; INSUFFICIENT EVIDENCE."
	}
}

func radarTargetMap(raw any) map[string]any {
	value, _ := raw.(map[string]any)
	if value == nil {
		return map[string]any{}
	}
	return value
}

func radarTargetString(raw any) string {
	value := strings.TrimSpace(fmt.Sprint(raw))
	if value == "<nil>" {
		return ""
	}
	return value
}

func compactRadarTargetError(err error) string {
	value := strings.TrimSpace(fmt.Sprint(err))
	if len(value) > 240 {
		value = value[:240]
	}
	return value
}
