from pathlib import Path


def replace_once(text: str, old: str, new: str, label: str) -> str:
    if old not in text:
        raise SystemExit(f"missing replacement target: {label}")
    return text.replace(old, new, 1)

path = Path("internal/services/holder_cluster_flow_intelligence.go")
text = path.read_text()
text = replace_once(
    text,
    '''\t\tcase "external_token_recipient":
\t\t\tif commonExitSources[observation.Destination] == nil {
\t\t\t\tcommonExitSources[observation.Destination] = map[string]bool{}
\t\t\t}
\t\t\tcommonExitSources[observation.Destination][observation.SourceWallet] = true
''',
    '''\t\tcase "external_token_recipient":
\t\t\t// A shared recipient inside a known DEX/pool route is commonly a vault
\t\t\t// or pool authority. Keep it as route context, but never score it as
\t\t\t// evidence of coordinated control.
\t\t\tif len(observation.ProgramIDs) > 0 {
\t\t\t\tcontinue
\t\t\t}
\t\t\tif commonExitSources[observation.Destination] == nil {
\t\t\t\tcommonExitSources[observation.Destination] = map[string]bool{}
\t\t\t}
\t\t\tcommonExitSources[observation.Destination][observation.SourceWallet] = true
''',
    "DEX common-exit guard",
)
path.write_text(text)

path = Path("internal/services/holder_cluster_flow_intelligence_test.go")
text = path.read_text()
anchor = '''func TestHolderClusterTokenOwnerDeltas(t *testing.T) {
'''
test = '''func TestHolderClusterFlowDoesNotScoreSharedDEXRecipientOwner(t *testing.T) {
\twallets := []HolderClusterWallet{
\t\t{Wallet: "WalletA", HolderPercentage: 10, FlowObservations: []HolderClusterFlowObservation{{SourceWallet: "WalletA", Destination: "PoolAuthority", Kind: "external_token_recipient", Amount: 2, Signature: "sig-a", ProgramIDs: []string{pumpLiquidityProgramID}}}},
\t\t{Wallet: "WalletB", HolderPercentage: 12, FlowObservations: []HolderClusterFlowObservation{{SourceWallet: "WalletB", Destination: "PoolAuthority", Kind: "external_token_recipient", Amount: 3, Signature: "sig-b", ProgramIDs: []string{pumpLiquidityProgramID}}}},
\t}
\tflow := summarizeHolderClusterFlow(wallets)
\tif flow.CommonExitGroupCount != 0 || flow.RiskContribution != 0 {
\t\tt.Fatalf("shared DEX recipient owner must remain route context only: %#v", flow)
\t}
}

'''
text = replace_once(text, anchor, test + anchor, "DEX recipient-owner test")
path.write_text(text)
