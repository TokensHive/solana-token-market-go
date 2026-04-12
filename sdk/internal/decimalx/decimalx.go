package decimalx

import "github.com/shopspring/decimal"

func SafeDiv(a, b decimal.Decimal) decimal.Decimal {
if b.IsZero() {
return decimal.Zero
}
return a.Div(b)
}
