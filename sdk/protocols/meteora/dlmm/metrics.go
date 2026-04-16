package dlmm

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
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
	poolMinDataSize = 216

	activeIDOffset = 76
	binStepOffset  = 80
	statusOffset   = 82

	tokenXMintOffset = 88
	tokenYMintOffset = 120
	reserveXOffset   = 152
	reserveYOffset   = 184

	tokenAmountOffset     = 64
	tokenAccountMinLength = 72

	mintDecimalsOffset = 44
	mintAccountMinSize = 45
)

var dlmmProgramID = solana.MustPublicKeyFromBase58("LBUZKhRxPF3XUpBCjp4YzTKgLccjZhTSDM9YuVaPwxo")

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
	activeID   int32
	binStepBPS uint16
	status     uint8
	tokenXMint solana.PublicKey
	tokenYMint solana.PublicKey
	reserveX   solana.PublicKey
	reserveY   solana.PublicKey
}

type reserveSnapshot struct {
	tokenXMint    solana.PublicKey
	tokenYMint    solana.PublicKey
	tokenXReserve decimal.Decimal
	tokenYReserve decimal.Decimal
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
	if req.MintA.IsZero() || req.MintB.IsZero() {
		return nil, fmt.Errorf("mintA and mintB are required")
	}

	poolInfo, err := c.rpc.GetAccount(ctx, req.PoolAddress)
	if err != nil {
		return nil, err
	}
	if poolInfo == nil || !poolInfo.Exists {
		return nil, fmt.Errorf("pool account not found")
	}
	if !poolInfo.Owner.Equals(dlmmProgramID) {
		return nil, fmt.Errorf("invalid meteora dlmm owner: %s", poolInfo.Owner.String())
	}

	state, err := decodePoolState(poolInfo.Data)
	if err != nil {
		return nil, err
	}
	if !poolMatchesRequest(req, state) {
		return nil, fmt.Errorf(
			"pool mint mismatch: request=(%s,%s) pool=(%s,%s)",
			req.MintA.String(),
			req.MintB.String(),
			state.tokenXMint.String(),
			state.tokenYMint.String(),
		)
	}

	accounts, err := c.rpc.GetMultipleAccounts(ctx, []solana.PublicKey{
		state.reserveX,
		state.reserveY,
		state.tokenXMint,
		state.tokenYMint,
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

	reserveXRaw, err := decodeTokenAmount(accounts[0].Data)
	if err != nil {
		return nil, fmt.Errorf("decode reserveX amount: %w", err)
	}
	reserveYRaw, err := decodeTokenAmount(accounts[1].Data)
	if err != nil {
		return nil, fmt.Errorf("decode reserveY amount: %w", err)
	}
	tokenXDecimals, err := decodeMintDecimals(accounts[2].Data)
	if err != nil {
		return nil, fmt.Errorf("decode tokenX decimals: %w", err)
	}
	tokenYDecimals, err := decodeMintDecimals(accounts[3].Data)
	if err != nil {
		return nil, fmt.Errorf("decode tokenY decimals: %w", err)
	}

	snapshot := &reserveSnapshot{
		tokenXMint:    state.tokenXMint,
		tokenYMint:    state.tokenYMint,
		tokenXReserve: decimalFromU64(reserveXRaw, tokenXDecimals),
		tokenYReserve: decimalFromU64(reserveYRaw, tokenYDecimals),
	}
	if snapshot.tokenXReserve.IsZero() || snapshot.tokenYReserve.IsZero() {
		return nil, fmt.Errorf("pool reserves are zero")
	}

	priceTokenXInTokenY := priceTokenXInTokenY(state.activeID, state.binStepBPS, tokenXDecimals, tokenYDecimals)
	priceAInB := priceOfMintAInMintB(req, state, priceTokenXInTokenY)
	liquidityInB := liquidityInMintB(req, snapshot)

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
			priceAInSOL, convErr = c.priceOfMintAInSOL(taskCtx, req, priceAInB)
			if convErr != nil {
				return convErr
			}
			liquidityInSOL, convErr = c.liquidityInSOL(taskCtx, req, liquidityInB)
			return convErr
		},
		func(taskCtx context.Context) error {
			var supplyErr error
			totalSupply, circulatingSupply, supplyMethod, supplyErr = c.supply.GetSupply(taskCtx, req.MintA)
			if supplyErr != nil {
				return supplyErr
			}
			fdvSupply, fdvMethod = supply.ResolveFDVSupply(taskCtx, c.rpc, req.MintA, totalSupply)
			return nil
		},
	)
	if err != nil {
		return nil, err
	}

	return &Result{
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
			"dex":                    "meteora",
			"pool_version":           "dlmm",
			"source":                 "meteora_dlmm_lb_pair_account",
			"pool_program_id":        dlmmProgramID.String(),
			"pool_status":            state.status,
			"pool_active_id":         state.activeID,
			"pool_bin_step_bps":      state.binStepBPS,
			"pool_token0_mint":       state.tokenXMint.String(),
			"pool_token1_mint":       state.tokenYMint.String(),
			"pool_token0_vault":      state.reserveX.String(),
			"pool_token1_vault":      state.reserveY.String(),
			"pool_token0_decimals":   tokenXDecimals,
			"pool_token1_decimals":   tokenYDecimals,
			"pool_token0_reserve":    snapshot.tokenXReserve.String(),
			"pool_token1_reserve":    snapshot.tokenYReserve.String(),
			"pool_price_token0_in_1": priceTokenXInTokenY.String(),
			"fdv_supply":             fdvSupply.String(),
			"fdv_method":             fdvMethod,
		},
	}, nil
}

