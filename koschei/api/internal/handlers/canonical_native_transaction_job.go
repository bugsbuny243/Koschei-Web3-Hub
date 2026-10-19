package handlers

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"koschei/api/internal/networktarget"
	"koschei/api/internal/services"
)

// Transactions are collected as bounded, independently network-identified
// read-only evidence, without adding address-only assumptions or a fake grade.
func canonicalNativeTransactionJob(ctx context.Context, client *http.Client, txid, network string, classified services.CustomerScanTarget) (map[string]any, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var result services.CustomerScanResult
	switch {
	case network == "bitcoin-mainnet":
		endpoint := strings.TrimSpace(os.Getenv("BITCOIN_ESPLORA_URL"))
		if endpoint == "" {
			return nil, fmt.Errorf("native Bitcoin transaction collector not configured")
		}
		probe, err := networktarget.ProbeBitcoinTransaction(ctx, client, endpoint, txid)
		if err != nil {
			return nil, fmt.Errorf("native Bitcoin transaction observation: %w", err)
		}
		projected, err := services.AdaptBitcoinTransactionEvidence(probe, time.Now().UTC())
		if err != nil {
			return nil, fmt.Errorf("native Bitcoin transaction projection: %w", err)
		}
		result, err = services.CustomerScanResultFromBitcoinTransaction(classified, projected)
		if err != nil {
			return nil, err
		}
	case canonicalNativeRPCEnv(network) != "":
		endpoint := canonicalNativeRPCEnv(network)
		probe, err := networktarget.ProbeEVMTransaction(ctx, client, endpoint, network, txid)
		if err != nil {
			return nil, fmt.Errorf("native EVM transaction observation: %w", err)
		}
		projected, err := services.AdaptEVMTransactionEvidence(probe, time.Now().UTC())
		if err != nil {
			return nil, fmt.Errorf("native EVM transaction projection: %w", err)
		}
		result, err = services.CustomerScanResultFromEVMTransaction(classified, projected)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("no connected native transaction collector for %s", network)
	}
	return canonicalNativeEvidenceEnvelope(result, "native_network_transaction_observation"), nil
}
