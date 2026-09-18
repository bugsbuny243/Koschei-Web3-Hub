package services

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"math/big"
	"strings"
)

const (
	SolanaSPLTokenProgramID        = "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"
	SolanaToken2022ProgramID       = "TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb"
	SolanaAssociatedTokenProgramID = "ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL"
)

const solanaPDASeedMarker = "ProgramDerivedAddress"
const solanaBase58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

var errSolanaProgramAddressOnCurve = errors.New("derived program address is on ed25519 curve")

type SolanaAssociatedTokenAddressCandidate struct {
	Address      string
	TokenProgram string
}

// DeriveSolanaAssociatedTokenAddress derives the canonical ATA PDA for one
// owner/mint/token-program tuple. No RPC request is required.
func DeriveSolanaAssociatedTokenAddress(owner, mint, tokenProgram string) (string, uint8, error) {
	ownerBytes, err := decodeSolanaBase58Pubkey(owner)
	if err != nil {
		return "", 0, fmt.Errorf("owner: %w", err)
	}
	mintBytes, err := decodeSolanaBase58Pubkey(mint)
	if err != nil {
		return "", 0, fmt.Errorf("mint: %w", err)
	}
	tokenProgramBytes, err := decodeSolanaBase58Pubkey(tokenProgram)
	if err != nil {
		return "", 0, fmt.Errorf("token program: %w", err)
	}
	associatedProgramBytes, err := decodeSolanaBase58Pubkey(SolanaAssociatedTokenProgramID)
	if err != nil {
		return "", 0, fmt.Errorf("associated token program: %w", err)
	}
	address, bump, err := findSolanaProgramAddress(
		[][]byte{ownerBytes, tokenProgramBytes, mintBytes},
		associatedProgramBytes,
	)
	if err != nil {
		return "", 0, err
	}
	return encodeSolanaBase58(address), bump, nil
}

// SolanaAssociatedTokenAddressCandidates returns only canonical ATA candidates.
// If the mint's token program is known, callers should pass it and exactly one
// address is produced. When it is unavailable, both supported token programs
// are derived so historical lookup can remain mint-specific without widening to
// wallet-wide signature history.
func SolanaAssociatedTokenAddressCandidates(owner, mint, tokenProgram string) ([]SolanaAssociatedTokenAddressCandidate, error) {
	tokenProgram = strings.TrimSpace(tokenProgram)
	programs := []string{}
	switch tokenProgram {
	case SolanaSPLTokenProgramID, SolanaToken2022ProgramID:
		programs = append(programs, tokenProgram)
	case "":
		programs = append(programs, SolanaSPLTokenProgramID, SolanaToken2022ProgramID)
	default:
		return nil, fmt.Errorf("unsupported mint token program %q", tokenProgram)
	}
	out := make([]SolanaAssociatedTokenAddressCandidate, 0, len(programs))
	for _, program := range programs {
		address, _, err := DeriveSolanaAssociatedTokenAddress(owner, mint, program)
		if err != nil {
			return nil, err
		}
		out = append(out, SolanaAssociatedTokenAddressCandidate{Address: address, TokenProgram: program})
	}
	return out, nil
}

func findSolanaProgramAddress(seeds [][]byte, programID []byte) ([]byte, uint8, error) {
	if len(seeds)+1 > 16 {
		return nil, 0, errors.New("too many program-address seeds")
	}
	for bump := 255; bump >= 0; bump-- {
		withBump := make([][]byte, 0, len(seeds)+1)
		withBump = append(withBump, seeds...)
		withBump = append(withBump, []byte{byte(bump)})
		address, err := createSolanaProgramAddress(withBump, programID)
		if errors.Is(err, errSolanaProgramAddressOnCurve) {
			continue
		}
		if err != nil {
			return nil, 0, err
		}
		return address, uint8(bump), nil
	}
	return nil, 0, errors.New("unable to find an off-curve Solana program address")
}

