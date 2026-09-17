package handlers

import (
	"math"
	"strings"
)

const lamportsPerSOL = 1_000_000_000.0

// unifiedWalletNativeSOLDelta returns the wallet's net native-SOL balance change
// for one parsed transaction. This is transaction-backed account-balance context,
// not a claim about swap proceeds: fees, rent, account creation and wrapped SOL
// can all make the net balance delta differ from economic trade value.
func unifiedWalletNativeSOLDelta(message, meta map[string]any, wallet string) (float64, bool) {
	wallet = strings.TrimSpace(wallet)
	if wallet == "" || message == nil || meta == nil {
		return 0, false
	}

	keys, _ := transactionInvestigationAccountKeys(message, meta)
	if len(keys) == 0 {
		return 0, false
	}
	pre, okPre := unifiedLamportBalances(meta["preBalances"])
	post, okPost := unifiedLamportBalances(meta["postBalances"])
	if !okPre || !okPost || len(pre) != len(post) || len(pre) != len(keys) {
		return 0, false
	}

	for index, key := range keys {
		if !strings.EqualFold(strings.TrimSpace(key), wallet) {
			continue
		}
		deltaLamports := post[index] - pre[index]
		return math.Round((deltaLamports/lamportsPerSOL)*1e9) / 1e9, true
	}
	return 0, false
}

func unifiedLamportBalances(value any) ([]float64, bool) {
	items, ok := value.([]any)
	if !ok || len(items) == 0 {
		return nil, false
	}
	out := make([]float64, 0, len(items))
	for _, item := range items {
		switch typed := item.(type) {
		case float64:
			out = append(out, typed)
		case float32:
			out = append(out, float64(typed))
		case int:
			out = append(out, float64(typed))
		case int64:
			out = append(out, float64(typed))
		case uint64:
			out = append(out, float64(typed))
		default:
			return nil, false
		}
	}
	return out, true
}
