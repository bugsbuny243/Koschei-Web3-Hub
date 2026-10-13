package handlers

import (
	"math"
	"testing"
)

func TestValidLiquidityRadarInputs(t *testing.T) {
	tests := []struct {
		name string
		req  liquidityRadarRequest
		want bool
	}{
		{name: "valid", req: liquidityRadarRequest{ReserveDropPct: 35, RemovedLiquidity: 50000, BlockDelay: 1}, want: true},
		{name: "zero values", req: liquidityRadarRequest{}, want: true},
		{name: "negative percentage", req: liquidityRadarRequest{ReserveDropPct: -1}},
		{name: "percentage over 100", req: liquidityRadarRequest{ReserveDropPct: 101}},
		{name: "negative removed amount", req: liquidityRadarRequest{RemovedLiquidity: -1}},
		{name: "negative block delay", req: liquidityRadarRequest{BlockDelay: -1}},
		{name: "nan percentage", req: liquidityRadarRequest{ReserveDropPct: math.NaN()}},
		{name: "infinite amount", req: liquidityRadarRequest{RemovedLiquidity: math.Inf(1)}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := validLiquidityRadarInputs(tc.req); got != tc.want {
				t.Fatalf("validLiquidityRadarInputs() = %v, want %v", got, tc.want)
			}
		})
	}
}