func createSolanaProgramAddress(seeds [][]byte, programID []byte) ([]byte, error) {
	if len(programID) != 32 {
		return nil, errors.New("program id must decode to 32 bytes")
	}
	if len(seeds) > 16 {
		return nil, errors.New("too many program-address seeds")
	}
	hasher := sha256.New()
	for _, seed := range seeds {
		if len(seed) > 32 {
			return nil, errors.New("program-address seed exceeds 32 bytes")
		}
		_, _ = hasher.Write(seed)
	}
	_, _ = hasher.Write(programID)
	_, _ = hasher.Write([]byte(solanaPDASeedMarker))
	address := hasher.Sum(nil)
	if solanaCompressedEd25519Point(address) {
		return nil, errSolanaProgramAddressOnCurve
	}
	return address, nil
}

func solanaCompressedEd25519Point(encoded []byte) bool {
	if len(encoded) != 32 {
		return false
	}
	yBytes := append([]byte{}, encoded...)
	yBytes[31] &= 0x7f
	y := littleEndianBigInt(yBytes)
	p := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 255), big.NewInt(19))
	if y.Cmp(p) >= 0 {
		return false
	}

	y2 := new(big.Int).Mul(y, y)
	y2.Mod(y2, p)

	numerator := new(big.Int).Sub(y2, big.NewInt(1))
	numerator.Mod(numerator, p)

	dNumerator := new(big.Int).Neg(big.NewInt(121665))
	dNumerator.Mod(dNumerator, p)
	dDenominator := big.NewInt(121666)
	dInverse := new(big.Int).Exp(dDenominator, new(big.Int).Sub(p, big.NewInt(2)), p)
	d := new(big.Int).Mul(dNumerator, dInverse)
	d.Mod(d, p)

	denominator := new(big.Int).Mul(d, y2)
	denominator.Add(denominator, big.NewInt(1))
	denominator.Mod(denominator, p)
	if denominator.Sign() == 0 {
		return false
	}
	denominatorInverse := new(big.Int).Exp(denominator, new(big.Int).Sub(p, big.NewInt(2)), p)
	x2 := new(big.Int).Mul(numerator, denominatorInverse)
	x2.Mod(x2, p)
	if x2.Sign() == 0 {
		return true
	}

	legendreExponent := new(big.Int).Sub(p, big.NewInt(1))
	legendreExponent.Rsh(legendreExponent, 1)
	legendre := new(big.Int).Exp(x2, legendreExponent, p)
	return legendre.Cmp(big.NewInt(1)) == 0
}

func decodeSolanaBase58Pubkey(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, errors.New("empty base58 public key")
	}
	number := big.NewInt(0)
	base := big.NewInt(58)
	for _, character := range value {
		index := strings.IndexRune(solanaBase58Alphabet, character)
		if index < 0 {
			return nil, fmt.Errorf("invalid base58 character %q", character)
		}
		number.Mul(number, base)
		number.Add(number, big.NewInt(int64(index)))
	}
	decoded := number.Bytes()
	leadingZeros := 0
	for leadingZeros < len(value) && value[leadingZeros] == solanaBase58Alphabet[0] {
		leadingZeros++
	}
	out := append(make([]byte, leadingZeros), decoded...)
	if len(out) != 32 {
		return nil, fmt.Errorf("public key decoded to %d bytes, want 32", len(out))
	}
	return out, nil
}

func encodeSolanaBase58(input []byte) string {
	if len(input) == 0 {
		return ""
	}
	number := new(big.Int).SetBytes(input)
	base := big.NewInt(58)
	mod := new(big.Int)
	zero := big.NewInt(0)
	encoded := make([]byte, 0, 44)
	for number.Cmp(zero) > 0 {
		number.DivMod(number, base, mod)
		encoded = append(encoded, solanaBase58Alphabet[mod.Int64()])
	}
	for _, item := range input {
		if item != 0 {
			break
		}
		encoded = append(encoded, solanaBase58Alphabet[0])
	}
	for left, right := 0, len(encoded)-1; left < right; left, right = left+1, right-1 {
		encoded[left], encoded[right] = encoded[right], encoded[left]
	}
	return string(encoded)
}

func littleEndianBigInt(input []byte) *big.Int {
	reversed := make([]byte, len(input))
	for index := range input {
		reversed[len(input)-1-index] = input[index]
	}
	return new(big.Int).SetBytes(reversed)
}
