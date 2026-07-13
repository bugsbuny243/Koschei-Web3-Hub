package handlers

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"koschei/api/internal/services"
)

type actorDefenseRequest struct {
	Target       string `json:"target"`
	Address      string `json:"address"`
	Network      string `json:"network"`
	LiveEvidence *bool  `json:"live_evidence,omitempty"`
}

type actorDefenseLiveCoverage struct {
	Status             string   `json:"status"`
	SignaturesSeen     int      `json:"signatures_seen"`
	TransactionsParsed int      `json:"transactions_parsed"`
	EvidencePersisted  int      `json:"evidence_persisted"`
	RPCFailures        int      `json:"rpc_failures"`
	SignatureLimit     int      `json:"signature_limit"`
	TransactionLimit   int      `json:"transaction_limit"`
	EvidenceLimit      int      `json:"evidence_limit"`
	Limitations        []string `json:"limitations"`
}

type actorDefenseTokenAccountOwner struct {
	Owner string
	Mint  string
}

// OwnerActorDefenseInvestigation is the wallet-first investigation surface for
// Koschei's defense network. It assembles existing Pump discovery, holder and
// trade sensors, then verifies bounded direct transaction evidence. It does not
// produce a token score for a wallet and it never turns a relation into an
// identity or wrongdoing claim.
func (h *Handler) OwnerActorDefenseInvestigation(w http.ResponseWriter, r *http.Request) {
	var input actorDefenseRequest
	if err := decodeJSON(r, &input); err != nil {
		writeAPIError(w, http.StatusBadRequest, APICodeInvalidInput, "Invalid request body")
		return
	}
	target := strings.TrimSpace(firstNonEmptyString(input.Target, input.Address))
	if target == "" {
		writeAPIError(w, http.StatusBadRequest, APICodeInvalidInput, "target is required")
		return
	}
	network := strings.TrimSpace(input.Network)
	if network == "" {
		network = "solana-mainnet"
	}
	classification := classifyRadarTarget(r.Context(), target)
	wallet := target
	switch classification.Type {
	case radarTargetWallet:
		// Direct wallet investigation.
	case radarTargetTokenAccount:
		wallet = strings.TrimSpace(classification.TokenOwnerWallet)
		if wallet == "" {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
				"ok": false, "error": "token_account_owner_unresolved",
				"target": target, "target_classification": classification,
				"message": "Token hesabının owner cüzdanı çözümlenemedi; wallet investigation başlatılmadı.",
			})
			return
		}
	default:
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"ok": false, "error": "wallet_target_required",
			"target": target, "target_classification": classification,
			"message": "Bu endpoint wallet-first actor investigation içindir; doğrulanmış bir cüzdan veya token hesabı gerekir.",
		})
		return
	}

	db := h.DBRead
	if db == nil {
		db = h.DB
	}
	if db == nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Actor defense database is unavailable")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 95*time.Second)
	defer cancel()
	store := services.NewActorDefenseStore(db)
	initial, err := store.LoadWalletDossier(ctx, wallet, network, 75)
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Actor defense dossier could not be assembled")
		return
	}

	liveEnabled := true
	if input.LiveEvidence != nil {
		liveEnabled = *input.LiveEvidence
	}
	coverage := actorDefenseLiveCoverage{Status: "stored_evidence_only"}
	if liveEnabled {
		coverage = h.collectActorDefenseLiveEvidence(ctx, store, initial)
	}
	final, err := store.LoadWalletDossier(ctx, wallet, network, 100)
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Actor defense dossier could not be refreshed")
		return
	}
	final.Coverage["live_evidence"] = coverage
	final.Coverage["requested_target"] = target
	final.Coverage["resolved_wallet"] = wallet
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true,
		"schema_version": "koschei-actor-defense-v1",
		"target": target,
		"wallet": wallet,
		"network": network,
		"target_classification": classification,
		"dossier": final,
	})
}

