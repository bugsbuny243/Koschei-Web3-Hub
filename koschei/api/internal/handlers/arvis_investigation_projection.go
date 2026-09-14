package handlers

import (
	"encoding/json"
	"fmt"
	"strings"
)

const arvisInvestigationProjectionVersion = "koschei-arvis-investigations-v1"

type arvisInvestigationResult struct {
	ID                 string         `json:"id"`
	Title              string         `json:"title"`
	Question           string         `json:"question"`
	Answer             string         `json:"answer"`
	Facts              []string       `json:"facts"`
	Actors             []string       `json:"actors"`
	Wallets            []string       `json:"wallets"`
	Tokens             []string       `json:"tokens"`
	Transactions       []string       `json:"transactions"`
	Counterparties     []string       `json:"counterparties"`
	Entities           []string       `json:"entities"`
	HistoricalFindings []string       `json:"historical_findings"`
	CurrentFindings    []string       `json:"current_findings"`
	ThreatFindings     []string       `json:"threat_findings"`
	EvidenceRefs       []string       `json:"evidence_refs"`
	Limitations        []string       `json:"limitations"`
	Sources            map[string]any `json:"sources,omitempty"`
}

// buildArvisInvestigationProjection converts the already-collected unified
// investigation report into fourteen concrete research answers. It is additive:
// it does not collect new network evidence, change deterministic rules, grade a
// target, or mutate the signed verdict.
func buildArvisInvestigationProjection(report map[string]any) map[string]any {
	root := normalizeProjectionMap(report)
	actor := projectionMap(root["actor_investigation"])
	dossier := projectionMap(actor["dossier"])
	lifecycle := projectionMap(actor["token_lifecycle_recurrence"])
	exit := projectionMap(actor["exit_event_recurrence"])
	funding := projectionMap(actor["funding_origin"])
	operational := projectionMap(actor["operational_memory"])
	genomeMatches := projectionMap(actor["campaign_genome_matches"])
	incidentHistory := projectionMap(actor["actor_incident_history"])
	behavioral := projectionMap(actor["behavioral_signatures"])
	holder := projectionMap(root["holder_intelligence"])
	cluster := projectionMap(root["holder_cluster"])
	launch := projectionMap(root["launch_forensics"])
	market := projectionMap(root["market"])
	lp := projectionMap(root["lp_control"])
	jupiter := projectionMap(root["jupiter_market_context"])
	exitLiquidity := projectionMap(root["exit_liquidity"])
	program := projectionMap(root["program_security"])
	threat := projectionMap(root["threat_anticipation"])
	transactions := projectionSlice(root["transaction_evidence"])

	creator := projectionString(actor["wallet"])
	currentTarget := projectionString(root["target"])

	results := []arvisInvestigationResult{
		projectionIdentityControl(currentTarget, creator, dossier, program),
		projectionCreatorHistory(creator, lifecycle, dossier),
		projectionTokenLifecycle(creator, lifecycle),
		projectionExitHistory(creator, exit, incidentHistory),
		projectionSybilNetwork(cluster, launch),
		projectionFundingOrigin(creator, funding, cluster),
		projectionMoneyFlow(creator, transactions),
		projectionCollaboratorNetwork(creator, operational, cluster),
		projectionCrossTokenCampaigns(creator, lifecycle, genomeMatches, behavioral),
		projectionCurrentControl(currentTarget, holder, lp, program),
		projectionLaunchIntelligence(currentTarget, launch),
		projectionMarketLiquidity(currentTarget, market, lp, jupiter, exitLiquidity),
		projectionInfrastructureRelations(transactions, market, jupiter, funding),
		projectionThreatCapacity(currentTarget, threat, behavioral),
	}

	return map[string]any{
		"version":        arvisInvestigationProjectionVersion,
		"target":         currentTarget,
		"creator_wallet": creator,
		"result_count":   len(results),
		"results":        results,
		"policy": map[string]any{
			"projection_only":                 true,
			"changes_signed_verdict":          false,
			"changes_deterministic_rules":     false,
			"real_world_identity_claim":       false,
			"same_operator_requires_evidence": true,
			"no_evidence_no_claim":            true,
		},
	}
}

