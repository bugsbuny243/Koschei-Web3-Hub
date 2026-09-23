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
	IntelligenceSubjectProgram     = "program"
	IntelligenceSubjectTransaction = "transaction"
	IntelligenceSubjectBlock       = "block"
	IntelligenceSubjectValidator   = "validator"
	IntelligenceSubjectBridge      = "bridge"
	IntelligenceSubjectGovernance  = "governance"
	IntelligenceSubjectProtocol    = "protocol"
	IntelligenceSubjectArtifact    = "artifact"
	IntelligenceSubjectProject     = "project"

	UniversalAdapterLive     = "live"
	UniversalAdapterProbe    = "probe"
	UniversalAdapterPlanned  = "planned"
	UniversalAdapterResearch = "research"
)

type UniversalInvestigationAdapterProfile struct {
	ID                   string   `json:"id"`
	Domain               string   `json:"domain"`
	ChainFamily          string   `json:"chain_family"`
	Status               string   `json:"status"`
	NetworkScope         string   `json:"network_scope"`
	Networks             []string `json:"networks,omitempty"`
	TargetKinds          []string `json:"target_kinds"`
	EvidenceSources      []string `json:"evidence_sources"`
	RequiredTrustAnchors []string `json:"required_trust_anchors"`
	VerdictPolicy        string   `json:"verdict_policy"`
}

func universalInvestigationProfile(
	id, domain, chainFamily, status, networkScope string,
	networks, targetKinds, evidenceSources, trustAnchors []string,
	verdictPolicy string,
) UniversalInvestigationAdapterProfile {
	return UniversalInvestigationAdapterProfile{
		ID: id, Domain: domain, ChainFamily: chainFamily, Status: status, NetworkScope: networkScope,
		Networks: networks, TargetKinds: targetKinds, EvidenceSources: evidenceSources,
		RequiredTrustAnchors: trustAnchors, VerdictPolicy: verdictPolicy,
	}
}