func (h *Handler) collectActorDefenseLiveEvidence(ctx context.Context, store *services.ActorDefenseStore, dossier services.ActorDefenseDossier) actorDefenseLiveCoverage {
	coverage := actorDefenseLiveCoverage{
		Status: "complete",
		SignatureLimit: actorDefenseEnvInt("ACTOR_DEFENSE_SIGNATURE_LIMIT", 120, 20, 500),
		TransactionLimit: actorDefenseEnvInt("ACTOR_DEFENSE_TRANSACTION_LIMIT", 40, 5, 120),
		EvidenceLimit: actorDefenseEnvInt("ACTOR_DEFENSE_EVIDENCE_LIMIT", 100, 10, 300),
	}
	rpcURL := creatorIntelRPCURL()
	if rpcURL == "" {
		coverage.Status = "rpc_unavailable"
		coverage.Limitations = append(coverage.Limitations, "Solana RPC yapılandırılmadığı için yeni transaction kanıtı toplanmadı.")
		return coverage
	}
	signatures, err := services.SolanaGetSignaturesForAddress(ctx, rpcURL, dossier.Wallet, coverage.SignatureLimit)
	if err != nil {
		coverage.Status = "rpc_failed"
		coverage.RPCFailures++
		coverage.Limitations = append(coverage.Limitations, "Wallet işlem imzaları RPC sağlayıcısından alınamadı: "+creatorIntelCompactError(err))
		return coverage
	}
	coverage.SignaturesSeen = len(signatures)
	knownMints := map[string]bool{}
	for _, token := range dossier.Tokens {
		if strings.TrimSpace(token.Mint) != "" {
			knownMints[token.Mint] = true
		}
	}
	relatedActors := map[string]bool{}
	for _, actor := range dossier.RelatedActors {
		if strings.TrimSpace(actor.Wallet) != "" {
			relatedActors[actor.Wallet] = true
		}
	}

	for _, signature := range signatures {
		if coverage.TransactionsParsed >= coverage.TransactionLimit || coverage.EvidencePersisted >= coverage.EvidenceLimit || ctx.Err() != nil {
			break
		}
		if signature.Err != nil || strings.TrimSpace(signature.Signature) == "" {
			continue
		}
		tx, txErr := services.SolanaGetTransactionJSONParsed(ctx, rpcURL, signature.Signature)
		if txErr != nil {
			coverage.RPCFailures++
			continue
		}
		coverage.TransactionsParsed++
		txMap := map[string]any(tx)
		meta := creatorIntelMap(txMap["meta"])
		message := creatorIntelMap(creatorIntelMap(txMap["transaction"])["message"])
		keys, signers := creatorIntelAccountKeys(message)
		actorSigned := creatorIntelContains(signers, dossier.Wallet)
		observedAt := actorDefenseObservedAt(signature, txMap)
		owners := actorDefenseTokenAccountOwners(meta, keys)
		instructions := actorDefenseInstructions(message, meta)

		for index, instruction := range instructions {
			if coverage.EvidencePersisted >= coverage.EvidenceLimit {
				break
			}
			parsed := creatorIntelMap(instruction["parsed"])
			kind := strings.ToLower(creatorIntelCleanString(parsed["type"]))
			info := creatorIntelMap(parsed["info"])
			program := strings.ToLower(creatorIntelCleanString(instruction["program"]))
			evidenceKey := fmt.Sprintf("%s:%d", signature.Signature, index)

			if program == "system" && kind == "transfer" {
				source := creatorIntelCleanString(info["source"])
				destination := creatorIntelCleanString(info["destination"])
				lamports := creatorIntelInt64(info["lamports"])
				if source == dossier.Wallet && destination != "" && destination != dossier.Wallet && actorSigned {
					item := services.ActorDefenseEvidenceRecord{
						Network: dossier.Network, ActorWallet: dossier.Wallet,
						CounterpartKind: "wallet", CounterpartID: destination,
						Relation: "direct_sol_transfer_out", VerificationStatus: "verified",
						EvidenceKey: evidenceKey, Source: "solana_jsonparsed_instruction",
						Signature: signature.Signature, Slot: signature.Slot, ObservedAt: observedAt,
						AmountNative: float64(lamports) / 1e9,
						Metadata: map[string]any{"actor_signed": true, "instruction_type": kind, "known_related_actor": relatedActors[destination]},
					}
					if store.UpsertEvidence(ctx, item) == nil {
						coverage.EvidencePersisted++
					}
				} else if destination == dossier.Wallet && source != "" && source != dossier.Wallet {
					item := services.ActorDefenseEvidenceRecord{
						Network: dossier.Network, ActorWallet: dossier.Wallet,
						CounterpartKind: "wallet", CounterpartID: source,
						Relation: "direct_sol_transfer_in", VerificationStatus: "verified",
						EvidenceKey: evidenceKey, Source: "solana_jsonparsed_instruction",
						Signature: signature.Signature, Slot: signature.Slot, ObservedAt: observedAt,
						AmountNative: float64(lamports) / 1e9,
						Metadata: map[string]any{"actor_signed": actorSigned, "instruction_type": kind, "known_related_actor": relatedActors[source]},
					}
					if store.UpsertEvidence(ctx, item) == nil {
						coverage.EvidencePersisted++
					}
				}
				continue
			}

			if strings.Contains(program, "token") && (kind == "transfer" || kind == "transferchecked") {
				sourceAccount := creatorIntelCleanString(info["source"])
				destinationAccount := creatorIntelCleanString(info["destination"])
				authority := creatorIntelCleanString(info["authority"])
				sourceOwner := owners[sourceAccount]
				destinationOwner := owners[destinationAccount]
				mint := firstNonEmptyString(sourceOwner.Mint, destinationOwner.Mint, creatorIntelCleanString(info["mint"]))
				amount := actorDefenseTokenAmount(info)
				if sourceOwner.Owner == dossier.Wallet && destinationOwner.Owner != "" && destinationOwner.Owner != dossier.Wallet && (authority == dossier.Wallet || actorSigned) {
					item := services.ActorDefenseEvidenceRecord{
						Network: dossier.Network, ActorWallet: dossier.Wallet,
						CounterpartKind: "wallet", CounterpartID: destinationOwner.Owner,
						Relation: "direct_token_transfer_out", VerificationStatus: "verified",
						EvidenceKey: evidenceKey, Source: "solana_jsonparsed_instruction",
						Signature: signature.Signature, Slot: signature.Slot, ObservedAt: observedAt,
						TokenMint: mint, TokenAmount: amount,
						Metadata: map[string]any{
							"actor_signed": actorSigned, "authority": authority,
							"source_token_account": sourceAccount, "destination_token_account": destinationAccount,
							"known_token": knownMints[mint], "known_related_actor": relatedActors[destinationOwner.Owner],
						},
					}
					if store.UpsertEvidence(ctx, item) == nil {
						coverage.EvidencePersisted++
					}
				} else if destinationOwner.Owner == dossier.Wallet && sourceOwner.Owner != "" && sourceOwner.Owner != dossier.Wallet {
					item := services.ActorDefenseEvidenceRecord{
						Network: dossier.Network, ActorWallet: dossier.Wallet,
						CounterpartKind: "wallet", CounterpartID: sourceOwner.Owner,
						Relation: "direct_token_transfer_in", VerificationStatus: "verified",
						EvidenceKey: evidenceKey, Source: "solana_jsonparsed_instruction",
						Signature: signature.Signature, Slot: signature.Slot, ObservedAt: observedAt,
						TokenMint: mint, TokenAmount: amount,
						Metadata: map[string]any{
							"actor_signed": actorSigned, "authority": authority,
							"source_token_account": sourceAccount, "destination_token_account": destinationAccount,
							"known_token": knownMints[mint], "known_related_actor": relatedActors[sourceOwner.Owner],
						},
					}
					if store.UpsertEvidence(ctx, item) == nil {
						coverage.EvidencePersisted++
					}
				}
			}
		}

		if actorSigned {
			if action, instructionTypes := actorDefenseLiquidityRemoval(message, meta); action {
				item := services.ActorDefenseEvidenceRecord{
					Network: dossier.Network, ActorWallet: dossier.Wallet,
					CounterpartKind: "transaction", CounterpartID: signature.Signature,
					Relation: "liquidity_remove_activity", VerificationStatus: "observed",
					EvidenceKey: signature.Signature + ":liquidity_remove", Source: "solana_transaction_logs",
					Signature: signature.Signature, Slot: signature.Slot, ObservedAt: observedAt,
					Metadata: map[string]any{"actor_signed": true, "instruction_types": instructionTypes, "classification_scope": "parsed instruction/log observation; not proof of malicious intent"},
				}
				if store.UpsertEvidence(ctx, item) == nil {
					coverage.EvidencePersisted++
				}
			}
		}
	}
	if coverage.TransactionsParsed == 0 && coverage.Status == "complete" {
		coverage.Status = "no_parsed_transactions"
	}
	coverage.Limitations = append(coverage.Limitations,
		fmt.Sprintf("Canlı kanıt taraması en fazla %d imza, %d başarılı transaction ve %d yeni kanıtla sınırlandırıldı.", coverage.SignatureLimit, coverage.TransactionLimit, coverage.EvidenceLimit),
		"Doğrudan transfer yalnız jsonParsed instruction ve token-account owner eşleşmesiyle VERIFIED olur.",
		"Likidite kaldırma davranışı parsed instruction/log gözlemidir; tek başına kötü niyet kanıtı değildir.",
	)
	return coverage
}

