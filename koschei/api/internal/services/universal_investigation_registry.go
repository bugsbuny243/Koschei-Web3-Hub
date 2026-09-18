
package services

import (
	"errors"
	"strings"
)

const (
	IntelligenceChainFamilyMove      = "move"
	IntelligenceChainFamilyCosmos    = "cosmos"
	IntelligenceChainFamilySubstrate = "substrate"
	IntelligenceChainFamilyTON       = "ton"
	IntelligenceChainFamilyNEAR      = "near"
	IntelligenceChainFamilyOffchain  = "offchain"

	IntelligenceSubjectToken       = "token"
	IntelligenceSubjectContract    = "contract"
	IntelligenceSubjectProgram     = "program"
	IntelligenceSubjectTransaction = "transaction"
	IntelligenceSubjectBlock       = "block"
	IntelligenceSubjectValidator   = "validator"
	IntelligenceSubjectBridge      = "bridge"
	IntelligenceSubjectGovernance  = "governance"
	IntelligenceSubjectIdentity    = "identity"
	IntelligenceSubjectCredential  = "credential"
	IntelligenceSubjectAgent       = "agent"
	IntelligenceSubjectProtocol    = "protocol"
	IntelligenceSubjectArtifact    = "artifact"
	IntelligenceSubjectProject     = "project"

	UniversalAdapterLive     = "live"
	UniversalAdapterProbe    = "probe"
	UniversalAdapterPlanned  = "planned"
	UniversalAdapterResearch = "research"
)

type UniversalInvestigationAdapterProfile struct {
	ID                   string
	Domain               string
	ChainFamily          string
	Status               string
	NetworkScope         string
	Networks             []string
	TargetKinds          []string
	EvidenceSources      []string
	RequiredTrustAnchors []string
	VerdictPolicy        string
}