func decodePoolState(data []byte) (poolState, error) {
	if len(data) < poolMinDataSize {
		return poolState{}, fmt.Errorf("invalid meteora dlmm pool data length: %d", len(data))
	}
	return poolState{
		activeID:   int32(binary.LittleEndian.Uint32(data[activeIDOffset : activeIDOffset+4])),
		binStepBPS: binary.LittleEndian.Uint16(data[binStepOffset : binStepOffset+2]),
		status:     data[statusOffset],
		tokenXMint: solana.PublicKeyFromBytes(data[tokenXMintOffset : tokenXMintOffset+32]),
		tokenYMint: solana.PublicKeyFromBytes(data[tokenYMintOffset : tokenYMintOffset+32]),
		reserveX:   solana.PublicKeyFromBytes(data[reserveXOffset : reserveXOffset+32]),
		reserveY:   solana.PublicKeyFromBytes(data[reserveYOffset : reserveYOffset+32]),
	}, nil
}

func poolMatchesRequest(req Request, state poolState) bool {
	return (mintsEquivalent(req.MintA, state.tokenXMint) && mintsEquivalent(req.MintB, state.tokenYMint)) ||
		(mintsEquivalent(req.MintA, state.tokenYMint) && mintsEquivalent(req.MintB, state.tokenXMint))
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

func priceTokenXInTokenY(activeID int32, binStepBPS uint16, tokenXDecimals uint8, tokenYDecimals uint8) decimal.Decimal {
	base := 1 + (float64(binStepBPS) / 10000)
	value := math.Pow(base, float64(activeID))
	if math.IsInf(value, 0) || math.IsNaN(value) {
		return decimal.Zero
	}
	return decimal.NewFromFloat(value).Shift(int32(tokenXDecimals) - int32(tokenYDecimals))
}

func priceOfMintAInMintB(req Request, state poolState, priceTokenXInTokenY decimal.Decimal) decimal.Decimal {
	if priceTokenXInTokenY.IsZero() {
		return decimal.Zero
	}
	switch {
	case mintsEquivalent(req.MintA, state.tokenXMint) && mintsEquivalent(req.MintB, state.tokenYMint):
		return priceTokenXInTokenY
	case mintsEquivalent(req.MintA, state.tokenYMint) && mintsEquivalent(req.MintB, state.tokenXMint):
		return decimal.NewFromInt(1).Div(priceTokenXInTokenY)
	default:
		return decimal.Zero
	}
}

func liquidityInMintB(req Request, snapshot *reserveSnapshot) decimal.Decimal {
	if snapshot == nil {
		return decimal.Zero
	}
	switch {
	case mintsEquivalent(req.MintB, snapshot.tokenYMint):
		return snapshot.tokenYReserve.Mul(decimal.NewFromInt(2))
	case mintsEquivalent(req.MintB, snapshot.tokenXMint):
		return snapshot.tokenXReserve.Mul(decimal.NewFromInt(2))
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
