package handlers

// isValidSolanaAddress keeps legacy callers on the same local base58 decoder
// used by wallet ownership verification. A valid Solana public key decodes to
// exactly 32 bytes; no RPC or external dependency is required.
func isValidSolanaAddress(value string) bool {
	_, err := decodeSolanaPublicKey(value)
	return err == nil
}
