package cachex

import (
	"fmt"
	"reflect"
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

func (c *InMemory) Get(key string, dst any) (bool, error) {
	c.mu.RLock()
	e, ok := c.m[key]
	c.mu.RUnlock()
	if !ok || (!e.exp.IsZero() && time.Now().After(e.exp)) {
		return false, nil
	}
	dv := reflect.ValueOf(dst)
	if !dv.IsValid() || dv.Kind() != reflect.Ptr || dv.IsNil() {
		return false, fmt.Errorf("destination parameter must be a non-nil pointer")
	}
	ev := dv.Elem()
	v := reflect.ValueOf(e.v)
	if !v.IsValid() {
		ev.Set(reflect.Zero(ev.Type()))
		return true, nil
	}
	if v.Type().AssignableTo(ev.Type()) {
		ev.Set(v)
		return true, nil
	}
	if v.Type().ConvertibleTo(ev.Type()) {
		ev.Set(v.Convert(ev.Type()))
		return true, nil
	}
	return false, fmt.Errorf("type mismatch: cached value of type %s cannot be assigned to destination type %s", v.Type(), ev.Type())
}

func (c *InMemory) Set(key string, value any, ttl time.Duration) error {
	exp := time.Time{}
	if ttl > 0 {
		exp = time.Now().Add(ttl)
	}
	c.mu.Lock()
	c.m[key] = entry{v: value, exp: exp}
	c.mu.Unlock()
	return nil
}

func (c *InMemory) Delete(key string) error {
	c.mu.Lock()
	delete(c.m, key)
	c.mu.Unlock()
	return nil
}
