package raydium

import "github.com/shopspring/decimal"

func ConstantProductPrice(baseReserve, quoteReserve decimal.Decimal) decimal.Decimal {
	if baseReserve.IsZero() {
		return decimal.Zero
	}
	return quoteReserve.Div(baseReserve)
}
