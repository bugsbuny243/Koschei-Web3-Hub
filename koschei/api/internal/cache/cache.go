package cache

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

var ErrMiss = errors.New("cache miss")

type Cache interface {
	GetJSON(ctx context.Context, key string, dst any) (bool, error)
	SetJSON(ctx context.Context, key string, value any, ttl time.Duration) error
	Delete(ctx context.Context, keys ...string) error
	Ping(ctx context.Context) error
	Close() error
}

func MarshalJSON(value any) ([]byte, error)    { return json.Marshal(value) }
func UnmarshalJSON(data []byte, dst any) error { return json.Unmarshal(data, dst) }
