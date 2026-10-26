package handlers

import (
	"context"
	"testing"
	"time"
)

func TestActorDiscoveryStageLeavesReserveForLaterActorStages(t *testing.T) {
	t.Setenv("ARVIS_ACTOR_STAGE_RESERVE_SECONDS", "45")
	parent, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()
	ctx, done := actorDiscoveryStageContext(parent)
	defer done()
	stage, _ := ctx.Deadline()
	whole, _ := parent.Deadline()
	if left := whole.Sub(stage); left < 44*time.Second || left > 46*time.Second {
		t.Fatalf("discovery left %s for funding origin and live evidence, want 45s", left)
	}

	short, cancelShort := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancelShort()
	ctx, done = actorDiscoveryStageContext(short)
	defer done()
	stage, _ = ctx.Deadline()
	whole, _ = short.Deadline()
	if left := whole.Sub(stage); left < 29*time.Second || left > 31*time.Second {
		t.Fatalf("with little time left discovery must take at most half, left %s", left)
	}

	ctx, done = actorDiscoveryStageContext(context.Background())
	defer done()
	if _, ok := ctx.Deadline(); ok {
		t.Fatal("an unbounded parent must not gain an artificial deadline")
	}
}
