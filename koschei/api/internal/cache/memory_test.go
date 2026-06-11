package cache

import (
	"context"
	"testing"
	"time"
)

func TestMemoryCacheJSONRoundTripAndTTL(t *testing.T) {
	c := NewMemory()
	ctx := context.Background()
	value := map[string]string{"status": "ok"}
	if err := c.SetJSON(ctx, "k", value, 10*time.Millisecond); err != nil {
		t.Fatalf("SetJSON: %v", err)
	}
	var got map[string]string
	ok, err := c.GetJSON(ctx, "k", &got)
	if err != nil || !ok {
		t.Fatalf("GetJSON ok=%v err=%v", ok, err)
	}
	if got["status"] != "ok" {
		t.Fatalf("got %v", got)
	}
	time.Sleep(20 * time.Millisecond)
	ok, err = c.GetJSON(ctx, "k", &got)
	if err != nil {
		t.Fatalf("GetJSON expired: %v", err)
	}
	if ok {
		t.Fatalf("expected expired cache miss")
	}
}
