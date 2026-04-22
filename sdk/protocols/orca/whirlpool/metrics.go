package whirlpool

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
	poolMinDataSize = 653

	whirlpoolsConfigOffset = 8
	whirlpoolBumpOffset    = 40
	tickSpacingOffset      = 41
	feeTierIndexSeedOffset = 43
	feeRateOffset          = 45
	protocolFeeRateOffset  = 47
	liquidityOffset        = 49
	sqrtPriceOffset        = 65
	tickCurrentIndexOffset = 81
	protocolFeeOwedAOffset = 85
	protocolFeeOwedBOffset = 93
	tokenMintAOffset       = 101
	tokenVaultAOffset      = 133
	feeGrowthGlobalAOffset = 165
	tokenMintBOffset       = 181
	tokenVaultBOffset      = 213
	feeGrowthGlobalBOffset = 245
	rewardLastUpdatedAt    = 261

	tokenAmountOffset     = 64
	tokenAccountMinLength = 72

	mintDecimalsOffset = 44
	mintAccountMinSize = 45
)

var whirlpoolProgramID = solana.MustPublicKeyFromBase58("whirLbMiicVdio4qvUfM5KAg6Ct8VwpYzGff3uctyCc")
var whirlpoolDiscriminator = []byte{63, 149, 209, 12, 225, 128, 99, 9}

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
	whirlpoolsConfig    solana.PublicKey
	whirlpoolBump       uint8
	tickSpacing         uint16
	feeTierIndexSeed    uint16
	feeRate             uint16
	protocolFeeRate     uint16
	liquidityRaw        *big.Int
	sqrtPriceRaw        *big.Int
	tickCurrentIndex    int32
	protocolFeeOwedA    uint64
	protocolFeeOwedB    uint64
	tokenMintA          solana.PublicKey
	tokenVaultA         solana.PublicKey
	feeGrowthGlobalA    *big.Int
	tokenMintB          solana.PublicKey
	tokenVaultB         solana.PublicKey
	feeGrowthGlobalB    *big.Int
	rewardLastUpdatedAt uint64
}

type reserveSnapshot struct {
	tokenMintA solana.PublicKey
	tokenMintB solana.PublicKey
	reserveA   decimal.Decimal
	reserveB   decimal.Decimal
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
	if !poolInfo.Owner.Equals(whirlpoolProgramID) {
		return nil, fmt.Errorf("invalid orca whirlpool owner: %s", poolInfo.Owner.String())
	}

	state, err := decodePoolState(poolInfo.Data)
	if err != nil {
		return nil, err
	}
	resolvedReq := Request{
		PoolAddress: req.PoolAddress,
		MintA:       state.tokenMintA,
		MintB:       state.tokenMintB,
	}

	accounts, err := c.rpc.GetMultipleAccounts(ctx, []solana.PublicKey{
		state.tokenVaultA,
		state.tokenVaultB,
		state.tokenMintA,
		state.tokenMintB,
	})
	if err != nil {
		return nil, err
	}
	if len(accounts) != 4 {
		return nil, fmt.Errorf("unexpected account batch size: %d", len(accounts))
	}
	for idx, acc := range accounts {
		if acc == nil || !acc.Exists {
			return nil, fmt.Errorf("required account missing at index %d", idx)
		}
	}

	tokenVaultARaw, err := decodeTokenAmount(accounts[0].Data)
	if err != nil {
		return nil, fmt.Errorf("decode tokenA vault amount: %w", err)
	}
	tokenVaultBRaw, err := decodeTokenAmount(accounts[1].Data)
	if err != nil {
		return nil, fmt.Errorf("decode tokenB vault amount: %w", err)
	}
	tokenADecimals, err := decodeMintDecimals(accounts[2].Data)
	if err != nil {
		return nil, fmt.Errorf("decode tokenA decimals: %w", err)
	}
	tokenBDecimals, err := decodeMintDecimals(accounts[3].Data)
	if err != nil {
		return nil, fmt.Errorf("decode tokenB decimals: %w", err)
	}

	tokenANetRaw := subtractProtocolFee(tokenVaultARaw, state.protocolFeeOwedA)
	tokenBNetRaw := subtractProtocolFee(tokenVaultBRaw, state.protocolFeeOwedB)
	snapshot := &reserveSnapshot{
		tokenMintA: state.tokenMintA,
		tokenMintB: state.tokenMintB,
		reserveA:   decimalFromU64(tokenANetRaw, tokenADecimals),
		reserveB:   decimalFromU64(tokenBNetRaw, tokenBDecimals),
	}
	if snapshot.reserveA.IsZero() || snapshot.reserveB.IsZero() {
		return nil, fmt.Errorf("pool reserves are zero")
	}

	priceTokenAInTokenB := priceTokenAInTokenBFromSqrt(state.sqrtPriceRaw, tokenADecimals, tokenBDecimals)
	priceAInB := priceOfMintAInMintB(resolvedReq, state, priceTokenAInTokenB)
	liquidityInB := liquidityInMintB(resolvedReq, state, snapshot, priceTokenAInTokenB)

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
			"dex":                         "orca",
			"pool_version":                "whirlpool",
			"source":                      "orca_whirlpool_account",
			"pool_program_id":             whirlpoolProgramID.String(),
			"pool_whirlpools_config":      state.whirlpoolsConfig.String(),
			"pool_whirlpool_bump":         state.whirlpoolBump,
			"pool_tick_spacing":           state.tickSpacing,
			"pool_fee_tier_index_seed":    state.feeTierIndexSeed,
			"pool_fee_rate":               state.feeRate,
			"pool_protocol_fee_rate":      state.protocolFeeRate,
			"pool_tick_current_index":     state.tickCurrentIndex,
			"pool_reward_last_updated_at": state.rewardLastUpdatedAt,
			"pool_liquidity":              state.liquidityRaw.String(),
			"pool_sqrt_price":             state.sqrtPriceRaw.String(),
			"pool_fee_growth_global_a":    state.feeGrowthGlobalA.String(),
			"pool_fee_growth_global_b":    state.feeGrowthGlobalB.String(),
			"pool_token0_mint":            state.tokenMintA.String(),
			"pool_token1_mint":            state.tokenMintB.String(),
			"pool_token0_vault":           state.tokenVaultA.String(),
			"pool_token1_vault":           state.tokenVaultB.String(),
			"pool_token0_decimals":        tokenADecimals,
			"pool_token1_decimals":        tokenBDecimals,
			"pool_token0_reserve":         snapshot.reserveA.String(),
			"pool_token1_reserve":         snapshot.reserveB.String(),
			"pool_protocol_fee_owed_a":    decimalFromU64(state.protocolFeeOwedA, tokenADecimals).String(),
			"pool_protocol_fee_owed_b":    decimalFromU64(state.protocolFeeOwedB, tokenBDecimals).String(),
			"pool_price_token0_in_1":      priceTokenAInTokenB.String(),
			"fdv_supply":                  fdvSupply.String(),
			"fdv_method":                  fdvMethod,
		},
	}, nil
}

