package agents

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestDemoFixtureRetentionIsBounded(t *testing.T) {
	s := NewDemoService()
	for i := 0; i < 330; i++ {
		s.Handle(context.Background(), Message{
			TenantID: "demo-automotive",
			Channel: ChannelWeb,
			ChannelUserID: fmt.Sprintf("fixture-user-%d", i),
			Text: "Hello",
			ReceivedAt: time.Now().UTC(),
		})
	}
	s.mu.RLock()
	count := len(s.leads)
	s.mu.RUnlock()
	if count > 256 {
		t.Fatalf("public fixture memory grew without bound: %d", count)
	}
}
