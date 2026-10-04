package agents

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestProductionChannelCannotFallBackToDemoInventory(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("TRADEPI_AGENT_LLM_API_KEY", "")
	s := NewService()
	for _, channel := range []Channel{ChannelTelegram, ChannelWhatsApp, ChannelWeb} {
		result := s.Handle(context.Background(), Message{TenantID: "actual-customer", Channel: channel, ChannelUserID: "fixture-user", Text: "BMW 320i", ReceivedAt: time.Now()})
		if len(result.Vehicles) != 0 {
			t.Fatalf("production %s received fixture inventory: %+v", channel, result.Vehicles)
		}
	}
	demo := NewDemoService()
	result := demo.Handle(context.Background(), Message{TenantID: "demo-automotive", Channel: ChannelWeb, ChannelUserID: "fixture-user", Text: "BMW 320i", ReceivedAt: time.Now()})
	if len(result.Vehicles) == 0 {
		t.Fatal("explicit public demo lost its fixture inventory")
	}
	for _, channel := range []Channel{ChannelTelegram, ChannelWhatsApp, ChannelWeb} {
		result = s.Handle(context.Background(), Message{TenantID: "demo-automotive", Channel: channel, ChannelUserID: "fixture-user", Text: "BMW 320i", ReceivedAt: time.Now()})
		if len(result.Vehicles) != 0 {
			t.Fatal("real channel inherited demo inventory from tenant name")
		}
	}
	if demo.db != nil || demo.llm != nil {
		t.Fatal("demo inherited customer persistence or paid provider")
	}
}

func TestPeriodicWorkerStopsWithParentContext(t *testing.T) {
	s := &Service{}
	ctx, cancel := context.WithCancel(context.Background())
	var calls atomic.Int32
	first := make(chan struct{}, 1)
	s.startPeriodic(ctx, time.Hour, func(context.Context) { calls.Add(1); first <- struct{}{} })
	select {
	case <-first:
	case <-time.After(time.Second):
		t.Fatal("worker did not start")
	}
	cancel()
	done := make(chan struct{})
	go func() { s.workerWG.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker ignored cancellation")
	}
	if calls.Load() != 1 {
		t.Fatal("worker ran again after cancellation")
	}
}
