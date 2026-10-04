package main

import (
	"context"
	"koschei/api/internal/runtimehealth"
	"testing"
)

func TestCampaignRuntimeActivationFailsClosed(t *testing.T) {
	t.Setenv("KOSCHEI_GLOBAL_CAMPAIGN_RUNTIME_ENABLED", "")
	health := runtimehealth.New()
	sink, cfg, err := buildGlobalCampaignRuntime(context.Background(), nil, nil, health)
	if err != nil || sink != nil || cfg != nil {
		t.Fatalf("default activation changed: %v %v %v", sink, cfg, err)
	}
	if health.Snapshot().Entries[0].Configured {
		t.Fatal("disabled worker reported configured")
	}
	t.Setenv("KOSCHEI_GLOBAL_CAMPAIGN_RUNTIME_ENABLED", "1")
	if _, _, err := buildGlobalCampaignRuntime(context.Background(), nil, nil, health); err == nil {
		t.Fatal("runtime enabled without durable sources")
	}
}
