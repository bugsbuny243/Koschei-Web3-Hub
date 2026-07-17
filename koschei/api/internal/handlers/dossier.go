package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"sort"
	"strings"
	"time"
)

const (
	dossierVersion = "koschei-dossier-v1"
	dossierMapperVersion = "koschei-verdict-card-v4+evidence-refs-v1"
	dossierVerifierRepo = "https://github.com/bugsbuny243/Koschei-Web3-Hub"
)

var errDossierSourceIncomplete = errors.New("dossier_source_incomplete")
var errDossierReferenceMissing = errors.New("populated_signal_missing_refs")

var dossierLimitations = []string{
	"Capability-not-intent policy: the report describes observed technical capability and behavior; it does not infer intent.",
	"Identity boundary: onchain_wallet_only. No person, organization, beneficial owner or real-world identity attribution is made or implied.",
	"Evidence-window boundary: every observation is bounded by the produced_at time, source timestamps, slots and stored collection limits in this bundle.",
	"This technical evidence export is not legal advice and is not investment advice.",
}

type DossierRefs struct {
	Wallets      []string               `json:"wallets"`
	Accounts     []string               `json:"accounts"`
	Transactions []DossierTransactionRef `json:"transactions"`
	EvidenceKeys []string               `json:"evidence_keys"`
}

type DossierTransactionRef struct {
	Signature string `json:"signature"`
	Slot      int64  `json:"slot,omitempty"`
}

type DossierSignalRow struct {
	ID      string      `json:"id"`
	Label   string      `json:"label"`
	State   string      `json:"state"`
	Value   any         `json:"value,omitempty"`
	Refs    DossierRefs `json:"refs"`
}

type dossierBody struct {
	DossierVersion       string         `json:"dossier_version"`
	CaseRef              string         `json:"case_ref"`
	Token                any            `json:"token"`
	Verdict              any            `json:"verdict"`
	VerdictCard          any            `json:"verdict_card"`
	ThreatAnticipation   any            `json:"threat_anticipation"`
	EvidenceArms         any            `json:"evidence_arms"`
	TransactionEvidence any            `json:"transaction_evidence"`
	ActorDossier         any            `json:"actor_dossier"`
	HolderContext        any            `json:"holder_concentration_context,omitempty"`
	Verification         any            `json:"verification"`
	Limitations          []string       `json:"limitations"`
}

type dossierBundle struct {
	dossierBody
	BundleHash string `json:"bundle_hash"`
}

type dossierSnapshot struct {
	ID               string
	Mint             string
	Network          string
	VerdictID        string
	VerdictSignature string
	RulesetVersion   string
	ProducedAt       time.Time
	Report           map[string]any
}

func (h *Handler) persistDossierSourceSnapshot(ctx context.Context, report map[string]any) {
	if h == nil || h.DB == nil || report == nil { return }
	final := anyMap(report["final_verdict"])
	signature := strings.TrimSpace(anyString(final["signature"]))
	if signature == "" || !anyBool(final["signed"]) { return }
	mint := strings.TrimSpace(anyString(report["target"]))
	if mint == "" { return }
	network := firstNonEmptyString(anyString(report["network"]), "solana-mainnet")
	ruleset := strings.TrimSpace(anyString(final["ruleset_version"]))
	produced := parseAnyTime(firstNonEmptyString(anyString(final["generated_at"]), anyString(report["generated_at"])))
	if produced.IsZero() { produced = time.Now().UTC() }
	raw, err := json.Marshal(report)
	if err != nil { return }
	_, _ = h.DB.ExecContext(ctx, `
		INSERT INTO dossier_source_snapshots
		(mint,network,verdict_id,verdict_signature,ruleset_version,produced_at,source_payload)
		VALUES ($1,$2,$3,$4,$5,$6,$7::jsonb)
		ON CONFLICT (verdict_signature) DO NOTHING`,
		mint, network, signature, signature, ruleset, produced, string(raw))
}

