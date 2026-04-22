package launchpad

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"math/big"

	"github.com/TokensHive/solana-token-market-go/sdk/internal/parallelx"
	"github.com/TokensHive/solana-token-market-go/sdk/internal/pubkeyx"
	"github.com/TokensHive/solana-token-market-go/sdk/quote"
	"github.com/TokensHive/solana-token-market-go/sdk/rpc"
	"github.com/TokensHive/solana-token-market-go/sdk/supply"
	"github.com/gagliardetto/solana-go"
	"github.com/shopspring/decimal"
)

const (
	poolStateLength   = 429
	configStateLength = 371

	// PoolState offsets.
	poolEpochOffset         = 8
	poolAuthBumpOffset      = 16
	poolStatusOffset        = 17
	poolBaseDecimalsOffset  = 18
	poolQuoteDecimalsOffset = 19
	poolMigrateTypeOffset   = 20
	poolSupplyOffset        = 21
	poolTotalSellBaseOffset = 29
	poolVirtualBaseOffset   = 37
	poolVirtualQuoteOffset  = 45
	poolRealBaseOffset      = 53
	poolRealQuoteOffset     = 61
	poolTotalRaiseOffset    = 69
	poolProtocolFeeOffset   = 77
	poolPlatformFeeOffset   = 85
	poolMigrateFeeOffset    = 93

	poolVestingTotalLockedOffset = 101
	poolVestingCliffOffset       = 109
	poolVestingUnlockOffset      = 117
	poolVestingStartOffset       = 125
	poolVestingAllocatedOffset   = 133

	poolConfigOffset       = 141
	poolPlatformCfgOffset  = 173
	poolBaseMintOffset     = 205
	poolQuoteMintOffset    = 237
	poolBaseVaultOffset    = 269
	poolQuoteVaultOffset   = 301
	poolCreatorOffset      = 333
	poolTokenProgramOffset = 365
	poolCpmmFeeOnOffset    = 366

	// GlobalConfig offsets.
	configCurveTypeOffset      = 16
	configIndexOffset          = 17
	configMigrateFeeOffset     = 19
	configTradeFeeRateOffset   = 27
	configMaxShareFeeRate      = 35
	configMinSupplyOffset      = 43
	configMaxLockRateOffset    = 51
	configMinSellRateOffset    = 59
	configMinMigrateRateOffset = 67
	configMinRaiseOffset       = 75
	configQuoteMintOffset      = 83

	curveTypeConstantProduct = 0
	curveTypeFixedPrice      = 1
	curveTypeLinearPrice     = 2
)

var (
	launchpadProgramID          = solana.MustPublicKeyFromBase58("LanMV9sAd7wArD4vJFi2qDdfnVhFxYSUg6eADduJ3uj")
	poolStateDiscriminator      = []byte{247, 237, 227, 245, 215, 195, 222, 70}
	globalConfigDiscriminator   = []byte{149, 8, 156, 202, 160, 252, 176, 217}
	twoPow64AsBigInt            = new(big.Int).Lsh(big.NewInt(1), 64)
	oneMillion                  = decimal.NewFromInt(1_000_000)
	launchpadSourceAccountLabel = "raydium_launchpad_pool_state"
)

type Calculator struct {
	rpc    rpc.Client
	quotes quote.Bridge
	supply supply.Provider
}

type Request struct {
	PoolAddress solana.PublicKey
	MintA       solana.PublicKey
	MintB       solana.PublicKey
}

type Result struct {
	MintA             solana.PublicKey
	MintB             solana.PublicKey
	PriceOfAInB       decimal.Decimal
	PriceOfAInSOL     decimal.Decimal
	LiquidityInB      decimal.Decimal
	LiquidityInSOL    decimal.Decimal
	MarketCapInSOL    decimal.Decimal
	FDVInSOL          decimal.Decimal
	TotalSupply       decimal.Decimal
	CirculatingSupply decimal.Decimal
	SupplyMethod      string
	Metadata          map[string]any
}

