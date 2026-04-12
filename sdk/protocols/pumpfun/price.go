package pumpfun

import "github.com/shopspring/decimal"

func ComputeCurvePriceAndLiquidity(state CurveState) (price decimal.Decimal, liquidity decimal.Decimal) {
	if state.TokenReserve.IsZero() {
		return decimal.Zero, state.SOLReserve.Mul(decimal.NewFromInt(2))
	}
	price = state.SOLReserve.Div(state.TokenReserve)
	liquidity = state.SOLReserve.Mul(decimal.NewFromInt(2))
	return price, liquidity
}
