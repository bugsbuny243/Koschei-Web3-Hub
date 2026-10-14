package solana

import (
 "bytes"
 "errors"
 "math/big"
)

const solanaBase58 = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

// DecodeSolanaPubkey rejects noncanonical or incorrectly sized base58 keys.
func DecodeSolanaPubkey(address string) ([32]byte, error) {
 var key [32]byte
 if len(address) < 32 || len(address) > 44 { return key, errors.New("invalid Solana public key length") }
 value := new(big.Int)
 base := big.NewInt(58)
 for _, ch := range address {
  digit := bytes.IndexByte([]byte(solanaBase58), byte(ch))
  if ch > 127 || digit < 0 { return key, errors.New("invalid Solana base58 character") }
  value.Mul(value, base)
  value.Add(value, big.NewInt(int64(digit)))
 }
 raw := value.Bytes()
 zeros := 0
 for zeros < len(address) && address[zeros] == '1' { zeros++ }
 if len(raw)+zeros != 32 { return key, errors.New("Solana public key must decode to 32 bytes") }
 copy(key[zeros:], raw)
 return key, nil
}
