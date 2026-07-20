package services

import "testing"

func TestHolderFlowClassifiesCEXDirectionsAndEntities(t *testing.T) {
	holders := map[string]bool{"HolderA": true, "HolderB": true}
	binance := &WalletLabel{Address: "CEXWallet", Entity: "Binance", Category: "CEX", Source: "helius_identity"}

	outbound := enrichHolderClusterFlowObservationWithLabels(HolderClusterFlowObservation{
		SourceWallet: "HolderA", Destination: "CEXWallet", Direction: "outbound", Kind: "external_token_recipient",
	}, holders, nil, binance)
	if outbound.TransferType != "CEX_OUT" || outbound.ToEntity != "Binance" || outbound.ToEntityCategory != "CEX" {
		t.Fatalf("unexpected CEX out classification: %#v", outbound)
	}

	inbound := enrichHolderClusterFlowObservationWithLabels(HolderClusterFlowObservation{
		SourceWallet: "CEXWallet", Destination: "HolderA", Direction: "inbound", Kind: "inbound_token_sender_context",
	}, holders, binance, nil)
	if inbound.TransferType != "CEX_IN" || inbound.FromEntity != "Binance" || inbound.FromEntitySource != "helius_identity" {
		t.Fatalf("unexpected CEX in classification: %#v", inbound)
	}

	internal := enrichHolderClusterFlowObservationWithLabels(HolderClusterFlowObservation{
		SourceWallet: "HolderA", Destination: "HolderB", Kind: "holder_to_holder",
	}, holders, nil, nil)
	if internal.TransferType != "INTERNAL" {
		t.Fatalf("unexpected internal classification: %#v", internal)
	}
}

func TestHolderFlowRiskFlagsRequireExplicitIdentityTaxonomy(t *testing.T) {
	holders := map[string]bool{"HolderA": true}
	drainer := &WalletLabel{Address: "BadWallet", Entity: "Unknown service", Category: "SERVICE", Labels: []string{"wallet_drainer"}, Source: "helius_identity"}
	observation := enrichHolderClusterFlowObservationWithLabels(HolderClusterFlowObservation{
		SourceWallet: "HolderA", Destination: "BadWallet", Direction: "outbound", Kind: "external_token_recipient",
	}, holders, nil, drainer)
	if observation.RiskFlag != "DRAINER" || observation.RiskFlagEndpoint != "destination" || observation.RiskFlagSource != "helius_identity" {
		t.Fatalf("explicit drainer taxonomy was not preserved: %#v", observation)
	}

	mixer := &WalletLabel{Address: "MixerWallet", Category: "MIXER", Source: "helius_identity"}
	observation = enrichHolderClusterFlowObservationWithLabels(HolderClusterFlowObservation{
		SourceWallet: "MixerWallet", Destination: "HolderA", Direction: "inbound", Kind: "inbound_token_sender_context",
	}, holders, mixer, nil)
	if observation.RiskFlag != "MIXER" || observation.RiskFlagEndpoint != "source" {
		t.Fatalf("explicit mixer taxonomy was not preserved: %#v", observation)
	}

	benign := &WalletLabel{Address: "ResearchWallet", Entity: "Scam Research Lab", Category: "SECURITY_RESEARCH", Source: "helius_identity"}
	observation = enrichHolderClusterFlowObservationWithLabels(HolderClusterFlowObservation{
		SourceWallet: "HolderA", Destination: "ResearchWallet", Direction: "outbound", Kind: "external_token_recipient",
	}, holders, nil, benign)
	if observation.RiskFlag != "" {
		t.Fatalf("entity name alone must not create a risk flag: %#v", observation)
	}
}

func TestHolderFlowDEXClassificationPrecedesCEX(t *testing.T) {
	holders := map[string]bool{"HolderA": true}
	cex := &WalletLabel{Address: "CEXWallet", Entity: "OKX", Category: "CEX", Source: "helius_identity"}
	observation := enrichHolderClusterFlowObservationWithLabels(HolderClusterFlowObservation{
		SourceWallet: "HolderA", Destination: "CEXWallet", Direction: "outbound", Kind: "external_token_recipient", ProgramIDs: []string{pumpLiquidityProgramID},
	}, holders, nil, cex)
	if observation.TransferType != "DEX" {
		t.Fatalf("DEX route context must take precedence over endpoint entity: %#v", observation)
	}
}
