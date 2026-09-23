package services

import "testing"

func bridgeObservation(subject IntelligenceSubject, evidenceID, network, chain, txHash, protocol, transferID string) GlobalRadarObservation {
	subject.ID = intelligenceStableID(network + ":" + subject.Raw)
	subject.Network = network
	subject.Chain = chain
	subject.CanonicalRef = intelligenceCanonicalRef(subject.ChainFamily, chain, network, subject.Raw)
	evidence := testGlobalRadarEvidence(subject)
	evidence.ID = evidenceID
	evidence.Network = network
	evidence.Chain = chain
	evidence.TransactionHash = txHash
	evidence.Attributes = map[string]any{
		"bridge_protocol":    protocol,
		"bridge_transfer_id": transferID,
	}
	observation, err := BuildGlobalRadarObservation(GlobalRadarObservationTransaction, subject, evidence)
	if err != nil {
		panic(err)
	}
	return observation
}

func TestBuildGlobalRadarBridgeLinkRequiresExplicitIdentityOnBothChains(t *testing.T) {
	sourceSubject := testGlobalRadarSubject()
	destinationSubject := sourceSubject
	source := bridgeObservation(sourceSubject, "ev-source", "ethereum-mainnet", "ethereum", "0xsource", "wormhole", "vaa:2/abc/7")
	destination := bridgeObservation(destinationSubject, "ev-destination", "base-mainnet", "base", "0xdestination", "wormhole", "vaa:2/abc/7")

	got, err := BuildGlobalRadarBridgeLink(source, destination)
	if err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != GlobalRadarBridgeLinkSchemaVersion || got.LinkID == "" {
		t.Fatalf("unexpected bridge link identity: %#v", got)
	}
	if got.Relation.Status != IntelligenceEvidenceVerified || !got.Relation.CrossNetwork {
		t.Fatalf("bridge relation not verified as cross-network: %#v", got.Relation)
	}
	if got.LinkEvidence.Status != IntelligenceEvidenceVerified ||
		got.LinkEvidence.Source != "global_radar_bridge_link" ||
		got.VerificationBoundary != "transfer_link_only_no_actor_identity_or_intent_claim" {
		t.Fatalf("unexpected link evidence boundary: %#v", got)
	}
}

func TestBuildGlobalRadarBridgeLinkRejectsTimingOrProtocolGuessing(t *testing.T) {
	subject := testGlobalRadarSubject()
	source := bridgeObservation(subject, "ev-source", "ethereum-mainnet", "ethereum", "0xsource", "wormhole", "vaa:2/abc/7")
	destination := bridgeObservation(subject, "ev-destination", "base-mainnet", "base", "0xdestination", "layerzero", "vaa:2/abc/7")
	if _, err := BuildGlobalRadarBridgeLink(source, destination); err == nil {
		t.Fatal("different bridge protocols were correlated")
	}

	destination = bridgeObservation(subject, "ev-destination-2", "base-mainnet", "base", "0xdestination2", "wormhole", "vaa:2/abc/8")
	if _, err := BuildGlobalRadarBridgeLink(source, destination); err == nil {
		t.Fatal("different bridge transfer ids were correlated")
	}
}

func TestBuildGlobalRadarBridgeLinkRejectsMissingTransactionEvidence(t *testing.T) {
	subject := testGlobalRadarSubject()
	source := bridgeObservation(subject, "ev-source", "ethereum-mainnet", "ethereum", "0xsource", "wormhole", "vaa:2/abc/7")
	destination := bridgeObservation(subject, "ev-destination", "base-mainnet", "base", "0xdestination", "wormhole", "vaa:2/abc/7")
	destination.Evidence.TransactionHash = ""
	if _, err := BuildGlobalRadarBridgeLink(source, destination); err == nil {
		t.Fatal("bridge link accepted destination without transaction hash")
	}
}

func TestBuildGlobalRadarBridgeLinkDoesNotTreatSameNetworkAsBridge(t *testing.T) {
	subject := testGlobalRadarSubject()
	source := bridgeObservation(subject, "ev-source", "ethereum-mainnet", "ethereum", "0xsource", "wormhole", "vaa:2/abc/7")
	destination := bridgeObservation(subject, "ev-destination", "ethereum-mainnet", "ethereum", "0xdestination", "wormhole", "vaa:2/abc/7")
	if _, err := BuildGlobalRadarBridgeLink(source, destination); err == nil {
		t.Fatal("same-network observations were accepted as bridge link")
	}
}
