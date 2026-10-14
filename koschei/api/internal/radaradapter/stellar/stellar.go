package stellar

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	r "koschei/api/internal/radaradapter"
	"koschei/api/internal/radaradapter/ton"
	"strconv"
	"strings"
)

const Passphrase = "Public Global Stellar Network ; September 2015"

func Address(s string) ([]byte, error) {
	if len(s) != 56 || s[0] != 'G' {
		return nil, errors.New("Stellar account requires G StrKey")
	}
	b, e := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(s)
	if e != nil || len(b) != 35 || b[0] != 48 {
		return nil, errors.New("invalid StrKey")
	}
	crc := ton.CRC16(b[:33])
	if b[33] != byte(crc) || b[34] != byte(crc>>8) {
		return nil, errors.New("invalid StrKey checksum")
	}
	return b[1:33], nil
}
func EncodeAddress(b []byte) string {
	p := append([]byte{48}, b...)
	crc := ton.CRC16(p)
	p = append(p, byte(crc), byte(crc>>8))
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(p)
}
func LedgerKey(target string, kind string) (string, error) {
	parts := strings.Split(target, "|")
	b, e := Address(parts[0])
	if e != nil {
		return "", e
	}
	var key bytes.Buffer
	put := func(n uint32) { binary.Write(&key, binary.BigEndian, n) }
	if kind == "trustline" {
		if len(parts) != 3 || len(parts[1]) < 1 || len(parts[1]) > 12 {
			return "", errors.New("trustline requires holder|assetCode|issuer")
		}
		issuer, e := Address(parts[2])
		if e != nil {
			return "", e
		}
		put(1)
		put(0)
		key.Write(b)
		n := 4
		assetType := uint32(1)
		if len(parts[1]) > 4 {
			n = 12
			assetType = 2
		}
		put(assetType)
		code := make([]byte, n)
		copy(code, parts[1])
		key.Write(code)
		put(0)
		key.Write(issuer)
	} else {
		if len(parts) != 1 {
			return "", errors.New("invalid account target")
		}
		put(0)
		put(0)
		key.Write(b)
	}
	return base64.StdEncoding.EncodeToString(key.Bytes()), nil
}

type Adapter struct {
	target        r.Target
	subject       r.SubjectIdentity
	transport     r.Transport
	key, provider string
}

func New(t r.Target, tr r.Transport) (r.Adapter, error) {
	key, e := LedgerKey(t.Raw, t.Kind)
	if e != nil {
		return nil, e
	}
	h := sha256.Sum256([]byte(Passphrase))
	return &Adapter{t, r.SubjectIdentity{Network: t.Network, NativeID: t.Raw, ChainIdentity: "stellar:" + hex.EncodeToString(h[:])}, tr, key, "primary"}, nil
}
func (a *Adapter) Supports(t r.Target) bool {
	return t.Network == "stellar-mainnet" && (t.Kind == "address" || t.Kind == "wallet" || t.Kind == "trustline")
}
func (a *Adapter) Subject() r.SubjectIdentity { return r.CollectorIdentity(a.subject) }
func (a *Adapter) Capability() r.Capability {
	return r.Capability{ID: "stellar-ledger-model", Network: a.target.Network, Family: "stellar", TargetClasses: []string{"address", "wallet", "trustline"}, Sources: []string{"Stellar RPC getNetwork/getLedgerEntries XDR"}, ArchivalRequirements: []string{"Historical ledger entries/events require Hubble or an archive; no implicit SDF mainnet RPC or Horizon fallback."}, Fixture: "stellar/stellar_test.go", EvidenceOnly: true}
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
	limits := []r.Limitation{{Code: "stellar_live_only", Reason: "Current ledger only. Historical comparison and look-alike enumeration require configured Hubble/indexed history. Home domain is off-chain context, never legitimacy proof."}}
	netRes, raw, e := r.RPC(ctx, a.transport, t.Network, a.provider, "getNetwork", map[string]any{})
	var net struct {
		Passphrase string `json:"passphrase"`
	}
	if e != nil || json.Unmarshal(raw, &net) != nil || net.Passphrase != Passphrase {
		return rows, limits, errors.New("Stellar network passphrase mismatch")
	}
	res, raw, e := r.RPC(ctx, a.transport, t.Network, a.provider, "getLedgerEntries", map[string]any{"keys": []string{a.key}, "xdrFormat": "base64"})
	if e != nil {
		return rows, limits, e
	}
	var payload struct {
		Ledger  uint64 `json:"latestLedger"`
		Entries []struct {
			Key      string `json:"key"`
			XDR      string `json:"xdr"`
			Modified uint64 `json:"lastModifiedLedgerSeq"`
		} `json:"entries"`
	}
	if json.Unmarshal(raw, &payload) != nil || payload.Ledger == 0 || len(payload.Entries) != 1 || payload.Entries[0].Key != a.key {
		return rows, limits, errors.New("bound ledger entry missing")
	}
	facts, e := DecodeEntry(payload.Entries[0].XDR, t.Raw, t.Kind)
	if e != nil {
		return rows, limits, e
	}
	facts["network_response_sha256"] = r.Digest(netRes.Raw)
	facts["last_modified_ledger"] = payload.Entries[0].Modified
	height := strconv.FormatUint(payload.Ledger, 10)
	rows = append(rows, r.NewRow(a.subject, "stellar-ledger-authority", res, height, facts))
	if flags, ok := facts["flags"].(uint32); ok {
		names := []string{"auth_required", "auth_revocable", "auth_immutable", "auth_clawback_enabled"}
		if t.Kind == "trustline" {
			names = []string{"authorized", "authorized_to_maintain_liabilities", "trustline_clawback_enabled"}
		}
		for i, name := range names {
			rows = append(rows, r.NewRow(a.subject, "stellar-"+name, res, height, map[string]any{"enabled": flags&(1<<uint(i)) != 0, "network_response_sha256": r.Digest(netRes.Raw)}))
		}
	}
	return rows, limits, nil
}

