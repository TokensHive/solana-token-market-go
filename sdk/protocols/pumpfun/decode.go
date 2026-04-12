package pumpfun

import "github.com/shopspring/decimal"

type CurveState struct {
	TokenReserve decimal.Decimal
	SOLReserve   decimal.Decimal
	Complete     bool
}

func DecodeCurveState(data []byte) CurveState {
	if len(data) < 24 {
		return CurveState{}
	}
	t := decimal.NewFromUint64(readU64(data, 8)).Shift(-6)
	s := decimal.NewFromUint64(readU64(data, 16)).Shift(-9)
	complete := data[len(data)-1]%2 == 1
	return CurveState{TokenReserve: t, SOLReserve: s, Complete: complete}
}
