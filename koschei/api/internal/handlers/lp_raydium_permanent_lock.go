package handlers

import (
	"fmt"
	"strings"

	"koschei/api/internal/services"
)

const unresolvedLockerLimitation = "A known locker-program-owned account was observed, but an unlock timestamp was not decoded; lock duration remains unverified."

// finalizeRaydiumPermanentLPLock upgrades only direct fungible-LP custody
// evidence for the pinned Raydium Burn & Earn program. It does not treat a
// market label, an arbitrary locker name or a transaction signer as current
// permanent custody.
func finalizeRaydiumPermanentLPLock(lp services.LPControlEvidence) services.LPControlEvidence {
	if lp.ControlModel != "lp_token" || (lp.PoolProgram != raydiumCPMMProgram && lp.PoolProgram != raydiumAMMV4Program) {
		return lp
	}
	if strings.TrimSpace(lp.LockerProgram) != raydiumLPLockProgram || lp.LPSupply <= 0 {
		return lp
	}

	lockedAmount := 0.0
	lockedAccounts := []string{}
	lockerOwners := []string{}
	for _, holder := range lp.LargestLPHolders {
		if strings.TrimSpace(holder.AccountOwner) != raydiumLPLockProgram || strings.TrimSpace(holder.Classification) != "raydium_burn_and_earn" {
			continue
		}
		if holder.Amount <= 0 || strings.TrimSpace(holder.TokenAccount) == "" || strings.TrimSpace(holder.OwnerWallet) == "" {
			continue
		}
		lockedAmount += holder.Amount
		lockedAccounts = append(lockedAccounts, holder.TokenAccount)
		lockerOwners = append(lockerOwners, holder.OwnerWallet)
		lp.EvidenceKeys = append(lp.EvidenceKeys,
			fmt.Sprintf("raydium_burn_and_earn_lp:%s:%.8f@%d", holder.TokenAccount, holder.Amount, lp.ReadSlot),
		)
	}
	lockedAccounts = uniqueStrings(lockedAccounts)
	lockerOwners = uniqueStrings(lockerOwners)
	if lockedAmount <= 0 || len(lockedAccounts) == 0 {
		return lp
	}

	lp.LockedLPAmount = creatorIntelRound(lockedAmount, 8)
	lp.LockedLPSharePct = roundCollectorPct(lockedAmount / lp.LPSupply * 100)
	if lp.LockedLPSharePct > 100 {
		lp.LockedLPSharePct = 100
	}
	lp.LockedLPTokenAccounts = lockedAccounts
	if len(lockerOwners) == 1 {
		lp.LockerAccount = lockerOwners[0]
	}
	lp.Available = true
	lp.Status = services.LPControlVerifiedPermanentLocked
	lp.ReasonCode = "raydium_burn_and_earn_permanent_lock_observed"
	lp.EvidenceKeys = append(lp.EvidenceKeys,
		fmt.Sprintf("raydium_burn_and_earn_program:%s", raydiumLPLockProgram),
		fmt.Sprintf("raydium_permanent_locked_lp_share:%.4f@%d", lp.LockedLPSharePct, lp.ReadSlot),
	)
	lp.EvidenceKeys = uniqueStrings(lp.EvidenceKeys)
	lp.Limitations = removeLPControlLimitation(lp.Limitations, unresolvedLockerLimitation)
	lp.Limitations = append(lp.Limitations,
		"Permanent lock status is limited to the resolved LP token accounts whose authority account is owned by the pinned Raydium Burn & Earn program; unenumerated LP accounts are not inferred.",
	)
	lp.Limitations = uniqueStrings(lp.Limitations)
	return lp
}

func removeLPControlLimitation(values []string, remove string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || value == remove {
			continue
		}
		out = append(out, value)
	}
	return out
}
