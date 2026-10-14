package evm

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	r "koschei/api/internal/radaradapter"
	"math/big"
	"strings"
)

// Discovery is deliberately bounded. Current allowance/operator state is read
// at the same canonical block hash as the account, not inferred from old logs.
func (a *Adapter) extraPermissions(ctx context.Context, from uint64, to string, anchor any, add func(string, r.Response, map[string]any), limits *[]r.Limitation) {
	ownerTopic := "0x" + strings.Repeat("0", 24) + a.subject.NativeID[2:]
	for _, kind := range []string{"nft_operator", "permit2"} {
		signature := "ApprovalForAll(address,address,bool)"
		if kind == "permit2" {
			signature = "Approval(address,address,address,uint160,uint48)"
		}
		topic := "0x" + Keccak([]byte(signature))
		query := map[string]any{"fromBlock": fmt.Sprintf("0x%x", from), "toBlock": to, "topics": []any{topic, ownerTopic}}
		// This is the canonical Permit2 deployment address; code and getter results
		// are still observed, never assumed safe from an address match alone.
		const permit2 = "0x000000000022d473030f116ddee9f6b43ac78ba3"
		if kind == "permit2" {
			query["address"] = permit2
		}
		response, raw, e := a.rpc(ctx, "eth_getLogs", []any{query})
		if e != nil {
			*limits = append(*limits, r.Limitation{Code: kind + "_discovery_unavailable", Reason: "Bounded permission log discovery unavailable."})
			continue
		}
		var logs []struct {
			Address string   `json:"address"`
			Topics  []string `json:"topics"`
			Removed bool     `json:"removed"`
			Tx      string   `json:"transactionHash"`
		}
		if json.Unmarshal(raw, &logs) != nil {
			*limits = append(*limits, r.Limitation{Code: kind + "_logs_invalid", Reason: "Permission log payload could not be decoded."})
			continue
		}
		seen := map[string]bool{}
		for _, log := range logs {
			if log.Removed || len(log.Topics) < 3 || log.Topics[0] != topic || log.Topics[1] != ownerTopic {
				continue
			}
			contract, e := Address(log.Address)
			if e != nil {
				continue
			}
			if len(seen) >= 8 {
				break
			}
			token, spender := "", ""
			if kind == "permit2" {
				if contract != permit2 || len(log.Topics) != 4 {
					continue
				}
				token, e = addressWord(log.Topics[2])
				if e != nil {
					continue
				}
				spender, e = addressWord(log.Topics[3])
			} else {
				if len(log.Topics) != 3 {
					continue
				}
				spender, e = addressWord(log.Topics[2])
			}
			if e != nil || seen[contract+token+spender] {
				continue
			}
			seen[contract+token+spender] = true
			data := selector("isApprovedForAll(address,address)") + ownerTopic[2:] + strings.Repeat("0", 24) + spender[2:]
			if kind == "permit2" {
				data = selector("allowance(address,address,address)") + ownerTopic[2:] + strings.Repeat("0", 24) + token[2:] + strings.Repeat("0", 24) + spender[2:]
			}
			current, value, e := a.str(ctx, "eth_call", []any{map[string]any{"to": contract, "data": data}, anchor})
			if e != nil {
				*limits = append(*limits, r.Limitation{Code: kind + "_state_unavailable", Reason: "Discovered permission could not be reconciled at the pinned block."})
				continue
			}
			bytes, e := hex.DecodeString(strings.TrimPrefix(value, "0x"))
			if e != nil {
				continue
			}
			facts := map[string]any{"contract": contract, "spender": spender, "discovery_transaction": log.Tx, "discovery_response_sha256": r.Digest(response.Raw)}
			if kind == "permit2" {
				if len(bytes) != 96 {
					continue
				}
				amount, expiration, nonce := new(big.Int).SetBytes(bytes[:32]), new(big.Int).SetBytes(bytes[32:64]), new(big.Int).SetBytes(bytes[64:])
				if amount.BitLen() > 160 || expiration.BitLen() > 48 || nonce.BitLen() > 48 {
					continue
				}
				facts["token"] = token
				facts["allowance"] = amount.String()
				facts["expiration"] = expiration.String()
				facts["nonce"] = nonce.String()
			} else {
				if len(bytes) != 32 {
					continue
				}
				v := new(big.Int).SetBytes(bytes)
				if v.Cmp(big.NewInt(1)) > 0 {
					continue
				}
				facts["approved_for_all"] = v.Sign() == 1
			}
			add("evm-"+kind, current, facts)
		}
	}
	*limits = append(*limits, r.Limitation{Code: "bounded_extended_permissions", Reason: "At most eight NFT operator and eight Permit2 pairs in the current log window are reconciled. Off-chain signatures and undiscovered older permissions remain unknown."})
}
