package web3

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSmartCache_SingleflightPreventsStampede(t *testing.T) {
	cache := NewSmartCache(NewMemoryCache())
	var loads int32
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b, err := cache.GetOrLoad(context.Background(), "k", time.Minute, func(context.Context) ([]byte, error) {
				atomic.AddInt32(&loads, 1)
				time.Sleep(10 * time.Millisecond)
				return []byte("ok"), nil
			})
			if err != nil || string(b) != "ok" {
				t.Errorf("bad result %q %v", string(b), err)
			}
		}()
	}
	wg.Wait()
	if loads != 1 {
		t.Fatalf("loads=%d", loads)
	}
}