func (h *Handler) DossierExport(w http.ResponseWriter, r *http.Request) {
	mint := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/api/v1/dossier/"))
	if mint == "" || strings.Contains(mint, "/") {
		writeAPIError(w, http.StatusBadRequest, APICodeInvalidInput, "mint is required")
		return
	}
	snapshot, err := h.loadDossierSnapshot(r.Context(), mint, strings.TrimSpace(r.URL.Query().Get("verdict_id")))
	if err != nil {
		if errors.Is(err, errDossierSourceIncomplete) || errors.Is(err, context.Canceled) {
			writeJSON(w, http.StatusConflict, map[string]string{"error":"dossier_source_incomplete","message":"An immutable signed scan snapshot is required; the export path never rescans or refreshes missing evidence."})
			return
		}
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error":"dossier_unavailable"})
		return
	}
	bundle, canonical, err := assembleDossierBundle(snapshot)
	if err != nil {
		code := "dossier_assembly_failed"
		if errors.Is(err, errDossierReferenceMissing) { code = "dossier_evidence_reference_missing" }
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error":code,"message":err.Error()})
		return
	}
	requester := dossierRequester(r)
	if stored, ok := h.loadStoredDossierBytes(r.Context(), bundle.CaseRef); ok {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("ETag", `"`+bundle.BundleHash+`"`)
		_, _ = w.Write(stored)
		return
	}
	if err := h.storeDossierBundle(r.Context(), snapshot, bundle, canonical, requester); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error":"dossier_store_failed"})
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("ETag", `"`+bundle.BundleHash+`"`)
	_, _ = w.Write(canonical)
}

func (h *Handler) loadDossierSnapshot(ctx context.Context, mint, verdictID string) (dossierSnapshot, error) {
	if h == nil || h.DB == nil { return dossierSnapshot{}, errDossierSourceIncomplete }
	query := `SELECT id::text,mint,network,COALESCE(verdict_id,''),verdict_signature,ruleset_version,produced_at,source_payload FROM dossier_source_snapshots WHERE mint=$1`
	args := []any{mint}
	if verdictID != "" { query += ` AND (verdict_id=$2 OR verdict_signature=$2)`; args=append(args,verdictID) }
	query += ` ORDER BY produced_at DESC LIMIT 1`
	var item dossierSnapshot
	var raw []byte
	if err := h.DB.QueryRowContext(ctx,query,args...).Scan(&item.ID,&item.Mint,&item.Network,&item.VerdictID,&item.VerdictSignature,&item.RulesetVersion,&item.ProducedAt,&raw); err != nil { return dossierSnapshot{},errDossierSourceIncomplete }
	if json.Unmarshal(raw,&item.Report)!=nil || item.Report==nil { return dossierSnapshot{},errDossierSourceIncomplete }
	return item,nil
}

func assembleDossierBundle(snapshot dossierSnapshot) (dossierBundle, []byte, error) {
	report:=snapshot.Report
	rows:=buildDossierSignalRows(report)
	for _,row:=range rows{
		if (row.State=="verified"||row.State=="observed") && !dossierRefsPresent(row.Refs){return dossierBundle{},nil,fmt.Errorf("%w: %s",errDossierReferenceMissing,row.ID)}
	}
	caseRef:=dossierCaseRef(snapshot.Mint,snapshot.VerdictSignature)
	body:=dossierBody{
		DossierVersion:dossierVersion,CaseRef:caseRef,
		Token:map[string]any{"mint":snapshot.Mint,"network":snapshot.Network,"market_snapshot":report["market"],"launch_metadata":report["launch_forensics"],"source_context":report["source_context"]},
		Verdict:report["final_verdict"],
		VerdictCard:map[string]any{"mapper_id":"koschei-verdict-card","mapper_version":dossierMapperVersion,"input":report,"signal_rows":rows},
		ThreatAnticipation:report["threat_anticipation"],EvidenceArms:firstAny(report["evidence_arms"],report["modules"]),
		TransactionEvidence:firstAny(report["transaction_evidence"],[]any{}),ActorDossier:report["actor_investigation"],HolderContext:report["holder_concentration_context"],
		Verification:map[string]any{"verifier_repo_url":dossierVerifierRepo,"verdict_signature":snapshot.VerdictSignature,"canonical_json_hashing_rule":"SHA-256 over UTF-8 JSON encoding of the dossier body with stable struct field order and lexicographically sorted map keys; bundle_hash is excluded from the hashed body.","command":"node oss/verifier/typescript/verify-dossier.mjs ./dossier.json"},
		Limitations:append([]string{},dossierLimitations...),
	}
	bodyBytes,err:=json.Marshal(body);if err!=nil{return dossierBundle{},nil,err}
	sum:=sha256.Sum256(bodyBytes)
	bundle:=dossierBundle{dossierBody:body,BundleHash:"sha256:"+hex.EncodeToString(sum[:])}
	canonical,err:=json.Marshal(bundle);if err!=nil{return dossierBundle{},nil,err}
	return bundle,canonical,nil
}

