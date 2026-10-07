package handlers

import (
	"context"
	"database/sql"
	"strings"

	"koschei/api/internal/services"
)

// buildRequestScopeC006Relation joins the already-collected canonical creator
// distribution evidence to Koschei's existing dominant-holder memory. It does
// not start a new provider crawl or background job.
func buildRequestScopeC006Relation(ctx context.Context, db *sql.DB, report services.ActorInitialRecipientReport, holder services.HolderIntelligence) services.CrossTokenCreatorHolderTransfer {
	out := services.CrossTokenCreatorHolderTransfer{
		Available: false, Status: "recipient_evidence_unavailable",
		Mint: strings.TrimSpace(report.Mint), CreatorWallet: strings.TrimSpace(report.CreatorWallet),
		OtherTokens: []services.RepeatDominantHolderMatch{}, Limitations: []string{},
	}
	var recipient *services.ActorInitialRecipient
	for i := range report.Recipients {
		item := &report.Recipients[i]
		if !strings.EqualFold(strings.TrimSpace(item.VerificationStatus), "verified") ||
			strings.TrimSpace(item.Wallet) == "" || strings.TrimSpace(item.Signature) == "" ||
			item.Slot <= 0 || item.ObservedAt.IsZero() {
			continue
		}
		recipient = item
		break
	}
	if recipient == nil {
		out.Limitations = append(out.Limitations, "URD-C006 not evaluated because no canonically parsed creator outbound transfer with signature, slot, and owner-resolved recipient is available in request scope.")
		return out
	}

	out.Status = "cross_token_memory_unavailable"
	out.RecipientTokenAccount = strings.TrimSpace(recipient.DestinationTokenAccount)
	out.RecipientOwnerWallet = strings.TrimSpace(recipient.Wallet)
	out.RecipientOwnerResolved = true
	out.TransferSignature = strings.TrimSpace(recipient.Signature)
	out.Slot = recipient.Slot
	out.Direction = "creator_to_recipient_owner"
	out.Amount = recipient.Amount
	out.Supply = holder.Supply
	out.ObservedAt = recipient.ObservedAt.UTC()

	memoryAvailable := false
	if db != nil {
		store := services.NewSecurityRadarStore(db)
		if matches, err := store.RepeatDominantHolderMatchesExceptMint(ctx, out.RecipientOwnerWallet, out.Mint, services.RepeatDominantObservationDays); err == nil {
			out.OtherTokens = append(out.OtherTokens, matches...)
			memoryAvailable = true
		} else {
			out.Limitations = append(out.Limitations, "Persistent dominant-holder memory could not be read for URD-C006.")
		}
	}

	// Request-scope holder intelligence is the bounded restore path when the
	// persistent radar store is unavailable or has not yet materialized this run.
	for _, row := range holder.Rows {
		if !row.OwnerResolved || strings.TrimSpace(row.OwnerWallet) != out.RecipientOwnerWallet {
			continue
		}
		memoryAvailable = true
		out.OtherTokens = append(out.OtherTokens, row.RepeatDominantMatches...)
		break
	}
	if !memoryAvailable {
		out.Limitations = append(out.Limitations, "URD-C006 not evaluated because cross-token dominant-holder memory is unavailable for the owner-resolved transfer recipient.")
		return out
	}

	out.Available = true
	out.Status = "verified"
	return out
}
