package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// validateArvisStreamNativeTransaction is a STRICT evidence admission gate,
// not a source of risk grades. The JSON must be the exact Solana jsonParsed
// getTransaction response for the stream signature, fetched from the configured
// RPC by the caller. A log match or token name is never sufficient authority.
//
// Successful execution and an exact mint reference establish only that this
// stream transaction concerns the subject token. They DO NOT verify historical
// actor/behavior assertions, which require independent native collectors.
func validateArvisStreamNativeTransaction(event arvisStreamTarget, tx SolanaTransactionResult) error {
	reject := func(reason string) error {
		return fmt.Errorf("%w: native Solana transaction evidence %s", errArvisStreamCanonicalFinalUnavailable, reason)
	}
	if event.Network != "solana-mainnet" || !isLikelyRadarSolanaAddress(event.Target) ||
		!looksLikeSolanaSignature(event.Signature) || event.ID == "" {
		return reject("has invalid stream identity")
	}
	if !arvisStreamSourceProgramPermitted(event.ModuleID, event.ProgramID) {
		return reject("source program is not an approved Pump/Raydium program")
	}
	if len(tx) == 0 || radarInt64(tx["slot"]) <= 0 {
		return reject("missing transaction or positive slot")
	}
	slot := radarInt64(tx["slot"])
	if event.Slot > 0 && slot != event.Slot {
		return reject("slot does not match journal event")
	}
	meta, ok := tx["meta"].(map[string]any)
	if !ok {
		return reject("missing transaction execution metadata")
	}
	if executionError, exists := meta["err"]; !exists || executionError != nil {
		return reject("execution status was absent or transaction failed")
	}
	rawTx, ok := tx["transaction"].(map[string]any)
	if !ok {
		return reject("missing transaction payload")
	}
	sigs, ok := rawTx["signatures"].([]any)
	if !ok || len(sigs) == 0 || strings.TrimSpace(anyString(sigs[0])) != event.Signature {
		return reject("original transaction signature is not authenticated by the RPC response")
	}
	message, ok := rawTx["message"].(map[string]any)
	if !ok {
		return reject("missing parsed transaction message")
	}

	invokedProgram := false
	mintReferenced := false
	// Only exact program IDs on executed top-level or inner instructions
	// count; accountKeys and free-form logs are non-authoritative here.
	var inspectInstructions func(any)
	inspectInstructions = func(raw any) {
		entries, _ := raw.([]any)
		for _, entry := range entries {
			instruction, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			if strings.TrimSpace(anyString(instruction["programId"])) == event.ProgramID {
				invokedProgram = true
			}
			parsed := asRadarMap(instruction["parsed"])
			info := asRadarMap(parsed["info"])
			for _, key := range []string{"mint", "tokenMint", "mintAddress"} {
				if strings.TrimSpace(anyString(info[key])) == event.Target {
					mintReferenced = true
				}
			}
		}
	}
	inspectInstructions(message["instructions"])
	if inner, ok := meta["innerInstructions"].([]any); ok {
		for _, entry := range inner {
			inspectInstructions(asRadarMap(entry)["instructions"])
		}
	}
	for _, key := range []string{"preTokenBalances", "postTokenBalances"} {
		entries, _ := meta[key].([]any)
		for _, entry := range entries {
			if strings.TrimSpace(anyString(asRadarMap(entry)["mint"])) == event.Target {
				mintReferenced = true
			}
		}
	}
	if !invokedProgram {
		return reject("approved stream program was not invoked")
	}
	if !mintReferenced {
		return reject("subject mint is not present in parsed instructions or token balances")
	}
	return nil
}

func arvisStreamSourceProgramPermitted(moduleID, programID string) bool {
	switch moduleID {
	case ModulePumpSybilRadar:
		return programID == defaultPumpProgramID || programID == defaultPumpSwapProgramID
	case ModuleRaydiumPoolGuardian:
		return programID == defaultRaydiumProgramID || programID == legacyRaydiumProgramID || programID == legacyRaydiumSourceID
	default:
		return false
	}
}

// persistArvisStreamNativeEvidenceFinalTx requires a native transaction proof
// BEFORE the existing canonical v1.4 actor/behavior evaluator, independent
// trusted Ed25519 verification and immutable receipt transaction. This helper
// never converts native provenance into a rule hit or upgrades observed data.
func persistArvisStreamNativeEvidenceFinalTx(
	ctx context.Context, tx *sql.Tx, event arvisStreamTarget,
	native SolanaTransactionResult, actor ActorDefenseRuleVerdict,
	behavior UnifiedRadarBehaviorReport, registry GlobalRadarTrustedVerdictRegistry,
) (UnifiedRadarVerdict, error) {
	if err := validateArvisStreamNativeTransaction(event, native); err != nil {
		return UnifiedRadarVerdict{}, err
	}
	return persistArvisStreamCanonicalEvidenceFinalTx(ctx, tx, event, actor, behavior, registry)
}

// persistArvisStreamNativeEvidenceFinal is an explicit, bounded, network-
// consuming entry point for a future source-aware investigator. Unlike the
// legacy worker it fetches the ORIGINAL journal signature, never picks the
// latest unrelated token transaction. The RPC endpoint is an operator-provided
// Solana mainnet trust dependency; caller must also supply independently
// collected, target-scoped actor/behavior evidence.
func persistArvisStreamNativeEvidenceFinal(
	ctx context.Context, db *sql.DB, event arvisStreamTarget,
	actor ActorDefenseRuleVerdict, behavior UnifiedRadarBehaviorReport,
	registry GlobalRadarTrustedVerdictRegistry, mainnetRPCURL string,
) (UnifiedRadarVerdict, error) {
	if db == nil {
		return UnifiedRadarVerdict{}, errors.New("ARVIS native evidence database unavailable")
	}
	if strings.TrimSpace(mainnetRPCURL) == "" {
		return UnifiedRadarVerdict{}, fmt.Errorf("%w: native Solana mainnet RPC provider unavailable", errArvisStreamCanonicalFinalUnavailable)
	}
	if ctx == nil {
		return UnifiedRadarVerdict{}, errors.New("ARVIS native evidence context unavailable")
	}
	rpcCtx, cancel := context.WithTimeout(ctx, 7*time.Second)
	defer cancel()
	native, err := SolanaGetTransactionJSONParsed(rpcCtx, mainnetRPCURL, event.Signature)
	if err != nil {
		return UnifiedRadarVerdict{}, fmt.Errorf("%w: Solana getTransaction unavailable: %v", errArvisStreamCanonicalFinalUnavailable, err)
	}
	// Reject bad provider responses BEFORE opening a write transaction.
	if err := validateArvisStreamNativeTransaction(event, native); err != nil {
		return UnifiedRadarVerdict{}, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return UnifiedRadarVerdict{}, fmt.Errorf("begin ARVIS native evidence transaction: %w", err)
	}
	defer tx.Rollback()
	final, err := persistArvisStreamNativeEvidenceFinalTx(ctx, tx, event, native, actor, behavior, registry)
	if err != nil {
		return UnifiedRadarVerdict{}, err
	}
	if err := tx.Commit(); err != nil {
		return UnifiedRadarVerdict{}, fmt.Errorf("commit ARVIS native evidence receipt: %w", err)
	}
	return final, nil
}
