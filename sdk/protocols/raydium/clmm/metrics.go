package clmm

import (
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
	poolExpectedMinDataSize = 1544

	ammConfigOffset      = 9
	ownerOffset          = 41
	token0MintOffset     = 73
	token1MintOffset     = 105
	token0VaultOffset    = 137
	token1VaultOffset    = 169
	observationKeyOffset = 201

	mint0DecimalsOffset = 233
	mint1DecimalsOffset = 234
	tickSpacingOffset   = 235
	liquidityOffset     = 237
	sqrtPriceX64Offset  = 253
	tickCurrentOffset   = 269

	observationIndexOffset          = 273
	observationUpdateDurationOffset = 275
	protocolFees0Offset             = 309
	protocolFees1Offset             = 317
	statusOffset                    = 389
	totalFees0Offset                = 1032
	totalFeesClaimed0Offset         = 1040
	totalFees1Offset                = 1048
	totalFeesClaimed1Offset         = 1056
	fundFees0Offset                 = 1064
	fundFees1Offset                 = 1072

	tokenAmountOffset     = 64
	tokenAccountMinLength = 72
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
	ammConfig                 solana.PublicKey
	owner                     solana.PublicKey
	token0Mint                solana.PublicKey
	token1Mint                solana.PublicKey
	token0Vault               solana.PublicKey
	token1Vault               solana.PublicKey
	observationKey            solana.PublicKey
	mint0Decimals             uint8
	mint1Decimals             uint8
	tickSpacing               uint16
	liquidityRaw              *big.Int
	sqrtPriceX64Raw           *big.Int
	tickCurrent               int32
	observationIndex          uint16
	observationUpdateDuration uint16
	protocolFees0             uint64
	protocolFees1             uint64
	fundFees0                 uint64
	fundFees1                 uint64
	status                    uint8
	totalFees0                uint64
	totalFeesClaimed0         uint64
	totalFees1                uint64
	totalFeesClaimed1         uint64
}

