package market

import (
	"github.com/gagliardetto/solana-go"
	"github.com/shopspring/decimal"
)

type Decimal = decimal.Decimal

type Dex string

const (
	DexPumpfun Dex = "pumpfun"
)

type PoolVersion string

const (
	PoolVersionPumpfunBondingCurve PoolVersion = "bonding_curve"
	PoolVersionPumpfunAmm          PoolVersion = "pumpswap_amm"
)

type PoolIdentifier struct {
	Dex         Dex
	PoolVersion PoolVersion
	MintA       solana.PublicKey
	MintB       solana.PublicKey
	PoolAddress solana.PublicKey
}

type GetMetricsByPoolRequest struct {
	Pool PoolIdentifier
}

type GetMetricsByPoolResponse struct {
	Pool              PoolIdentifier
	PriceOfAInB       Decimal
	PriceOfAInSOL     Decimal
	LiquidityInB      Decimal
	LiquidityInSOL    Decimal
	MarketCapInSOL    Decimal
	TotalSupply       Decimal
	CirculatingSupply Decimal
	SupplyMethod      string
	Metadata          map[string]any
}
