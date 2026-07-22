package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"koschei/api/internal/services"
)

func TestCollectRaydiumCLMMLockEvidenceVerifiesFullCustodyChain(t *testing.T) {
	fixture := newCLMMLockFixture()
	rpc := fixture.rpc(t, clmmFixtureOptions{})
	got := collectProtocolLPControlEvidence(context.Background(), rpc, "solana-mainnet", fixture.tokenMint, fixture.creator, fixture.pool)

	if got.Status != services.LPControlVerifiedPermanentLocked || got.ReasonCode != "raydium_clmm_burn_and_earn_positions_verified" {
		t.Fatalf("CLMM lock evidence was not verified: %#v", got)
	}
	if got.PoolProgram != raydiumCLMMProgram || got.ControlModel != "position_nft" || got.PositionModel != "raydium_clmm_position_nft" {
		t.Fatalf("unexpected CLMM pool model: %#v", got)
	}
	if got.TokenVault != fixture.tokenVault || got.QuoteVault != fixture.quoteVault || got.QuoteMint != fixture.quoteMint {
		t.Fatalf("CLMM vault orientation failed: %#v", got)
	}
	if got.LockedPositionCount != 1 || got.LockedPositionLiquidityRaw != fixture.positionLiquidity.String() {
		t.Fatalf("locked position aggregate = count %d liquidity %s", got.LockedPositionCount, got.LockedPositionLiquidityRaw)
	}
	if got.PositionEnumerationStatus != "verified_complete_bounded_filter" || got.PositionEnumerationLimit != raydiumCLMMLockResultLimit {
		t.Fatalf("unexpected position enumeration state: %#v", got)
	}
	if got.PermanentLockedSharePct != 0 || got.LockedLPSharePct != 0 {
		t.Fatalf("CLMM collector invented a lock percentage: permanent %.4f LP %.4f", got.PermanentLockedSharePct, got.LockedLPSharePct)
	}
	if len(got.LockedPositions) != 1 {
		t.Fatalf("locked positions = %d, want 1", len(got.LockedPositions))
	}
	position := got.LockedPositions[0]
	if position.LockedPositionAccount != fixture.lockAccount || position.PositionAccount != fixture.positionAccount || position.PositionNFTMint != fixture.positionNFTMint {
		t.Fatalf("position identity mismatch: %#v", position)
	}
	if position.LockedNFTAccount != fixture.lockedNFTAccount || position.CustodyAuthority != raydiumCLMMLockAuthority || position.FeeNFTMint != fixture.feeNFTMint {
		t.Fatalf("position custody mismatch: %#v", position)
	}
	if position.TickLowerIndex != -120 || position.TickUpperIndex != 240 || position.VerificationStatus != "VERIFIED" {
		t.Fatalf("position range/status mismatch: %#v", position)
	}
	if !containsStringValue(got.EvidenceKeys, fmt.Sprintf("raydium_clmm_lock:%s@%d", fixture.lockAccount, fixture.readSlot)) {
		t.Fatalf("lock evidence key missing: %v", got.EvidenceKeys)
	}
}

func TestCollectRaydiumCLMMLockEvidenceRejectsCustodyMismatch(t *testing.T) {
	fixture := newCLMMLockFixture()
	rpc := fixture.rpc(t, clmmFixtureOptions{custodyOwner: testCLMMPubkey(90)})
	got := collectProtocolLPControlEvidence(context.Background(), rpc, "solana-mainnet", fixture.tokenMint, fixture.creator, fixture.pool)
	if got.Status == services.LPControlVerifiedPermanentLocked || got.LockedPositionCount != 0 {
		t.Fatalf("wrong custody authority was accepted: %#v", got)
	}
	if got.ReasonCode != "raydium_clmm_position_custody_unverified" || got.PositionEnumerationStatus != "unverified_records" {
		t.Fatalf("custody mismatch did not fail closed: %#v", got)
	}
}

