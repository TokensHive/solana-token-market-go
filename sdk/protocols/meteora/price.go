package meteora

import "github.com/shopspring/decimal"

func ConstantProductPrice(baseReserve, quoteReserve decimal.Decimal) decimal.Decimal {
if baseReserve.IsZero() {
return decimal.Zero
}
return quoteReserve.Div(baseReserve)
}

func DLMMActiveBinPrice(binPrice decimal.Decimal) decimal.Decimal { return binPrice }
