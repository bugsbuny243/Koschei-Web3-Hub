package ton

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	r "koschei/api/internal/radaradapter"
	"net/url"
	"strconv"
	"strings"
)

type ParsedAddress struct {
	Raw       string
	Workchain int32
	Form      string
	Testnet   bool
}

func CRC16(b []byte) uint16 {
	var crc uint16
	for _, v := range b {
		crc ^= uint16(v) << 8
		for i := 0; i < 8; i++ {
			if crc&0x8000 != 0 {
				crc = (crc << 1) ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}
func Address(s string) (ParsedAddress, error) {
	if strings.Contains(s, ":") {
		parts := strings.Split(s, ":")
		if len(parts) != 2 || len(parts[1]) != 64 {
			return ParsedAddress{}, errors.New("invalid TON raw address")
		}
		wc, e := strconv.ParseInt(parts[0], 10, 32)
		if e != nil || (wc != 0 && wc != -1) {
			return ParsedAddress{}, errors.New("unsupported workchain")
		}
		if _, e = hex.DecodeString(parts[1]); e != nil {
			return ParsedAddress{}, e
		}
		return ParsedAddress{Raw: fmt.Sprintf("%d:%s", wc, strings.ToLower(parts[1])), Workchain: int32(wc), Form: "raw"}, nil
	}
	if len(s) != 48 {
		return ParsedAddress{}, errors.New("TON friendly address must contain 48 characters")
	}
	b, e := base64.RawURLEncoding.DecodeString(s)
	if e != nil {
		b, e = base64.RawStdEncoding.DecodeString(s)
	}
	if e != nil || len(b) != 36 {
		return ParsedAddress{}, errors.New("invalid TON base64")
	}
	crc := CRC16(b[:34])
	if b[34] != byte(crc>>8) || b[35] != byte(crc) {
		return ParsedAddress{}, errors.New("invalid TON CRC16")
	}
	tag := b[0] & 0x7f
	if tag != 0x11 && tag != 0x51 {
		return ParsedAddress{}, errors.New("invalid TON flags")
	}
	wc := int32(int8(b[1]))
	if wc != 0 && wc != -1 {
		return ParsedAddress{}, errors.New("unsupported workchain")
	}
	form := "bounceable"
	if tag == 0x51 {
		form = "non_bounceable"
	}
	return ParsedAddress{Raw: fmt.Sprintf("%d:%s", wc, hex.EncodeToString(b[2:34])), Workchain: wc, Form: form, Testnet: b[0]&0x80 != 0}, nil
}

type Adapter struct {
	target    r.Target
	subject   r.SubjectIdentity
	transport r.Transport
	raw       string
	provider  string
}

func New(t r.Target, tr r.Transport) (r.Adapter, error) {
	a := &Adapter{target: t, subject: r.SubjectIdentity{Network: t.Network, NativeID: t.Raw}, transport: tr, provider: "primary"}
	if t.Kind == "jetton_wallet" {
		parts, e := jettonTarget(t.Raw)
		if e != nil {
			return nil, e
		}
		a.subject.NativeID = strings.Join(parts, "|")
		return a, nil
	}
	if t.Kind == "transaction" {
		if len(t.Raw) != 64 {
			return nil, errors.New("TON transaction requires 32-byte hex hash")
		}
		if _, e := hex.DecodeString(t.Raw); e != nil {
			return nil, e
		}
		a.raw = t.Raw
		return a, nil
	}
	p, e := Address(t.Raw)
	if e != nil {
		return nil, e
	}
	if p.Testnet {
		return nil, errors.New("testnet flag rejected on mainnet")
	}
	a.raw = p.Raw
	a.subject.NativeID = p.Raw
	a.subject.Workchain = &p.Workchain
	a.subject.AddressForm = p.Form
	return a, nil
}
func (a *Adapter) Supports(t r.Target) bool {
	return t.Network == "ton-mainnet" && (t.Kind == "jetton_wallet" || t.Kind == "address" || t.Kind == "wallet" || t.Kind == "contract" || t.Kind == "token" || t.Kind == "transaction")
}
func (a *Adapter) Subject() r.SubjectIdentity { return r.CollectorIdentity(a.subject) }
func (a *Adapter) Capability() r.Capability {
	return r.Capability{ID: "ton-account-message-model", Network: a.target.Network, Family: "ton", TargetClasses: []string{"jetton_wallet", "address", "wallet", "contract", "token", "transaction"}, Sources: []string{"toncenter v2 live state", "toncenter v3 account/jetton/trace index"}, ArchivalRequirements: []string{"Old traces require indexed or archival provider access."}, Fixture: "ton/ton_test.go", EvidenceOnly: true}
}
func (a *Adapter) get(ctx context.Context, path string) (r.Response, error) {
	return a.transport.Do(ctx, r.Request{Network: a.target.Network, Provider: a.provider, Method: "GET", Path: path})
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
	limits := []r.Limitation{}
	info, e := a.get(ctx, "/api/v3/masterchainInfo")
	var mc struct {
		Last struct {
			GlobalID int    `json:"global_id"`
			Seqno    int64  `json:"seqno"`
			Hash     string `json:"root_hash"`
		} `json:"last"`
	}
	if e != nil || r.Decode(info, &mc) != nil || mc.Last.GlobalID != -239 || mc.Last.Seqno <= 0 || mc.Last.Hash == "" {
		return rows, []r.Limitation{{Code: "ton_identity_unknown", Reason: "Mainnet global_id and masterchain identity unavailable."}}, errors.New("TON identity mismatch")
	}
	a.subject.ChainIdentity = "global_id:-239"
	height := strconv.FormatInt(mc.Last.Seqno, 10)
	if t.Kind == "jetton_wallet" {
		return a.jettonWallet(ctx, limits, height)
	}
	if t.Kind == "transaction" {
		traces, e := a.get(ctx, "/api/v3/traces?tx_hash="+url.QueryEscape(a.raw)+"&limit=1&include_actions=true")
		if e != nil {
			return rows, append(limits, r.Limitation{Code: "ton-trace-incomplete", Reason: "Trace unavailable; transfer settlement unknown."}), e
		}
		var payload struct {
			Traces []json.RawMessage `json:"traces"`
		}
		if r.Decode(traces, &payload) != nil || len(payload.Traces) != 1 {
			return rows, []r.Limitation{{Code: "ton-trace-incomplete", Reason: "No unique trace returned."}}, errors.New("trace missing")
		}
		var binding struct {
			Transactions map[string]json.RawMessage `json:"transactions"`
		}
		_ = json.Unmarshal(payload.Traces[0], &binding)
		bound := false
		for key := range binding.Transactions {
			if strings.EqualFold(key, a.raw) {
				bound = true
			}
			b, e := base64.StdEncoding.DecodeString(key)
			if e == nil && hex.EncodeToString(b) == strings.ToLower(a.raw) {
				bound = true
			}
		}
		if !bound {
			return rows, []r.Limitation{{Code: "trace_subject_mismatch", Reason: "Returned trace does not contain requested transaction hash."}}, errors.New("unbound trace")
		}
		state := TraceState(payload.Traces[0])
		rows = append(rows, r.NewRow(a.subject, "ton-trace-state", traces, height, map[string]any{"trace": payload.Traces[0], "state": state, "masterchain_response_sha256": r.Digest(info.Raw)}))
		if state != "complete_no_failure_observed" {
			limits = append(limits, r.Limitation{Code: "ton-trace-incomplete", Reason: "A missing, bounced or aborted downstream transaction prevents settlement."})
		}
		limits = append(limits, r.Limitation{Code: "trace_effect_unverified", Reason: "Trace completeness is separate from token amount, recipient and business intent verification."})
		return rows, limits, nil
	}
	account, e := a.get(ctx, "/api/v2/getAddressInformation?address="+url.QueryEscape(a.raw))
	var state struct {
		OK     bool `json:"ok"`
		Result struct {
			State string `json:"state"`
			Code  string `json:"code"`
			Block struct {
				Seqno int64  `json:"seqno"`
				Hash  string `json:"root_hash"`
			} `json:"block_id"`
		} `json:"result"`
	}
	if e != nil || r.Decode(account, &state) != nil || !state.OK || state.Result.State == "" || state.Result.Block.Seqno <= 0 || state.Result.Block.Hash == "" {
		return rows, []r.Limitation{{Code: "account_unknown", Reason: "No block-bound TON account state."}}, errors.New("account state missing")
	}
	codeHash, codeErr := CodeHash(state.Result.Code)
	if codeErr != nil {
		limits = append(limits, r.Limitation{Code: "code_hash_unavailable", Reason: "Account BOC could not be decoded into a cell hash."})
	}
	rows = append(rows, r.NewRow(a.subject, "ton-account-state", account, strconv.FormatInt(state.Result.Block.Seqno, 10), map[string]any{"raw_address": a.raw, "state": state.Result.State, "code_boc_sha256": r.Digest([]byte(state.Result.Code)), "code_hash": codeHash, "block_root_hash": state.Result.Block.Hash, "masterchain_seqno": height, "masterchain_response_sha256": r.Digest(info.Raw)}))
	indexed, ie := a.get(ctx, "/api/v3/accountStates?address="+url.QueryEscape(a.raw))
	if ie == nil {
		var accounts struct {
			Accounts []struct {
				Address  string `json:"address"`
				CodeHash string `json:"code_hash"`
				Status   string `json:"status"`
			} `json:"accounts"`
		}
		if r.Decode(indexed, &accounts) == nil {
			for _, acc := range accounts.Accounts {
				p, pe := Address(acc.Address)
				if pe == nil && p.Raw == a.raw && acc.CodeHash != "" {
					rows = append(rows, r.NewRow(a.subject, "ton-code-hash", indexed, height, map[string]any{"code_hash": acc.CodeHash, "wallet_version": "unknown_without_canonical_code_registry"}))
				}
			}
		}
	}
	if t.Kind == "token" {
		masters, me := a.get(ctx, "/api/v3/jetton/masters?address="+url.QueryEscape(a.raw)+"&limit=1")
		if me == nil {
			var payload struct {
				Masters []struct {
					Address  string          `json:"address"`
					Admin    *string         `json:"admin_address"`
					Mintable *bool           `json:"mintable"`
					Supply   string          `json:"total_supply"`
					Content  json.RawMessage `json:"jetton_content"`
				} `json:"jetton_masters"`
			}
			if r.Decode(masters, &payload) == nil {
				for _, m := range payload.Masters {
					p, pe := Address(m.Address)
					if pe == nil && p.Raw == a.raw && m.Mintable != nil && m.Supply != "" {
						rows = append(rows, r.NewRow(a.subject, "ton-jetton-master", masters, height, map[string]any{"admin": m.Admin, "mintable": *m.Mintable, "total_supply": m.Supply, "metadata": m.Content}))
					}
				}
			}
		}
	}
	limits = append(limits, r.Limitation{Code: "jetton_balance_unattributed", Reason: "No holder balance is attributed without get_wallet_address derivation and a matching wallet master. Admin presence alone does not imply mintable. Wallet version requires a trusted code-hash registry."})
	return rows, limits, nil
}

// TraceState is intentionally fail-closed on omitted booleans/counts. An external
// message alone and a root transaction alone cannot establish complete effects.
func TraceState(raw []byte) string {
	var t struct {
		Incomplete *bool `json:"is_incomplete"`
		Info       struct {
			Pending      *int   `json:"pending_messages"`
			State        string `json:"trace_state"`
			Transactions *int   `json:"transactions"`
		} `json:"trace_info"`
		Transactions map[string]struct {
			Description struct {
				Aborted *bool `json:"aborted"`
			} `json:"description"`
			InMsg *struct {
				Bounced *bool `json:"bounced"`
			} `json:"in_msg"`
		} `json:"transactions"`
	}
	if json.Unmarshal(raw, &t) != nil || t.Incomplete == nil || *t.Incomplete || t.Info.Pending == nil || *t.Info.Pending != 0 || t.Info.State != "complete" || t.Info.Transactions == nil || *t.Info.Transactions == 0 || len(t.Transactions) != *t.Info.Transactions {
		return "incomplete"
	}
	for _, tx := range t.Transactions {
		if tx.Description.Aborted == nil {
			return "incomplete"
		}
		if *tx.Description.Aborted {
			return "failed"
		}
		if tx.InMsg == nil || tx.InMsg.Bounced == nil {
			return "incomplete"
		}
		if tx.InMsg != nil && tx.InMsg.Bounced != nil && *tx.InMsg.Bounced {
			return "bounced"
		}
	}
	return "complete_no_failure_observed"
}