func projectionIdentityControl(target, creator string, dossier, program map[string]any) arvisInvestigationResult {
	facts := []string{}
	if creator != "" {
		facts = append(facts, "Creator/deployer wallet observed: "+creator)
	}
	if status := projectionString(program["status"]); status != "" {
		facts = append(facts, "Program-security status: "+status)
	}
	answer := "Creator/deployer control could not be resolved from the collected evidence."
	if creator != "" {
		answer = "On-chain creator/deployer control is anchored to " + creator + "; real-world identity is not inferred."
	}
	return projectionResult("01", "Identity & Control", "Who created or controls the target on-chain?", answer, facts, map[string]any{"dossier": dossier, "program_security": program})
}

func projectionCreatorHistory(creator string, lifecycle, dossier map[string]any) arvisInvestigationResult {
	total := projectionInt(firstProjectionValue(lifecycle, "total_tokens", "token_count", "created_token_count"))
	other := projectionStrings(firstProjectionValue(lifecycle, "other_mints", "tokens", "mints"))
	if total == 0 && len(other) > 0 {
		total = len(other) + 1
	}
	facts := []string{}
	if total > 0 {
		facts = append(facts, fmt.Sprintf("Observed creator token count: %d", total))
	}
	if len(other) > 0 {
		facts = append(facts, fmt.Sprintf("Other observed mint count: %d", len(other)))
	}
	answer := "No cross-token creator history was available in the collected evidence."
	if total > 0 {
		answer = fmt.Sprintf("The creator has %d token lifecycle observation(s) in persistent evidence.", total)
	}
	return projectionResult("02", "Creator History", "How many tokens has this creator launched before?", answer, facts, map[string]any{"creator_wallet": creator, "lifecycle": lifecycle, "dossier": dossier})
}

func projectionTokenLifecycle(creator string, lifecycle map[string]any) arvisInvestigationResult {
	active := projectionInt(firstProjectionValue(lifecycle, "active_tokens", "active_token_count"))
	inactive := projectionInt(firstProjectionValue(lifecycle, "inactive_or_dead_tokens", "inactive_or_dead_count", "dead_token_count"))
	facts := []string{}
	if active > 0 {
		facts = append(facts, fmt.Sprintf("Active token observations: %d", active))
	}
	if inactive > 0 {
		facts = append(facts, fmt.Sprintf("Inactive/dead token observations: %d", inactive))
	}
	answer := "Token lifecycle history is present only to the extent stored evidence supports it."
	if active+inactive > 0 {
		answer = fmt.Sprintf("Creator lifecycle currently shows %d active and %d inactive/dead token observation(s).", active, inactive)
	}
	return projectionResult("03", "Token Lifecycle History", "What happened to the creator's previous tokens?", answer, facts, map[string]any{"creator_wallet": creator, "lifecycle": lifecycle})
}

func projectionExitHistory(creator string, exit, incidents map[string]any) arvisInvestigationResult {
	events := projectionSlice(firstProjectionValue(exit, "events", "event_references"))
	kinds := projectionStrings(firstProjectionValue(exit, "event_kinds", "kinds"))
	facts := []string{}
	if len(events) > 0 {
		facts = append(facts, fmt.Sprintf("Cross-token exit event references: %d", len(events)))
	}
	if len(kinds) > 0 {
		facts = append(facts, "Observed exit event kinds: "+strings.Join(kinds, ", "))
	}
	answer := "No verified cross-token exit event was available in the current evidence set."
	if len(events) > 0 || len(kinds) > 0 {
		answer = fmt.Sprintf("Persistent actor memory contains %d exit-event reference(s); event labels are evidence descriptions, not automatic real-world wrongdoing claims.", len(events))
	}
	return projectionResult("04", "Exit / Rug Event History", "What verified exit or rug-like events recur across this actor's tokens?", answer, facts, map[string]any{"creator_wallet": creator, "exit_recurrence": exit, "incident_history": incidents})
}

