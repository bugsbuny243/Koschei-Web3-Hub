package main

import "testing"

func TestParseRuntimeRole(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want runtimeRole
	}{
		{name: "default", raw: "", want: runtimeRoleCombined},
		{name: "combined", raw: "combined", want: runtimeRoleCombined},
		{name: "api", raw: "api", want: runtimeRoleAPI},
		{name: "worker", raw: "worker", want: runtimeRoleWorker},
		{name: "trimmed case insensitive", raw: "  WORKER  ", want: runtimeRoleWorker},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseRuntimeRole(tt.raw)
			if err != nil {
				t.Fatalf("parseRuntimeRole(%q) error: %v", tt.raw, err)
			}
			if got != tt.want {
				t.Fatalf("parseRuntimeRole(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestParseRuntimeRoleRejectsUnknownRole(t *testing.T) {
	if _, err := parseRuntimeRole("everything"); err == nil {
		t.Fatal("parseRuntimeRole accepted unsupported role")
	}
}

func TestRuntimeRoleCapabilities(t *testing.T) {
	tests := []struct {
		role              runtimeRole
		wantHTTP          bool
		wantBackgroundRun bool
	}{
		{role: runtimeRoleCombined, wantHTTP: true, wantBackgroundRun: true},
		{role: runtimeRoleAPI, wantHTTP: true, wantBackgroundRun: false},
		{role: runtimeRoleWorker, wantHTTP: false, wantBackgroundRun: true},
	}
	for _, tt := range tests {
		if got := tt.role.servesHTTP(); got != tt.wantHTTP {
			t.Fatalf("%s servesHTTP() = %t, want %t", tt.role, got, tt.wantHTTP)
		}
		if got := tt.role.runsBackgroundWorkers(); got != tt.wantBackgroundRun {
			t.Fatalf("%s runsBackgroundWorkers() = %t, want %t", tt.role, got, tt.wantBackgroundRun)
		}
	}
}
