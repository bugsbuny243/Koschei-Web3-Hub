package fabric

import (
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

const (
	SecurityCaseEnvelopeV1 = "fabric.security-case-envelope.v1"
	Web3AdapterVersionV1   = "web3.fabric-case-adapter.v1"
)

type NativeBindingV1 struct {
	Owner              string `json:"owner"`
	NativeSchema       string `json:"nativeSchema"`
	NativeRef          string `json:"nativeRef"`
	NativeDigestSHA256 string `json:"nativeDigestSha256"`
	AdapterVersion     string `json:"adapterVersion"`
	MappingState       string `json:"mappingState"`
}

type CaseV1 struct {
	CaseID      string  `json:"caseId"`
	CreatedAt   string  `json:"createdAt"`
	Coordinator string  `json:"coordinator"`
	TraceID     *string `json:"traceId"`
}

type RequestV1 struct {
	RequestDigestSHA256 string  `json:"requestDigestSha256"`
	Network             string  `json:"network"`
	Target              string  `json:"target"`
	RequestedOperation  string  `json:"requestedOperation"`
	Nonce               *string `json:"nonce"`
}

type EffectV1 struct {
	RequestedEffect              string  `json:"requestedEffect"`
	ActualEffectState            string  `json:"actualEffectState"`
	ActualEffectDigestSHA256     *string `json:"actualEffectDigestSha256"`
	IndependentObservationState string  `json:"independentObservationState"`
	ReceiptRef                   *string `json:"receiptRef"`
	ReceiptDigestSHA256          *string `json:"receiptDigestSha256"`
}

type Web3ProjectionV1 struct {
	SchemaVersion string          `json:"schemaVersion"`
	Case          CaseV1          `json:"case"`
	Request       RequestV1       `json:"request"`
	Effect        EffectV1        `json:"effect"`
	NativeBinding NativeBindingV1 `json:"nativeBinding"`
}

type Web3ProjectionInputV1 struct {
	CaseID                       string
	CreatedAt                    time.Time
	TraceID                      *string
	RequestDigestSHA256          string
	Network                      string
	Target                       string
	RequestedOperation           string
	Nonce                        *string
	RequestedEffect              string
	ActualEffectState            string
	ActualEffectDigestSHA256     *string
	IndependentObservationState string
	ReceiptRef                   *string
	ReceiptDigestSHA256          *string
	NativeSchema                 string
	NativeRef                    string
	NativeDigestSHA256           string
	MappingState                 string
}

var (
	actualEffectStates = map[string]struct{}{
		"NONE": {}, "SUCCEEDED": {}, "BLOCKED": {}, "FAILED": {}, "UNKNOWN": {},
	}
	observationStates = map[string]struct{}{
		"VERIFIED": {}, "PARTIAL": {}, "UNVERIFIED": {}, "UNAVAILABLE": {},
	}
	mappingStates = map[string]struct{}{
		"VERIFIED": {}, "PARTIAL": {}, "UNVERIFIED": {},
	}
)

func BuildWeb3ProjectionV1(input Web3ProjectionInputV1) (Web3ProjectionV1, error) {
	if strings.TrimSpace(input.CaseID) == "" {
		return Web3ProjectionV1{}, errors.New("case ID is required")
	}
	if input.CreatedAt.IsZero() {
		return Web3ProjectionV1{}, errors.New("created time is required")
	}
	if !validSHA256(input.RequestDigestSHA256) {
		return Web3ProjectionV1{}, errors.New("request digest must be lowercase SHA-256 hex")
	}
	if strings.TrimSpace(input.Network) == "" || strings.TrimSpace(input.Target) == "" || strings.TrimSpace(input.RequestedOperation) == "" {
		return Web3ProjectionV1{}, errors.New("network, target and requested operation are required")
	}
	if strings.TrimSpace(input.RequestedEffect) == "" {
		return Web3ProjectionV1{}, errors.New("requested effect is required")
	}
	if _, ok := actualEffectStates[input.ActualEffectState]; !ok {
		return Web3ProjectionV1{}, errors.New("unsupported actual effect state")
	}
	if _, ok := observationStates[input.IndependentObservationState]; !ok {
		return Web3ProjectionV1{}, errors.New("unsupported independent observation state")
	}
	if _, ok := mappingStates[input.MappingState]; !ok {
		return Web3ProjectionV1{}, errors.New("unsupported mapping state")
	}
	if strings.TrimSpace(input.NativeSchema) == "" || strings.TrimSpace(input.NativeRef) == "" {
		return Web3ProjectionV1{}, errors.New("native schema and ref are required")
	}
	if !validSHA256(input.NativeDigestSHA256) {
		return Web3ProjectionV1{}, errors.New("native digest must be lowercase SHA-256 hex")
	}
	if input.ActualEffectDigestSHA256 != nil && !validSHA256(*input.ActualEffectDigestSHA256) {
		return Web3ProjectionV1{}, errors.New("actual effect digest must be lowercase SHA-256 hex")
	}
	if input.ReceiptDigestSHA256 != nil && !validSHA256(*input.ReceiptDigestSHA256) {
		return Web3ProjectionV1{}, errors.New("receipt digest must be lowercase SHA-256 hex")
	}
	if input.IndependentObservationState == "VERIFIED" && input.ReceiptDigestSHA256 == nil {
		return Web3ProjectionV1{}, errors.New("verified independent observation requires receipt digest")
	}

	return Web3ProjectionV1{
		SchemaVersion: SecurityCaseEnvelopeV1,
		Case: CaseV1{
			CaseID:      input.CaseID,
			CreatedAt:   input.CreatedAt.UTC().Format(time.RFC3339Nano),
			Coordinator: "koschei-web3",
			TraceID:     input.TraceID,
		},
		Request: RequestV1{
			RequestDigestSHA256: input.RequestDigestSHA256,
			Network:             input.Network,
			Target:              input.Target,
			RequestedOperation:  input.RequestedOperation,
			Nonce:               input.Nonce,
		},
		Effect: EffectV1{
			RequestedEffect:              input.RequestedEffect,
			ActualEffectState:            input.ActualEffectState,
			ActualEffectDigestSHA256:     input.ActualEffectDigestSHA256,
			IndependentObservationState: input.IndependentObservationState,
			ReceiptRef:                   input.ReceiptRef,
			ReceiptDigestSHA256:          input.ReceiptDigestSHA256,
		},
		NativeBinding: NativeBindingV1{
			Owner:              "koschei-web3",
			NativeSchema:       input.NativeSchema,
			NativeRef:          input.NativeRef,
			NativeDigestSHA256: input.NativeDigestSHA256,
			AdapterVersion:     Web3AdapterVersionV1,
			MappingState:       input.MappingState,
		},
	}, nil
}

func validSHA256(value string) bool {
	if len(value) != 64 || strings.ToLower(value) != value {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 32
}