func projectionSybilNetwork(cluster, launch map[string]any) arvisInvestigationResult {
	linked := projectionInt(firstProjectionValue(cluster, "linked_holder_count", "linked_wallet_count", "wallet_count"))
	creatorLinked := projectionInt(launch["creator_linked_count"])
	facts := []string{}
	if linked > 0 {
		facts = append(facts, fmt.Sprintf("Cluster-linked wallet/holder observations: %d", linked))
	}
	if creatorLinked > 0 {
		facts = append(facts, fmt.Sprintf("Creator-linked launch wallets: %d", creatorLinked))
	}
	answer := "No common-control claim is emitted without corroborating funding, timing or direct-transfer evidence."
	if linked+creatorLinked > 0 {
		answer = fmt.Sprintf("The investigation observed %d cluster-linked and %d creator-linked launch wallet observation(s); these links do not by themselves prove common ownership.", linked, creatorLinked)
	}
	return projectionResult("05", "Sybil / Associated Wallet Network", "How many wallets repeatedly move with the target actor or campaign?", answer, facts, map[string]any{"holder_cluster": cluster, "launch_forensics": launch})
}

func projectionFundingOrigin(creator string, funding, cluster map[string]any) arvisInvestigationResult {
	status := projectionString(funding["status"])
	trail := projectionString(funding["trail_status"])
	facts := []string{}
	if status != "" {
		facts = append(facts, "Funding-origin status: "+status)
	}
	if trail != "" {
		facts = append(facts, "Funding trail: "+trail)
	}
	answer := "Funding origin was not resolved from the collected evidence."
	if trail != "" {
		answer = "Funding-origin investigation resolved the trail status as " + trail + "."
	}
	return projectionResult("06", "Funding Origin", "Who funded the creator and principal actor wallets?", answer, facts, map[string]any{"creator_wallet": creator, "funding_origin": funding, "holder_cluster": cluster})
}

func projectionMoneyFlow(creator string, transactions []any) arvisInvestigationResult {
	facts := []string{fmt.Sprintf("Transaction evidence rows available for flow analysis: %d", len(transactions))}
	answer := "No transaction flow rows were available for sender/recipient projection."
	if len(transactions) > 0 {
		answer = fmt.Sprintf("%d transaction evidence row(s) are available to reconstruct incoming and outgoing actor flows.", len(transactions))
	}
	return projectionResult("07", "Money Flow", "Who sent value to the actor and where did the actor send value?", answer, facts, map[string]any{"creator_wallet": creator, "transaction_evidence": transactions})
}

func projectionCollaboratorNetwork(creator string, operational, cluster map[string]any) arvisInvestigationResult {
	matches := projectionSlice(operational["matches"])
	facts := []string{}
	if len(matches) > 0 {
		facts = append(facts, fmt.Sprintf("Persistent operational overlap matches: %d", len(matches)))
	}
	answer := "No persistent collaborator relation was available in the current evidence set."
	if len(matches) > 0 {
		answer = fmt.Sprintf("Persistent memory contains %d operational-overlap match(es); overlap proves interaction patterns only, not shared real-world identity.", len(matches))
	}
	return projectionResult("08", "Collaborator Network", "Which wallets repeatedly fund, launch, transfer, exit or operate with this actor?", answer, facts, map[string]any{"creator_wallet": creator, "operational_memory": operational, "holder_cluster": cluster})
}

