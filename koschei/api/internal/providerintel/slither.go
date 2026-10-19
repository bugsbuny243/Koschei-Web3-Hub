package providerintel

import (
	"regexp"
	"encoding/json"
	"fmt"
	"strings"
)

// SlitherFinding is an imported static-analysis OBSERVATION, not verified
// exploitability, a CertiK audit, or permission to execute arbitrary Solidity.
// Slither is maintained by Trail of Bits (crytic/slither).
var regexpSlitherDetector = regexp.MustCompile(`^[a-z0-9_-]{2,80}package providerintel

import (
	"regexp"
	"encoding/json"
	"fmt"
	"strings"
)

// SlitherFinding is an imported static-analysis OBSERVATION, not verified
// exploitability, a CertiK audit, or permission to execute arbitrary Solidity.
// Slither is maintained by Trail of Bits (crytic/slither).
)

type SlitherFinding struct {
	Detector string `json:"detector"`
	Impact string `json:"impact"`
	Confidence string `json:"confidence"`
	Description string `json:"description"`
}
type SlitherEvidence struct {
	Tool string `json:"tool"`
	Status string `json:"status"`
	Findings []SlitherFinding `json:"findings"`
}

// ParseSlitherFindings accepts output from a SEPARATELY isolated Slither run.
// The API must not run solc, npm, a shell, or user-submitted compiler plugins.
func ParseSlitherFindings(raw json.RawMessage) (SlitherEvidence,error) {
	out:=SlitherEvidence{Tool:"trail_of_bits_slither",Status:"imported_unverified_static_analysis",Findings:[]SlitherFinding{}}
	if len(raw)==0||len(raw)>64<<10{return out,fmt.Errorf("%w: Slither JSON size",ErrInvalid)}
	var report struct {
		Success bool `json:"success"`
		Results struct {
			Detectors []struct {
				Check string `json:"check"`
				Impact string `json:"impact"`
				Confidence string `json:"confidence"`
				Description string `json:"description"`
			} `json:"detectors"`
		} `json:"results"`
	}
	if err:=json.Unmarshal(raw,&report);err!=nil||!report.Success{return out,fmt.Errorf("%w: Slither scan did not finish successfully",ErrInvalid)}
	if len(report.Results.Detectors)>250{return out,fmt.Errorf("%w: too many detectors",ErrInvalid)}
	for _,d:=range report.Results.Detectors{
		d.Check=strings.TrimSpace(d.Check)
		switch d.Impact{
		case "High","Medium","Low","Informational","Optimization":
		default:return out,fmt.Errorf("%w: unknown detector impact",ErrInvalid)
		}
		switch d.Confidence{
		case "High","Medium","Low":
		default:return out,fmt.Errorf("%w: unknown detector confidence",ErrInvalid)
		}
		if len(d.Check)<2||len(d.Check)>80||!regexpSlitherDetector.MatchString(d.Check){return out,ErrInvalid}
		desc:=strings.TrimSpace(d.Description)
		if len(desc)>2048{desc=desc[:2048]}
		out.Findings=append(out.Findings,SlitherFinding{Detector:d.Check,Impact:d.Impact,Confidence:d.Confidence,Description:desc})
	}
	return out,nil
}
