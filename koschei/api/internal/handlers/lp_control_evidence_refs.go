package handlers

import "koschei/api/internal/services"

func applyLPControlEvidenceReferences(refs map[string]unifiedEvidenceReference, lp services.LPControlEvidence) map[string]unifiedEvidenceReference {
	if refs == nil {
		refs = map[string]unifiedEvidenceReference{}
	}
	poolRef := unifiedEvidenceReference{
		Wallets:      []string{lp.PoolCreator, lp.CreatorWallet, lp.DominantLPOwner},
		Accounts:     []string{lp.PoolAddress, lp.PoolProgram, lp.TokenMint, lp.QuoteMint, lp.LPMint, lp.TokenVault, lp.QuoteVault, lp.LockerAccount, lp.LockerProgram},
		Slots:        []int64{int64(lp.ReadSlot)},
		EvidenceKeys: append([]string{}, lp.EvidenceKeys...),
	}
	movementRef := unifiedEvidenceReference{}
	for _, movement := range lp.LiquidityMovements {
		movementRef.Wallets = append(movementRef.Wallets, movement.ActorWallet, movement.SourceWallet, movement.DestinationWallet)
		movementRef.Accounts = append(movementRef.Accounts, movement.PoolAddress, movement.Program)
		movementRef.Signatures = append(movementRef.Signatures, movement.Signature)
		movementRef.Slots = append(movementRef.Slots, movement.Slot)
		movementRef.EvidenceKeys = append(movementRef.EvidenceKeys, movement.EvidenceKey)
	}
	refs["liquidity"] = mergeUnifiedEvidenceReferences(refs["liquidity"], poolRef, movementRef)
	refs["liq-move"] = mergeUnifiedEvidenceReferences(refs["liq-move"], poolRef, movementRef)
	return refs
}