type poolState struct {
	epoch             uint64
	authBump          uint8
	status            uint8
	baseDecimals      uint8
	quoteDecimals     uint8
	migrateType       uint8
	supplyRaw         uint64
	totalSellBaseRaw  uint64
	virtualBaseRaw    uint64
	virtualQuoteRaw   uint64
	realBaseRaw       uint64
	realQuoteRaw      uint64
	totalRaiseQuote   uint64
	protocolFeeQuote  uint64
	platformFeeQuote  uint64
	migrateFeeQuote   uint64
	totalLockedRaw    uint64
	cliffPeriodRaw    uint64
	unlockPeriodRaw   uint64
	vestingStartRaw   uint64
	allocatedShareRaw uint64
	config            solana.PublicKey
	platformConfig    solana.PublicKey
	baseMint          solana.PublicKey
	quoteMint         solana.PublicKey
	baseVault         solana.PublicKey
	quoteVault        solana.PublicKey
	creator           solana.PublicKey
	tokenProgramFlag  uint8
	cpmmCreatorFeeOn  uint8
}

type configState struct {
	curveType           uint8
	index               uint16
	migrateFee          uint64
	tradeFeeRate        uint64
	maxShareFeeRate     uint64
	minSupplyBaseRaw    uint64
	maxLockRate         uint64
	minSellRate         uint64
	minMigrateRate      uint64
	minFundRaisingQuote uint64
	quoteMint           solana.PublicKey
}

type reserveSnapshot struct {
	baseMint     solana.PublicKey
	quoteMint    solana.PublicKey
	baseReserve  decimal.Decimal
	quoteReserve decimal.Decimal
}

func NewCalculator(rpcClient rpc.Client, quoteBridge quote.Bridge, supplyProvider supply.Provider) *Calculator {
	return &Calculator{
		rpc:    rpcClient,
		quotes: quoteBridge,
		supply: supplyProvider,
	}
}

