package cache

import (
	"context"
	"time"
)

type Noop struct{}

func NewNoop() Noop                                                    { return Noop{} }
func (Noop) GetJSON(context.Context, string, any) (bool, error)        { return false, nil }
func (Noop) SetJSON(context.Context, string, any, time.Duration) error { return nil }
func (Noop) Delete(context.Context, ...string) error                   { return nil }
func (Noop) Ping(context.Context) error                                { return nil }
func (Noop) Close() error                                              { return nil }
