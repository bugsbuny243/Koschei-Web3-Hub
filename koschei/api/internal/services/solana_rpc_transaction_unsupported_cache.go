package services

import (
	"errors"
	"strings"
	"sync"
	"time"
)

// Transaction version failures are deterministic for the currently supported
// parser contract, unlike transient 429/timeouts. Keep this cache bounded and
// short-lived; never store raw signatures or credential-bearing RPC URLs.
var errSolanaUnsupportedTransactionVersion = errors.New("solana rpc error -32015: unsupported transaction version (ARVIS evidence unavailable)")

const solanaUnsupportedTransactionTTL = 10 * time.Minute
const solanaUnsupportedTransactionMaxEntries = 2048

var solanaUnsupportedTransactionVersions = struct {
	sync.Mutex
	items map[string]time.Time
}{items: map[string]time.Time{}}

func isSolanaUnsupportedTransactionVersionError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "-32015") && strings.Contains(message, "version")
}

func cachedSolanaUnsupportedTransactionVersion(key string) bool {
	now := time.Now()
	solanaUnsupportedTransactionVersions.Lock()
	defer solanaUnsupportedTransactionVersions.Unlock()
	expiry, ok := solanaUnsupportedTransactionVersions.items[key]
	if !ok {
		return false
	}
	if !expiry.After(now) {
		delete(solanaUnsupportedTransactionVersions.items, key)
		return false
	}
	return true
}

func cacheSolanaUnsupportedTransactionVersion(key string) {
	if key == "" {
		return
	}
	now := time.Now()
	solanaUnsupportedTransactionVersions.Lock()
	defer solanaUnsupportedTransactionVersions.Unlock()
	for existing, expires := range solanaUnsupportedTransactionVersions.items {
		if !expires.After(now) {
			delete(solanaUnsupportedTransactionVersions.items, existing)
		}
	}
	if len(solanaUnsupportedTransactionVersions.items) >= solanaUnsupportedTransactionMaxEntries {
		// Do not grow memory without bound during large signature investigations.
		// Refusing a cache entry still leaves the caller with an explicit RPC error.
		return
	}
	solanaUnsupportedTransactionVersions.items[key] = now.Add(solanaUnsupportedTransactionTTL)
}

func resetSolanaUnsupportedTransactionVersionForTest() {
	solanaUnsupportedTransactionVersions.Lock()
	solanaUnsupportedTransactionVersions.items = map[string]time.Time{}
	solanaUnsupportedTransactionVersions.Unlock()
}
