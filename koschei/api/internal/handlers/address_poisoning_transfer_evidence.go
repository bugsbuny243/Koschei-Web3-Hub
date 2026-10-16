package handlers

import (
	"fmt"
	"strconv"
	"strings"

	"koschei/api/internal/services"
)

const (
	addressPoisoningSystemProgramID    = "11111111111111111111111111111111"
	addressPoisoningSPLTokenProgramID  = "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"
	addressPoisoningToken2022ProgramID = "TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb"
)

// addressPoisoningOutboundEvidence deliberately contains observed recipients,
// not a verified contact book. Only successful parsed wallet-origin transfers
// can contribute recipient comparisons; account-key and mint appearances do not.
type addressPoisoningOutboundEvidence struct {
	Recipients           []string
	TransferCount        int
	UnresolvedTransfers  int
	UnparsedInstructions int
	FailedTransaction    bool
	MissingStructure     bool
}

type addressPoisoningTokenAccountOwner struct {
	Owner     string
	Mint      string
	Ambiguous bool
}

// extractSolanaOutboundRecipients never treats unrelated account keys,
// token mints, incoming dust, or failed transactions as the user's contacts.
// SPL token recipients are wallet owners resolved from token-balance account
// indexes, not the destination token accounts themselves.
func extractSolanaOutboundRecipients(tx services.SolanaTransactionResult, wallet string) addressPoisoningOutboundEvidence {
	result := addressPoisoningOutboundEvidence{Recipients: []string{}}
	wallet = strings.TrimSpace(wallet)
	transaction, ok := tx["transaction"].(map[string]any)
	if !ok || transaction == nil {
		result.MissingStructure = true
		return result
	}
	message, ok := transaction["message"].(map[string]any)
	if !ok || message == nil {
		result.MissingStructure = true
		return result
	}
	meta, ok := tx["meta"].(map[string]any)
	if !ok || meta == nil {
		result.MissingStructure = true
		return result
	}
	if meta["err"] != nil {
		result.FailedTransaction = true
		return result
	}
	if wallet == "" {
		result.MissingStructure = true
		return result
	}

	keys := addressPoisoningAccountKeys(message)
	owners := addressPoisoningTokenOwners(meta, keys)
	scan := func(raw any) {
		rows, ok := raw.([]any)
		if !ok {
			result.UnparsedInstructions++
			return
		}
		for _, rawInstruction := range rows {
			instruction, ok := rawInstruction.(map[string]any)
			if !ok {
				result.UnparsedInstructions++
				continue
			}
			programID, _ := instruction["programId"].(string)
			switch programID {
			case addressPoisoningSystemProgramID, addressPoisoningSPLTokenProgramID, addressPoisoningToken2022ProgramID:
			default:
				// Other programs do not establish wallet recipient identities.
				continue
			}
			parsed, ok := instruction["parsed"].(map[string]any)
			if !ok {
				result.UnparsedInstructions++
				continue
			}
			kind, _ := parsed["type"].(string)
			info, ok := parsed["info"].(map[string]any)
			if !ok {
				result.UnparsedInstructions++
				continue
			}
			source, _ := info["source"].(string)
			destination, _ := info["destination"].(string)
			source = strings.TrimSpace(source)
			destination = strings.TrimSpace(destination)
			if programID == addressPoisoningSystemProgramID {
				if kind == "transfer" && source == wallet && destination != wallet && destination != "" {
					if !addressPoisoningPositiveAmount(info["lamports"]) {
						continue
					}
					result.TransferCount++
					result.Recipients = append(result.Recipients, destination)
				}
				continue
			}
			if kind != "transfer" && kind != "transferChecked" {
				continue
			}
			authority, _ := info["authority"].(string)
			if strings.TrimSpace(authority) != wallet {
				continue
			}
			tokenAmount := info["amount"]
			if amount, ok := info["tokenAmount"].(map[string]any); ok {
				tokenAmount = amount["amount"]
			}
			if !addressPoisoningPositiveAmount(tokenAmount) {
				continue
			}
			sourceOwner, sourceOK := owners[source]
			destOwner, destOK := owners[destination]
			declaredMint, _ := info["mint"].(string)
			if !sourceOK || !destOK || sourceOwner.Ambiguous || destOwner.Ambiguous ||
				sourceOwner.Owner != wallet || destOwner.Owner == "" ||
				sourceOwner.Mint == "" || sourceOwner.Mint != destOwner.Mint ||
				(declaredMint != "" && declaredMint != sourceOwner.Mint) {
				result.UnresolvedTransfers++
				continue
			}
			result.TransferCount++
			if destOwner.Owner != wallet {
				result.Recipients = append(result.Recipients, destOwner.Owner)
			}
		}
	}
	if _, present := message["instructions"]; !present {
		result.MissingStructure = true
		return result
	}
	scan(message["instructions"])
	if inner, ok := meta["innerInstructions"].([]any); ok {
		for _, item := range inner {
			group, ok := item.(map[string]any)
			if !ok {
				result.UnparsedInstructions++
				continue
			}
			scan(group["instructions"])
		}
	} else if meta["innerInstructions"] != nil {
		result.UnparsedInstructions++
	}
	result.Recipients = normalizeAddressList(result.Recipients)
	return result
}

func addressPoisoningAccountKeys(message map[string]any) []string {
	rows, _ := message["accountKeys"].([]any)
	keys := make([]string, len(rows))
	for index, raw := range rows {
		switch value := raw.(type) {
		case string:
			keys[index] = strings.TrimSpace(value)
		case map[string]any:
			keys[index], _ = value["pubkey"].(string)
			keys[index] = strings.TrimSpace(keys[index])
		}
	}
	return keys
}

func addressPoisoningTokenOwners(meta map[string]any, keys []string) map[string]addressPoisoningTokenAccountOwner {
	owners := map[string]addressPoisoningTokenAccountOwner{}
	collect := func(raw any) {
		rows, _ := raw.([]any)
		for _, rawBalance := range rows {
			balance, ok := rawBalance.(map[string]any)
			if !ok {
				continue
			}
			index, err := strconv.Atoi(fmt.Sprint(balance["accountIndex"]))
			if err != nil || index < 0 || index >= len(keys) || keys[index] == "" {
				continue
			}
			owner, _ := balance["owner"].(string)
			mint, _ := balance["mint"].(string)
			owner, mint = strings.TrimSpace(owner), strings.TrimSpace(mint)
			if owner == "" || mint == "" {
				continue
			}
			key := keys[index]
			candidate := addressPoisoningTokenAccountOwner{Owner: owner, Mint: mint}
			if prior, found := owners[key]; found {
				if prior.Ambiguous || prior.Owner != owner || prior.Mint != mint {
					candidate.Ambiguous = true
				}
			}
			owners[key] = candidate
		}
	}
	collect(meta["preTokenBalances"])
	collect(meta["postTokenBalances"])
	return owners
}

func addressPoisoningPositiveAmount(raw any) bool {
	value := strings.TrimSpace(fmt.Sprint(raw))
	if value == "" || value == "<nil>" {
		return false
	}
	if count, err := strconv.ParseFloat(value, 64); err == nil {
		return count > 0
	}
	return false
}
