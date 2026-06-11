package web3

import (
	"context"
	"sync"
	"time"

	"koschei/api/internal/singleflight"
)

type CacheClient interface {
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
}

type SmartCache struct {
	store CacheClient
	group singleflight.Group
}

func NewSmartCache(store CacheClient) *SmartCache { return &SmartCache{store: store} }

func (c *SmartCache) GetOrLoad(ctx context.Context, key string, ttl time.Duration, loader func(context.Context) ([]byte, error)) ([]byte, error) {
	if c == nil || c.store == nil {
		return loader(ctx)
	}
	if b, ok, err := c.store.Get(ctx, key); err != nil {
		return nil, err
	} else if ok {
		return b, nil
	}
	v, err, _ := c.group.Do(key, func() (interface{}, error) {
		if b, ok, err := c.store.Get(ctx, key); err != nil {
			return nil, err
		} else if ok {
			return b, nil
		}
		b, err := loader(ctx)
		if err != nil {
			return nil, err
		}
		if err := c.store.Set(ctx, key, b, ttl); err != nil {
			return nil, err
		}
		return b, nil
	})
	if err != nil {
		return nil, err
	}
	return v.([]byte), nil
}

type MemoryCache struct {
	mu    sync.Mutex
	items map[string]memoryCacheItem
}

type memoryCacheItem struct {
	value     []byte
	expiresAt time.Time
}

func NewMemoryCache() *MemoryCache { return &MemoryCache{items: map[string]memoryCacheItem{}} }

func (c *MemoryCache) Get(_ context.Context, key string) ([]byte, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	item, ok := c.items[key]
	if !ok || time.Now().After(item.expiresAt) {
		delete(c.items, key)
		return nil, false, nil
	}
	return append([]byte(nil), item.value...), true, nil
}

func (c *MemoryCache) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = memoryCacheItem{value: append([]byte(nil), value...), expiresAt: time.Now().Add(ttl)}
	return nil
}
