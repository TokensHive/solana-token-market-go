package cachex

import (
	"testing"
	"time"
)

func TestInMemorySetGetDelete(t *testing.T) {
	cache := NewInMemory()

	if err := cache.Set("k", 42, time.Minute); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	var out int
	ok, err := cache.Get("k", &out)
	if err != nil || !ok {
		t.Fatalf("get failed ok=%v err=%v", ok, err)
	}
	if out != 42 {
		t.Fatalf("unexpected value: %d", out)
	}

	if err := cache.Delete("k"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	ok, err = cache.Get("k", &out)
	if err != nil || ok {
		t.Fatalf("expected cache miss after delete, ok=%v err=%v", ok, err)
	}
}

func TestInMemoryGetErrors(t *testing.T) {
	cache := NewInMemory()
	_ = cache.Set("k", 42, 0)

	if _, err := cache.Get("k", nil); err == nil {
		t.Fatal("expected invalid destination error")
	}

	var out int
	if _, err := cache.Get("k", out); err == nil {
		t.Fatal("expected pointer destination error")
	}

	var ptr *int
	if _, err := cache.Get("k", ptr); err == nil {
		t.Fatal("expected non-nil pointer error")
	}
}

func TestInMemoryGetTypePaths(t *testing.T) {
	cache := NewInMemory()

	_ = cache.Set("convertible", int32(7), 0)
	var converted int64
	ok, err := cache.Get("convertible", &converted)
	if err != nil || !ok || converted != 7 {
		t.Fatalf("expected convertible assignment, ok=%v err=%v converted=%d", ok, err, converted)
	}

	_ = cache.Set("mismatch", "value", 0)
	var mismatch int
	if _, err := cache.Get("mismatch", &mismatch); err == nil {
		t.Fatal("expected type mismatch error")
	}

	_ = cache.Set("nilvalue", nil, 0)
	var zeroed int
	ok, err = cache.Get("nilvalue", &zeroed)
	if err != nil || !ok || zeroed != 0 {
		t.Fatalf("expected zero value for nil cache entry, ok=%v err=%v zeroed=%d", ok, err, zeroed)
	}
}

func TestInMemoryExpirationAndMiss(t *testing.T) {
	cache := NewInMemory()
	_ = cache.Set("soon-expire", 1, 2*time.Millisecond)
	time.Sleep(10 * time.Millisecond)

	var out int
	ok, err := cache.Get("soon-expire", &out)
	if err != nil || ok {
		t.Fatalf("expected expired miss, ok=%v err=%v", ok, err)
	}

	ok, err = cache.Get("unknown", &out)
	if err != nil || ok {
		t.Fatalf("expected missing key miss, ok=%v err=%v", ok, err)
	}
}
