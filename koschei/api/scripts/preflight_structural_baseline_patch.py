from pathlib import Path


def replace_once(text: str, old: str, new: str, label: str) -> str:
    if old not in text:
        raise SystemExit(f"missing replacement target: {label}")
    return text.replace(old, new, 1)

structural_path = Path("internal/services/security_radar_structural.go")
structural = structural_path.read_text()
if "func (s *SecurityRadarStore) StructuralBaseline" in structural:
    raise SystemExit("StructuralBaseline already exists")
structural += r'''

// StructuralBaseline exposes the strongest fresh, verified structural floor
// for quick read paths such as Safe Check. A quick heuristic answer must never
// contradict stronger holder/authority evidence Koschei already verified.
func (s *SecurityRadarStore) StructuralBaseline(ctx context.Context, target, network string) (int, string, time.Time, bool) {
	if s == nil || s.DB == nil {
		return 0, "", time.Time{}, false
	}
	target = strings.TrimSpace(target)
	if target == "" || IsSecurityRadarInfraTarget(target) {
		return 0, "", time.Time{}, false
	}

	cacheCtx, cancel := context.WithTimeout(ctx, structuralCacheTimeout)
	defer cancel()
	var cached tokenStructuralSignals
	err := s.DB.QueryRowContext(cacheCtx, `
		SELECT largest_holder_pct, top10_holder_pct, has_holder_data,
		       mint_authority_present, freeze_authority_present, has_authority_data,
		       holder_observed_at, authority_observed_at
		FROM token_structural_signals
		WHERE target = $1 AND network = $2`, target, normalizeRadarNetwork(network)).Scan(
		&cached.LargestHolderPct, &cached.Top10HolderPct, &cached.HasHolderData,
		&cached.MintAuthorityPresent, &cached.FreezeAuthorityPresent, &cached.HasAuthorityData,
		&cached.HolderObservedAt, &cached.AuthorityObservedAt)
	if err != nil {
		return 0, "", time.Time{}, false
	}

	floor, observedAt := cached.structuralFloor(time.Now().UTC())
	if floor <= 0 || observedAt.IsZero() {
		return 0, "", time.Time{}, false
	}
	return floor, riskLevelFromIndex(floor), observedAt, true
}
'''
structural_path.write_text(structural)

preflight_path = Path("internal/handlers/arvis_preflight.go")
preflight = preflight_path.read_text()
preflight = replace_once(
    preflight,
    'import (\n\t"net/http"',
    'import (\n\t"context"\n\t"net/http"',
    "context import",
)
preflight = replace_once(
    preflight,
    '\t"regexp"\n\t"strings"',
    '\t"regexp"\n\t"strconv"\n\t"strings"',
    "strconv import",
)
preflight = replace_once(
    preflight,
    '\t"koschei/api/internal/router"\n)',
    '\t"koschei/api/internal/router"\n\t"koschei/api/internal/services"\n)',
    "services import",
)
preflight = replace_once(
    preflight,
    '\tresp := evaluateARVISPreflight(req)\n\tif aiProviderConfigured()',
    '\tresp := evaluateARVISPreflight(req)\n\tresp = h.alignARVISPreflightWithStructuralBaseline(r.Context(), req, resp)\n\tif aiProviderConfigured()',
    "preflight baseline call",
)
helper = r'''
func (h *Handler) alignARVISPreflightWithStructuralBaseline(ctx context.Context, req arvisPreflightRequest, resp arvisPreflightResponse) arvisPreflightResponse {
	target := strings.TrimSpace(req.Target)
	if !solanaPreflightAddressLike.MatchString(target) || isOfficialKOSCHMint(target) {
		return resp
	}
	db := h.DBRead
	if db == nil {
		db = h.DB
	}
	if db == nil {
		return resp
	}
	floor, level, observedAt, ok := services.NewSecurityRadarStore(db).StructuralBaseline(ctx, target, "solana-mainnet")
	if !ok || floor <= resp.Score {
		return resp
	}
	return applyARVISStructuralBaseline(resp, floor, level, observedAt)
}

func applyARVISStructuralBaseline(resp arvisPreflightResponse, floor int, level string, observedAt time.Time) arvisPreflightResponse {
	if floor <= resp.Score {
		return resp
	}
	resp.Score = floor
	if strings.TrimSpace(level) != "" {
		resp.RiskLevel = strings.ToLower(strings.TrimSpace(level))
	}
	switch resp.RiskLevel {
	case "critical", "high":
		if resp.Decision != "blocked" {
			resp.Decision = "warn"
		}
	case "medium":
		if resp.Decision == "allow" {
			resp.Decision = "review"
		}
	}
	reason := "Koschei yapısal hafızası: bu mint için doğrulanmış holder/authority tabanı " + strconv.Itoa(floor) + "/100."
	if !observedAt.IsZero() {
		reason = "Koschei yapısal hafızası: bu mint için " + observedAt.UTC().Format("02.01.2006 15:04 UTC") + " tarihinde doğrulanmış holder/authority tabanı " + strconv.Itoa(floor) + "/100."
	}
	seen := false
	for _, existing := range resp.Reasons {
		if existing == reason {
			seen = true
			break
		}
	}
	if !seen {
		resp.Reasons = append(resp.Reasons, reason)
	}
	resp.HumanMessage = "ARVIS ön kontrol sonucu: " + resp.Decision + " / " + resp.RiskLevel + ". Doğrulanmış yapısal risk tabanı uygulandı; detaylı Radar kanıtlarını incele."
	return resp
}

'''
preflight = replace_once(preflight, 'func evaluateARVISPreflight(req arvisPreflightRequest) arvisPreflightResponse {', helper + 'func evaluateARVISPreflight(req arvisPreflightRequest) arvisPreflightResponse {', "baseline helpers")
preflight_path.write_text(preflight)