func buildDossierSignalRows(report map[string]any) []DossierSignalRow {
	target:=strings.TrimSpace(anyString(report["target"]));source:=anyMap(report["source_context"]);launch:=anyMap(report["launch_forensics"]);holder:=anyMap(report["holder_intelligence"]);market:=anyMap(report["market"]);lp:=anyMap(report["lp_control"]);final:=anyMap(report["final_verdict"]);threat:=anyMap(report["threat_anticipation"]);ledger:=anyMap(report["trade_ledger_aggregates"]);actor:=anyMap(report["actor_investigation"])
	base:=DossierRefs{Accounts:nonEmptyUnique([]string{target}),Wallets:[]string{},Transactions:[]DossierTransactionRef{},EvidenceKeys:[]string{}}
	owner:=firstResolvedOwner(holder);creator:=strings.TrimSpace(anyString(source["creator_wallet"]));pool:=strings.TrimSpace(anyString(lp["pool_address"]));lpMint:=strings.TrimSpace(anyString(lp["lp_mint"]));readSlot:=anyInt64(lp["read_slot"]);signature:=strings.TrimSpace(anyString(final["signature"]))
	ownerRefs:=mergeDossierRefs(base,DossierRefs{Wallets:nonEmptyUnique([]string{owner})})
	creatorRefs:=mergeDossierRefs(base,DossierRefs{Wallets:nonEmptyUnique([]string{creator})})
	lpRefs:=mergeDossierRefs(base,DossierRefs{Accounts:nonEmptyUnique([]string{pool,lpMint,anyString(lp["token_vault"]),anyString(lp["quote_vault"])}),EvidenceKeys:anyStringSlice(lp["evidence_keys"])})
	if readSlot>0{lpRefs.Transactions=[]DossierTransactionRef{{Slot:readSlot}}}
	signedRefs:=mergeDossierRefs(base,DossierRefs{Transactions:[]DossierTransactionRef{{Signature:signature}}})
	rows:=[]DossierSignalRow{
		dossierRow("launch_time","Launch time / age",launch,base),dossierRow("mint_authority","Mint authority",findModule(report,"token_authority"),base),dossierRow("freeze_authority","Freeze authority",findModule(report,"token_authority"),base),
		dossierRow("wash_trading","Wash-trading context",ledger,base),dossierRow("address_behavior","Address behavior",actor,mergeDossierRefs(base,DossierRefs{Wallets:nonEmptyUnique([]string{creator,owner})})),dossierRow("liquidity_amount","Liquidity amount + lock status",firstAnyMap(lp,market),lpRefs),
		dossierRow("creator_funding","Creator funding origin",findModule(report,"funding"),creatorRefs),dossierRow("holder_concentration","Holder concentration",holder,ownerRefs),dossierRow("sniper_timing","Sniper timing",findModule(report,"sniper"),base),dossierRow("first_buyer_linkage","First-buyer linkage",findModule(report,"pump_sybil"),creatorRefs),
		dossierRow("creator_track_record","Creator track record",actor,creatorRefs),dossierRow("creator_sell_behavior","Creator sell behavior",findBehavior(report,"URD-C003"),creatorRefs),dossierRow("dominant_holder_exit","Dominant holder exit",findPathway(threat,"dominant_holder_exit"),ownerRefs),dossierRow("liquidity_movement","Liquidity movement",findModule(report,"liquidity_movement"),lpRefs),
		dossierRow("program_relations","Program / contract relations",findModule(report,"program"),base),dossierGapRow("metadata_impersonation","Metadata / impersonation check","arm_pending"),dossierGapRow("claim_surface","Claim / airdrop surface","not_applicable"),dossierGapRow("mev_exposure","MEV exposure","not_applicable"),dossierRow("launch_distribution","Launch distribution fairness",findModule(report,"launch_distribution"),base),dossierRow("signed_final_verdict","Signed final verdict",final,signedRefs),
	}
	return rows
}