type reserveSnapshot struct {
	token0Mint    solana.PublicKey
	token1Mint    solana.PublicKey
	token0Reserve decimal.Decimal
	token1Reserve decimal.Decimal
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

	state, err := decodePoolState(poolInfo.Data)
	if err != nil {
		return nil, err
	}
	resolvedReq := Request{
		PoolAddress: req.PoolAddress,
		MintA:       state.token0Mint,
		MintB:       state.token1Mint,
	}

	accounts, err := c.rpc.GetMultipleAccounts(ctx, []solana.PublicKey{
		state.token0Vault,
		state.token1Vault,
	})
	if err != nil {
		return nil, err
	}
	if len(accounts) != 2 {
		return nil, fmt.Errorf("unexpected account batch size: %d", len(accounts))
	}
	for idx, acc := range accounts {
		if acc == nil || !acc.Exists {
			return nil, fmt.Errorf("required account missing at index %d", idx)
		}
	}

	token0VaultRaw, err := decodeTokenAmount(accounts[0].Data)
	if err != nil {
		return nil, fmt.Errorf("decode token0 vault amount: %w", err)
	}
	token1VaultRaw, err := decodeTokenAmount(accounts[1].Data)
	if err != nil {
		return nil, fmt.Errorf("decode token1 vault amount: %w", err)
	}

	token0NetRaw := subtractFees(token0VaultRaw, state.protocolFees0, state.fundFees0)
	token1NetRaw := subtractFees(token1VaultRaw, state.protocolFees1, state.fundFees1)
	snapshot := &reserveSnapshot{
		token0Mint:    state.token0Mint,
		token1Mint:    state.token1Mint,
		token0Reserve: decimalFromU64(token0NetRaw, state.mint0Decimals),
		token1Reserve: decimalFromU64(token1NetRaw, state.mint1Decimals),
	}
	if snapshot.token0Reserve.IsZero() || snapshot.token1Reserve.IsZero() {
		return nil, fmt.Errorf("pool reserves are zero")
	}

	priceToken1InToken0 := priceToken1InToken0FromSqrt(state.sqrtPriceX64Raw, state.mint0Decimals, state.mint1Decimals)
	priceAInB := priceOfMintAInMintB(resolvedReq, state, priceToken1InToken0)
	liquidityInB := liquidityInMintB(resolvedReq, snapshot)

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
			"dex":                              "raydium",
			"pool_version":                     "clmm",
			"source":                           "raydium_clmm_pool_account",
			"pool_amm_config":                  state.ammConfig.String(),
			"pool_owner":                       state.owner.String(),
			"pool_observation_key":             state.observationKey.String(),
			"pool_token0_mint":                 state.token0Mint.String(),
			"pool_token1_mint":                 state.token1Mint.String(),
			"pool_token0_vault":                state.token0Vault.String(),
			"pool_token1_vault":                state.token1Vault.String(),
			"pool_token0_decimals":             state.mint0Decimals,
			"pool_token1_decimals":             state.mint1Decimals,
			"pool_tick_spacing":                state.tickSpacing,
			"pool_tick_current":                state.tickCurrent,
			"pool_observation_index":           state.observationIndex,
			"pool_observation_update_duration": state.observationUpdateDuration,
			"pool_liquidity":                   state.liquidityRaw.String(),
			"pool_sqrt_price_x64":              state.sqrtPriceX64Raw.String(),
			"pool_status":                      state.status,
			"pool_protocol_fees_0":             decimalFromU64(state.protocolFees0, state.mint0Decimals).String(),
			"pool_protocol_fees_1":             decimalFromU64(state.protocolFees1, state.mint1Decimals).String(),
			"pool_fund_fees_0":                 decimalFromU64(state.fundFees0, state.mint0Decimals).String(),
			"pool_fund_fees_1":                 decimalFromU64(state.fundFees1, state.mint1Decimals).String(),
			"pool_total_fees_0":                decimalFromU64(state.totalFees0, state.mint0Decimals).String(),
			"pool_total_fees_claimed_0":        decimalFromU64(state.totalFeesClaimed0, state.mint0Decimals).String(),
			"pool_total_fees_1":                decimalFromU64(state.totalFees1, state.mint1Decimals).String(),
			"pool_total_fees_claimed_1":        decimalFromU64(state.totalFeesClaimed1, state.mint1Decimals).String(),
			"pool_token0_reserve":              snapshot.token0Reserve.String(),
			"pool_token1_reserve":              snapshot.token1Reserve.String(),
			"pool_price_token1_in_token0":      priceToken1InToken0.String(),
			"fdv_supply":                       fdvSupply.String(),
			"fdv_method":                       fdvMethod,
		},
	}, nil
}

func decodePoolState(data []byte) (poolState, error) {
	if len(data) < poolExpectedMinDataSize {
		return poolState{}, fmt.Errorf("invalid raydium clmm pool data length: %d", len(data))
	}

	return poolState{
		ammConfig:                 solana.PublicKeyFromBytes(data[ammConfigOffset : ammConfigOffset+32]),
		owner:                     solana.PublicKeyFromBytes(data[ownerOffset : ownerOffset+32]),
		token0Mint:                solana.PublicKeyFromBytes(data[token0MintOffset : token0MintOffset+32]),
		token1Mint:                solana.PublicKeyFromBytes(data[token1MintOffset : token1MintOffset+32]),
		token0Vault:               solana.PublicKeyFromBytes(data[token0VaultOffset : token0VaultOffset+32]),
		token1Vault:               solana.PublicKeyFromBytes(data[token1VaultOffset : token1VaultOffset+32]),
		observationKey:            solana.PublicKeyFromBytes(data[observationKeyOffset : observationKeyOffset+32]),
		mint0Decimals:             data[mint0DecimalsOffset],
		mint1Decimals:             data[mint1DecimalsOffset],
		tickSpacing:               binary.LittleEndian.Uint16(data[tickSpacingOffset : tickSpacingOffset+2]),
		liquidityRaw:              readU128(data[liquidityOffset : liquidityOffset+16]),
		sqrtPriceX64Raw:           readU128(data[sqrtPriceX64Offset : sqrtPriceX64Offset+16]),
		tickCurrent:               int32(binary.LittleEndian.Uint32(data[tickCurrentOffset : tickCurrentOffset+4])),
		observationIndex:          binary.LittleEndian.Uint16(data[observationIndexOffset : observationIndexOffset+2]),
		observationUpdateDuration: binary.LittleEndian.Uint16(data[observationUpdateDurationOffset : observationUpdateDurationOffset+2]),
		protocolFees0:             binary.LittleEndian.Uint64(data[protocolFees0Offset : protocolFees0Offset+8]),
		protocolFees1:             binary.LittleEndian.Uint64(data[protocolFees1Offset : protocolFees1Offset+8]),
		fundFees0:                 binary.LittleEndian.Uint64(data[fundFees0Offset : fundFees0Offset+8]),
		fundFees1:                 binary.LittleEndian.Uint64(data[fundFees1Offset : fundFees1Offset+8]),
		status:                    data[statusOffset],
		totalFees0:                binary.LittleEndian.Uint64(data[totalFees0Offset : totalFees0Offset+8]),
		totalFeesClaimed0:         binary.LittleEndian.Uint64(data[totalFeesClaimed0Offset : totalFeesClaimed0Offset+8]),
		totalFees1:                binary.LittleEndian.Uint64(data[totalFees1Offset : totalFees1Offset+8]),
		totalFeesClaimed1:         binary.LittleEndian.Uint64(data[totalFeesClaimed1Offset : totalFeesClaimed1Offset+8]),
	}, nil
}