func decodePoolState(data []byte) (poolState, error) {
	if len(data) < poolMinDataSize {
		return poolState{}, fmt.Errorf("invalid orca whirlpool data length: %d", len(data))
	}
	if !bytes.Equal(data[:8], whirlpoolDiscriminator) {
		return poolState{}, fmt.Errorf("invalid orca whirlpool discriminator")
	}
	return poolState{
		whirlpoolsConfig:    solana.PublicKeyFromBytes(data[whirlpoolsConfigOffset : whirlpoolsConfigOffset+32]),
		whirlpoolBump:       data[whirlpoolBumpOffset],
		tickSpacing:         binary.LittleEndian.Uint16(data[tickSpacingOffset : tickSpacingOffset+2]),
		feeTierIndexSeed:    binary.LittleEndian.Uint16(data[feeTierIndexSeedOffset : feeTierIndexSeedOffset+2]),
		feeRate:             binary.LittleEndian.Uint16(data[feeRateOffset : feeRateOffset+2]),
		protocolFeeRate:     binary.LittleEndian.Uint16(data[protocolFeeRateOffset : protocolFeeRateOffset+2]),
		liquidityRaw:        readU128(data[liquidityOffset : liquidityOffset+16]),
		sqrtPriceRaw:        readU128(data[sqrtPriceOffset : sqrtPriceOffset+16]),
		tickCurrentIndex:    int32(binary.LittleEndian.Uint32(data[tickCurrentIndexOffset : tickCurrentIndexOffset+4])),
		protocolFeeOwedA:    binary.LittleEndian.Uint64(data[protocolFeeOwedAOffset : protocolFeeOwedAOffset+8]),
		protocolFeeOwedB:    binary.LittleEndian.Uint64(data[protocolFeeOwedBOffset : protocolFeeOwedBOffset+8]),
		tokenMintA:          solana.PublicKeyFromBytes(data[tokenMintAOffset : tokenMintAOffset+32]),
		tokenVaultA:         solana.PublicKeyFromBytes(data[tokenVaultAOffset : tokenVaultAOffset+32]),
		feeGrowthGlobalA:    readU128(data[feeGrowthGlobalAOffset : feeGrowthGlobalAOffset+16]),
		tokenMintB:          solana.PublicKeyFromBytes(data[tokenMintBOffset : tokenMintBOffset+32]),
		tokenVaultB:         solana.PublicKeyFromBytes(data[tokenVaultBOffset : tokenVaultBOffset+32]),
		feeGrowthGlobalB:    readU128(data[feeGrowthGlobalBOffset : feeGrowthGlobalBOffset+16]),
		rewardLastUpdatedAt: binary.LittleEndian.Uint64(data[rewardLastUpdatedAt : rewardLastUpdatedAt+8]),
	}, nil
}

