package solana

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	radar "koschei/api/internal/radaradapter"
)

const (
	holderSPLTokenProgramID  = "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"
	holderToken2022ProgramID = "TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb"
)

// HolderRawSourceSet takes the actual bytes delivered by an existing trusted
// RPC transport. RequestedTokenAccounts must be the original ordered request
// arguments; getMultipleAccounts does not repeat account addresses in results.
type HolderRawSourceSet struct {
	Network                string
	Mint                   string
	SupplyResponse         []byte
	LargestResponse        []byte
	AccountsResponse       []byte
	RequestedTokenAccounts []string
}

// HolderRawAccount separates SPL token-account pubkeys, token-program owners,
// and the controlling wallet/pubkey encoded inside the token account.
type HolderRawAccount struct {
	TokenAccount      string `json:"token_account"`
	OwnerWallet       string `json:"owner_wallet"`
	TokenProgram      string `json:"token_program"`
	LargestAmountRaw  string `json:"largest_amount_raw"`
	ResolvedAmountRaw string `json:"resolved_amount_raw"`
}

// HolderRawConsistency is a read-only source-consistency projection, never a
// verified chain fact, complete holder census, risk decision or ARVIS verdict.
type HolderRawConsistency struct {
	Status                 string             `json:"status"`
	Network                string             `json:"network"`
	Mint                   string             `json:"mint"`
	SupplySlot             uint64             `json:"supply_slot"`
	LargestSlot            uint64             `json:"largest_slot"`
	AccountsSlot           uint64             `json:"accounts_slot"`
	SameReportedSlot       bool               `json:"same_reported_slot"`
	SupplyAmountRaw        string             `json:"supply_amount_raw"`
	LargestAccountsSumRaw  string             `json:"largest_accounts_sum_raw"`
	Decimals               uint8              `json:"decimals"`
	SupplyResponseSHA256   string             `json:"supply_response_sha256"`
	LargestResponseSHA256  string             `json:"largest_response_sha256"`
	AccountsResponseSHA256 string             `json:"accounts_response_sha256"`
	Accounts               []HolderRawAccount `json:"accounts"`
	ChainEvidenceVerified  bool               `json:"chain_evidence_verified"`
	VerdictBound           bool               `json:"verdict_bound"`
	AtomicSnapshotProven   bool               `json:"atomic_snapshot_proven"`
}

func holderSourceField(raw []byte, keys ...string) (json.RawMessage, error) {
	var err error
	for _, key := range keys {
		raw, err = uniqueJSONMember(raw, key)
		if err != nil {
			return nil, fmt.Errorf("source %s: %w", key, err)
		}
	}
	return raw, nil
}

func holderRawText(raw []byte, keys ...string) (string, error) {
	b, err := holderSourceField(raw, keys...)
	if err != nil {
		return "", err
	}
	var value string
	if err := json.Unmarshal(b, &value); err != nil || strings.TrimSpace(value) == "" {
		return "", errors.New("source text is missing or invalid")
	}
	return value, nil
}

func holderDecimal(s string) (*big.Int, error) {
	if len(s) == 0 || len(s) > 40 || (len(s) > 1 && s[0] == '0') {
		return nil, errors.New("noncanonical token amount")
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return nil, errors.New("invalid token amount digits")
		}
	}
	number, ok := new(big.Int).SetString(s, 10)
	if !ok || number.BitLen() > 64 {
		return nil, errors.New("raw token amount exceeds uint64")
	}
	return number, nil
}

func holderRawAmount(raw []byte, keys ...string) (string, *big.Int, error) {
	value, err := holderRawText(raw, keys...)
	if err != nil {
		return "", nil, err
	}
	number, err := holderDecimal(value)
	return value, number, err
}

func holderDecimals(raw []byte) (uint8, error) {
	number, err := uniqueJSONMember(raw, "decimals")
	if err != nil {
		return 0, err
	}
	value, err := strconv.ParseUint(string(number), 10, 8)
	if err != nil || strconv.FormatUint(value, 10) != string(number) {
		return 0, errors.New("token decimals must be a canonical uint8")
	}
	return uint8(value), nil
}