func UniversalInvestigationAdapterProfiles() []UniversalInvestigationAdapterProfile {
	return []UniversalInvestigationAdapterProfile{
		{
			ID: "solana-account-model", Domain: "web3", ChainFamily: IntelligenceChainFamilySolana,
			Status: UniversalAdapterLive, NetworkScope: "registered_networks",
			Networks: []string{"solana-mainnet"},
			TargetKinds: []string{
				IntelligenceSubjectAddress, IntelligenceSubjectToken, IntelligenceSubjectProgram,
				IntelligenceSubjectTransaction, IntelligenceSubjectBlock, IntelligenceSubjectValidator,
				IntelligenceSubjectBridge, IntelligenceSubjectGovernance,
			},
			EvidenceSources: []string{"solana_jsonrpc", "parsed_transactions", "program_state", "actor_memory"},
			RequiredTrustAnchors: []string{"genesis_or_network_identity", "slot", "signature", "program_id"},
			VerdictPolicy: "Existing deterministic ARVIS rules may affect a signed verdict only when their required Solana evidence is VERIFIED.",
		},
		{
			ID: "evm-account-model", Domain: "web3", ChainFamily: IntelligenceChainFamilyEVM,
			Status: UniversalAdapterProbe, NetworkScope: "eip155_family",
			Networks: []string{"ethereum-mainnet", "base-mainnet", "arbitrum-mainnet", "optimism-mainnet", "polygon-mainnet", "bnb-mainnet", "avalanche-mainnet"},
			TargetKinds: []string{
				IntelligenceSubjectAddress, IntelligenceSubjectToken, IntelligenceSubjectContract,
				IntelligenceSubjectTransaction, IntelligenceSubjectBlock, IntelligenceSubjectBridge,
				IntelligenceSubjectGovernance,
			},
			EvidenceSources: []string{"ethereum_jsonrpc", "contract_code", "storage", "logs", "receipts", "eip7702", "erc1967"},
			RequiredTrustAnchors: []string{"eth_chainId", "block_hash_or_number", "transaction_hash", "contract_address"},
			VerdictPolicy: "Current EVM probes remain evidence-only until chain-specific deterministic verdict rules are promoted.",
		},
		{
			ID: "bitcoin-utxo-model", Domain: "web3", ChainFamily: IntelligenceChainFamilyUTXO,
			Status: UniversalAdapterProbe, NetworkScope: "bitcoin_family",
			Networks: []string{"bitcoin-mainnet"},
			TargetKinds: []string{IntelligenceSubjectAddress, IntelligenceSubjectTransaction, IntelligenceSubjectBlock},
			EvidenceSources: []string{"bitcoin_core_rpc_or_esplora", "utxo_set", "transaction_graph", "block_headers"},
			RequiredTrustAnchors: []string{"genesis_hash", "block_hash", "transaction_id", "script_pubkey"},
			VerdictPolicy: "UTXO observations remain evidence-only until deterministic UTXO behavior rules are promoted.",
		},
		{
			ID: "move-object-model", Domain: "web3", ChainFamily: IntelligenceChainFamilyMove,
			Status: UniversalAdapterPlanned, NetworkScope: "move_family",
			Networks: []string{"sui-mainnet", "aptos-mainnet"},
			TargetKinds: []string{
				IntelligenceSubjectAddress, IntelligenceSubjectToken, IntelligenceSubjectContract,
				IntelligenceSubjectTransaction, IntelligenceSubjectBlock, IntelligenceSubjectValidator,
				IntelligenceSubjectGovernance,
			},
			EvidenceSources: []string{"sui_graphql_or_rpc", "aptos_rest", "objects_resources_modules", "events", "transactions"},
			RequiredTrustAnchors: []string{"chain_identity", "checkpoint_or_ledger_version", "transaction_digest_or_hash", "object_or_resource_id"},
			VerdictPolicy: "No Move-family claim may affect a verdict before a network adapter proves chain identity and canonical state/transaction evidence.",
		},
		{
			ID: "cosmos-cometbft-model", Domain: "web3", ChainFamily: IntelligenceChainFamilyCosmos,
			Status: UniversalAdapterPlanned, NetworkScope: "cosmos_sdk_cometbft_family",
			Networks: []string{"cosmoshub-mainnet", "osmosis-mainnet"},
			TargetKinds: []string{
				IntelligenceSubjectAddress, IntelligenceSubjectToken, IntelligenceSubjectContract,
				IntelligenceSubjectTransaction, IntelligenceSubjectBlock, IntelligenceSubjectValidator,
				IntelligenceSubjectBridge, IntelligenceSubjectGovernance,
			},
			EvidenceSources: []string{"cometbft_rpc", "cosmos_sdk_query", "abci_state", "events", "ibc"},
			RequiredTrustAnchors: []string{"chain_id", "block_height_hash", "transaction_hash", "module_or_contract_id"},
			VerdictPolicy: "IBC or cross-zone linkage must remain OBSERVED/UNVERIFIED until both source and destination evidence are independently anchored.",
		},
		{
			ID: "substrate-runtime-model", Domain: "web3", ChainFamily: IntelligenceChainFamilySubstrate,
			Status: UniversalAdapterPlanned, NetworkScope: "substrate_family",
			Networks: []string{"polkadot-mainnet"},
			TargetKinds: []string{
				IntelligenceSubjectAddress, IntelligenceSubjectContract, IntelligenceSubjectTransaction,
				IntelligenceSubjectBlock, IntelligenceSubjectValidator, IntelligenceSubjectBridge,
				IntelligenceSubjectGovernance,
			},
			EvidenceSources: []string{"polkadot_jsonrpc", "runtime_metadata", "storage", "events", "extrinsics"},
			RequiredTrustAnchors: []string{"chain_spec_or_genesis", "block_hash", "runtime_version", "extrinsic_or_event_reference"},
			VerdictPolicy: "Runtime upgrades must be version-bound; metadata from one runtime must not be applied to another without compatibility proof.",
		},
		{
			ID: "ton-account-message-model", Domain: "web3", ChainFamily: IntelligenceChainFamilyTON,
			Status: UniversalAdapterPlanned, NetworkScope: "ton_family",
			Networks: []string{"ton-mainnet"},
			TargetKinds: []string{
				IntelligenceSubjectAddress, IntelligenceSubjectToken, IntelligenceSubjectContract,
				IntelligenceSubjectTransaction, IntelligenceSubjectBlock, IntelligenceSubjectValidator,
				IntelligenceSubjectBridge,
			},
			EvidenceSources: []string{"ton_api", "account_state", "messages", "transactions", "shard_blocks"},
			RequiredTrustAnchors: []string{"workchain", "account_id", "block_or_transaction_anchor", "message_hash"},
			VerdictPolicy: "Message intent and transaction effect remain separate; a message alone is not proof that the requested state transition completed.",
		},
		{
			ID: "near-account-wasm-model", Domain: "web3", ChainFamily: IntelligenceChainFamilyNEAR,
			Status: UniversalAdapterPlanned, NetworkScope: "near_family",
			Networks: []string{"near-mainnet"},
			TargetKinds: []string{
				IntelligenceSubjectAddress, IntelligenceSubjectToken, IntelligenceSubjectContract,
				IntelligenceSubjectTransaction, IntelligenceSubjectBlock, IntelligenceSubjectValidator,
				IntelligenceSubjectBridge, IntelligenceSubjectGovernance,
			},
			EvidenceSources: []string{"near_jsonrpc", "account_state", "wasm_code", "receipts", "transactions"},
			RequiredTrustAnchors: []string{"chain_id", "block_hash_or_height", "transaction_or_receipt_id", "account_id"},
			VerdictPolicy: "Receipt execution and final outcome must be verified separately from transaction submission.",
		},
		{
			ID: "web5-identity-evidence", Domain: "web5", ChainFamily: IntelligenceChainFamilyOffchain,
			Status: UniversalAdapterResearch, NetworkScope: "standards_family",
			TargetKinds: []string{IntelligenceSubjectIdentity, IntelligenceSubjectCredential},
			EvidenceSources: []string{"w3c_did", "did_resolution", "w3c_verifiable_credentials", "data_integrity"},
			RequiredTrustAnchors: []string{"did_method", "verification_method", "proof_or_signature", "status_or_revocation_state"},
			VerdictPolicy: "Verified identity proves only the credential/controller statement it actually covers; it never grants unlimited authority or a safety verdict.",
		},
		{
			ID: "web4-agent-evidence", Domain: "web4", ChainFamily: IntelligenceChainFamilyOffchain,
			Status: UniversalAdapterResearch, NetworkScope: "agent_protocol_family",
			TargetKinds: []string{IntelligenceSubjectAgent, IntelligenceSubjectProtocol, IntelligenceSubjectArtifact},
			EvidenceSources: []string{"mcp", "a2a", "oauth_authorization", "signed_provenance", "tool_and_task_telemetry"},
			RequiredTrustAnchors: []string{"agent_or_server_identity", "capability_declaration", "authorization_context", "request_response_provenance"},
			VerdictPolicy: "Agent capability, authorization, execution and effect are separate evidence axes; tool availability must never imply authorization.",
		},
		{
			ID: "web6-crypto-agility-evidence", Domain: "web6", ChainFamily: IntelligenceChainFamilyOffchain,
			Status: UniversalAdapterResearch, NetworkScope: "crypto_agility_profile",
			TargetKinds: []string{IntelligenceSubjectProtocol, IntelligenceSubjectArtifact, IntelligenceSubjectProject},
			EvidenceSources: []string{"algorithm_inventory", "signature_provenance", "key_lifecycle", "pq_transition_evidence"},
			RequiredTrustAnchors: []string{"algorithm_identifier", "key_or_certificate_reference", "signed_artifact_hash", "migration_state"},
			VerdictPolicy: "Post-quantum readiness is an evidence-backed migration state, not a binary safety label.",
		},
	}
}