func TestCollectRaydiumCLMMLockEvidenceRejectsPositionPoolMismatch(t *testing.T) {
	fixture := newCLMMLockFixture()
	rpc := fixture.rpc(t, clmmFixtureOptions{positionPool: testCLMMPubkey(91)})
	got := collectProtocolLPControlEvidence(context.Background(), rpc, "solana-mainnet", fixture.tokenMint, fixture.creator, fixture.pool)
	if got.Status == services.LPControlVerifiedPermanentLocked || got.LockedPositionCount != 0 {
		t.Fatalf("position from another pool was accepted: %#v", got)
	}
}

func TestCollectRaydiumCLMMLockEvidenceFailsClosedAboveBound(t *testing.T) {
	fixture := newCLMMLockFixture()
	rpc := fixture.rpc(t, clmmFixtureOptions{lockRecordCount: raydiumCLMMLockResultLimit + 1})
	got := collectProtocolLPControlEvidence(context.Background(), rpc, "solana-mainnet", fixture.tokenMint, fixture.creator, fixture.pool)
	if got.Status == services.LPControlVerifiedPermanentLocked || got.LockedPositionCount != 0 {
		t.Fatalf("oversized position index was truncated and accepted: %#v", got)
	}
	if got.ReasonCode != "raydium_clmm_lock_index_exceeds_limit" || got.PositionEnumerationStatus != "limit_exceeded" {
		t.Fatalf("oversized index did not fail closed: %#v", got)
	}
}

func TestDecodeRaydiumCLMMPoolRejectsWrongDiscriminator(t *testing.T) {
	fixture := newCLMMLockFixture()
	poolData := fixture.poolData()
	poolData[0] ^= 0xff
	got := decodeRaydiumCLMMLPControl(context.Background(), fixture.rpc(t, clmmFixtureOptions{}), "solana-mainnet", services.LPControlEvidence{
		PoolAddress: fixture.pool, PoolProgram: raydiumCLMMProgram, TokenMint: fixture.tokenMint,
		LargestLPHolders: []services.LPHolderEvidence{}, LockedPositions: []services.CLMMLockedPositionEvidence{},
		LiquidityMovements: []services.LiquidityMovementEvidence{}, EvidenceKeys: []string{}, Limitations: []string{},
	}, poolData)
	if got.Status != services.LPControlSourceUnavailable || got.ReasonCode != "raydium_clmm_pool_state_invalid" {
		t.Fatalf("wrong PoolState discriminator was accepted: %#v", got)
	}
}

type clmmFixtureOptions struct {
	custodyOwner   string
	positionPool   string
	lockRecordCount int
}

type clmmLockFixture struct {
	pool              string
	creator           string
	tokenMint         string
	quoteMint         string
	tokenVault        string
	quoteVault        string
	lockOwner         string
	lockAccount       string
	positionAccount   string
	lockedNFTAccount  string
	feeNFTMint        string
	positionNFTMint   string
	positionLiquidity *big.Int
	readSlot          uint64
}

func newCLMMLockFixture() clmmLockFixture {
	return clmmLockFixture{
		pool: testCLMMPubkey(1), creator: testCLMMPubkey(2), tokenMint: testCLMMPubkey(3), quoteMint: testCLMMPubkey(4),
		tokenVault: testCLMMPubkey(5), quoteVault: testCLMMPubkey(6), lockOwner: testCLMMPubkey(7),
		lockAccount: testCLMMPubkey(8), positionAccount: testCLMMPubkey(9), lockedNFTAccount: testCLMMPubkey(10),
		feeNFTMint: testCLMMPubkey(11), positionNFTMint: testCLMMPubkey(12),
		positionLiquidity: new(big.Int).SetUint64(987654321), readSlot: 777,
	}
}

func (fixture clmmLockFixture) poolData() []byte {
	data := make([]byte, 1544)
	copyAnchorDiscriminator(data, "PoolState")
	copy(data[41:73], testCLMMPubkeyBytes(2))
	copy(data[73:105], testCLMMPubkeyBytes(3))
	copy(data[105:137], testCLMMPubkeyBytes(4))
	copy(data[137:169], testCLMMPubkeyBytes(5))
	copy(data[169:201], testCLMMPubkeyBytes(6))
	putLittleEndian128(data[237:253], new(big.Int).SetUint64(123456789))
	return data
}