func (c *Calculator) Compute(ctx context.Context, req Request) (*Result, error) {
	if c.rpc == nil {
		return nil, fmt.Errorf("rpc client is required")
	}
	if c.supply == nil {
		return nil, fmt.Errorf("supply provider is required")
	}
	if req.PoolAddress.IsZero() {
		return nil, fmt.Errorf("pool address is required")
	}

	poolInfo, err := c.rpc.GetAccount(ctx, req.PoolAddress)
	if err != nil {
		return nil, err
	}
	if poolInfo == nil || !poolInfo.Exists {
		return nil, fmt.Errorf("pool account not found")
	}
	if !poolInfo.Owner.Equals(launchpadProgramID) {
		return nil, fmt.Errorf("invalid raydium launchpad owner: %s", poolInfo.Owner.String())
	}

	state, err := decodePoolState(poolInfo.Data)
	if err != nil {
		return nil, err
	}
	resolvedReq := Request{
		PoolAddress: req.PoolAddress,
		MintA:       state.baseMint,
		MintB:       state.quoteMint,
	}

	configInfo, err := c.rpc.GetAccount(ctx, state.config)
	if err != nil {
		return nil, err
	}
	if configInfo == nil || !configInfo.Exists {
		return nil, fmt.Errorf("global config account not found")
	}
	if !configInfo.Owner.Equals(launchpadProgramID) {
		return nil, fmt.Errorf("invalid raydium launchpad config owner: %s", configInfo.Owner.String())
	}

	cfg, err := decodeConfigState(configInfo.Data)
	if err != nil {
		return nil, err
	}
	if !mintsEquivalent(cfg.quoteMint, state.quoteMint) {
		return nil, fmt.Errorf("pool/config quote mint mismatch: pool=%s config=%s", state.quoteMint.String(), cfg.quoteMint.String())
	}

	priceBaseInQuote, err := priceBaseInQuoteByCurve(state, cfg.curveType)
	if err != nil {
		return nil, err
	}
	snapshot := reserveForPoolState(state)
	priceAInB := priceOfMintAInMintB(resolvedReq, snapshot, priceBaseInQuote)
	liquidityInB := liquidityInMintB(resolvedReq, snapshot, priceBaseInQuote)

	var (
		priceAInSOL       decimal.Decimal
		liquidityInSOL    decimal.Decimal
		totalSupply       decimal.Decimal
		circulatingSupply decimal.Decimal
		supplyMethod      string
		fdvSupply         decimal.Decimal
		fdvMethod         string
	)
	err = parallelx.Run(ctx,
		func(taskCtx context.Context) error {
			var convErr error
			priceAInSOL, convErr = c.priceOfMintAInSOL(taskCtx, resolvedReq, priceAInB)
			if convErr != nil {
				return convErr
			}
			liquidityInSOL, convErr = c.liquidityInSOL(taskCtx, resolvedReq, liquidityInB)
			return convErr
		},
		func(taskCtx context.Context) error {
			var supplyErr error
			totalSupply, circulatingSupply, supplyMethod, supplyErr = c.supply.GetSupply(taskCtx, resolvedReq.MintA)
			if supplyErr != nil {
				return supplyErr
			}
			fdvSupply, fdvMethod = supply.ResolveFDVSupply(taskCtx, c.rpc, resolvedReq.MintA, totalSupply)
			return nil
		},
	)
	if err != nil {
		return nil, err
	}

	return &Result{
		MintA:             resolvedReq.MintA,
		MintB:             resolvedReq.MintB,
		PriceOfAInB:       priceAInB,
		PriceOfAInSOL:     priceAInSOL,
		LiquidityInB:      liquidityInB,
		LiquidityInSOL:    liquidityInSOL,
		MarketCapInSOL:    priceAInSOL.Mul(circulatingSupply),
		FDVInSOL:          priceAInSOL.Mul(fdvSupply),
		TotalSupply:       totalSupply,
		CirculatingSupply: circulatingSupply,
		SupplyMethod:      supplyMethod,
		Metadata: map[string]any{
			"dex":                           "raydium",
			"pool_version":                  "launchpad",
			"source":                        launchpadSourceAccountLabel,
			"pool_program_id":               launchpadProgramID.String(),
			"pool_epoch":                    state.epoch,
			"pool_auth_bump":                state.authBump,
			"pool_status":                   state.status,
			"pool_curve_type":               curveTypeName(cfg.curveType),
			"pool_curve_type_id":            cfg.curveType,
			"pool_migrate_type":             state.migrateType,
			"pool_token_program_flag":       state.tokenProgramFlag,
			"pool_cpmm_creator_fee_on":      state.cpmmCreatorFeeOn,
			"pool_config":                   state.config.String(),
			"pool_platform_config":          state.platformConfig.String(),
			"pool_base_mint":                state.baseMint.String(),
			"pool_quote_mint":               state.quoteMint.String(),
			"pool_base_vault":               state.baseVault.String(),
			"pool_quote_vault":              state.quoteVault.String(),
			"pool_creator":                  state.creator.String(),
			"pool_base_decimals":            state.baseDecimals,
			"pool_quote_decimals":           state.quoteDecimals,
			"pool_supply":                   decimalFromU64(state.supplyRaw, state.baseDecimals).String(),
			"pool_total_base_sell":          decimalFromU64(state.totalSellBaseRaw, state.baseDecimals).String(),
			"pool_total_quote_fund_raising": decimalFromU64(state.totalRaiseQuote, state.quoteDecimals).String(),
			"pool_virtual_base":             decimalFromU64(state.virtualBaseRaw, state.baseDecimals).String(),
			"pool_virtual_quote":            decimalFromU64(state.virtualQuoteRaw, state.quoteDecimals).String(),
			"pool_real_base_sold":           decimalFromU64(state.realBaseRaw, state.baseDecimals).String(),
			"pool_real_quote_raised":        decimalFromU64(state.realQuoteRaw, state.quoteDecimals).String(),
			"pool_base_reserve":             snapshot.baseReserve.String(),
			"pool_quote_reserve":            snapshot.quoteReserve.String(),
			"pool_protocol_fee_quote":       decimalFromU64(state.protocolFeeQuote, state.quoteDecimals).String(),
			"pool_platform_fee_quote":       decimalFromU64(state.platformFeeQuote, state.quoteDecimals).String(),
			"pool_migrate_fee_quote":        decimalFromU64(state.migrateFeeQuote, state.quoteDecimals).String(),
			"pool_vesting_total_locked":     decimalFromU64(state.totalLockedRaw, state.baseDecimals).String(),
			"pool_vesting_cliff_period":     state.cliffPeriodRaw,
			"pool_vesting_unlock_period":    state.unlockPeriodRaw,
			"pool_vesting_start_time":       state.vestingStartRaw,
			"pool_vesting_allocated_share":  decimalFromU64(state.allocatedShareRaw, state.baseDecimals).String(),
			"pool_price_base_in_quote":      priceBaseInQuote.String(),
			"config_index":                  cfg.index,
			"config_migrate_fee_quote":      decimalFromU64(cfg.migrateFee, state.quoteDecimals).String(),
			"config_trade_fee_rate":         decimal.NewFromUint64(cfg.tradeFeeRate).Div(oneMillion).String(),
			"config_max_share_fee_rate":     decimal.NewFromUint64(cfg.maxShareFeeRate).Div(oneMillion).String(),
			"config_min_supply_base":        decimalFromU64(cfg.minSupplyBaseRaw, 0).String(),
			"config_max_lock_rate":          decimal.NewFromUint64(cfg.maxLockRate).Div(oneMillion).String(),
			"config_min_sell_rate":          decimal.NewFromUint64(cfg.minSellRate).Div(oneMillion).String(),
			"config_min_migrate_rate":       decimal.NewFromUint64(cfg.minMigrateRate).Div(oneMillion).String(),
			"config_min_quote_fund_raising": decimalFromU64(cfg.minFundRaisingQuote, state.quoteDecimals).String(),
			"fdv_supply":                    fdvSupply.String(),
			"fdv_method":                    fdvMethod,
		},
	}, nil
}