func projectionCrossTokenCampaigns(creator string, lifecycle, genomeMatches, behavioral map[string]any) arvisInvestigationResult {
	matches := projectionSlice(genomeMatches["matches"])
	other := projectionStrings(lifecycle["other_mints"])
	facts := []string{}
	if len(matches) > 0 {
		facts = append(facts, fmt.Sprintf("Campaign-genome pattern matches: %d", len(matches)))
	}
	if len(other) > 0 {
		facts = append(facts, fmt.Sprintf("Other creator mints in lifecycle memory: %d", len(other)))
	}
	answer := "No cross-token campaign recurrence was available in the current evidence set."
	if len(matches)+len(other) > 0 {
		answer = fmt.Sprintf("Cross-token memory contains %d campaign-pattern match(es) and %d other creator mint(s).", len(matches), len(other))
	}
	return projectionResult("09", "Cross-Token Campaign History", "Where does the same actor or operational pattern recur across tokens?", answer, facts, map[string]any{"creator_wallet": creator, "lifecycle": lifecycle, "campaign_genome_matches": genomeMatches, "behavioral_signatures": behavioral})
}

func projectionCurrentControl(target string, holder, lp, program map[string]any) arvisInvestigationResult {
	top1 := projectionFloat(firstProjectionValue(holder, "top1_percentage", "largest_holder_percentage", "largest_holder_percent"))
	facts := []string{}
	if top1 > 0 {
		facts = append(facts, fmt.Sprintf("Owner-resolved largest-holder share: %.4f%%", top1))
	}
	answer := "Current holder, LP and authority evidence is retained as the control surface for the target."
	if top1 > 0 {
		answer = fmt.Sprintf("The largest owner-resolved holder controls approximately %.4f%% of the analyzed circulating supply; LP and program-control evidence are attached separately.", top1)
	}
	return projectionResult("10", "Current Token Control", "Who can materially control the token today?", answer, facts, map[string]any{"target": target, "holder_intelligence": holder, "lp_control": lp, "program_security": program})
}

func projectionLaunchIntelligence(target string, launch map[string]any) arvisInvestigationResult {
	snipers := projectionInt(launch["sniper_count"])
	bots := projectionInt(launch["rhythm_bot_count"])
	creatorLinked := projectionInt(launch["creator_linked_count"])
	owners := projectionInt(launch["owners_with_trade_history"])
	facts := []string{fmt.Sprintf("Launch owners with trade history: %d", owners), fmt.Sprintf("Sniper=%d, rhythm-bot=%d, creator-linked=%d", snipers, bots, creatorLinked)}
	answer := fmt.Sprintf("Launch analysis observed %d owner history record(s), including %d sniper, %d rhythm-bot and %d creator-linked profile(s).", owners, snipers, bots, creatorLinked)
	return projectionResult("11", "Launch Intelligence", "How was the token launched and who appeared in the first distribution window?", answer, facts, map[string]any{"target": target, "launch_forensics": launch})
}

func projectionMarketLiquidity(target string, market, lp, jupiter, exitLiquidity map[string]any) arvisInvestigationResult {
	liquidity := projectionFloat(firstProjectionValue(market, "liquidity_usd", "best_pair_liquidity_usd"))
	facts := []string{}
	if liquidity > 0 {
		facts = append(facts, fmt.Sprintf("Observed market liquidity: $%.2f", liquidity))
	}
	answer := "Market depth, LP control and quote-only exit capacity are reported separately so missing attribution is not mistaken for safety."
	if liquidity > 0 {
		answer = fmt.Sprintf("Observed liquidity is approximately $%.2f; LP ownership/control and exit-liquidity evidence remain separately attributable.", liquidity)
	}
	return projectionResult("12", "Market & Liquidity Behavior", "How deep is the market and who can add, remove or exit through liquidity?", answer, facts, map[string]any{"target": target, "market": market, "lp_control": lp, "jupiter": jupiter, "exit_liquidity": exitLiquidity})
}

