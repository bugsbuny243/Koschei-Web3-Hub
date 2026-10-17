package solana

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	radar "koschei/api/internal/radaradapter"
)

const holderSourceStatus = "observed_non_atomic_source_reconciliation_only"
const splTokenProgramID = "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"
const splToken2022ProgramID = "TokenzQdBNbLqP5VEhdkAS6EPF6oVj1mMG1z7J1bC"

// HolderRawRPCSnapshot contains caller-captured original JSON response bytes
// and the digest recorded at capture. It does not authenticate the RPC server.
type HolderRawRPCSnapshot struct {
	RawJSON       []byte
	CaptureSHA256 string
}

// HolderRawSourceSet must be supplied by an already-authorized read-only RPC
// path. This parser never fetches, signs, broadcasts or writes to storage.
type HolderRawSourceSet struct {
	Mint                   string
	RequestedTokenAccounts []string
	CapturedAt             time.Time
	Supply                 HolderRawRPCSnapshot
	Largest                HolderRawRPCSnapshot
	Accounts               HolderRawRPCSnapshot
}

// HolderTokenAccountObservation separates an SPL token-account address from
// its parsed owner authority. An owner authority may be a PDA or multisig,
// not necessarily an economically controlled wallet or person.
type HolderTokenAccountObservation struct {
	TokenAccount   string `json:"token_account"`
	OwnerAuthority string `json:"owner_authority"`
	TokenProgram   string `json:"token_program"`
	RawAmount      string `json:"raw_amount"`
}

// HolderRawReconciliation is an observational snapshot comparison, never a
// final holder distribution, market verdict or trusted on-chain attestation.
type HolderRawReconciliation struct {
	Status                 string                          `json:"status"`
	Mint                   string                          `json:"mint"`
	RawSupplyAmount        string                          `json:"raw_supply_amount"`
	RawTopAccountSum       string                          `json:"raw_top_account_sum"`
	Decimals               uint8                           `json:"decimals"`
	SupplySlot             uint64                          `json:"supply_slot"`
	LargestAccountsSlot    uint64                          `json:"largest_accounts_slot"`
	TokenAccountsSlot      uint64                          `json:"token_accounts_slot"`
	SameReportedSlot       bool                            `json:"same_reported_slot"`
	AtomicSnapshotVerified bool                            `json:"atomic_snapshot_verified"`
	OwnerIdentityVerified  bool                            `json:"owner_identity_verified"`
	ChainEvidenceVerified  bool                            `json:"chain_evidence_verified"`
	VerdictBound           bool                            `json:"verdict_bound"`
	TokenAccounts          []HolderTokenAccountObservation `json:"token_accounts"`
	SourceWitnesses        []radar.RawJSONSourceWitness    `json:"source_witnesses"`
	Limitations            []string                        `json:"limitations"`
}

func holderMember(raw []byte, key string) (json.RawMessage, error) {
	value, err := uniqueJSONMember(raw, key)
	if err != nil || string(value) == "null" {
		return nil, fmt.Errorf("missing or ambiguous holder source field %s", key)
	}
	return value, nil
}

func holderString(raw []byte, key string) (string, error) {
	value, err := holderMember(raw, key)
	if err != nil {
		return "", err
	}
	var out string
	if err := json.Unmarshal(value, &out); err != nil || strings.TrimSpace(out) == "" {
		return "", fmt.Errorf("holder source field %s is not a nonempty string", key)
	}
	return out, nil
}

func holderUint(raw []byte, key string, bits int) (uint64, error) {
	value, err := holderMember(raw, key)
	if err != nil {
		return 0, err
	}
	out, err := strconv.ParseUint(string(value), 10, bits)
	if err != nil || strconv.FormatUint(out, 10) != string(value) {
		return 0, fmt.Errorf("holder source field %s is not a canonical integer", key)
	}
	return out, nil
}

func holderAmount(raw []byte) (uint64, error) {
	value, err := holderString(raw, "amount")
	if err != nil {
		return 0, err
	}
	amount, err := strconv.ParseUint(value, 10, 64)
	if err != nil || strconv.FormatUint(amount, 10) != value {
		return 0, errors.New("token amount is not an exact uint64 decimal string")
	}
	return amount, nil
}

func holderSourceValue(raw []byte) (json.RawMessage, error) {
	result, err := holderMember(raw, "result")
	if err != nil {
		return nil, err
	}
	return holderMember(result, "value")
}