// DecodeEntry reads the stable account/trustline prefix of LedgerEntryData. It
// rejects truncated XDR and unsupported variants before exposing any flags.
func DecodeEntry(encoded, target, kind string) (map[string]any, error) {
	b, e := base64.StdEncoding.DecodeString(encoded)
	if e != nil || len(b) > 16384 {
		return nil, errors.New("invalid XDR size")
	}
	rd := bytes.NewReader(b)
	read := func(n int) ([]byte, error) { v := make([]byte, n); _, e := io.ReadFull(rd, v); return v, e }
	u32 := func() (uint32, error) { var v uint32; e := binary.Read(rd, binary.BigEndian, &v); return v, e }
	typ, e := u32()
	expected := uint32(0)
	if kind == "trustline" {
		expected = 1
	}
	if e != nil || typ != expected {
		return nil, errors.New("ledger type mismatch")
	}
	kt, e := u32()
	if e != nil || kt != 0 {
		return nil, errors.New("unsupported account ID")
	}
	key, e := read(32)
	parts := strings.Split(target, "|")
	if e != nil || EncodeAddress(key) != parts[0] {
		return nil, errors.New("XDR account mismatch")
	}
	facts := map[string]any{}
	if kind == "trustline" {
		assetType, e := u32()
		n := 4
		if assetType == 2 {
			n = 12
		} else if assetType != 1 {
			return nil, errors.New("unsupported asset type")
		}
		if e != nil {
			return nil, e
		}
		code, e := read(n)
		if e != nil || strings.TrimRight(string(code), "\x00") != parts[1] {
			return nil, errors.New("asset code mismatch")
		}
		issuerType, e := u32()
		if e != nil || issuerType != 0 {
			return nil, errors.New("issuer type")
		}
		issuer, e := read(32)
		if e != nil || EncodeAddress(issuer) != parts[2] {
			return nil, errors.New("issuer mismatch")
		}
		if _, e = read(16); e != nil {
			return nil, e
		}
		flags, e := u32()
		if e != nil {
			return nil, e
		}
		facts["flags"] = flags
		facts["asset_code"] = parts[1]
		facts["issuer"] = parts[2]
		return facts, nil
	}
	if _, e = read(20); e != nil {
		return nil, e
	}
	inflation, e := u32()
	if e != nil || inflation > 1 {
		return nil, errors.New("inflation destination malformed")
	}
	if inflation == 1 {
		if _, e = read(36); e != nil {
			return nil, e
		}
	}
	flags, e := u32()
	if e != nil {
		return nil, e
	}
	n, e := u32()
	if e != nil || n > 32 {
		return nil, errors.New("invalid home domain")
	}
	domain, e := read(int((n + 3) / 4 * 4))
	if e != nil {
		return nil, e
	}
	threshold, e := read(4)
	if e != nil {
		return nil, e
	}
	count, e := u32()
	if e != nil || count > 20 {
		return nil, errors.New("invalid signer count")
	}
	signers := []map[string]any{}
	for i := uint32(0); i < count; i++ {
		typ, e := u32()
		if e != nil || typ > 2 {
			return nil, errors.New("unsupported signer type")
		}
		key, e := read(32)
		if e != nil {
			return nil, e
		}
		weight, e := u32()
		if e != nil || weight > 255 {
			return nil, errors.New("invalid signer weight")
		}
		signers = append(signers, map[string]any{"type": typ, "key_hex": hex.EncodeToString(key), "weight": weight})
	}
	facts["flags"] = flags
	facts["home_domain"] = string(domain[:n])
	facts["thresholds"] = map[string]any{"master_weight": threshold[0], "low": threshold[1], "medium": threshold[2], "high": threshold[3]}
	facts["master_meets_high_threshold"] = threshold[0] > 0 && threshold[0] >= threshold[3]
	facts["signers"] = signers
	return facts, nil
}