func poolMatchesRequest(req Request, state poolState) bool {
	return (mintsEquivalent(req.MintA, state.tokenMintA) && mintsEquivalent(req.MintB, state.tokenMintB)) ||
		(mintsEquivalent(req.MintA, state.tokenMintB) && mintsEquivalent(req.MintB, state.tokenMintA))
}

func decodeTokenAmount(data []byte) (uint64, error) {
	if len(data) < tokenAccountMinLength {
		return 0, fmt.Errorf("invalid token account data length: %d", len(data))
	}
	return binary.LittleEndian.Uint64(data[tokenAmountOffset : tokenAmountOffset+8]), nil
}

func decodeMintDecimals(data []byte) (uint8, error) {
	if len(data) < mintAccountMinSize {
		return 0, fmt.Errorf("invalid mint account data length: %d", len(data))
	}
	return data[mintDecimalsOffset], nil
}

func readU128(data []byte) *big.Int {
	if len(data) < 16 {
		return big.NewInt(0)
	}
	low := binary.LittleEndian.Uint64(data[:8])
	high := binary.LittleEndian.Uint64(data[8:16])
	n := new(big.Int).SetUint64(high)
	n.Lsh(n, 64)
	return n.Or(n, new(big.Int).SetUint64(low))
}

func subtractProtocolFee(vaultAmount uint64, protocolFee uint64) uint64 {
	if protocolFee > vaultAmount {
		return 0
	}
	return vaultAmount - protocolFee
}

func priceTokenAInTokenBFromSqrt(sqrtPriceRaw *big.Int, tokenADecimals uint8, tokenBDecimals uint8) decimal.Decimal {
	if sqrtPriceRaw == nil || sqrtPriceRaw.Sign() <= 0 {
		return decimal.Zero
	}
	sqrtPrice := decimal.NewFromBigInt(sqrtPriceRaw, 0)
	sqrtPriceSquared := sqrtPrice.Mul(sqrtPrice)
	twoPow128 := decimal.NewFromBigInt(new(big.Int).Lsh(big.NewInt(1), 128), 0)
	price := sqrtPriceSquared.Div(twoPow128)
	return price.Shift(int32(tokenADecimals) - int32(tokenBDecimals))
}

func priceOfMintAInMintB(req Request, state poolState, priceTokenAInTokenB decimal.Decimal) decimal.Decimal {
	if priceTokenAInTokenB.IsZero() {
		return decimal.Zero
	}
	switch {
	case mintsEquivalent(req.MintA, state.tokenMintA) && mintsEquivalent(req.MintB, state.tokenMintB):
		return priceTokenAInTokenB
	case mintsEquivalent(req.MintA, state.tokenMintB) && mintsEquivalent(req.MintB, state.tokenMintA):
		return decimal.NewFromInt(1).Div(priceTokenAInTokenB)
	default:
		return decimal.Zero
	}
}

func liquidityInMintB(req Request, state poolState, snapshot *reserveSnapshot, priceTokenAInTokenB decimal.Decimal) decimal.Decimal {
	if snapshot == nil {
		return decimal.Zero
	}
	switch {
	case mintsEquivalent(req.MintB, state.tokenMintB):
		return snapshot.reserveB.Add(snapshot.reserveA.Mul(priceTokenAInTokenB))
	case mintsEquivalent(req.MintB, state.tokenMintA):
		if priceTokenAInTokenB.IsZero() {
			return snapshot.reserveA
		}
		return snapshot.reserveA.Add(snapshot.reserveB.Div(priceTokenAInTokenB))
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

func decimalFromU64(v uint64, decimals uint8) decimal.Decimal {
	n := new(big.Int).SetUint64(v)
	return decimal.NewFromBigInt(n, -int32(decimals))
}

func mintsEquivalent(a, b solana.PublicKey) bool {
	return a.Equals(b) || (pubkeyx.IsSOLMint(a) && pubkeyx.IsSOLMint(b))
}
