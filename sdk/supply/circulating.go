package supply

import "github.com/shopspring/decimal"

type CirculatingAdjustment struct {
	Label  string
	Amount decimal.Decimal
}

func ApplyAdjustments(total decimal.Decimal, adjustments []CirculatingAdjustment) decimal.Decimal {
	out := total
	for _, adj := range adjustments {
		out = out.Sub(adj.Amount)
	}
	if out.IsNegative() {
		return decimal.Zero
	}
	return out
}
