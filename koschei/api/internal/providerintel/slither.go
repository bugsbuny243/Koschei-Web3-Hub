package providerintel

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Slither is a Trail of Bits tool, not a CertiK product.
// Findings are imported observations, not verified exploitability or ARVIS grades.
var regexpSlitherDetector = regexp.MustCompile("^[a-z0-9_-]{2,80}$")

type SlitherFinding struct {
	Detector    string `json:"detector"`
	Impact      string `json:"impact"`
	Confidence  string `json:"confidence"`
	Description string `json:"description"`
}

type SlitherEvidence struct {
	Tool     string           `json:"tool"`
	Status   string           `json:"status"`
	Findings []SlitherFinding `json:"findings"`
}

// ParseSlitherFindings imports output from a separately isolated Slither run.
// No untrusted compiler, Solidity source, or shell runs inside the API process.
func ParseSlitherFindings(raw json.RawMessage) (SlitherEvidence, error) {
	out := SlitherEvidence{
		Tool:     "trail_of_bits_slither",
		Status:   "imported_unverified_static_analysis",
		Findings: []SlitherFinding{},
	}
	if len(raw) == 0 || len(raw) > 64<<10 {
		return out, fmt.Errorf("%w: Slither JSON size", ErrInvalid)
	}
	var report struct {
		Success bool `json:"success"`
		Results struct {
			Detectors []struct {
				Check       string `json:"check"`
				Impact      string `json:"impact"`
				Confidence  string `json:"confidence"`
				Description string `json:"description"`
			} `json:"detectors"`
		} `json:"results"`
	}
	if err := json.Unmarshal(raw, &report); err != nil || !report.Success {
		return out, fmt.Errorf("%w: Slither scan did not finish successfully", ErrInvalid)
	}
	if len(report.Results.Detectors) > 250 {
		return out, fmt.Errorf("%w: too many detectors", ErrInvalid)
	}
	for _, finding := range report.Results.Detectors {
		check := strings.TrimSpace(finding.Check)
		switch finding.Impact {
		case "High", "Medium", "Low", "Informational", "Optimization":
		default:
			return out, fmt.Errorf("%w: unknown detector impact", ErrInvalid)
		}
		switch finding.Confidence {
		case "High", "Medium", "Low":
		default:
			return out, fmt.Errorf("%w: unknown detector confidence", ErrInvalid)
		}
		if len(check) < 2 || len(check) > 80 || !regexpSlitherDetector.MatchString(check) {
			return out, ErrInvalid
		}
		description := strings.TrimSpace(finding.Description)
		if len(description) > 2048 {
			description = description[:2048]
		}
		out.Findings = append(out.Findings, SlitherFinding{
			Detector:    check,
			Impact:      finding.Impact,
			Confidence:  finding.Confidence,
			Description: description,
		})
	}
	return out, nil
}
