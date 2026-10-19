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

// canonicalNativeNetworkJob persists every observation supported by the
// connected EVM/Bitcoin address adapters in the existing customer-owned
// durable job store. A native probe is NOT a full historical ARVIS analysis;
// missing collectors and the absence of signed decision authority stay visible.
func canonicalNativeNetworkJob(ctx context.Context, target, network string) (map[string]any, error) {
	return canonicalNativeNetworkJobWithClient(ctx, nil, target, network)
}

func canonicalNativeNetworkJobWithClient(ctx context.Context, client *http.Client, target, network string) (map[string]any, error) {
	resolution, err := networktarget.Resolve(network, target)
	if err != nil {
		return nil, fmt.Errorf("native customer job target: %w", err)
	}
	classified, err := services.ClassifyCustomerScanTarget(target, network)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	observedAt := time.Now().UTC()

	var result services.CustomerScanResult
	switch {
	case resolution.Network.Family == "evm" && classified.Route == services.CustomerScanRouteEVMProbe:
		endpoint := canonicalNativeRPCEnv(network)
		if endpoint == "" {
			return nil, fmt.Errorf("native EVM customer collector not configured")
		}
		probe, err := networktarget.ProbeEVM(ctx, client, endpoint, resolution)
		if err != nil {
			return nil, fmt.Errorf("native EVM customer observation: %w", err)
		}
		projection, err := services.AdaptEVMProbeEvidence(probe, observedAt)
		if err != nil {
			return nil, fmt.Errorf("native EVM customer evidence projection: %w", err)
		}
		result, err = services.CustomerScanResultFromNetworkProbe(classified, projection)
		if err != nil {
			return nil, err
		}
		proxy, proxyErr := networktarget.ProbeEVMProxyAuthority(ctx, client, endpoint, resolution)
		if proxyErr != nil {
			canonicalNativeGap(&result, "EVM_AUTHORITY_PROBE_UNAVAILABLE")
		} else {
			snapshot, snapshotErr := services.BuildEVMSpenderAuthoritySnapshot(probe, proxy)
			if snapshotErr != nil {
				canonicalNativeGap(&result, "EVM_AUTHORITY_PROJECTION_UNAVAILABLE")
			} else {
				withAuthority, attachErr := services.CustomerScanResultFromEVMAuthority(classified, projection, snapshot)
				if attachErr != nil {
					canonicalNativeGap(&result, "EVM_AUTHORITY_RESULT_UNAVAILABLE")
				} else {
					// Keep the complete base network observation; the optional
					// authority builder constructs a fresh base envelope.
					withAuthority.NetworkEvidence = result.NetworkEvidence
					result = withAuthority
				}
			}
		}
		if probe.ContractCodeState == "contract_code_observed" {
			asset, assetErr := networktarget.ProbeEVMERC20Asset(ctx, client, endpoint, probe)
			if assetErr != nil {
				canonicalNativeGap(&result, "EVM_ASSET_PROBE_UNAVAILABLE")
			} else {
				assetProjection, projectionErr := services.AdaptEVMAssetEvidence(asset, observedAt)
				if projectionErr != nil {
					canonicalNativeGap(&result, "EVM_ASSET_PROJECTION_UNAVAILABLE")
				} else {
					withAsset, attachErr := services.CustomerScanResultWithEVMAssetEvidence(result, classified, assetProjection)
					if attachErr != nil {
						canonicalNativeGap(&result, "EVM_ASSET_RESULT_UNAVAILABLE")
					} else {
						result = withAsset
					}
				}
			}
		}
	case network == "bitcoin-mainnet" && classified.Route == services.CustomerScanRouteBitcoinProbe:
		endpoint := strings.TrimSpace(os.Getenv("BITCOIN_ESPLORA_URL"))
		if endpoint == "" {
			return nil, fmt.Errorf("native Bitcoin customer collector not configured")
		}
		probe, err := networktarget.ProbeBitcoin(ctx, client, endpoint, resolution)
		if err != nil {
			return nil, fmt.Errorf("native Bitcoin customer observation: %w", err)
		}
		projection, err := services.AdaptBitcoinProbeEvidence(probe, observedAt)
		if err != nil {
			return nil, fmt.Errorf("native Bitcoin customer evidence projection: %w", err)
		}
		result, err = services.CustomerScanResultFromNetworkProbe(classified, projection)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("no connected native customer job collector for %s", network)
	}
	return map[string]any{
		"schema_version":     "koschei-canonical-native-investigation-v1",
		"status":             "evidence_only_partial",
		"target":             target,
		"network":            network,
		"investigation_kind": "native_network_address_observation",
		"customer_scan":      result,
		"evidence_refs":      result.EvidenceRefs,
		"coverage":           result.Coverage,
		"arm_count":          result.ArmCount,
		"final_verdict": map[string]any{
			"grade": "-", "signed": false, "withheld": true,
			"risk_level": "unknown",
			"reason":     "no_canonical_native_grade_authority",
		},
		"limitations": []string{
			"Native read-only address collection is not a completed historical investigation or a signed ARVIS safety grade.",
			"Transaction graph, historical fund flows, cross-chain links and unconnected modules are not asserted as completed.",
		},
	}, nil
}

func canonicalNativeGap(result *services.CustomerScanResult, reason string) {
	result.Reasons = services.NormalizeWeb3TrustReasons(append(result.Reasons, reason))
	result.Trust.Reasons = append([]string(nil), result.Reasons...)
}

// Select a closed list of chain-specific environment names. Target strings
// never choose environment variables or arbitrary network/RPC destinations.
func canonicalNativeRPCEnv(network string) string {
	switch network {
	case "ethereum-mainnet":
		return strings.TrimSpace(os.Getenv("ETHEREUM_RPC_URL"))
	case "base-mainnet":
		return strings.TrimSpace(os.Getenv("BASE_RPC_URL"))
	case "arbitrum-mainnet":
		return strings.TrimSpace(os.Getenv("ARBITRUM_RPC_URL"))
	case "optimism-mainnet":
		return strings.TrimSpace(os.Getenv("OPTIMISM_RPC_URL"))
	case "polygon-mainnet":
		return strings.TrimSpace(os.Getenv("POLYGON_RPC_URL"))
	case "bnb-mainnet":
		return strings.TrimSpace(os.Getenv("BNB_RPC_URL"))
	case "avalanche-mainnet":
		return strings.TrimSpace(os.Getenv("AVALANCHE_RPC_URL"))
	default:
		return ""
	}
}