func (fixture clmmLockFixture) lockData(pool string) []byte {
	data := make([]byte, 177)
	copyAnchorDiscriminator(data, "LockedClmmPositionState")
	data[8] = 254
	copy(data[9:41], testCLMMPubkeyBytes(7))
	copy(data[41:73], mustCLMMDecodePubkey(pool))
	copy(data[73:105], testCLMMPubkeyBytes(9))
	copy(data[105:137], testCLMMPubkeyBytes(10))
	copy(data[137:169], testCLMMPubkeyBytes(11))
	binary.LittleEndian.PutUint64(data[169:177], 42)
	return data
}

func (fixture clmmLockFixture) positionData(pool string) []byte {
	data := make([]byte, raydiumCLMMPositionSize)
	copyAnchorDiscriminator(data, "PersonalPositionState")
	data[8] = 253
	copy(data[9:41], testCLMMPubkeyBytes(12))
	copy(data[41:73], mustCLMMDecodePubkey(pool))
	binary.LittleEndian.PutUint32(data[73:77], uint32(int32(-120)))
	binary.LittleEndian.PutUint32(data[77:81], uint32(int32(240)))
	putLittleEndian128(data[81:97], fixture.positionLiquidity)
	return data
}

func (fixture clmmLockFixture) rpc(t *testing.T, options clmmFixtureOptions) solanaRPCCall {
	t.Helper()
	if options.custodyOwner == "" {
		options.custodyOwner = raydiumCLMMLockAuthority
	}
	if options.positionPool == "" {
		options.positionPool = fixture.pool
	}
	if options.lockRecordCount == 0 {
		options.lockRecordCount = 1
	}
	return func(_ context.Context, _ string, method string, params any, out any) error {
		switch method {
		case "getAccountInfo":
			response := out.(*rpcAccountInfoResponse)
			response.Context.Slot = fixture.readSlot - 2
			response.Value = &struct {
				Owner string `json:"owner"`
				Data  any    `json:"data"`
			}{Owner: raydiumCLMMProgram, Data: []any{base64.StdEncoding.EncodeToString(fixture.poolData()), "base64"}}
		case "getTokenAccountBalance":
			response := out.(*rpcTokenBalanceResponse)
			response.Context.Slot = fixture.readSlot - 1
			response.Value.UIAmountString = "1000"
		case "getProgramAccounts":
			assertCLMMProgramAccountFilters(t, params, fixture.pool)
			response := out.(*rpcProgramAccountsContextResponse)
			response.Context.Slot = fixture.readSlot
			response.Value = make([]struct {
				Pubkey  string `json:"pubkey"`
				Account struct {
					Owner string `json:"owner"`
					Data  any    `json:"data"`
				} `json:"account"`
			}, options.lockRecordCount)
			for index := range response.Value {
				response.Value[index].Pubkey = fixture.lockAccount
				response.Value[index].Account.Owner = raydiumLPLockProgram
				response.Value[index].Account.Data = []any{base64.StdEncoding.EncodeToString(fixture.lockData(fixture.pool)), "base64"}
			}
		case "getMultipleAccounts":
			arguments, ok := params.([]any)
			if !ok || len(arguments) < 2 {
				return fmt.Errorf("malformed getMultipleAccounts params")
			}
			addresses, ok := arguments[0].([]string)
			if !ok {
				return fmt.Errorf("malformed getMultipleAccounts addresses")
			}
			config, _ := arguments[1].(map[string]any)
			encoding, _ := config["encoding"].(string)
			values := make([]any, 0, len(addresses))
			for _, address := range addresses {
				switch encoding {
				case "base64":
					if address != fixture.positionAccount {
						values = append(values, nil)
						continue
					}
					values = append(values, map[string]any{"owner": raydiumCLMMProgram, "data": []any{base64.StdEncoding.EncodeToString(fixture.positionData(options.positionPool)), "base64"}})
				case "jsonParsed":
					if address != fixture.lockedNFTAccount {
						values = append(values, nil)
						continue
					}
					values = append(values, map[string]any{
						"owner": clmmSPLTokenProgram,
						"data": map[string]any{"parsed": map[string]any{"info": map[string]any{
							"mint": fixture.positionNFTMint, "owner": options.custodyOwner,
							"tokenAmount": map[string]any{"amount": "1", "decimals": 0},
						}}},
					})
				default:
					return fmt.Errorf("unexpected encoding: %s", encoding)
				}
			}
			encoded, _ := json.Marshal(map[string]any{"value": values})
			if err := json.Unmarshal(encoded, out); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unexpected RPC method: %s", method)
		}
		return nil
	}
}

