package pumpswap

import "github.com/shopspring/decimal"

func ComputePrice(baseReserve, quoteReserve decimal.Decimal, targetIsBase bool) decimal.Decimal {
if targetIsBase {
if baseReserve.IsZero() {
return decimal.Zero
}
return quoteReserve.Div(baseReserve)
}
if quoteReserve.IsZero() {
return decimal.Zero
}
return baseReserve.Div(quoteReserve)
}
