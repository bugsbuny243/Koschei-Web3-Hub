// radar-smoke validates operator-selected public targets through the production
// collector bridge. It never discovers credentials or enables grading.
package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"koschei/api/internal/radaradapter"
	"koschei/api/internal/services"
)

type smokeCase struct {
	Target   radaradapter.Target `json:"target"`
	Required []string            `json:"required_relations"`
}
type receipt struct {
	Network         string            `json:"network"`
	Target          string            `json:"target"`
	Passed          bool              `json:"passed"`
	CollectorStatus string            `json:"collector_status"`
	Requests        int               `json:"provider_request_attempts"`
	Missing         []string          `json:"missing_requirements"`
	Evidence        []evidenceReceipt `json:"evidence"`
}
type evidenceReceipt struct {
	ID         string    `json:"id"`
	Relation   string    `json:"relation"`
	Provider   string    `json:"provider_host"`
	Hash       string    `json:"raw_response_sha256"`
	Height     int64     `json:"height_context"`
	ObservedAt time.Time `json:"observed_at"`
}

func manifest(r io.Reader) ([]smokeCase, error) {
	decoder := json.NewDecoder(io.LimitReader(r, 65537))
	decoder.DisallowUnknownFields()
	var cases []smokeCase
	if err := decoder.Decode(&cases); err != nil {
		return nil, fmt.Errorf("invalid smoke manifest")
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return nil, fmt.Errorf("trailing manifest content")
	}
	if len(cases) == 0 || len(cases) > 10 {
		return nil, fmt.Errorf("manifest requires 1 to 10 cases")
	}
	for _, c := range cases {
		if c.Target.Network == "" || c.Target.Kind == "" || c.Target.Raw == "" || len(c.Target.Raw) > 512 || len(c.Required) == 0 {
			return nil, fmt.Errorf("each case needs a target and required relations")
		}
		for _, rel := range c.Required {
			if rel == "" {
				return nil, fmt.Errorf("empty required relation")
			}
		}
	}
	return cases, nil
}
func evaluate(c smokeCase, result services.RadarInvestigationResult) receipt {
	out := receipt{Network: c.Target.Network, Target: c.Target.Raw, CollectorStatus: result.Status, Requests: result.Requests, Missing: []string{}, Evidence: []evidenceReceipt{}}
	if !result.OK || result.GradeIssued || result.Verdict != "withhold" || result.Subject.Network != c.Target.Network || result.Subject.ID == "" {
		out.Missing = append(out.Missing, "observe_only_subject_contract")
	}
	found := map[string]bool{}
	for _, row := range result.Evidence {
		relation, _ := row.Attributes["relation"].(string)
		hash, _ := row.Attributes["raw_response_sha256"].(string)
		identity, _ := row.Attributes["native_chain_identity"].(string)
		digest, err := hex.DecodeString(hash)
		if row.SubjectID != result.Subject.ID || row.Network != c.Target.Network || row.Source == "" || row.ObservedAt.IsZero() || err != nil || len(digest) != 32 || identity == "" {
			out.Missing = append(out.Missing, "invalid_evidence_provenance")
			continue
		}
		found[relation] = true
		out.Evidence = append(out.Evidence, evidenceReceipt{row.ID, relation, row.Source, hash, row.BlockOrSlot, row.ObservedAt})
	}
	for _, rel := range c.Required {
		if !found[rel] {
			out.Missing = append(out.Missing, "relation:"+rel)
		}
	}
	out.Passed = len(out.Missing) == 0 && len(out.Evidence) > 0
	return out
}
func main() {
	path := flag.String("manifest", "", "JSON cases with explicit public targets and required evidence relations")
	live := flag.Bool("live", false, "allow bounded provider reads using RADAR_PROVIDERS_JSON")
	flag.Parse()
	if !*live {
		json.NewEncoder(os.Stdout).Encode(map[string]any{"mode": "configuration_only", "live_verified": false, "runtime": services.RadarCapabilityRuntime()})
		return
	}
	file, err := os.Open(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "smoke manifest unavailable")
		os.Exit(2)
	}
	cases, err := manifest(file)
	file.Close()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	receipts := []receipt{}
	passed := true
	for _, c := range cases {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		result := services.ExecuteRadarInvestigation(ctx, c.Target)
		cancel()
		report := evaluate(c, result)
		receipts = append(receipts, report)
		passed = passed && report.Passed
	}
	json.NewEncoder(os.Stdout).Encode(map[string]any{"mode": "live_read_only", "passed": passed, "scope": "required relations only; no safety grade or full production readiness claim", "receipts": receipts})
	if !passed {
		os.Exit(1)
	}
}
