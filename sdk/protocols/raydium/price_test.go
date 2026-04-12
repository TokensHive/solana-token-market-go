package raydium

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestConstantProductPrice(t *testing.T) {
	base := decimal.NewFromInt(100)
	quote := decimal.NewFromInt(25)
	got := ConstantProductPrice(base, quote)
	if !got.Equal(decimal.NewFromFloat(0.25)) {
		t.Fatalf("expected 0.25, got %s", got)
	}
}