func dossierRow(id,label string,value any,refs DossierRefs)DossierSignalRow{m:=anyMap(value);state:="arm_pending";if len(m)>0{state="observed";status:=strings.ToLower(firstNonEmptyString(anyString(m["evidence_status"]),anyString(m["verification_status"]),anyString(m["status"])));if status=="verified"||anyBool(m["signed"]){state="verified"};if status=="not_applicable"{state="not_applicable"}};return DossierSignalRow{ID:id,Label:label,State:state,Value:value,Refs:normalizeDossierRefs(refs)}}
func dossierGapRow(id,label,state string)DossierSignalRow{return DossierSignalRow{ID:id,Label:label,State:state,Refs:DossierRefs{Wallets:[]string{},Accounts:[]string{},Transactions:[]DossierTransactionRef{},EvidenceKeys:[]string{}}}}

func dossierCaseRef(mint,signature string)string{sum:=sha256.Sum256([]byte(strings.TrimSpace(mint)+"\n"+strings.TrimSpace(signature)));return "KD1-"+strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(sum[:20]))}
func dossierRefsPresent(refs DossierRefs)bool{return len(refs.Wallets)+len(refs.Accounts)+len(refs.Transactions)+len(refs.EvidenceKeys)>0}
func normalizeDossierRefs(refs DossierRefs)DossierRefs{refs.Wallets=nonEmptyUnique(refs.Wallets);refs.Accounts=nonEmptyUnique(refs.Accounts);refs.EvidenceKeys=nonEmptyUnique(refs.EvidenceKeys);if refs.Transactions==nil{refs.Transactions=[]DossierTransactionRef{}};return refs}
func mergeDossierRefs(a,b DossierRefs)DossierRefs{return normalizeDossierRefs(DossierRefs{Wallets:append(a.Wallets,b.Wallets...),Accounts:append(a.Accounts,b.Accounts...),Transactions:append(a.Transactions,b.Transactions...),EvidenceKeys:append(a.EvidenceKeys,b.EvidenceKeys...)})}
func nonEmptyUnique(values []string)[]string{seen:=map[string]bool{};out:=[]string{};for _,v:=range values{v=strings.TrimSpace(v);if v!=""&&!seen[v]{seen[v]=true;out=append(out,v)}};sort.Strings(out);return out}

