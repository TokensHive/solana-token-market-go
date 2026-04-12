package cachex

import (
	"context"
	"sync"
	"time"
)

type entry struct {
	v   any
	exp time.Time
}

type InMemory struct {
	mu sync.RWMutex
	m  map[string]entry
}

func NewInMemory() *InMemory { return &InMemory{m: map[string]entry{}} }

func (c *InMemory) Get(_ context.Context, key string, dst any) (bool, error) {
	c.mu.RLock()
	e, ok := c.m[key]
	c.mu.RUnlock()
	if !ok || (!e.exp.IsZero() && time.Now().After(e.exp)) {
		return false, nil
	}
	switch d := dst.(type) {
	case *any:
		*d = e.v
	}
	return true, nil
}

func (c *InMemory) Set(_ context.Context, key string, value any, ttl time.Duration) error {
	exp := time.Time{}
	if ttl > 0 {
		exp = time.Now().Add(ttl)
	}
	c.mu.Lock()
	c.m[key] = entry{v: value, exp: exp}
	c.mu.Unlock()
	return nil
}

func (c *InMemory) Delete(_ context.Context, key string) error {
	c.mu.Lock()
	delete(c.m, key)
	c.mu.Unlock()
	return nil
}