func assertCLMMProgramAccountFilters(t *testing.T, params any, pool string) {
	t.Helper()
	arguments, ok := params.([]any)
	if !ok || len(arguments) != 2 || arguments[0] != raydiumLPLockProgram {
		t.Fatalf("unexpected getProgramAccounts params: %#v", params)
	}
	config, ok := arguments[1].(map[string]any)
	if !ok || config["withContext"] != true {
		t.Fatalf("getProgramAccounts is not context-bound: %#v", config)
	}
	dataSlice, _ := config["dataSlice"].(map[string]any)
	if dataSlice["offset"] != 0 || dataSlice["length"] != 177 {
		t.Fatalf("unexpected CLMM lock data slice: %#v", dataSlice)
	}
	filters, ok := config["filters"].([]any)
	if !ok || len(filters) != 2 {
		t.Fatalf("unexpected CLMM lock filters: %#v", config["filters"])
	}
	dataSizeFilter, _ := filters[0].(map[string]any)
	if dataSizeFilter["dataSize"] != raydiumCLMMLockAccountSize {
		t.Fatalf("unexpected lock account dataSize: %#v", dataSizeFilter)
	}
	memcmpFilter, _ := filters[1].(map[string]any)
	memcmp, _ := memcmpFilter["memcmp"].(map[string]any)
	if memcmp["offset"] != 41 || memcmp["bytes"] != pool {
		t.Fatalf("unexpected pool memcmp filter: %#v", memcmp)
	}
}

func copyAnchorDiscriminator(data []byte, accountName string) {
	digest := sha256.Sum256([]byte("account:" + accountName))
	copy(data[:8], digest[:8])
}

func testCLMMPubkey(seed byte) string {
	return base58Encode(testCLMMPubkeyBytes(seed))
}

func testCLMMPubkeyBytes(seed byte) []byte {
	out := make([]byte, 32)
	for index := range out {
		out[index] = seed + byte(index)
	}
	return out
}

func mustCLMMDecodePubkey(value string) []byte {
	decoded := decodeBase58Test(value)
	if len(decoded) != 32 {
		panic("test pubkey did not decode to 32 bytes: " + value)
	}
	return decoded
}

func decodeBase58Test(value string) []byte {
	result := big.NewInt(0)
	base := big.NewInt(58)
	for _, character := range value {
		index := strings.IndexRune(base58Alphabet, character)
		if index < 0 {
			return nil
		}
		result.Mul(result, base)
		result.Add(result, big.NewInt(int64(index)))
	}
	decoded := result.Bytes()
	zeros := 0
	for zeros < len(value) && value[zeros] == '1' {
		zeros++
	}
	return append(make([]byte, zeros), decoded...)
}

func putLittleEndian128(target []byte, value *big.Int) {
	if len(target) < 16 {
		return
	}
	for index := range target[:16] {
		target[index] = 0
	}
	encoded := value.Bytes()
	for index := 0; index < len(encoded) && index < 16; index++ {
		target[index] = encoded[len(encoded)-1-index]
	}
}