func holderSourceCheck(snapshot HolderRawRPCSnapshot) (string, error) {
	if len(snapshot.RawJSON) == 0 || len(snapshot.RawJSON) > radar.RawJSONSourceWitnessMaxBytes ||
		!json.Valid(snapshot.RawJSON) {
		return "", errors.New("holder RPC source JSON is empty, oversized or invalid")
	}
	sum := sha256.Sum256(snapshot.RawJSON)
	actual := hex.EncodeToString(sum[:])
	if len(snapshot.CaptureSHA256) != 64 ||
		subtle.ConstantTimeCompare([]byte(snapshot.CaptureSHA256), []byte(actual)) != 1 {
		return "", errors.New("holder RPC raw bytes differ from captured SHA-256")
	}
	return actual, nil
}

func holderTokenProgram(value string) (string, error) {
	switch value {
	case splTokenProgramID:
		return "spl-token", nil
	case splToken2022ProgramID:
		return "spl-token-2022", nil
	default:
		return "", errors.New("token account executable owner is not a recognized SPL token program")
	}
}

// ReconcileHolderRawSources binds three separately captured Solana RPC sources:
// getTokenSupply, getTokenLargestAccounts and getMultipleAccounts(jsonParsed)
// for exactly the largest-token-account addresses in order. Same RPC slots do
// not prove atomicity. This does not resolve independent owner-wallet evidence.
func ReconcileHolderRawSources(input HolderRawSourceSet) (HolderRawReconciliation, error) {
	if _, err := DecodeSolanaPubkey(input.Mint); err != nil || input.CapturedAt.IsZero() {
		return HolderRawReconciliation{}, errors.New("holder mint or capture timestamp unavailable")
	}
	sources := []struct {
		name string
		data HolderRawRPCSnapshot
	}{
		{"supply", input.Supply},
		{"largest_accounts", input.Largest},
		{"token_accounts", input.Accounts},
	}
	var slots [3]uint64
	witnessInputs := make([]radar.RawJSONObservation, 0, 3)
	for i, source := range sources {
		digest, err := holderSourceCheck(source.data)
		if err != nil {
			return HolderRawReconciliation{}, fmt.Errorf("%s: %w", source.name, err)
		}
		slot, err := accountResponseContextSlot(source.data.RawJSON, 1)
		if err != nil {
			return HolderRawReconciliation{}, fmt.Errorf("%s: %w", source.name, err)
		}
		slots[i] = slot
		witnessInputs = append(witnessInputs, radar.RawJSONObservation{
			Row: radar.EvidenceRow{
				ID:         "solana-holder-" + source.name + "-" + digest,
				Network:    "solana-mainnet",
				Relation:   "holder_" + source.name,
				SubjectID:  input.Mint,
				Status:     "observed",
				Source:     "caller_captured_solana_json_rpc",
				ObservedAt: input.CapturedAt.UTC(),
				Height:     strconv.FormatUint(slot, 10),
				RawHash:    digest,
				Attributes: map[string]any{"method_scope": source.name},
			},
			ResponseJSON: source.data.RawJSON,
		})
	}

	supplyValue, err := holderSourceValue(input.Supply.RawJSON)
	if err != nil {
		return HolderRawReconciliation{}, fmt.Errorf("supply value: %w", err)
	}
	supply, err := holderAmount(supplyValue)
	if err != nil {
		return HolderRawReconciliation{}, err
	}
	decimals, err := holderUint(supplyValue, "decimals", 8)
	if err != nil {
		return HolderRawReconciliation{}, err
	}

	largestValue, err := holderSourceValue(input.Largest.RawJSON)
	if err != nil {
		return HolderRawReconciliation{}, fmt.Errorf("largest accounts: %w", err)
	}
	var largest []json.RawMessage
	if err := json.Unmarshal(largestValue, &largest); err != nil || len(largest) == 0 || len(largest) > 20 {
		return HolderRawReconciliation{}, errors.New("largest token accounts must contain 1 to 20 entries")
	}
	accountValue, err := holderSourceValue(input.Accounts.RawJSON)
	if err != nil {
		return HolderRawReconciliation{}, fmt.Errorf("token account values: %w", err)
	}
	var accountItems []json.RawMessage
	if err := json.Unmarshal(accountValue, &accountItems); err != nil || len(accountItems) != len(largest) ||
		len(input.RequestedTokenAccounts) != len(largest) {
		return HolderRawReconciliation{}, errors.New("token account response count or requested address list differs from largest accounts")
	}

	accounts := make([]HolderTokenAccountObservation, 0, len(largest))
	var topAmountSum big.Int
	seen := make(map[string]bool, len(largest))
	for i, item := range largest {
		address, err := holderString(item, "address")
		if err != nil {
			return HolderRawReconciliation{}, err
		}
		if _, err := DecodeSolanaPubkey(address); err != nil || seen[address] {
			return HolderRawReconciliation{}, errors.New("duplicate or invalid largest token account address")
		}
		seen[address] = true
		if input.RequestedTokenAccounts[i] != address {
			return HolderRawReconciliation{}, errors.New("requested token account order is not bound to largest account response")
		}
		largestAmount, err := holderAmount(item)
		if err != nil {
			return HolderRawReconciliation{}, err
		}
		topAmountSum.Add(&topAmountSum, new(big.Int).SetUint64(largestAmount))
		largestDecimals, err := holderUint(item, "decimals", 8)
		if err != nil || largestDecimals != decimals {
			return HolderRawReconciliation{}, errors.New("largest token account decimals disagree with supply")
		}

		rawAccount := accountItems[i]
		if string(rawAccount) == "null" {
			return HolderRawReconciliation{}, errors.New("token account value missing")
		}
		programID, err := holderString(rawAccount, "owner")
		if err != nil {
			return HolderRawReconciliation{}, err
		}
		expectedParser, err := holderTokenProgram(programID)
		if err != nil {
			return HolderRawReconciliation{}, err
		}
		data, err := holderMember(rawAccount, "data")
		if err != nil {
			return HolderRawReconciliation{}, err
		}
		program, err := holderString(data, "program")
		if err != nil || program != expectedParser {
			return HolderRawReconciliation{}, errors.New("jsonParsed token program does not match executable owner")
		}
		parsed, err := holderMember(data, "parsed")
		if err != nil {
			return HolderRawReconciliation{}, err
		}
		kind, err := holderString(parsed, "type")
		if err != nil || kind != "account" {
			return HolderRawReconciliation{}, errors.New("account is not parsed SPL token account data")
		}
		info, err := holderMember(parsed, "info")
		if err != nil {
			return HolderRawReconciliation{}, err
		}
		mint, err := holderString(info, "mint")
		if err != nil || mint != input.Mint {
			return HolderRawReconciliation{}, errors.New("token account parsed mint does not match requested mint")
		}
		owner, err := holderString(info, "owner")
		if err != nil {
			return HolderRawReconciliation{}, err
		}
		if _, err := DecodeSolanaPubkey(owner); err != nil {
			return HolderRawReconciliation{}, errors.New("token account owner authority is not a valid Solana pubkey")
		}
		tokenAmount, err := holderMember(info, "tokenAmount")
		if err != nil {
			return HolderRawReconciliation{}, err
		}
		accountAmount, err := holderAmount(tokenAmount)
		if err != nil || accountAmount != largestAmount {
			return HolderRawReconciliation{}, errors.New("token balance changed or conflicts across separate RPC reads")
		}
		accountDecimals, err := holderUint(tokenAmount, "decimals", 8)
		if err != nil || accountDecimals != decimals {
			return HolderRawReconciliation{}, errors.New("token account decimals disagree with supply")
		}
		accounts = append(accounts, HolderTokenAccountObservation{
			TokenAccount: address, OwnerAuthority: owner, TokenProgram: programID,
			RawAmount: strconv.FormatUint(accountAmount, 10),
		})
	}

	if topAmountSum.Cmp(new(big.Int).SetUint64(supply)) > 0 {
		return HolderRawReconciliation{}, errors.New("largest account amount sum exceeds separately observed token supply; non-atomic source discrepancy")
	}

	subject := radar.SubjectIdentity{
		Network: "solana-mainnet", NativeID: input.Mint, ChainIdentity: "solana:mainnet-beta",
		IdentityKind: "mint", IdentityValue: input.Mint,
	}
	witnesses, err := radar.BindRawJSONSourceWitnesses(subject, witnessInputs)
	if err != nil {
		return HolderRawReconciliation{}, err
	}
	return HolderRawReconciliation{
		Status:           holderSourceStatus,
		Mint:             input.Mint,
		RawSupplyAmount:  strconv.FormatUint(supply, 10),
		RawTopAccountSum: topAmountSum.String(),
		Decimals:         uint8(decimals),
		SupplySlot:       slots[0], LargestAccountsSlot: slots[1], TokenAccountsSlot: slots[2],
		SameReportedSlot:       slots[0] == slots[1] && slots[1] == slots[2],
		AtomicSnapshotVerified: false, OwnerIdentityVerified: false,
		ChainEvidenceVerified: false, VerdictBound: false,
		TokenAccounts:   accounts,
		SourceWitnesses: witnesses,
		Limitations: []string{
			"Three separate RPC replies are not an atomic snapshot, even with equal context.slot.",
			"Source SHA-256 binds caller-captured response bytes but does not authenticate the provider or chain consensus.",
			"Parsed token owner is an authority pubkey, not verified economic ownership, entity identity or a wallet-only classification.",
			"Reconciliation is limited to returned largest token accounts and cannot prove whole supply or holder distribution.",
		},
	}, nil
}
