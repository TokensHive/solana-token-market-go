package orca

import (
	"encoding/binary"
	"math"

	"github.com/shopspring/decimal"
)

func DecodeWhirlpoolPrice(data []byte) decimal.Decimal {
	if len(data) < 24 {
		return decimal.Zero
	}
	sqrtX64 := binary.LittleEndian.Uint64(data[16:24])
	if sqrtX64 == 0 {
		return decimal.Zero
	}
	f := float64(sqrtX64) / math.Pow(2, 64)
	return decimal.NewFromFloat(f * f)
}
