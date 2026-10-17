package solana

import (
	"context"
	"encoding/hex"
	"errors"
	"strconv"

	r "koschei/api/internal/radaradapter"
)

type Adapter struct {
	target    r.Target
	subject   r.SubjectIdentity
	transport r.Transport
}

func New(t r.Target, transport r.Transport) (r.Adapter, error) {
	if t.Network != "solana-mainnet" || (t.Kind != "program" && t.Kind != "address") {
		return nil, errors.New("unsupported Solana target")
	}
	if _, err := DecodeSolanaPubkey(t.Raw); err != nil {
		return nil, err
	}
	return &Adapter{target: t, transport: transport, subject: r.SubjectIdentity{
		Network: t.Network, NativeID: t.Raw, IdentityKind: "program",
		IdentityValue: t.Raw, ChainIdentity: "solana:mainnet-beta",
	}}, nil
}

func (a *Adapter) Supports(t r.Target) bool {
	return t.Network == "solana-mainnet" && (t.Kind == "program" || t.Kind == "address")
}

func (a *Adapter) Subject() r.SubjectIdentity { return r.CollectorIdentity(a.subject) }

func (a *Adapter) Capability() r.Capability {
	return r.Capability{
		ID: "solana-upgradeable-program", Network: "solana-mainnet", Family: "solana",
		TargetClasses: []string{"program", "address"},
		Sources:       []string{"Finalized Solana getSlot and getAccountInfo"},
		ArchivalRequirements: []string{
			"minContextSlot is a lower bound, not an atomic historical snapshot; upgrade history is not reconstructed.",
		},
		Fixture: "builtin/fixtures/solana.json", EvidenceOnly: true,
	}
}

func (a *Adapter) Collect(ctx context.Context, t r.Target) ([]r.EvidenceRow, []r.Limitation, error) {
	limits := []r.Limitation{{
		Code:   "solana_non_atomic_reads",
		Reason: "Finalized minContextSlot only bounds account reads; it does not provide an atomic snapshot or upgrade history.",
	}}
	slot, err := ReadFinalizedSlot(ctx, a.transport, t.Network)
	if err != nil {
		return nil, limits, err
	}
	res, account, err := ReadAccountAtSlot(ctx, a.transport, t.Network, t.Raw, slot)
	if err != nil {
		return nil, limits, err
	}
	// The first getSlot value is a minimum, not the identity of the later
	// getAccountInfo result. Read the later context directly from raw bytes.
	accountSlot, err := accountResponseContextSlot(res.Raw, slot)
	if err != nil {
		return nil, append(limits, r.Limitation{
			Code:   "solana_account_context_slot_unbound",
			Reason: "The raw account response has no unique valid context.slot at or above the finalized lower bound.",
		}), errors.New("solana account context slot unavailable")
	}
	linked, err := DecodeUpgradeableProgramAccount(account)
	if err != nil {
		return nil, append(limits, r.Limitation{
			Code: "not_upgradeable_program", Reason: "Account is not an executable upgradeable BPF program.",
		}), err
	}
	row := r.NewRow(a.subject, "solana-program-link", res, strconv.FormatUint(accountSlot, 10), map[string]any{
		"programdata_pubkey_hex":     hex.EncodeToString(linked[:]),
		"program_owner":              account.Owner,
		"program_executable":         account.Executable,
		"lamports":                   account.Lamports,
		"account_context_slot":       strconv.FormatUint(accountSlot, 10),
		"requested_min_context_slot": strconv.FormatUint(slot, 10),
		"account_context_scope":      "raw_rpc_context_slot_not_consensus_or_atomic_snapshot",
	})
	// Bind the exact account-response bytes without inferring that getSlot and
	// getAccountInfo were atomic or that program authority was proven.
	witnesses, err := r.BindRawJSONSourceWitnesses(a.subject, []r.RawJSONObservation{{
		Row:          row,
		ResponseJSON: res.Raw,
	}})
	if err != nil || len(witnesses) != 1 {
		return nil, append(limits, r.Limitation{
			Code:   "raw_account_response_unbound",
			Reason: "Original account response bytes did not pass source integrity binding; no program safety conclusion is available.",
		}), errors.New("solana account source response integrity unavailable")
	}
	row.Attributes["source_integrity_scope"] = witnesses[0].IntegrityStatus
	row.Attributes["source_integrity_commitment_sha256"] = witnesses[0].CommitmentSHA256
	row.Attributes["source_integrity_chain_verified"] = false
	return []r.EvidenceRow{row}, append(limits, r.Limitation{
		Code: "programdata_not_fetched", Reason: "ProgramData authority and bytecode require a separate validated account read.",
	}), nil
}
