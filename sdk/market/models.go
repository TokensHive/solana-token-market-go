package market

import (
	"context"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/shopspring/decimal"
)

type Decimal = decimal.Decimal

type Protocol string

const (
	ProtocolPumpfun          Protocol = "pumpfun"
	ProtocolPumpswap         Protocol = "pumpswap"
	ProtocolRaydiumV4        Protocol = "raydium_v4"
	ProtocolRaydiumCPMM      Protocol = "raydium_cpmm"
	ProtocolRaydiumCLMM      Protocol = "raydium_clmm"
	ProtocolRaydiumLaunchLab Protocol = "raydium_launchlab"
	ProtocolOrcaWhirlpool    Protocol = "orca_whirlpool"
	ProtocolMeteoraDLMM      Protocol = "meteora_dlmm"
	ProtocolMeteoraDAMM      Protocol = "meteora_damm"
	ProtocolMeteoraDBC       Protocol = "meteora_dbc"
)

type MarketType string

const (
	MarketTypeBondingCurve          MarketType = "bonding_curve"
	MarketTypeConstantProduct       MarketType = "constant_product"
	MarketTypeConcentratedLiquidity MarketType = "concentrated_liquidity"
	MarketTypeDynamicLiquidity      MarketType = "dynamic_liquidity"
	MarketTypeLaunchpad             MarketType = "launchpad"
)

type DiscoveryMode string

const (
	DiscoveryModeOnChain  DiscoveryMode = "onchain"
	DiscoveryModeAPIFirst DiscoveryMode = "api-first"
	DiscoveryModeHybrid   DiscoveryMode = "hybrid"
)

type Pool struct {
	Address            string
	Protocol           Protocol
	MarketType         MarketType
	BaseMint           string
	QuoteMint          string
	LPTokenMint        string
	BaseDecimals       uint8
	QuoteDecimals      uint8
	BaseReserve        Decimal
	QuoteReserve       Decimal
	PriceOfBaseInQuote Decimal
	PriceOfQuoteInBase Decimal
	PriceOfTokenInSOL  Decimal
	LiquidityInQuote   Decimal
	LiquidityInSOL     Decimal
	TotalSupply        Decimal
	CirculatingSupply  Decimal
	MarketCapInSOL     Decimal
	IsPrimary          bool
	IsVerified         bool
	IsActive           bool
	IsMigrated         bool
	IsComplete         bool
	Slot               uint64
	LastUpdatedAt      time.Time
	SelectionScore     Decimal
	SelectionReason    string
	Derived            DerivedAddresses
	Raw                RawPoolData
	Metadata           map[string]any
}

type DerivedAddresses struct {
	BondingCurve string
	Oracle       string
	VaultA       string
	VaultB       string
	TickArray0   string
	TickArray1   string
	Observation  string
	Config       string
}

type RawPoolData struct {
	ProgramID    string
	Accounts     map[string]string
	StateVersion string
	Extra        map[string]any
}

type ResolvePoolsRequest struct {
	Mint              solana.PublicKey
	QuoteMint         *solana.PublicKey
	Protocols         []Protocol
	MarketTypes       []MarketType
	IncludeInactive   bool
	IncludeUnverified bool
	DiscoveryMode     DiscoveryMode
	SelectPrimary     bool
	PoolAddresses     []solana.PublicKey
	DirectSOLOnly     bool
}

type ResolveMetadata struct {
	DiscoveryMode DiscoveryMode
	PoolsFound    int
	RankedAt      time.Time
	Debug         map[string]any
}

type ResolvePoolsResponse struct {
	Mint        string
	PrimaryPool *Pool
	Pools       []*Pool
	Metadata    ResolveMetadata
}

type GetPoolRequest struct {
	PoolAddress solana.PublicKey
}

type GetTokenMarketRequest struct {
	Mint          solana.PublicKey
	QuoteMint     *solana.PublicKey
	Protocols     []Protocol
	DiscoveryMode DiscoveryMode
}

type MarketMetadata struct {
	DiscoveryMode DiscoveryMode
	SupplyMethod  string
	Debug         map[string]any
}

type GetTokenMarketResponse struct {
	Mint              string
	PrimaryPool       *Pool
	Pools             []*Pool
	PriceInSOL        Decimal
	LiquidityInSOL    Decimal
	MarketCapInSOL    Decimal
	TotalSupply       Decimal
	CirculatingSupply Decimal
	Metadata          MarketMetadata
}

type DiscoveryRequest struct {
	Mint              solana.PublicKey
	QuoteMint         *solana.PublicKey
	Protocols         []Protocol
	MarketTypes       []MarketType
	IncludeInactive   bool
	IncludeUnverified bool
	DiscoveryMode     DiscoveryMode
	PoolAddresses     []solana.PublicKey
	DirectSOLOnly     bool
}

type DiscoveryEngine interface {
	Discover(ctx context.Context, req DiscoveryRequest) ([]*Pool, map[string]any, error)
	FindByPoolAddress(ctx context.Context, addr solana.PublicKey) ([]*Pool, map[string]any, error)
}

type PoolMetrics struct {
	PriceOfTokenInSOL  Decimal
	LiquidityInSOL     Decimal
	LiquidityInQuote   Decimal
	MarketCapInSOL     Decimal
	TotalSupply        Decimal
	CirculatingSupply  Decimal
	EstimatedLiquidity bool
}

type MetricsProvider interface {
	ComputeMetrics(targetMint solana.PublicKey, quoteMint *solana.PublicKey) (*PoolMetrics, error)
}
