package evm

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/crypto/sha3"
	r "koschei/api/internal/radaradapter"
	"math/big"
	"strconv"
	"strings"
)

var Chains = map[string]uint64{"ethereum-mainnet": 1, "base-mainnet": 8453, "optimism-mainnet": 10, "arbitrum-mainnet": 42161, "polygon-mainnet": 137, "bnb-mainnet": 56, "avalanche-mainnet": 43114, "linea-mainnet": 59144, "scroll-mainnet": 534352, "zksync-mainnet": 324, "monad-mainnet": 143, "berachain-mainnet": 80094, "sei-evm-mainnet": 1329, "hyperevm-mainnet": 999}

const ImplementationSlot = "0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc"
const AdminSlot = "0xb53127684a568b3173ae13b9f8a6016e243e63b6e8ee1178d6a717850b5d6103"
const BeaconSlot = "0xa3f0ad74e5423aebfd80d3ef4346578335a9a72aeaee59ff6cb3582b35133d50"

func Keccak(b []byte) string {
	h := sha3.NewLegacyKeccak256()
	h.Write(b)
	return hex.EncodeToString(h.Sum(nil))
}
func Address(s string) (string, error) {
	if len(s) != 42 || !strings.HasPrefix(s, "0x") {
		return "", errors.New("EVM address must contain 20 bytes")
	}
	raw := s[2:]
	if _, e := hex.DecodeString(raw); e != nil {
		return "", errors.New("invalid address hex")
	}
	lower := strings.ToLower(raw)
	if raw != lower && raw != strings.ToUpper(raw) {
		hash := Keccak([]byte(lower))
		for i, c := range raw {
			if c >= 'A' && c <= 'F' || c >= 'a' && c <= 'f' {
				n, _ := strconv.ParseUint(hash[i:i+1], 16, 4)
				upper := c >= 'A' && c <= 'F'
				if upper != (n >= 8) {
					return "", errors.New("invalid EIP-55 checksum")
				}
			}
		}
	}
	return "0x" + lower, nil
}
func word(s string) ([]byte, error) {
	b, e := hex.DecodeString(strings.TrimPrefix(s, "0x"))
	if e != nil || len(b) != 32 {
		return nil, errors.New("invalid ABI word")
	}
	return b, nil
}
func addressWord(s string) (string, error) {
	b, e := word(s)
	if e != nil {
		return "", e
	}
	for _, v := range b[:12] {
		if v != 0 {
			return "", errors.New("noncanonical address word")
		}
	}
	return "0x" + hex.EncodeToString(b[12:]), nil
}
func isZero(s string) bool     { return strings.Trim(s, "0x") == "" }
func selector(s string) string { return "0x" + Keccak([]byte(s))[:8] }

type Adapter struct {
	target    r.Target
	subject   r.SubjectIdentity
	transport r.Transport
	provider  string
}