func decodePoolState(data []byte) (poolState, error) {
	if len(data) < poolStateLength {
		return poolState{}, fmt.Errorf("invalid raydium launchpad pool data length: %d", len(data))
	}
	if !bytes.Equal(data[:8], poolStateDiscriminator) {
		return poolState{}, fmt.Errorf("invalid raydium launchpad pool discriminator")
	}
	return poolState{
		epoch:             binary.LittleEndian.Uint64(data[poolEpochOffset : poolEpochOffset+8]),
		authBump:          data[poolAuthBumpOffset],
		status:            data[poolStatusOffset],
		baseDecimals:      data[poolBaseDecimalsOffset],
		quoteDecimals:     data[poolQuoteDecimalsOffset],
		migrateType:       data[poolMigrateTypeOffset],
		supplyRaw:         binary.LittleEndian.Uint64(data[poolSupplyOffset : poolSupplyOffset+8]),
		totalSellBaseRaw:  binary.LittleEndian.Uint64(data[poolTotalSellBaseOffset : poolTotalSellBaseOffset+8]),
		virtualBaseRaw:    binary.LittleEndian.Uint64(data[poolVirtualBaseOffset : poolVirtualBaseOffset+8]),
		virtualQuoteRaw:   binary.LittleEndian.Uint64(data[poolVirtualQuoteOffset : poolVirtualQuoteOffset+8]),
		realBaseRaw:       binary.LittleEndian.Uint64(data[poolRealBaseOffset : poolRealBaseOffset+8]),
		realQuoteRaw:      binary.LittleEndian.Uint64(data[poolRealQuoteOffset : poolRealQuoteOffset+8]),
		totalRaiseQuote:   binary.LittleEndian.Uint64(data[poolTotalRaiseOffset : poolTotalRaiseOffset+8]),
		protocolFeeQuote:  binary.LittleEndian.Uint64(data[poolProtocolFeeOffset : poolProtocolFeeOffset+8]),
		platformFeeQuote:  binary.LittleEndian.Uint64(data[poolPlatformFeeOffset : poolPlatformFeeOffset+8]),
		migrateFeeQuote:   binary.LittleEndian.Uint64(data[poolMigrateFeeOffset : poolMigrateFeeOffset+8]),
		totalLockedRaw:    binary.LittleEndian.Uint64(data[poolVestingTotalLockedOffset : poolVestingTotalLockedOffset+8]),
		cliffPeriodRaw:    binary.LittleEndian.Uint64(data[poolVestingCliffOffset : poolVestingCliffOffset+8]),
		unlockPeriodRaw:   binary.LittleEndian.Uint64(data[poolVestingUnlockOffset : poolVestingUnlockOffset+8]),
		vestingStartRaw:   binary.LittleEndian.Uint64(data[poolVestingStartOffset : poolVestingStartOffset+8]),
		allocatedShareRaw: binary.LittleEndian.Uint64(data[poolVestingAllocatedOffset : poolVestingAllocatedOffset+8]),
		config:            solana.PublicKeyFromBytes(data[poolConfigOffset : poolConfigOffset+32]),
		platformConfig:    solana.PublicKeyFromBytes(data[poolPlatformCfgOffset : poolPlatformCfgOffset+32]),
		baseMint:          solana.PublicKeyFromBytes(data[poolBaseMintOffset : poolBaseMintOffset+32]),
		quoteMint:         solana.PublicKeyFromBytes(data[poolQuoteMintOffset : poolQuoteMintOffset+32]),
		baseVault:         solana.PublicKeyFromBytes(data[poolBaseVaultOffset : poolBaseVaultOffset+32]),
		quoteVault:        solana.PublicKeyFromBytes(data[poolQuoteVaultOffset : poolQuoteVaultOffset+32]),
		creator:           solana.PublicKeyFromBytes(data[poolCreatorOffset : poolCreatorOffset+32]),
		tokenProgramFlag:  data[poolTokenProgramOffset],
		cpmmCreatorFeeOn:  data[poolCpmmFeeOnOffset],
	}, nil
}

