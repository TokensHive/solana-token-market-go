package market

import (
	"github.com/gagliardetto/solana-go"
	"github.com/shopspring/decimal"
)

type Decimal = decimal.Decimal

type Dex string

const (
	DexPumpfun Dex = "pumpfun"
	DexRaydium Dex = "raydium"
	DexMeteora Dex = "meteora"
	DexOrca    Dex = "orca"
)

type PoolVersion string

const (
	PoolVersionPumpfunBondingCurve PoolVersion = "bonding_curve"
	PoolVersionPumpfunAmm          PoolVersion = "pumpswap_amm"
	PoolVersionRaydiumLiquidityV4  PoolVersion = "liquidity_v4"
	PoolVersionRaydiumCPMM         PoolVersion = "cpmm"
	PoolVersionRaydiumCLMM         PoolVersion = "clmm"
	PoolVersionRaydiumLaunchpad    PoolVersion = "launchpad"
	PoolVersionMeteoraDLMM         PoolVersion = "dlmm"
	PoolVersionMeteoraDBC          PoolVersion = "dbc"
	PoolVersionMeteoraDAMMV1       PoolVersion = "damm_v1"
	PoolVersionMeteoraDAMMV2       PoolVersion = "damm_v2"
	PoolVersionOrcaWhirlpool       PoolVersion = "whirlpool"
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
	FDVInSOL          Decimal
	TotalSupply       Decimal
	CirculatingSupply Decimal
	SupplyMethod      string
	Metadata          map[string]any
}