func holderArray(raw []byte, keys ...string) ([]json.RawMessage, error) {
	value, err := holderSourceField(raw, keys...)
	if err != nil {
		return nil, err
	}
	var items []json.RawMessage
	if err := json.Unmarshal(value, &items); err != nil || items == nil {
		return nil, errors.New("source array is missing or invalid")
	}
	return items, nil
}

func holderValidatePubkey(value string) error {
	if _, err := DecodeSolanaPubkey(value); err != nil {
		return errors.New("invalid Solana public key")
	}
	return nil
}

func holderHash(raw []byte) string {
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

func holderValidateSource(raw []byte) error {
	if len(raw) == 0 || len(raw) > radar.RawJSONSourceWitnessMaxBytes || !json.Valid(raw) {
		return errors.New("holder RPC response bytes are missing, oversized or invalid")
	}
	if _, err := uniqueJSONMember(raw, "result"); err != nil {
		return err
	}
	// A valid-looking result cannot override a simultaneous RPC error.
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return errors.New("RPC response envelope invalid")
	}
	if reportedError, present := envelope["error"]; present && string(reportedError) != "null" {
		return errors.New("RPC provider reported an error")
	}
	return nil
}

// CheckHolderRawSources does not fetch anything or mutate ARVIS, Sentinel,
// security-alert storage or holder grades. Each raw buffer must be supplied by
// the real RPC transport, not reconstructed from decoded values. Same reported
// slots are not independent evidence of atomic reads or canonical consensus.
func CheckHolderRawSources(in HolderRawSourceSet) (HolderRawConsistency, error) {
	if in.Network != "solana-mainnet" {
		return HolderRawConsistency{}, errors.New("holder source requires explicit supported Solana mainnet identity")
	}
	if err := holderValidatePubkey(in.Mint); err != nil {
		return HolderRawConsistency{}, err
	}
	for _, raw := range [][]byte{in.SupplyResponse, in.LargestResponse, in.AccountsResponse} {
		if err := holderValidateSource(raw); err != nil {
			return HolderRawConsistency{}, err
		}
	}
	supplySlot, err := accountResponseContextSlot(in.SupplyResponse, 1)
	if err != nil {
		return HolderRawConsistency{}, fmt.Errorf("supply context: %w", err)
	}
	largestSlot, err := accountResponseContextSlot(in.LargestResponse, 1)
	if err != nil {
		return HolderRawConsistency{}, fmt.Errorf("largest context: %w", err)
	}
	accountsSlot, err := accountResponseContextSlot(in.AccountsResponse, 1)
	if err != nil {
		return HolderRawConsistency{}, fmt.Errorf("account context: %w", err)
	}
	supplyValue, err := holderSourceField(in.SupplyResponse, "result", "value")
	if err != nil {
		return HolderRawConsistency{}, err
	}
	supplyText, supply, err := holderRawAmount(supplyValue, "amount")
	if err != nil {
		return HolderRawConsistency{}, err
	}
	decimals, err := holderDecimals(supplyValue)
	if err != nil {
		return HolderRawConsistency{}, err
	}
	largest, err := holderArray(in.LargestResponse, "result", "value")
	if err != nil {
		return HolderRawConsistency{}, err
	}
	resolved, err := holderArray(in.AccountsResponse, "result", "value")
	if err != nil {
		return HolderRawConsistency{}, err
	}
	if len(largest) == 0 || len(largest) > 20 ||
		len(in.RequestedTokenAccounts) != len(largest) || len(resolved) != len(largest) {
		return HolderRawConsistency{}, errors.New("bounded largest-account and ordered getMultipleAccounts results must match")
	}

	accounts := make([]HolderRawAccount, 0, len(largest))
	seen := make(map[string]bool, len(largest))
	sum := new(big.Int)
	previous := new(big.Int).SetUint64(^uint64(0))
	sameSlot := supplySlot == largestSlot && largestSlot == accountsSlot
	for i, item := range largest {
		address, err := holderRawText(item, "address")
		if err != nil {
			return HolderRawConsistency{}, err
		}
		if err := holderValidatePubkey(address); err != nil {
			return HolderRawConsistency{}, err
		}
		if seen[address] || in.RequestedTokenAccounts[i] != address {
			return HolderRawConsistency{}, errors.New("token account order, identity or uniqueness mismatch")
		}
		seen[address] = true
		largestText, largestNumber, err := holderRawAmount(item, "amount")
		if err != nil {
			return HolderRawConsistency{}, err
		}
		accountDecimals, err := holderDecimals(item)
		if err != nil || accountDecimals != decimals {
			return HolderRawConsistency{}, errors.New("largest account decimals differ from supply")
		}
		if largestNumber.Cmp(previous) > 0 {
			return HolderRawConsistency{}, errors.New("largest token accounts not descending")
		}
		previous = largestNumber
		sum.Add(sum, largestNumber)
		if string(resolved[i]) == "null" {
			return HolderRawConsistency{}, errors.New("requested token account is missing")
		}
		tokenProgram, err := holderRawText(resolved[i], "owner")
		if err != nil {
			return HolderRawConsistency{}, err
		}
		expectedParser := ""
		switch tokenProgram {
		case holderSPLTokenProgramID:
			expectedParser = "spl-token"
		case holderToken2022ProgramID:
			expectedParser = "spl-token-2022"
		default:
			return HolderRawConsistency{}, errors.New("account is not owned by supported token program")
		}
		parser, err := holderRawText(resolved[i], "data", "program")
		if err != nil || parser != expectedParser {
			return HolderRawConsistency{}, errors.New("account parser program does not match chain program")
		}
		kind, err := holderRawText(resolved[i], "data", "parsed", "type")
		if err != nil || kind != "account" {
			return HolderRawConsistency{}, errors.New("source is not a parsed token account")
		}
		info, err := holderSourceField(resolved[i], "data", "parsed", "info")
		if err != nil {
			return HolderRawConsistency{}, err
		}
		mint, err := holderRawText(info, "mint")
		if err != nil || mint != in.Mint {
			return HolderRawConsistency{}, errors.New("token account mint differs from requested mint")
		}
		ownerWallet, err := holderRawText(info, "owner")
		if err != nil {
			return HolderRawConsistency{}, err
		}
		if err := holderValidatePubkey(ownerWallet); err != nil {
			return HolderRawConsistency{}, err
		}
		tokenAmount, err := holderSourceField(info, "tokenAmount")
		if err != nil {
			return HolderRawConsistency{}, err
		}
		resolvedText, _, err := holderRawAmount(tokenAmount, "amount")
		if err != nil {
			return HolderRawConsistency{}, err
		}
		resolvedDecimals, err := holderDecimals(tokenAmount)
		if err != nil || resolvedDecimals != decimals {
			return HolderRawConsistency{}, errors.New("resolved token decimals differ from supply")
		}
		if sameSlot && resolvedText != largestText {
			return HolderRawConsistency{}, errors.New("same-reported-slot token amounts conflict")
		}
		accounts = append(accounts, HolderRawAccount{
			TokenAccount: address, OwnerWallet: ownerWallet, TokenProgram: tokenProgram,
			LargestAmountRaw: largestText, ResolvedAmountRaw: resolvedText,
		})
	}
	if sameSlot && sum.Cmp(supply) > 0 {
		return HolderRawConsistency{}, errors.New("same-reported-slot top account balances exceed supply")
	}
	status := "observed_non_atomic_slot_set"
	if sameSlot {
		status = "observed_same_reported_slot_consistent_not_independent_proof"
	}
	return HolderRawConsistency{
		Status: status, Network: in.Network, Mint: in.Mint,
		SupplySlot: supplySlot, LargestSlot: largestSlot, AccountsSlot: accountsSlot, SameReportedSlot: sameSlot,
		SupplyAmountRaw: supplyText, LargestAccountsSumRaw: sum.String(), Decimals: decimals,
		SupplyResponseSHA256: holderHash(in.SupplyResponse), LargestResponseSHA256: holderHash(in.LargestResponse),
		AccountsResponseSHA256: holderHash(in.AccountsResponse), Accounts: accounts,
		ChainEvidenceVerified: false, VerdictBound: false, AtomicSnapshotProven: false,
	}, nil
}
