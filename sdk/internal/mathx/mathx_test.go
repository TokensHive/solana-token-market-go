package mathx

import "testing"

func TestMax(t *testing.T) {
	if got := Max(1, 2); got != 2 {
		t.Fatalf("expected 2, got %d", got)
	}
	if got := Max(int64(5), int64(3)); got != 5 {
		t.Fatalf("expected 5, got %d", got)
	}
	if got := Max(uint64(4), uint64(9)); got != 9 {
		t.Fatalf("expected 9, got %d", got)
	}
	if got := Max(3.5, 2.1); got != 3.5 {
		t.Fatalf("expected 3.5, got %f", got)
	}
}
