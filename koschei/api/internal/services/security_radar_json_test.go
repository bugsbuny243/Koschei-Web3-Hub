package services

import (
	"encoding/json"
	"math"
	"testing"
)

func TestMarshalSecurityRadarJSONSanitizesNonFiniteFloats(t *testing.T) {
	input := map[string]any{
		"ok":  12.5,
		"nan": math.NaN(),
		"inf": math.Inf(1),
		"nested": []any{
			map[string]any{"negative_inf": math.Inf(-1)},
		},
	}

	encoded, err := marshalSecurityRadarJSON(input)
	if err != nil {
		t.Fatalf("marshalSecurityRadarJSON returned error: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("encoded payload is not valid JSON: %v", err)
	}
	if decoded["ok"] != 12.5 {
		t.Fatalf("finite value changed: %#v", decoded)
	}
	if decoded["nan"] != nil || decoded["inf"] != nil {
		t.Fatalf("non-finite values were not sanitized: %#v", decoded)
	}
}

func TestMarshalSecurityRadarJSONHandlesUnsupportedValues(t *testing.T) {
	input := map[string]any{
		"channel": make(chan int),
		"func":    func() {},
	}
	encoded, err := marshalSecurityRadarJSON(input)
	if err != nil {
		t.Fatalf("marshalSecurityRadarJSON returned error: %v", err)
	}
	if !json.Valid(encoded) {
		t.Fatalf("encoded payload is invalid JSON: %s", string(encoded))
	}
}