func New(t r.Target, tr r.Transport) (r.Adapter, error) {
	chain, ok := Chains[t.Network]
	if !ok {
		return nil, errors.New("unregistered EVM chain")
	}
	id := t.Raw
	if t.Kind == "transaction" {
		if len(id) != 66 || !strings.HasPrefix(id, "0x") {
			return nil, errors.New("invalid transaction hash")
		}
		if _, e := hex.DecodeString(id[2:]); e != nil {
			return nil, e
		}
		id = strings.ToLower(id)
	} else {
		var e error
		id, e = Address(id)
		if e != nil {
			return nil, e
		}
	}
	return &Adapter{target: t, subject: r.SubjectIdentity{Network: t.Network, NativeID: id, ChainIdentity: "eip155:" + strconv.FormatUint(chain, 10)}, transport: tr, provider: "primary"}, nil
}
func (a *Adapter) Supports(t r.Target) bool {
	_, ok := Chains[t.Network]
	return ok && (t.Kind == "address" || t.Kind == "wallet" || t.Kind == "contract" || t.Kind == "token" || t.Kind == "transaction" || t.Kind == "governance")
}
func (a *Adapter) Subject() r.SubjectIdentity { return r.CollectorIdentity(a.subject) }
func (a *Adapter) Capability() r.Capability {
	return r.Capability{ID: "evm-account-model", Network: a.target.Network, Family: "evm", TargetClasses: []string{"address", "wallet", "contract", "token", "transaction", "governance"}, Sources: []string{"JSON-RPC block-hash-pinned state", "EIP-1967/7702", "bounded logs and allowance calls", "EIP-1967 control/implementation-code snapshot comparisons"}, ArchivalRequirements: []string{"Control history needs block-hash archive reads (up to 512 blocks); equal endpoints do not exclude intermediate upgrades. Traces require debug support; bounded logs cannot enumerate all approvals."}, Fixture: "evm/evm_test.go", EvidenceOnly: true}
}
func (a *Adapter) rpc(ctx context.Context, m string, p any) (r.Response, json.RawMessage, error) {
	return r.RPC(ctx, a.transport, a.target.Network, a.provider, m, p)
}
func (a *Adapter) str(ctx context.Context, m string, p any) (r.Response, string, error) {
	res, raw, e := a.rpc(ctx, m, p)
	var s string
	if e == nil {
		e = json.Unmarshal(raw, &s)
	}
	return res, s, e
}
func (a *Adapter) Collect(ctx context.Context, t r.Target) ([]r.EvidenceRow, []r.Limitation, error) {
	rows, limits, e := a.collect(ctx, t)
	if e != nil {
		a.provider = "secondary"
		return a.collect(ctx, t)
	}
	return rows, limits, nil
}
func (a *Adapter) collect(ctx context.Context, t r.Target) ([]r.EvidenceRow, []r.Limitation, error) {
	rows := []r.EvidenceRow{}
	limits := []r.Limitation{{Code: "evm_observe_only", Reason: "EVM grading is not promoted. Delegation, approvals and admin interfaces are observations, not proof of malicious intent or effective authority."}}
	_, chain, e := a.str(ctx, "eth_chainId", []any{})
	expected := fmt.Sprintf("0x%x", Chains[t.Network])
	if e != nil || chain != expected {
		return rows, append(limits, r.Limitation{Code: "chain_identity_unknown", Reason: "Provider chain ID did not match configuration."}), errors.New("chain mismatch")
	}
	if t.Kind == "transaction" {
		return a.transaction(ctx, limits)
	}
	headRes, headRaw, e := a.rpc(ctx, "eth_getBlockByNumber", []any{"latest", false})
	if e != nil {
		return rows, limits, e
	}
	block, height, e := parseBlockIdentity(headRaw)
	if e != nil {
		return rows, limits, e
	}
	anchor := map[string]any{"blockHash": block.Hash, "requireCanonical": true}
	add := func(relation string, res r.Response, facts map[string]any) {
		facts["block_hash"] = block.Hash
		facts["block_response_sha256"] = r.Digest(headRes.Raw)
		rows = append(rows, r.NewRow(a.subject, relation, res, block.Number, facts))
	}
	res, code, e := a.str(ctx, "eth_getCode", []any{a.subject.NativeID, anchor})
	if e != nil {
		return rows, append(limits, r.Limitation{Code: "pinned_state_unavailable", Reason: "Provider must support canonical block-hash state reads; latest-state substitution is forbidden."}), e
	}
	b, e := hex.DecodeString(strings.TrimPrefix(code, "0x"))
	if e != nil || !strings.HasPrefix(code, "0x") {
		return rows, limits, errors.New("malformed account code")
	}
	add("evm-code", res, map[string]any{"code_keccak256": Keccak(b), "code_bytes": len(b)})
	if len(b) == 23 && hex.EncodeToString(b[:3]) == "ef0100" {
		delegate := "0x" + hex.EncodeToString(b[3:])
		facts := map[string]any{"delegate": delegate, "authorization_chain_id": "unknown_from_state", "known_sweeper_match": "unknown_registry_not_configured"}
		delegateRes, delegateCode, err := a.str(ctx, "eth_getCode", []any{delegate, anchor})
		if err == nil {
			db, de := hex.DecodeString(strings.TrimPrefix(delegateCode, "0x"))
			if de == nil {
				facts["delegate_code_keccak256"] = Keccak(db)
				if known, ok := a.transport.(interface{ KnownSweeper(string) (string, bool) }); ok {
					source, match := known.KnownSweeper(Keccak(db))
					facts["known_sweeper_match"] = match
					if match {
						facts["sweeper_registry_source"] = source
					}
				}

				facts["delegate_response_sha256"] = r.Digest(delegateRes.Raw)
			}
		}
		add("evm-7702-delegation", res, facts)
	}
	if len(b) > 0 && !(len(b) == 23 && hex.EncodeToString(b[:3]) == "ef0100") {
		currentControls := map[string]controlSnapshot{}
		for _, slot := range []struct{ name, key string }{{"implementation", ImplementationSlot}, {"admin", AdminSlot}, {"beacon", BeaconSlot}} {
			storage, value, err := a.str(ctx, "eth_getStorageAt", []any{a.subject.NativeID, slot.key, anchor})
			if err != nil {
				limits = append(limits, r.Limitation{Code: "proxy_slot_unknown", Reason: slot.name + " storage read failed"})
				continue
			}
			addr, err := addressWord(value)
			if err != nil {
				limits = append(limits, r.Limitation{Code: "proxy_slot_invalid", Reason: slot.name})
				continue
			}
			currentControls[slot.name] = controlSnapshot{addr, storage}
			add("evm-proxy-"+slot.name, storage, map[string]any{"slot": slot.key, "address": addr})
			if !isZero(addr) && slot.name == "admin" {
				a.authority(ctx, addr, anchor, add, &limits)
			}
			if !isZero(addr) && slot.name == "beacon" {
				br, bv, be := a.str(ctx, "eth_call", []any{map[string]any{"to": addr, "data": selector("implementation()")}, anchor})
				if be == nil {
					impl, ie := addressWord(bv)
					if ie == nil {
						add("evm-beacon-implementation", br, map[string]any{"beacon": addr, "implementation": impl})
					}
				}
			}
			if !isZero(addr) && slot.name == "implementation" {
				codeRes, implementationCode, codeErr := a.str(ctx, "eth_getCode", []any{addr, anchor})
				if codeErr == nil {
					if raw, err := hex.DecodeString(strings.TrimPrefix(implementationCode, "0x")); err == nil {
						add("evm-implementation-code", codeRes, map[string]any{"implementation": addr, "code_keccak256": Keccak(raw), "code_bytes": len(raw)})
					}
				}
				ur, uv, ue := a.str(ctx, "eth_call", []any{map[string]any{"to": addr, "data": selector("proxiableUUID()")}, anchor})
				if ue == nil && uv == ImplementationSlot {
					add("evm-uups-interface", ur, map[string]any{"implementation": addr, "proxiable_uuid": uv, "effective_upgrade_authority": "unknown"})
				}
			}
		}
		a.controlHistory(ctx, block, height, currentControls, add, &limits)
		a.authority(ctx, a.subject.NativeID, anchor, add, &limits)
		diamond, dv, de := a.str(ctx, "eth_call", []any{map[string]any{"to": a.subject.NativeID, "data": selector("facetAddresses()")}, anchor})
		if de == nil {
			if addresses, err := addressArray(dv); err == nil {
				add("evm-diamond-interface", diamond, map[string]any{"facets": addresses, "interface_only": true})
			}
		}
		if t.Kind == "token" {
			for _, method := range []string{"totalSupply()", "decimals()"} {
				cr, cv, ce := a.str(ctx, "eth_call", []any{map[string]any{"to": a.subject.NativeID, "data": selector(method)}, anchor})
				if ce == nil {
					v, we := word(cv)
					if we == nil {
						add("evm-token-"+strings.TrimSuffix(method, "()"), cr, map[string]any{"value": new(big.Int).SetBytes(v).String()})
					}
				}
			}
		}
	}
	from := uint64(0)
	if height > 512 {
		from = height - 512
	}
	approvalTopic := "0x" + Keccak([]byte("Approval(address,address,uint256)"))
	ownerTopic := "0x" + strings.Repeat("0", 24) + a.subject.NativeID[2:]
	logsRes, logsRaw, err := a.rpc(ctx, "eth_getLogs", []any{map[string]any{"fromBlock": fmt.Sprintf("0x%x", from), "toBlock": block.Number, "topics": []any{approvalTopic, ownerTopic}}})
	if err == nil {
		var logs []struct {
			Address   string   `json:"address"`
			Topics    []string `json:"topics"`
			Removed   bool     `json:"removed"`
			Tx        string   `json:"transactionHash"`
			BlockHash string   `json:"blockHash"`
		}
		if json.Unmarshal(logsRaw, &logs) == nil {
			seen := map[string]bool{}
			for _, log := range logs {
				if log.Removed || len(log.Topics) != 3 || log.Topics[0] != approvalTopic || log.Topics[1] != ownerTopic {
					continue
				}
				spender, ee := addressWord(log.Topics[2])
				token, te := Address(log.Address)
				key := token + spender
				if ee != nil || te != nil || seen[key] || len(seen) >= 12 {
					continue
				}
				seen[key] = true
				allowance, av, ae := a.str(ctx, "eth_call", []any{map[string]any{"to": token, "data": selector("allowance(address,address)") + ownerTopic[2:] + strings.Repeat("0", 24) + spender[2:]}, anchor})
				if ae != nil {
					continue
				}
				v, ve := word(av)
				if ve != nil {
					continue
				}
				amount := new(big.Int).SetBytes(v)
				add("evm-allowance", allowance, map[string]any{"token": token, "spender": spender, "allowance": amount.String(), "unlimited": strings.TrimLeft(hex.EncodeToString(v), "f") == "", "discovery_response_sha256": r.Digest(logsRes.Raw), "discovery_transaction": log.Tx})
			}
		}
	} else {
		limits = append(limits, r.Limitation{Code: "approval_discovery_unavailable", Reason: "Approval log query unavailable; no zero-exposure conclusion."})
	}
	a.extraPermissions(ctx, from, block.Number, anchor, add, &limits)
	events, eventsRaw, ee := a.rpc(ctx, "eth_getLogs", []any{map[string]any{"address": a.subject.NativeID, "fromBlock": fmt.Sprintf("0x%x", from), "toBlock": block.Number, "topics": []any{[]string{"0x" + Keccak([]byte("ChangedThreshold(uint256)")), "0x" + Keccak([]byte("AddedOwner(address)")), "0x" + Keccak([]byte("RemovedOwner(address)")), "0x" + Keccak([]byte("CallScheduled(bytes32,uint256,address,uint256,bytes,bytes32,uint256)")), "0x" + Keccak([]byte("CallExecuted(bytes32,uint256,address,uint256,bytes)"))}}}})
	if ee == nil {
		var logs []json.RawMessage
		if json.Unmarshal(eventsRaw, &logs) == nil {
			add("evm-governance-events", events, map[string]any{"logs": logs, "removed_state_preserved": true, "from_block": from, "bypass_conclusion": "unknown_without_complete_history"})
		}
	}
	limits = append(limits, r.Limitation{Code: "bounded_approval_window", Reason: "Only the last 512 blocks and at most 12 distinct ERC-20 pairs are checked. Older grants, token-specific NFT approvals and revoked/reorged logs need indexed reconciliation; empty logs are not zero exposure."})
	return rows, limits, nil
}
func addressArray(s string) ([]string, error) {
	b, e := hex.DecodeString(strings.TrimPrefix(s, "0x"))
	if e != nil || len(b) < 64 || new(big.Int).SetBytes(b[:32]).Cmp(big.NewInt(32)) != 0 {
		return nil, errors.New("invalid dynamic ABI array")
	}
	n := new(big.Int).SetBytes(b[32:64])
	if !n.IsUint64() || n.Uint64() > 64 || len(b) != 64+32*int(n.Uint64()) {
		return nil, errors.New("invalid ABI array length")
	}
	out := []string{}
	for i := 64; i < len(b); i += 32 {
		a, e := addressWord("0x" + hex.EncodeToString(b[i:i+32]))
		if e != nil {
			return nil, e
		}
		out = append(out, a)
	}
	return out, nil
}
func (a *Adapter) authority(ctx context.Context, admin string, anchor any, add func(string, r.Response, map[string]any), limits *[]r.Limitation) {
	res, code, e := a.str(ctx, "eth_getCode", []any{admin, anchor})
	if e != nil {
		*limits = append(*limits, r.Limitation{Code: "admin_code_unknown", Reason: "Admin code unavailable"})
		return
	}
	if code == "0x" {
		add("evm-admin-code-empty", res, map[string]any{"admin": admin, "classification": "no_code_observed", "key_ownership": "unknown"})
		return
	}
	ownerRes, ownerValue, ownerErr := a.str(ctx, "eth_call", []any{map[string]any{"to": admin, "data": selector("owner()")}, anchor})
	if ownerErr == nil {
		if owner, e := addressWord(ownerValue); e == nil {
			add("evm-owner-interface", ownerRes, map[string]any{"contract": admin, "owner": owner, "effective_role_authority": "not_established_by_interface"})
		}
	}
	threshold, tv, te := a.str(ctx, "eth_call", []any{map[string]any{"to": admin, "data": selector("getThreshold()")}, anchor})
	owners, ov, oe := a.str(ctx, "eth_call", []any{map[string]any{"to": admin, "data": selector("getOwners()")}, anchor})
	if te == nil && oe == nil {
		tw, err := word(tv)
		addresses, err2 := addressArray(ov)
		if err == nil && err2 == nil {
			n := new(big.Int).SetBytes(tw)
			if n.Sign() > 0 && n.Cmp(big.NewInt(int64(len(addresses)))) <= 0 {
				add("evm-safe-interface", threshold, map[string]any{"admin": admin, "threshold": n.String(), "owners": addresses, "owners_response_sha256": r.Digest(owners.Raw), "classification": "safe_compatible_interface_unverified_implementation"})
			}
		}
	}
	delay, dv, de := a.str(ctx, "eth_call", []any{map[string]any{"to": admin, "data": selector("getMinDelay()")}, anchor})
	if de == nil {
		v, e := word(dv)
		if e == nil {
			add("evm-timelock-interface", delay, map[string]any{"admin": admin, "min_delay_seconds": new(big.Int).SetBytes(v).String(), "all_privileged_paths_timelocked": "unknown"})
		}
	}
}
func (a *Adapter) transaction(ctx context.Context, limits []r.Limitation) ([]r.EvidenceRow, []r.Limitation, error) {
	tx, tr, e := a.rpc(ctx, "eth_getTransactionByHash", []any{a.subject.NativeID})
	if e != nil {
		return nil, limits, e
	}
	receipt, rr, e := a.rpc(ctx, "eth_getTransactionReceipt", []any{a.subject.NativeID})
	if e != nil {
		return nil, append(limits, r.Limitation{Code: "receipt_missing", Reason: "Pending or unavailable receipt is not settled."}), e
	}
	var t struct {
		Hash          string `json:"hash"`
		BlockHash     string `json:"blockHash"`
		BlockNumber   string `json:"blockNumber"`
		Authorization []struct {
			ChainID string `json:"chainId"`
			Address string `json:"address"`
		} `json:"authorizationList"`
	}
	var rec struct {
		Hash        string `json:"transactionHash"`
		BlockHash   string `json:"blockHash"`
		BlockNumber string `json:"blockNumber"`
	}
	if json.Unmarshal(tr, &t) != nil || json.Unmarshal(rr, &rec) != nil || t.Hash != a.subject.NativeID || rec.Hash != t.Hash || len(t.BlockHash) != 66 || rec.BlockHash != t.BlockHash || t.BlockNumber == "" || rec.BlockNumber != t.BlockNumber {
		return nil, limits, errors.New("transaction receipt identity mismatch")
	}
	row := r.NewRow(a.subject, "evm-transaction-receipt", receipt, t.BlockNumber, map[string]any{"transaction": tr, "receipt": rr, "transaction_response_sha256": r.Digest(tx.Raw), "block_hash": t.BlockHash, "finality": "not_established"})
	rows := []r.EvidenceRow{row}
	for _, auth := range t.Authorization {
		if auth.ChainID == "0x0" {
			rows = append(rows, r.NewRow(a.subject, "evm-7702-chain-zero", tx, t.BlockNumber, map[string]any{"delegate": auth.Address, "scope": "authorization_in_transaction", "signature_recovery": "not_performed"}))
		}
	}
	return rows, limits, nil
}
