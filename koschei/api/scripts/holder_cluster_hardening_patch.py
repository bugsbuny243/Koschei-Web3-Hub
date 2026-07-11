from pathlib import Path

p = Path('internal/services/holder_cluster_intelligence.go')
s = p.read_text()
s = s.replace('''\tSignaturesObserved    int      `json:"signatures_observed"`
\tHistoryExhausted''', '''\tSignaturesObserved    int      `json:"signatures_observed"`
\tParsedTransactions   int      `json:"parsed_transactions"`
\tHistoryExhausted''', 1)
s = s.replace('''\t\ttxMap := map[string]any(tx)
\t\tblockTime :=''', '''\t\trow.ParsedTransactions++
\t\ttxMap := map[string]any(tx)
\t\tblockTime :=''', 1)
s = s.replace('''\trow.Status = "verified_bounded_observation"
\trow.Evidence = append(row.Evidence, fmt.Sprintf("Observed %d signatures; history exhausted within query window: %t.", row.SignaturesObserved, row.HistoryExhausted))''', '''\tif row.ParsedTransactions > 0 {
\t\trow.Status = "verified_bounded_observation"
\t} else {
\t\trow.Status = "signature_only_observation"
\t}
\trow.Evidence = append(row.Evidence, fmt.Sprintf("Observed %d signatures and parsed %d transactions; history exhausted within query window: %t.", row.SignaturesObserved, row.ParsedTransactions, row.HistoryExhausted))''', 1)
s = s.replace('''\t\tif wallet.Status == "verified_bounded_observation" {
\t\t\tout.WalletsAnalyzed++
\t\t}''', '''\t\tif wallet.Status == "verified_bounded_observation" && wallet.ParsedTransactions > 0 {
\t\t\tout.WalletsAnalyzed++
\t\t}''', 1)
s = s.replace('''Fewer than three holder wallets produced verified bounded observations; no LOW verdict is issued.''', '''Fewer than three holder wallets produced parsed transaction evidence; no LOW verdict is issued.''', 1)
p.write_text(s)

p = Path('internal/services/holder_cluster_intelligence_test.go')
s = p.read_text()
s = s.replace('Status: "verified_bounded_observation", FreshNearLaunch:', 'Status: "verified_bounded_observation", ParsedTransactions: 1, FreshNearLaunch:')
s = s.replace('{Wallet: "A", Status: "verified_bounded_observation"}', '{Wallet: "A", Status: "verified_bounded_observation", ParsedTransactions: 1}')
p.write_text(s)
