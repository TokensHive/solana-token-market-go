package pumpfun

import (
	"github.com/TokensHive/solana-token-market-go/sdk/market"
	"github.com/shopspring/decimal"
)

func ShouldPreferMigratedPools(curve *market.Pool, migrated []*market.Pool) bool {
	if curve == nil || !curve.IsComplete {
		return false
	}
	for _, p := range migrated {
		if p != nil && p.Protocol != market.ProtocolPumpfun && p.LiquidityInSOL.GreaterThan(decimal.Zero) {
			return true
		}
	}
	return false
}