func projectionInfrastructureRelations(transactions []any, market, jupiter, funding map[string]any) arvisInvestigationResult {
	entities := []string{}
	if dex := projectionString(firstProjectionValue(market, "best_pair_dex", "dex")); dex != "" {
		entities = appendUniqueProjectionString(entities, dex)
	}
	if provider := projectionString(jupiter["provider"]); provider != "" {
		entities = appendUniqueProjectionString(entities, provider)
	}
	trail := projectionString(funding["trail_status"])
	facts := []string{}
	if len(entities) > 0 {
		facts = append(facts, "Observed market/infrastructure entities: "+strings.Join(entities, ", "))
	}
	if strings.Contains(strings.ToLower(trail), "cex") {
		facts = append(facts, "Funding trail reaches a CEX-classified endpoint; account identity remains opaque.")
	}
	answer := "Infrastructure relations are limited to provider/entity evidence actually observed on-chain or by trusted market context."
	if len(entities) > 0 || strings.Contains(strings.ToLower(trail), "cex") {
		answer = "Observed DEX/CEX/service relations are attached without treating shared infrastructure as proof of common ownership."
	}
	result := projectionResult("13", "Exchange / Infrastructure Relations", "Which DEX, CEX, bridge or service infrastructure appears in the actor's flows?", answer, facts, map[string]any{"market": market, "jupiter": jupiter, "funding_origin": funding, "transaction_evidence": transactions})
	result.Entities = entities
	return result
}

func projectionThreatCapacity(target string, threat, behavioral map[string]any) arvisInvestigationResult {
	status := projectionString(threat["status"])
	facts := []string{}
	if status != "" {
		facts = append(facts, "Threat-anticipation status: "+status)
	}
	answer := "No evidence-backed future pathway was available; ARVIS does not infer intent or assign a rug probability."
	if status != "" && status != "not_investigated" && status != "unavailable" {
		answer = "Threat anticipation exposes evidence-backed attack/exit capacity only; it does not predict intent or guarantee a future action."
	}
	result := projectionResult("14", "Threat / Next-Move Capacity", "Given the actor's history and current control, what evidence-backed action pathways are presently possible?", answer, facts, map[string]any{"target": target, "threat_anticipation": threat, "behavioral_signatures": behavioral})
	result.ThreatFindings = projectionStrings(firstProjectionValue(threat, "findings", "pathways", "summaries"))
	return result
}

func projectionResult(id, title, question, answer string, facts []string, sources map[string]any) arvisInvestigationResult {
	return arvisInvestigationResult{
		ID: id, Title: title, Question: question, Answer: answer,
		Facts: cleanProjectionStrings(facts), Actors: []string{}, Wallets: []string{}, Tokens: []string{}, Transactions: []string{},
		Counterparties: []string{}, Entities: []string{}, HistoricalFindings: []string{}, CurrentFindings: []string{}, ThreatFindings: []string{},
		EvidenceRefs: []string{}, Limitations: []string{}, Sources: sources,
	}
}

func normalizeProjectionMap(value any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	if direct, ok := value.(map[string]any); ok {
		return direct
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil || out == nil {
		return map[string]any{}
	}
	return out
}

func projectionMap(value any) map[string]any { return normalizeProjectionMap(value) }

func projectionSlice(value any) []any {
	if value == nil {
		return []any{}
	}
	if direct, ok := value.([]any); ok {
		return direct
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return []any{}
	}
	var out []any
	if err := json.Unmarshal(raw, &out); err != nil || out == nil {
		return []any{}
	}
	return out
}

func firstProjectionValue(values map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := values[key]; ok && value != nil {
			return value
		}
	}
	return nil
}

func projectionString(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case fmt.Stringer:
		return strings.TrimSpace(typed.String())
	default:
		return ""
	}
}

func projectionInt(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		parsed, _ := typed.Int64()
		return int(parsed)
	default:
		return 0
	}
}

func projectionFloat(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case json.Number:
		parsed, _ := typed.Float64()
		return parsed
	default:
		return 0
	}
}

func projectionStrings(value any) []string {
	if value == nil {
		return []string{}
	}
	if direct, ok := value.([]string); ok {
		return cleanProjectionStrings(direct)
	}
	items := projectionSlice(value)
	out := []string{}
	for _, item := range items {
		if text := projectionString(item); text != "" {
			out = appendUniqueProjectionString(out, text)
		}
	}
	return out
}

func cleanProjectionStrings(values []string) []string {
	out := []string{}
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			out = appendUniqueProjectionString(out, trimmed)
		}
	}
	return out
}

func appendUniqueProjectionString(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
