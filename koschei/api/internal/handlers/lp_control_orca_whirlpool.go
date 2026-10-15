package handlers

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"

	"koschei/api/internal/services"
)

const orcaWhirlpoolAccountLen = 653

var orcaWhirlpoolAccountDiscriminator = []byte{0x3f, 0x95, 0xd1, 0x0c, 0xe1, 0x80, 0x63, 0x09}

// decodeOrcaWhirlpoolLPControl decodes the official 653-byte Anchor Whirlpool
// account. The caller has already verified getAccountInfo.owner matches the
// pinned Orca program; a pool account never proves ownership of its positions.
func decodeOrcaWhirlpoolLPControl(ctx context.Context, rpc solanaRPCCall, network string, out services.LPControlEvidence, data []byte) services.LPControlEvidence {
	if len(data) != orcaWhirlpoolAccountLen || !bytes.Equal(data[:8], orcaWhirlpoolAccountDiscriminator) {
		out.Status, out.ReasonCode = services.LPControlSourceUnavailable, "orca_whirlpool_account_invalid"
		out.Limitations = append(out.Limitations, "The Orca Whirlpool account length or Anchor discriminator does not match the pinned layout.")
		return out
	}
	// Orca Whirlpool Anchor layout: token A mint/vault at [101:133]/[133:165],
	// token B mint/vault at [181:213]/[213:245]. No fungible LP mint exists.
	mintA, vaultA := base58Encode(data[101:133]), base58Encode(data[133:165])
	mintB, vaultB := base58Encode(data[181:213]), base58Encode(data[213:245])
	out.PoolType = "orca_whirlpool"
	out.ControlModel = "position_nft"
	out.PositionModel = "orca_whirlpool_position_nft"
	if mintA == out.TokenMint {
		out.TokenVault, out.QuoteVault, out.QuoteMint = vaultA, vaultB, mintB
	} else if mintB == out.TokenMint {
		out.TokenVault, out.QuoteVault, out.QuoteMint = vaultB, vaultA, mintA
	} else {
		out.ReasonCode = "orca_whirlpool_pool_mint_mismatch"
		out.Limitations = append(out.Limitations, "The verified Whirlpool pool account does not contain the requested token mint.")
		return out
	}
	poolSlot := out.ReadSlot
	out.PoolLiquidityRaw = unsignedLittleEndian128(data[49:65]).String()
	tickSpacing := binary.LittleEndian.Uint16(data[41:43])
	feeRate := binary.LittleEndian.Uint16(data[45:47])
	out = populatePositionPoolReserves(ctx, rpc, network, out)
	out.Available = true
	out.Status, out.ReasonCode = services.LPControlUnverified, "ORCA_POSITION_OWNERSHIP_UNRESOLVED"
	out.EvidenceKeys = append(out.EvidenceKeys,
		fmt.Sprintf("orca_whirlpool_pool:%s@%d", out.PoolAddress, poolSlot),
		fmt.Sprintf("orca_whirlpool_token_vault:%s", out.TokenVault),
		fmt.Sprintf("orca_whirlpool_quote_vault:%s", out.QuoteVault),
		"orca_whirlpool_liquidity_raw:"+out.PoolLiquidityRaw,
		fmt.Sprintf("orca_whirlpool_tick_spacing:%d", tickSpacing),
		fmt.Sprintf("orca_whirlpool_fee_rate_raw:%d", feeRate),
	)
	out.Limitations = append(out.Limitations, "Orca Whirlpool positions are NFT-based. Pool reserves and liquidity do not identify position owners, creator control, position locks or burned LP shares; position ownership has not been enumerated.")
	return out
}
