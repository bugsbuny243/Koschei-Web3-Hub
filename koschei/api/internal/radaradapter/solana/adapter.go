package solana

import (
	"context"
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
	return t.Network == a.target.Network && t.Raw == a.target.Raw && (t.Kind == "program" || t.Kind == "address")
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
	if !a.Supports(t) {
		return nil, limits, errors.New("target does not match Solana adapter subject")
	}
	slot, err := ReadFinalizedSlot(ctx, a.transport, t.Network)
	if err != nil {
		return nil, limits, err
	}
	res, account, err := ReadAccountAtSlot(ctx, a.transport, t.Network, t.Raw, slot)
	if err != nil {
		return nil, limits, err
	}
	linked, err := DecodeUpgradeableProgramAccount(account)
	if err != nil {
		return nil, append(limits, r.Limitation{
			Code: "not_upgradeable_program", Reason: "Account is not an executable upgradeable BPF program.",
		}), err
	}
	dataAddress := EncodeSolanaPubkey(linked)
	dataResponse, dataAccount, err := ReadAccountAtSlot(ctx, a.transport, t.Network, dataAddress, slot)
	if err != nil {
		return nil, append(limits, r.Limitation{Code: "programdata_read_failed", Reason: "Linked ProgramData account could not be read at the finalized slot lower bound."}), err
	}
	state, err := DecodeUpgradeableProgramDataAccount(dataAccount)
	if err != nil || state.Slot > slot {
		return nil, append(limits, r.Limitation{Code: "programdata_invalid", Reason: "Linked ProgramData account is invalid or its upgrade slot exceeds the finalized anchor."}), errors.New("invalid linked ProgramData state")
	}
	row := r.NewRow(a.subject, "solana-program-link", res, strconv.FormatUint(slot, 10), map[string]any{
		"programdata_pubkey_hex": dataAddress,
		"program_owner":          account.Owner,
		"program_executable":     account.Executable,
		"lamports":               account.Lamports,
	})
	dataRow := r.NewRow(a.subject, "solana-programdata-authority", dataResponse, strconv.FormatUint(slot, 10), map[string]any{
		"programdata_address": dataAddress,
		"upgrade_slot": state.Slot,
		"authority_present": state.HasAuthority,
		"programdata_owner": dataAccount.Owner,
	})
	return []r.EvidenceRow{row, dataRow}, limits, nil
}