func UniversalInvestigationTargetKinds() []string {
	return []string{
		IntelligenceSubjectAddress,
		IntelligenceSubjectToken,
		IntelligenceSubjectContract,
		IntelligenceSubjectProgram,
		IntelligenceSubjectTransaction,
		IntelligenceSubjectBlock,
		IntelligenceSubjectValidator,
		IntelligenceSubjectBridge,
		IntelligenceSubjectGovernance,
		IntelligenceSubjectIdentity,
		IntelligenceSubjectCredential,
		IntelligenceSubjectAgent,
		IntelligenceSubjectProtocol,
		IntelligenceSubjectArtifact,
		IntelligenceSubjectProject,
	}
}

func UniversalInvestigationProfileForNetwork(network string) (UniversalInvestigationAdapterProfile, bool) {
	network = strings.ToLower(strings.TrimSpace(network))
	if network == "" {
		return UniversalInvestigationAdapterProfile{}, false
	}
	for _, profile := range UniversalInvestigationAdapterProfiles() {
		for _, candidate := range profile.Networks {
			if strings.EqualFold(candidate, network) {
				return profile, true
			}
		}
	}
	return UniversalInvestigationAdapterProfile{}, false
}

func ClassifyUniversalInvestigationSubject(target, network, kindHint string) (IntelligenceSubject, error) {
	target = strings.TrimSpace(target)
	network = strings.ToLower(strings.TrimSpace(network))
	kindHint = strings.ToLower(strings.TrimSpace(kindHint))
	if target == "" {
		return IntelligenceSubject{}, errors.New("universal target is required")
	}

	if kindHint == "" || kindHint == IntelligenceSubjectAddress {
		subject := ClassifyIntelligenceSubject(target, network)
		if subject.Kind == IntelligenceSubjectUnknown {
			return IntelligenceSubject{}, errors.New("network and target kind are required for non-address subjects")
		}
		return subject, nil
	}
	if !universalInvestigationKindAllowed(kindHint) {
		return IntelligenceSubject{}, errors.New("unsupported universal target kind")
	}

	if kindHint == IntelligenceSubjectIdentity || kindHint == IntelligenceSubjectCredential ||
		kindHint == IntelligenceSubjectAgent || kindHint == IntelligenceSubjectProtocol ||
		kindHint == IntelligenceSubjectArtifact || kindHint == IntelligenceSubjectProject {
		chain := kindHint
		canonical := intelligenceCanonicalRef(IntelligenceChainFamilyOffchain, chain, "global", target)
		return IntelligenceSubject{
			ID: intelligenceStableID(canonical), Raw: target, CanonicalRef: canonical,
			ChainFamily: IntelligenceChainFamilyOffchain, Chain: chain, Network: "global",
			Kind: kindHint, ClassificationBasis: "explicit_non_chain_target_kind",
		}, nil
	}

	profile, ok := UniversalInvestigationProfileForNetwork(network)
	if !ok || profile.ChainFamily == IntelligenceChainFamilyOffchain {
		return IntelligenceSubject{}, errors.New("registered network is required for chain target kind")
	}
	if !universalProfileSupportsKind(profile, kindHint) {
		return IntelligenceSubject{}, errors.New("target kind is not declared for selected network family")
	}
	chain := universalChainName(network, profile.ChainFamily)
	canonical := intelligenceCanonicalRef(profile.ChainFamily, chain, network, target)
	return IntelligenceSubject{
		ID: intelligenceStableID(canonical), Raw: target, CanonicalRef: canonical,
		ChainFamily: profile.ChainFamily, Chain: chain, Network: network,
		Kind: kindHint, ClassificationBasis: "explicit_target_kind_with_registered_network",
	}, nil
}