func decodeConfigState(data []byte) (configState, error) {
	if len(data) < configStateLength {
		return configState{}, fmt.Errorf("invalid raydium launchpad config data length: %d", len(data))
	}
	if !bytes.Equal(data[:8], globalConfigDiscriminator) {
		return configState{}, fmt.Errorf("invalid raydium launchpad config discriminator")
	}
	return configState{
		curveType:           data[configCurveTypeOffset],
		index:               binary.LittleEndian.Uint16(data[configIndexOffset : configIndexOffset+2]),
		migrateFee:          binary.LittleEndian.Uint64(data[configMigrateFeeOffset : configMigrateFeeOffset+8]),
		tradeFeeRate:        binary.LittleEndian.Uint64(data[configTradeFeeRateOffset : configTradeFeeRateOffset+8]),
		maxShareFeeRate:     binary.LittleEndian.Uint64(data[configMaxShareFeeRate : configMaxShareFeeRate+8]),
		minSupplyBaseRaw:    binary.LittleEndian.Uint64(data[configMinSupplyOffset : configMinSupplyOffset+8]),
		maxLockRate:         binary.LittleEndian.Uint64(data[configMaxLockRateOffset : configMaxLockRateOffset+8]),
		minSellRate:         binary.LittleEndian.Uint64(data[configMinSellRateOffset : configMinSellRateOffset+8]),
		minMigrateRate:      binary.LittleEndian.Uint64(data[configMinMigrateRateOffset : configMinMigrateRateOffset+8]),
		minFundRaisingQuote: binary.LittleEndian.Uint64(data[configMinRaiseOffset : configMinRaiseOffset+8]),
		quoteMint:           solana.PublicKeyFromBytes(data[configQuoteMintOffset : configQuoteMintOffset+32]),
	}, nil
}

func poolMatchesRequest(req Request, baseMint, quoteMint solana.PublicKey) bool {
	return (mintsEquivalent(req.MintA, baseMint) && mintsEquivalent(req.MintB, quoteMint)) ||
		(mintsEquivalent(req.MintA, quoteMint) && mintsEquivalent(req.MintB, baseMint))
}

func reserveForPoolState(state poolState) *reserveSnapshot {
	remainingBaseRaw := uint64(0)
	if state.totalSellBaseRaw > state.realBaseRaw {
		remainingBaseRaw = state.totalSellBaseRaw - state.realBaseRaw
	}
	return &reserveSnapshot{
		baseMint:     state.baseMint,
		quoteMint:    state.quoteMint,
		baseReserve:  decimalFromU64(remainingBaseRaw, state.baseDecimals),
		quoteReserve: decimalFromU64(state.realQuoteRaw, state.quoteDecimals),
	}
}

