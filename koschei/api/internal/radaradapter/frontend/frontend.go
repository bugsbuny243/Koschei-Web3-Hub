package frontend

import (
	"context"
	"encoding/json"
	"errors"
	r "koschei/api/internal/radaradapter"
	"net/url"
	"regexp"
	"strings"
)

type Adapter struct {
	target    r.Target
	subject   r.SubjectIdentity
	transport r.Transport
	origin    *url.URL
}

func New(t r.Target, tr r.Transport) (r.Adapter, error) {
	u, e := url.Parse(t.Raw)
	if e != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" || u.RawQuery != "" || u.Port() != "" {
		return nil, errors.New("frontend target requires an HTTPS URL without credentials, query or custom port")
	}
	return &Adapter{t, r.SubjectIdentity{Network: t.Network, NativeID: t.Raw, ChainIdentity: "origin:" + u.Hostname()}, tr, u}, nil
}
func (a *Adapter) Supports(t r.Target) bool   { return t.Network == "offchain" && t.Kind == "frontend" }
func (a *Adapter) Subject() r.SubjectIdentity { return r.CollectorIdentity(a.subject) }
func (a *Adapter) Capability() r.Capability {
	return r.Capability{ID: "frontend-surface-model", Network: "offchain", Family: "offchain", TargetClasses: []string{"frontend"}, Sources: []string{"operator-configured HTTPS origin", "configured DNS-over-HTTPS and CT API", "captured wallet-request recorder"}, Fixture: "frontend/frontend_test.go", EvidenceOnly: true}
}
func (a *Adapter) get(ctx context.Context, provider, path string) (r.Response, error) {
	return a.transport.Do(ctx, r.Request{Network: "offchain:" + a.origin.Hostname(), Provider: provider, Method: "GET", Path: path, Origin: a.origin.Hostname()})
}

var scripts = regexp.MustCompile(`(?i)<script\b[^>]*\bsrc\s*=\s*["']([^"']+)["']`)

func (a *Adapter) Collect(ctx context.Context, t r.Target) ([]r.EvidenceRow, []r.Limitation, error) {
	rows := []r.EvidenceRow{}
	limits := []r.Limitation{{Code: "frontend_scope", Reason: "Only the configured origin and up to five same-origin script bundles are fetched. A changed hash is not proof of compromise; DNS, CT and recorder observations do not establish user authorization."}}
	stamp := "0"
	path := a.origin.EscapedPath()
	if path == "" {
		path = "/"
	}
	response, e := a.get(ctx, "primary", path)
	if e != nil {
		return rows, limits, e
	}
	rows = append(rows, r.NewRow(a.subject, "frontend-document", response, stamp, map[string]any{"origin": a.origin.Hostname(), "tls_certificate_sha256": response.Certificates}))
	for i, match := range scripts.FindAllSubmatch(response.Raw, 6) {
		if i >= 5 {
			limits = append(limits, r.Limitation{Code: "bundle_limit", Reason: "More than five scripts; remaining bundles were not fetched."})
			break
		}
		ref, e := url.Parse(string(match[1]))
		if e != nil {
			continue
		}
		u := a.origin.ResolveReference(ref)
		if u.Scheme != "https" || u.Host != a.origin.Host || u.User != nil {
			limits = append(limits, r.Limitation{Code: "external_script", Reason: "Cross-origin bundle requires separate provider configuration."})
			continue
		}
		bundle, e := a.get(ctx, "primary", u.RequestURI())
		if e != nil {
			limits = append(limits, r.Limitation{Code: "bundle_unavailable", Reason: u.EscapedPath()})
			continue
		}
		facts := map[string]any{"script_path": u.EscapedPath(), "bundle_sha256": r.Digest(bundle.Raw), "baseline_status": "not_configured"}
		if pinned, ok := a.transport.(interface{ Baseline(string, string) string }); ok {
			digest := pinned.Baseline(a.origin.Hostname(), u.RequestURI())
			if digest != "" {
				facts["baseline_status"] = "compared"
				facts["baseline_sha256"] = digest
				facts["changed"] = digest != r.Digest(bundle.Raw)
			}
		}
		rows = append(rows, r.NewRow(a.subject, "frontend-bundle-integrity", bundle, stamp, facts))
	}
	for _, source := range []struct{ provider, path, relation string }{{"dns", "/resolve?name=" + url.QueryEscape(a.origin.Hostname()) + "&type=A", "frontend-dns"}, {"certificate", "/?q=" + url.QueryEscape(a.origin.Hostname()) + "&output=json", "frontend-certificate-transparency"}, {"capture", "/requests?origin=" + url.QueryEscape("https://"+a.origin.Hostname()), "frontend-wallet-requests"}} {
		res, e := a.get(ctx, source.provider, source.path)
		if e != nil || !json.Valid(res.Raw) {
			limits = append(limits, r.Limitation{Code: "source_unavailable", Reason: source.relation})
			continue
		}
		facts := map[string]any{"response": json.RawMessage(res.Raw)}
		if source.provider == "capture" {
			var capture struct {
				Origin   string            `json:"origin"`
				Requests []json.RawMessage `json:"requests"`
			}
			if r.Decode(res, &capture) != nil || capture.Origin != "https://"+a.origin.Hostname() {
				limits = append(limits, r.Limitation{Code: "capture_origin_mismatch", Reason: "Wallet-request recorder is not bound to target origin."})
				continue
			}
			signals := []string{}
			for _, request := range capture.Requests {
				signals = append(signals, SignatureSurfaces(request)...)
			}
			facts["surfaces"] = signals
			facts["scope"] = "operator_recorder_observation_not_signature_verification"
		}
		rows = append(rows, r.NewRow(a.subject, source.relation, res, stamp, facts))
	}
	return rows, limits, nil
}

// SignatureSurfaces classifies request shape only. It cannot attest origin,
// recovered signer, user intent, or the canonical master of a TON jetton.
func SignatureSurfaces(raw []byte) []string {
	var request struct {
		Method string            `json:"method"`
		Params []json.RawMessage `json:"params"`
		Type   string            `json:"type"`
	}
	if json.Unmarshal(raw, &request) != nil {
		return nil
	}
	out := []string{}
	switch request.Method {
	case "eth_signTypedData_v4":
		for _, p := range request.Params {
			var text string
			if json.Unmarshal(p, &text) == nil {
				p = json.RawMessage(text)
			}
			var typed struct {
				Primary string `json:"primaryType"`
			}
			if json.Unmarshal(p, &typed) == nil {
				if strings.Contains(typed.Primary, "Permit") {
					out = append(out, "evm-permit-signature")
				}
			}
		}
	case "wallet_sendCalls":
		out = append(out, "evm-batch-request")
	case "eth_sign", "personal_sign":
		out = append(out, "opaque-signature-request")
	case "sendTransaction":
		out = append(out, "ton-transfer-request-unverified")
	}
	if request.Type == "AccountPermissionUpdateContract" {
		out = append(out, "tron-permission-update-request")
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) == nil {
		if _, ok := object["authorizationList"]; ok {
			out = append(out, "evm-7702-authorization-request")
		}
	}
	return out
}