func universalInvestigationKindAllowed(kind string) bool {
	for _, candidate := range UniversalInvestigationTargetKinds() {
		if candidate == kind {
			return true
		}
	}
	return false
}

func universalProfileSupportsKind(profile UniversalInvestigationAdapterProfile, kind string) bool {
	for _, candidate := range profile.TargetKinds {
		if candidate == kind {
			return true
		}
	}
	return false
}

func universalChainName(network, family string) string {
	network = strings.ToLower(strings.TrimSpace(network))
	switch family {
	case IntelligenceChainFamilySolana:
		return "solana"
	case IntelligenceChainFamilyEVM:
		return evmChainFromNetwork(network)
	case IntelligenceChainFamilyUTXO:
		return "bitcoin"
	case IntelligenceChainFamilyMove:
		if strings.Contains(network, "sui") {
			return "sui"
		}
		if strings.Contains(network, "aptos") {
			return "aptos"
		}
	case IntelligenceChainFamilyCosmos:
		if strings.Contains(network, "osmosis") {
			return "osmosis"
		}
		return "cosmos"
	case IntelligenceChainFamilySubstrate:
		return "polkadot"
	case IntelligenceChainFamilyTON:
		return "ton"
	case IntelligenceChainFamilyNEAR:
		return "near"
	}
	return family
}
