package cache

import (
	"context"
	"sync"
	"time"
)

type memoryItem struct {
	data      []byte
	expiresAt time.Time
}

type Memory struct {
	mu    sync.RWMutex
	items map[string]memoryItem
}

func NewMemory() *Memory { return &Memory{items: map[string]memoryItem{}} }
func (m *Memory) GetJSON(_ context.Context, key string, dst any) (bool, error) {
	m.mu.RLock()
	item, ok := m.items[key]
	m.mu.RUnlock()
	if !ok || (!item.expiresAt.IsZero() && time.Now().After(item.expiresAt)) {
		if ok {
			_ = m.Delete(context.Background(), key)
		}
		return false, nil
	}
	if err := UnmarshalJSON(item.data, dst); err != nil {
		return false, err
	}
	return true, nil
}
func (m *Memory) SetJSON(_ context.Context, key string, value any, ttl time.Duration) error {
	data, err := MarshalJSON(value)
	if err != nil {
		return err
	}
	item := memoryItem{data: data}
	if ttl > 0 {
		item.expiresAt = time.Now().Add(ttl)
	}
	m.mu.Lock()
	m.items[key] = item
	m.mu.Unlock()
	return nil
}
func (m *Memory) Delete(_ context.Context, keys ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, k := range keys {
		delete(m.items, k)
	}
	return nil
}
func (m *Memory) Ping(context.Context) error { return nil }
func (m *Memory) Close() error               { return nil }