func priceBaseInQuoteByCurve(state poolState, curveType uint8) (decimal.Decimal, error) {
	switch curveType {
	case curveTypeConstantProduct:
		if state.virtualBaseRaw <= state.realBaseRaw {
			return decimal.Zero, nil
		}
		quoteRaw := new(big.Int).SetUint64(state.virtualQuoteRaw)
		quoteRaw = quoteRaw.Add(quoteRaw, new(big.Int).SetUint64(state.realQuoteRaw))
		baseRaw := state.virtualBaseRaw - state.realBaseRaw
		base := decimalFromU64(baseRaw, state.baseDecimals)
		quote := decimal.NewFromBigInt(quoteRaw, -int32(state.quoteDecimals))
		return quote.Div(base), nil
	case curveTypeFixedPrice:
		base := decimalFromU64(state.virtualBaseRaw, state.baseDecimals)
		if base.IsZero() {
			return decimal.Zero, nil
		}
		quote := decimalFromU64(state.virtualQuoteRaw, state.quoteDecimals)
		return quote.Div(base), nil
	case curveTypeLinearPrice:
		if state.virtualBaseRaw == 0 || state.realBaseRaw == 0 {
			return decimal.Zero, nil
		}
		num := new(big.Int).SetUint64(state.virtualBaseRaw)
		num = num.Mul(num, new(big.Int).SetUint64(state.realBaseRaw))
		price := decimal.NewFromBigInt(num, 0).Div(decimal.NewFromBigInt(twoPow64AsBigInt, 0))
		return price.Shift(int32(state.baseDecimals) - int32(state.quoteDecimals)), nil
	default:
		return decimal.Zero, fmt.Errorf("unsupported raydium launchpad curve type: %d", curveType)
	}
}

func priceOfMintAInMintB(req Request, snapshot *reserveSnapshot, priceBaseInQuote decimal.Decimal) decimal.Decimal {
	if snapshot == nil || priceBaseInQuote.IsZero() {
		return decimal.Zero
	}
	switch {
	case mintsEquivalent(req.MintA, snapshot.baseMint) && mintsEquivalent(req.MintB, snapshot.quoteMint):
		return priceBaseInQuote
	case mintsEquivalent(req.MintA, snapshot.quoteMint) && mintsEquivalent(req.MintB, snapshot.baseMint):
		return decimal.NewFromInt(1).Div(priceBaseInQuote)
	default:
		return decimal.Zero
	}
}

func liquidityInMintB(req Request, snapshot *reserveSnapshot, priceBaseInQuote decimal.Decimal) decimal.Decimal {
	if snapshot == nil {
		return decimal.Zero
	}
	switch {
	case mintsEquivalent(req.MintB, snapshot.quoteMint):
		return snapshot.quoteReserve.Add(snapshot.baseReserve.Mul(priceBaseInQuote))
	case mintsEquivalent(req.MintB, snapshot.baseMint):
		if priceBaseInQuote.IsZero() {
			return snapshot.baseReserve
		}
		return snapshot.baseReserve.Add(snapshot.quoteReserve.Div(priceBaseInQuote))
	default:
		return decimal.Zero
	}
}

func (c *Calculator) priceOfMintAInSOL(ctx context.Context, req Request, priceAInB decimal.Decimal) (decimal.Decimal, error) {
	if pubkeyx.IsSOLMint(req.MintA) {
		return decimal.NewFromInt(1), nil
	}
	if pubkeyx.IsSOLMint(req.MintB) {
		return priceAInB, nil
	}
	if c.quotes == nil {
		return decimal.Zero, nil
	}
	oneBInSOL, err := c.quotes.ToSOL(ctx, req.MintB, decimal.NewFromInt(1))
	if err != nil {
		return decimal.Zero, err
	}
	if oneBInSOL.IsZero() {
		return decimal.Zero, nil
	}
	return priceAInB.Mul(oneBInSOL), nil
}

func (c *Calculator) liquidityInSOL(ctx context.Context, req Request, liquidityInB decimal.Decimal) (decimal.Decimal, error) {
	if pubkeyx.IsSOLMint(req.MintB) {
		return liquidityInB, nil
	}
	if c.quotes == nil {
		return decimal.Zero, nil
	}
	return c.quotes.ToSOL(ctx, req.MintB, liquidityInB)
}

func curveTypeName(curveType uint8) string {
	switch curveType {
	case curveTypeConstantProduct:
		return "constant_product"
	case curveTypeFixedPrice:
		return "fixed_price"
	case curveTypeLinearPrice:
		return "linear_price"
	default:
		return "unknown"
	}
}

func decimalFromU64(v uint64, decimals uint8) decimal.Decimal {
	return decimal.NewFromBigInt(new(big.Int).SetUint64(v), -int32(decimals))
}

func mintsEquivalent(a, b solana.PublicKey) bool {
	return a.Equals(b) || (pubkeyx.IsSOLMint(a) && pubkeyx.IsSOLMint(b))
}