func poolMatchesRequest(req Request, state poolState) bool {
	return (mintsEquivalent(req.MintA, state.token0Mint) && mintsEquivalent(req.MintB, state.token1Mint)) ||
		(mintsEquivalent(req.MintA, state.token1Mint) && mintsEquivalent(req.MintB, state.token0Mint))
}

func decodeTokenAmount(data []byte) (uint64, error) {
	if len(data) < tokenAccountMinLength {
		return 0, fmt.Errorf("invalid token account data length: %d", len(data))
	}
	return binary.LittleEndian.Uint64(data[tokenAmountOffset : tokenAmountOffset+8]), nil
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

func subtractFees(vaultAmount uint64, protocolFee uint64, fundFee uint64) uint64 {
	if protocolFee > vaultAmount {
		return 0
	}
	remaining := vaultAmount - protocolFee
	if fundFee > remaining {
		return 0
	}
	return remaining - fundFee
}

func priceToken1InToken0FromSqrt(sqrtPriceX64Raw *big.Int, token0Decimals uint8, token1Decimals uint8) decimal.Decimal {
	if sqrtPriceX64Raw == nil || sqrtPriceX64Raw.Sign() <= 0 {
		return decimal.Zero
	}
	sqrtPrice := decimal.NewFromBigInt(sqrtPriceX64Raw, 0)
	sqrtPriceSquared := sqrtPrice.Mul(sqrtPrice)
	twoPow128 := decimal.NewFromBigInt(new(big.Int).Lsh(big.NewInt(1), 128), 0)
	price := twoPow128.Div(sqrtPriceSquared)
	return price.Shift(int32(token1Decimals) - int32(token0Decimals))
}

func priceOfMintAInMintB(req Request, state poolState, priceToken1InToken0 decimal.Decimal) decimal.Decimal {
	if priceToken1InToken0.IsZero() {
		return decimal.Zero
	}
	switch {
	case mintsEquivalent(req.MintA, state.token1Mint) && mintsEquivalent(req.MintB, state.token0Mint):
		return priceToken1InToken0
	case mintsEquivalent(req.MintA, state.token0Mint) && mintsEquivalent(req.MintB, state.token1Mint):
		return decimal.NewFromInt(1).Div(priceToken1InToken0)
	default:
		return decimal.Zero
	}
}

func liquidityInMintB(req Request, snapshot *reserveSnapshot) decimal.Decimal {
	if snapshot == nil {
		return decimal.Zero
	}
	switch {
	case mintsEquivalent(req.MintB, snapshot.token1Mint):
		return snapshot.token1Reserve.Mul(decimal.NewFromInt(2))
	case mintsEquivalent(req.MintB, snapshot.token0Mint):
		return snapshot.token0Reserve.Mul(decimal.NewFromInt(2))
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