func UniversalInvestigationAdapterProfiles() []UniversalInvestigationAdapterProfile {
	return []UniversalInvestigationAdapterProfile{
		universalInvestigationProfile(
			"solana-account-model", "web3", IntelligenceChainFamilySolana, UniversalAdapterLive, "registered_networks",
			[]string{"solana-mainnet"},
			[]string{
				IntelligenceSubjectAddress, IntelligenceSubjectToken, IntelligenceSubjectProgram,
				IntelligenceSubjectTransaction, IntelligenceSubjectBlock, IntelligenceSubjectValidator,
				IntelligenceSubjectBridge, IntelligenceSubjectGovernance,
			},
			[]string{"solana_jsonrpc", "parsed_transactions", "program_state", "actor_memory"},
			[]string{"genesis_or_network_identity", "slot", "signature", "program_id"},
			"Existing deterministic ARVIS rules may affect a signed verdict only when their required Solana evidence is VERIFIED.",
		),
		universalInvestigationProfile(
			"evm-account-model", "web3", IntelligenceChainFamilyEVM, UniversalAdapterProbe, "eip155_family",
			[]string{"ethereum-mainnet", "base-mainnet", "arbitrum-mainnet", "optimism-mainnet", "polygon-mainnet", "bnb-mainnet", "avalanche-mainnet"},
			[]string{
				IntelligenceSubjectAddress, IntelligenceSubjectToken, IntelligenceSubjectContract,
				IntelligenceSubjectTransaction, IntelligenceSubjectBlock, IntelligenceSubjectBridge,
				IntelligenceSubjectGovernance,
			},
			[]string{"ethereum_jsonrpc", "contract_code", "storage", "logs", "receipts", "eip7702", "erc1967"},
			[]string{"eth_chainId", "block_hash_or_number", "transaction_hash", "contract_address"},
			"Current EVM probes remain evidence-only until chain-specific deterministic verdict rules are promoted.",
		),
		universalInvestigationProfile(
			"bitcoin-utxo-model", "web3", IntelligenceChainFamilyUTXO, UniversalAdapterProbe, "bitcoin_family",
			[]string{"bitcoin-mainnet"},
			[]string{IntelligenceSubjectAddress, IntelligenceSubjectTransaction, IntelligenceSubjectBlock},
			[]string{"bitcoin_core_rpc_or_esplora", "utxo_set", "transaction_graph", "block_headers"},
			[]string{"genesis_hash", "block_hash", "transaction_id", "script_pubkey"},
			"UTXO observations remain evidence-only until deterministic UTXO behavior rules are promoted.",
		),
		universalInvestigationProfile(
			"move-object-model", "web3", IntelligenceChainFamilyMove, UniversalAdapterProbe, "move_family",
			[]string{"sui-mainnet", "aptos-mainnet"},
			[]string{
				IntelligenceSubjectAddress, IntelligenceSubjectToken, IntelligenceSubjectContract,
				IntelligenceSubjectTransaction, IntelligenceSubjectBlock, IntelligenceSubjectValidator,
				IntelligenceSubjectGovernance,
			},
			[]string{"sui_graphql_or_rpc", "aptos_rest", "objects_resources_modules", "events", "transactions"},
			[]string{"chain_identity", "checkpoint_or_ledger_version", "transaction_digest_or_hash", "object_or_resource_id"},
			"Sui/Aptos address probes verify canonical target syntax and selected mainnet identity only. They remain evidence-only and cannot affect a deterministic grade until chain-specific address/object/resource/transaction rules are promoted.",
		),
		universalInvestigationProfile(
			"cosmos-cometbft-model", "web3", IntelligenceChainFamilyCosmos, UniversalAdapterPlanned, "cosmos_sdk_cometbft_family",
			[]string{"cosmoshub-mainnet", "osmosis-mainnet"},
			[]string{
				IntelligenceSubjectAddress, IntelligenceSubjectToken, IntelligenceSubjectContract,
				IntelligenceSubjectTransaction, IntelligenceSubjectBlock, IntelligenceSubjectValidator,
				IntelligenceSubjectBridge, IntelligenceSubjectGovernance,
			},
			[]string{"cometbft_rpc", "cosmos_sdk_query", "abci_state", "events", "ibc"},
			[]string{"chain_id", "block_height_hash", "transaction_hash", "module_or_contract_id"},
			"IBC or cross-zone linkage must remain OBSERVED/UNVERIFIED until both source and destination evidence are independently anchored.",
		),
		universalInvestigationProfile(
			"substrate-runtime-model", "web3", IntelligenceChainFamilySubstrate, UniversalAdapterPlanned, "substrate_family",
			[]string{"polkadot-mainnet"},
			[]string{
				IntelligenceSubjectAddress, IntelligenceSubjectContract, IntelligenceSubjectTransaction,
				IntelligenceSubjectBlock, IntelligenceSubjectValidator, IntelligenceSubjectBridge,
				IntelligenceSubjectGovernance,
			},
			[]string{"polkadot_jsonrpc", "runtime_metadata", "storage", "events", "extrinsics"},
			[]string{"chain_spec_or_genesis", "block_hash", "runtime_version", "extrinsic_or_event_reference"},
			"Runtime upgrades must be version-bound; metadata from one runtime must not be applied to another without compatibility proof.",
		),
		universalInvestigationProfile(
			"ton-account-message-model", "web3", IntelligenceChainFamilyTON, UniversalAdapterPlanned, "ton_family",
			[]string{"ton-mainnet"},
			[]string{
				IntelligenceSubjectAddress, IntelligenceSubjectToken, IntelligenceSubjectContract,
				IntelligenceSubjectTransaction, IntelligenceSubjectBlock, IntelligenceSubjectValidator,
				IntelligenceSubjectBridge,
			},
			[]string{"ton_api", "account_state", "messages", "transactions", "shard_blocks"},
			[]string{"workchain", "account_id", "block_or_transaction_anchor", "message_hash"},
			"Message intent and transaction effect remain separate; a message alone is not proof that the requested state transition completed.",
		),
		universalInvestigationProfile(
			"near-account-wasm-model", "web3", IntelligenceChainFamilyNEAR, UniversalAdapterPlanned, "near_family",
			[]string{"near-mainnet"},
			[]string{
				IntelligenceSubjectAddress, IntelligenceSubjectToken, IntelligenceSubjectContract,
				IntelligenceSubjectTransaction, IntelligenceSubjectBlock, IntelligenceSubjectValidator,
				IntelligenceSubjectBridge, IntelligenceSubjectGovernance,
			},
			[]string{"near_jsonrpc", "account_state", "wasm_code", "receipts", "transactions"},
			[]string{"chain_id", "block_hash_or_height", "transaction_or_receipt_id", "account_id"},
			"Receipt execution and final outcome must be verified separately from transaction submission.",
		),
		universalInvestigationProfile(
			"web5-identity-evidence", "web5", IntelligenceChainFamilyOffchain, UniversalAdapterResearch, "standards_family",
			nil,
			[]string{IntelligenceSubjectIdentity, IntelligenceSubjectCredential},
			[]string{"w3c_did", "did_resolution", "w3c_verifiable_credentials", "data_integrity"},
			[]string{"did_method", "verification_method", "proof_or_signature", "status_or_revocation_state"},
			"Verified identity proves only the credential/controller statement it actually covers; it never grants unlimited authority or a safety verdict.",
		),
		universalInvestigationProfile(
			"web4-agent-evidence", "web4", IntelligenceChainFamilyOffchain, UniversalAdapterResearch, "agent_protocol_family",
			nil,
			[]string{IntelligenceSubjectAgent, IntelligenceSubjectProtocol, IntelligenceSubjectArtifact},
			[]string{"mcp", "a2a", "oauth_authorization", "signed_provenance", "tool_and_task_telemetry"},
			[]string{"agent_or_server_identity", "capability_declaration", "authorization_context", "request_response_provenance"},
			"Agent capability, authorization, execution and effect are separate evidence axes; tool availability must never imply authorization.",
		),
		universalInvestigationProfile(
			"web6-crypto-agility-evidence", "web6", IntelligenceChainFamilyOffchain, UniversalAdapterResearch, "crypto_agility_profile",
			nil,
			[]string{IntelligenceSubjectProtocol, IntelligenceSubjectArtifact, IntelligenceSubjectProject},
			[]string{"algorithm_inventory", "signature_provenance", "key_lifecycle", "pq_transition_evidence"},
			[]string{"algorithm_identifier", "key_or_certificate_reference", "signed_artifact_hash", "migration_state"},
			"Post-quantum readiness is an evidence-backed migration state, not a binary safety label.",
		),
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

	if kindHint == "" {
		subject := ClassifyIntelligenceSubject(target, network)
		if subject.Kind == IntelligenceSubjectUnknown {
			return IntelligenceSubject{}, errors.New("network and target kind are required for non-address subjects")
		}
		return subject, nil
	}
	if kindHint == IntelligenceSubjectAddress {
		subject := ClassifyIntelligenceSubject(target, network)
		if subject.Kind != IntelligenceSubjectUnknown {
			return subject, nil
		}
		profile, ok := UniversalInvestigationProfileForNetwork(network)
		if !ok || profile.Status != UniversalAdapterPlanned || !universalProfileSupportsKind(profile, kindHint) {
			return IntelligenceSubject{}, errors.New("address syntax is not recognized by a live/probe adapter")
		}
		chain := universalChainName(network, profile.ChainFamily)
		canonical := intelligenceCanonicalRef(profile.ChainFamily, chain, network, target)
		return IntelligenceSubject{
			ID: intelligenceStableID(canonical), Raw: target, CanonicalRef: canonical,
			ChainFamily: profile.ChainFamily, Chain: chain, Network: network,
			Kind: kindHint, ClassificationBasis: "explicit_planned_target_kind_unvalidated",
		}, nil
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