func actorDefenseInstructions(message, meta map[string]any) []map[string]any {
	out := []map[string]any{}
	appendRows := func(raw any) {
		items, _ := raw.([]any)
		for _, item := range items {
			row := creatorIntelMap(item)
			if len(row) > 0 {
				out = append(out, row)
			}
		}
	}
	appendRows(message["instructions"])
	inner, _ := meta["innerInstructions"].([]any)
	for _, item := range inner {
		appendRows(creatorIntelMap(item)["instructions"])
	}
	return out
}

func actorDefenseTokenAccountOwners(meta map[string]any, keys []string) map[string]actorDefenseTokenAccountOwner {
	out := map[string]actorDefenseTokenAccountOwner{}
	collect := func(raw any) {
		items, _ := raw.([]any)
		for _, item := range items {
			row := creatorIntelMap(item)
			index := creatorIntelInt(row["accountIndex"])
			if index < 0 || index >= len(keys) {
				continue
			}
			owner := creatorIntelCleanString(row["owner"])
			mint := creatorIntelCleanString(row["mint"])
			if owner != "" || mint != "" {
				out[keys[index]] = actorDefenseTokenAccountOwner{Owner: owner, Mint: mint}
			}
		}
	}
	collect(meta["preTokenBalances"])
	collect(meta["postTokenBalances"])
	return out
}

