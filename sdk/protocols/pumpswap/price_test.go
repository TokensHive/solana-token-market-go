package pumpswap

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestComputePrice_BaseAndQuote(t *testing.T) {
	base := decimal.NewFromInt(20)
	quote := decimal.NewFromInt(10)
	if !ComputePrice(base, quote, true).Equal(decimal.NewFromFloat(0.5)) {
		t.Fatal("base-target price mismatch")
	}
	if !ComputePrice(base, quote, false).Equal(decimal.NewFromInt(2)) {
		t.Fatal("quote-target price mismatch")
	}
}