func (h *Handler) storeDossierBundle(ctx context.Context,s dossierSnapshot,b dossierBundle,canonical []byte,requestedBy string)error{raw,err:=json.Marshal(b);if err!=nil{return err};_,err=h.DB.ExecContext(ctx,`INSERT INTO dossier_exports(case_ref,mint,verdict_id,source_snapshot_id,bundle_hash,canonical_bundle,bundle_json,requested_by) VALUES($1,$2,$3,$4::uuid,$5,$6,$7::jsonb,$8) ON CONFLICT(case_ref) DO NOTHING`,b.CaseRef,s.Mint,s.VerdictID,s.ID,b.BundleHash,canonical,string(raw),requestedBy);return err}
func(h *Handler)loadStoredDossierBytes(ctx context.Context,caseRef string)([]byte,bool){if h==nil||h.DB==nil{return nil,false};var raw []byte;if h.DB.QueryRowContext(ctx,`SELECT canonical_bundle FROM dossier_exports WHERE case_ref=$1`,caseRef).Scan(&raw)!=nil{return nil,false};return raw,len(raw)>0}
func dossierRequester(r *http.Request)string{if p,ok:=apiPrincipalFromContext(r.Context());ok{return "api_key:"+p.KeyID};if u,ok:=userFromContext(r.Context());ok{return "user:"+u.Sub};return "owner"}

func (h *Handler) DossierPage(w http.ResponseWriter,r *http.Request){caseRef:=strings.TrimSpace(strings.TrimPrefix(r.URL.Path,"/dossier/"));if caseRef==""||strings.Contains(caseRef,"/"){http.NotFound(w,r);return};var raw []byte;if h.DB.QueryRowContext(r.Context(),`SELECT canonical_bundle FROM dossier_exports WHERE case_ref=$1`,caseRef).Scan(&raw)!=nil{http.NotFound(w,r);return};var bundle dossierBundle;if json.Unmarshal(raw,&bundle)!=nil{http.Error(w,"export unavailable",http.StatusServiceUnavailable);return};lang:="en";if strings.EqualFold(r.URL.Query().Get("lang"),"tr"){lang="tr"};data:=map[string]any{"Bundle":bundle,"Lang":lang,"TR":lang=="tr"};w.Header().Set("Content-Type","text/html; charset=utf-8");_ = dossierHTML.Execute(w,data)}

var dossierHTML=template.Must(template.New("dossier").Parse(`<!doctype html><html lang="{{if .TR}}tr{{else}}en{{end}}"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Koschei Technical Evidence Export {{.Bundle.CaseRef}}</title><style>@page{size:A4;margin:18mm 14mm 20mm}*{box-sizing:border-box}body{font:12px/1.5 Arial,sans-serif;color:#111;margin:0}header,section{break-inside:avoid;margin:0 0 18px}h1{font-size:25px}h2{font-size:16px;border-bottom:2px solid #111;padding-bottom:5px}small{display:block;color:#555}.mono{font-family:monospace;overflow-wrap:anywhere}.grid{display:grid;grid-template-columns:1fr 1fr;gap:8px}.box{border:1px solid #777;padding:9px}pre{white-space:pre-wrap;overflow-wrap:anywhere;border:1px solid #aaa;padding:10px;font-size:9px}footer{position:fixed;bottom:0;font-size:9px;color:#555}.pagebreak{break-before:page}@media print{a{color:#111;text-decoration:none}}</style></head><body><header><h1>{{if .TR}}Koschei Teknik Kanıt Çıktısı{{else}}Koschei Technical Evidence Export{{end}}</h1><div class="grid"><div class="box"><b>Reference</b><div class="mono">{{.Bundle.CaseRef}}</div></div><div class="box"><b>Bundle hash</b><div class="mono">{{.Bundle.BundleHash}}</div></div></div></header><section><h2>1 · {{if .TR}}Hedef ve imzalı sonuç{{else}}Target and signed result{{end}} <small>{{.Bundle.CaseRef}} · 1/8</small></h2><pre>{{printf "%+v" .Bundle.Token}}</pre><pre>{{printf "%+v" .Bundle.Verdict}}</pre></section><section><h2>2 · {{if .TR}}Sinyal kapsamı{{else}}Signal coverage{{end}} <small>{{.Bundle.CaseRef}} · 2/8</small></h2><pre>{{printf "%+v" .Bundle.VerdictCard}}</pre></section><section class="pagebreak"><h2>3 · Threat pathways <small>{{.Bundle.CaseRef}} · 3/8</small></h2><pre>{{printf "%+v" .Bundle.ThreatAnticipation}}</pre></section><section><h2>4 · Evidence arms <small>{{.Bundle.CaseRef}} · 4/8</small></h2><pre>{{printf "%+v" .Bundle.EvidenceArms}}</pre></section><section class="pagebreak"><h2>5 · Transaction evidence <small>{{.Bundle.CaseRef}} · 5/8</small></h2><pre>{{printf "%+v" .Bundle.TransactionEvidence}}</pre></section><section><h2>6 · Actor observations <small>{{.Bundle.CaseRef}} · 6/8</small></h2><pre>{{printf "%+v" .Bundle.ActorDossier}}</pre></section><section><h2>7 · Independent verification <small>{{.Bundle.CaseRef}} · 7/8</small></h2><pre>{{printf "%+v" .Bundle.Verification}}</pre></section><section><h2>8 · Limitations <small>{{.Bundle.CaseRef}} · 8/8</small></h2>{{range .Bundle.Limitations}}<p>• {{.}}</p>{{end}}</section><footer>{{.Bundle.CaseRef}} · Koschei evidence-first technical export</footer></body></html>`))

