package decimalx

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestSafeDiv(t *testing.T) {
	if got := SafeDiv(decimal.NewFromInt(10), decimal.NewFromInt(2)); !got.Equal(decimal.NewFromInt(5)) {
		t.Fatalf("expected 5, got %s", got.String())
	}
	if got := SafeDiv(decimal.NewFromInt(10), decimal.Zero); !got.Equal(decimal.Zero) {
		t.Fatalf("expected zero on division by zero, got %s", got.String())
	}
}