func actorDefenseTokenAmount(info map[string]any) float64 {
	if tokenAmount := creatorIntelMap(info["tokenAmount"]); len(tokenAmount) > 0 {
		return creatorIntelUIAmount(tokenAmount)
	}
	raw := creatorIntelCleanString(info["amount"])
	value, _ := strconv.ParseFloat(raw, 64)
	return value
}

func actorDefenseLiquidityRemoval(message, meta map[string]any) (bool, []string) {
	instructionTypes, _ := creatorIntelInstructions(message, meta)
	logs := strings.ToLower(strings.Join(creatorIntelStringSlice(meta["logMessages"]), "\n"))
	for _, kind := range instructionTypes {
		value := strings.ToLower(kind)
		if strings.Contains(value, "removeliquidity") || strings.Contains(value, "remove_liquidity") || strings.Contains(value, "withdrawliquidity") || strings.Contains(value, "withdraw_liquidity") {
			return true, instructionTypes
		}
	}
	markers := []string{"remove_liquidity", "remove liquidity", "withdraw liquidity", "withdraw all token types"}
	for _, marker := range markers {
		if strings.Contains(logs, marker) {
			return true, instructionTypes
		}
	}
	return false, instructionTypes
}

func actorDefenseObservedAt(signature services.SolanaSignatureInfo, tx map[string]any) time.Time {
	if blockTime := creatorIntelInt64(tx["blockTime"]); blockTime > 0 {
		return time.Unix(blockTime, 0).UTC()
	}
	if signature.BlockTime != nil && *signature.BlockTime > 0 {
		return time.Unix(*signature.BlockTime, 0).UTC()
	}
	return time.Now().UTC()
}

func actorDefenseEnvInt(name string, fallback, minimum, maximum int) int {
	value := fallback
	if raw := strings.TrimSpace(os.Getenv(name)); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			value = parsed
		}
	}
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}