func anyMap(value any)map[string]any{if value==nil{return map[string]any{}};if m,ok:=value.(map[string]any);ok{return m};raw,err:=json.Marshal(value);if err!=nil{return map[string]any{}};var out map[string]any;if json.Unmarshal(raw,&out)!=nil{return map[string]any{}};return out}
func anyString(value any)string{if value==nil{return ""};if s,ok:=value.(string);ok{return strings.TrimSpace(s)};return strings.TrimSpace(fmt.Sprint(value))}
func anyBool(value any)bool{v,ok:=value.(bool);return ok&&v}
func anyInt64(value any)int64{switch v:=value.(type){case int64:return v;case int:return int64(v);case float64:return int64(v);case json.Number:n,_:=v.Int64();return n};return 0}
func anyStringSlice(value any)[]string{out:=[]string{};switch v:=value.(type){case []string:return nonEmptyUnique(v);case []any:for _,item:=range v{out=append(out,anyString(item))}};return nonEmptyUnique(out)}
func firstAny(values ...any)any{for _,v:=range values{if v!=nil{return v}};return nil}
func firstAnyMap(values ...any)map[string]any{for _,v:=range values{if m:=anyMap(v);len(m)>0{return m}};return map[string]any{}}
func parseAnyTime(value string)time.Time{for _,layout:=range []string{time.RFC3339Nano,time.RFC3339}{if t,err:=time.Parse(layout,value);err==nil{return t.UTC()}};return time.Time{}}
func findModule(report map[string]any,needle string)map[string]any{needle=strings.ToLower(needle);for _,item:=range anySlice(firstAny(report["evidence_arms"],report["modules"])){m:=anyMap(item);id:=strings.ToLower(anyString(firstAny(m["module_id"],m["module"])));if strings.Contains(id,needle){return m}};return map[string]any{}}
func findBehavior(report map[string]any,rule string)map[string]any{behavior:=anyMap(report["behavior_signals"]);for _,item:=range anySlice(behavior["signals"]){m:=anyMap(item);if anyString(m["rule_id"])==rule{return m}};return map[string]any{}}
func findPathway(threat map[string]any,id string)map[string]any{for _,item:=range anySlice(threat["pathways"]){m:=anyMap(item);if anyString(m["id"])==id{return m}};return map[string]any{}}
func anySlice(value any)[]any{if value==nil{return []any{}};if s,ok:=value.([]any);ok{return s};raw,_:=json.Marshal(value);var out []any;_ = json.Unmarshal(raw,&out);return out}
func firstResolvedOwner(holder map[string]any)string{for _,item:=range anySlice(holder["rows"]){m:=anyMap(item);if anyBool(m["owner_resolved"])&&anyBool(m["risk_bearing"]){if wallet:=anyString(m["owner_wallet"]);wallet!=""{return wallet}}};return ""}
