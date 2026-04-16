package liquidity_v4

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
	liquidityStateV4MinDataSize = 752

	baseDecimalOffset      = 32
	quoteDecimalOffset     = 40
	baseNeedTakePnlOffset  = 192
	quoteNeedTakePnlOffset = 200
	baseVaultOffset        = 336
	quoteVaultOffset       = 368
	baseMintOffset         = 400
	quoteMintOffset        = 432
	openOrdersOffset       = 496
	marketProgramIDOffset  = 560

	tokenAccountAmountOffset = 64
	tokenAccountMinDataSize  = 72

	openOrdersBaseTotalOffset  = 85
	openOrdersQuoteTotalOffset = 101
	openOrdersMinDataSize      = 109
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
	baseMint         solana.PublicKey
	quoteMint        solana.PublicKey
	baseVault        solana.PublicKey
	quoteVault       solana.PublicKey
	openOrders       solana.PublicKey
	marketProgramID  solana.PublicKey
	baseDecimals     uint8
	quoteDecimals    uint8
	baseNeedTakePnl  uint64
	quoteNeedTakePnl uint64
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

	state, err := decodePoolState(poolInfo.Data)
	if err != nil {
		return nil, err
	}
	if !poolMatchesRequest(req, state) {
		return nil, fmt.Errorf(
			"pool mint mismatch: request=(%s,%s) pool=(%s,%s)",
			req.MintA.String(),
			req.MintB.String(),
			state.baseMint.String(),
			state.quoteMint.String(),
		)
	}

	accounts, err := c.rpc.GetMultipleAccounts(ctx, []solana.PublicKey{
		state.baseVault,
		state.quoteVault,
		state.openOrders,
	})
	if err != nil {
		return nil, err
	}
	if len(accounts) != 3 {
		return nil, fmt.Errorf("unexpected account batch size: %d", len(accounts))
	}
	for idx, acc := range accounts {
		if acc == nil || !acc.Exists {
			return nil, fmt.Errorf("required account missing at index %d", idx)
		}
	}

	baseVaultAmountRaw, err := decodeTokenAmount(accounts[0].Data)
	if err != nil {
		return nil, fmt.Errorf("decode base vault amount: %w", err)
	}
	quoteVaultAmountRaw, err := decodeTokenAmount(accounts[1].Data)
	if err != nil {
		return nil, fmt.Errorf("decode quote vault amount: %w", err)
	}
	openOrdersBaseRaw, openOrdersQuoteRaw, err := decodeOpenOrdersTotals(accounts[2].Data)
	if err != nil {
		return nil, fmt.Errorf("decode open orders totals: %w", err)
	}

	baseReserve := decimalFromU64(baseVaultAmountRaw, state.baseDecimals).
		Add(decimalFromU64(openOrdersBaseRaw, state.baseDecimals)).
		Sub(decimalFromU64(state.baseNeedTakePnl, state.baseDecimals))
	if baseReserve.IsNegative() {
		baseReserve = decimal.Zero
	}
	quoteReserve := decimalFromU64(quoteVaultAmountRaw, state.quoteDecimals).
		Add(decimalFromU64(openOrdersQuoteRaw, state.quoteDecimals)).
		Sub(decimalFromU64(state.quoteNeedTakePnl, state.quoteDecimals))
	if quoteReserve.IsNegative() {
		quoteReserve = decimal.Zero
	}

	snapshot := &reserveSnapshot{
		baseMint:     state.baseMint,
		quoteMint:    state.quoteMint,
		baseReserve:  baseReserve,
		quoteReserve: quoteReserve,
	}
	if snapshot.baseReserve.IsZero() || snapshot.quoteReserve.IsZero() {
		return nil, fmt.Errorf("pool reserves are zero")
	}

	priceAInB := priceOfMintAInMintB(req, snapshot)
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
	marketCapInSOL := priceAInSOL.Mul(circulatingSupply)
	fdvInSOL := priceAInSOL.Mul(fdvSupply)

	return &Result{
		PriceOfAInB:       priceAInB,
		PriceOfAInSOL:     priceAInSOL,
		LiquidityInB:      liquidityInB,
		LiquidityInSOL:    liquidityInSOL,
		MarketCapInSOL:    marketCapInSOL,
		FDVInSOL:          fdvInSOL,
		TotalSupply:       totalSupply,
		CirculatingSupply: circulatingSupply,
		SupplyMethod:      supplyMethod,
		Metadata: map[string]any{
			"dex":                      "raydium",
			"pool_version":             "liquidity_v4",
			"source":                   "raydium_pool_v4_account",
			"pool_base_mint":           state.baseMint.String(),
			"pool_quote_mint":          state.quoteMint.String(),
			"pool_base_vault":          state.baseVault.String(),
			"pool_quote_vault":         state.quoteVault.String(),
			"pool_open_orders":         state.openOrders.String(),
			"pool_market_program_id":   state.marketProgramID.String(),
			"pool_base_decimal":        state.baseDecimals,
			"pool_quote_decimal":       state.quoteDecimals,
			"pool_base_need_take_pnl":  decimalFromU64(state.baseNeedTakePnl, state.baseDecimals).String(),
			"pool_quote_need_take_pnl": decimalFromU64(state.quoteNeedTakePnl, state.quoteDecimals).String(),
			"pool_base_reserve":        snapshot.baseReserve.String(),
			"pool_quote_reserve":       snapshot.quoteReserve.String(),
			"fdv_supply":               fdvSupply.String(),
			"fdv_method":               fdvMethod,
		},
	}, nil
}

func decodePoolState(data []byte) (poolState, error) {
	if len(data) < liquidityStateV4MinDataSize {
		return poolState{}, fmt.Errorf("invalid raydium liquidity v4 pool data length: %d", len(data))
	}
	baseDecimal := binary.LittleEndian.Uint64(data[baseDecimalOffset : baseDecimalOffset+8])
	quoteDecimal := binary.LittleEndian.Uint64(data[quoteDecimalOffset : quoteDecimalOffset+8])
	if baseDecimal > 255 || quoteDecimal > 255 {
		return poolState{}, fmt.Errorf("invalid pool decimals: base=%d quote=%d", baseDecimal, quoteDecimal)
	}
	return poolState{
		baseMint:         solana.PublicKeyFromBytes(data[baseMintOffset : baseMintOffset+32]),
		quoteMint:        solana.PublicKeyFromBytes(data[quoteMintOffset : quoteMintOffset+32]),
		baseVault:        solana.PublicKeyFromBytes(data[baseVaultOffset : baseVaultOffset+32]),
		quoteVault:       solana.PublicKeyFromBytes(data[quoteVaultOffset : quoteVaultOffset+32]),
		openOrders:       solana.PublicKeyFromBytes(data[openOrdersOffset : openOrdersOffset+32]),
		marketProgramID:  solana.PublicKeyFromBytes(data[marketProgramIDOffset : marketProgramIDOffset+32]),
		baseDecimals:     uint8(baseDecimal),
		quoteDecimals:    uint8(quoteDecimal),
		baseNeedTakePnl:  binary.LittleEndian.Uint64(data[baseNeedTakePnlOffset : baseNeedTakePnlOffset+8]),
		quoteNeedTakePnl: binary.LittleEndian.Uint64(data[quoteNeedTakePnlOffset : quoteNeedTakePnlOffset+8]),
	}, nil
}

func decodeTokenAmount(data []byte) (uint64, error) {
	if len(data) < tokenAccountMinDataSize {
		return 0, fmt.Errorf("invalid token account data length: %d", len(data))
	}
	return binary.LittleEndian.Uint64(data[tokenAccountAmountOffset : tokenAccountAmountOffset+8]), nil
}

func decodeOpenOrdersTotals(data []byte) (uint64, uint64, error) {
	if len(data) < openOrdersMinDataSize {
		return 0, 0, fmt.Errorf("invalid open orders data length: %d", len(data))
	}
	baseTotal := binary.LittleEndian.Uint64(data[openOrdersBaseTotalOffset : openOrdersBaseTotalOffset+8])
	quoteTotal := binary.LittleEndian.Uint64(data[openOrdersQuoteTotalOffset : openOrdersQuoteTotalOffset+8])
	return baseTotal, quoteTotal, nil
}

func poolMatchesRequest(req Request, state poolState) bool {
	return (mintsEquivalent(req.MintA, state.baseMint) && mintsEquivalent(req.MintB, state.quoteMint)) ||
		(mintsEquivalent(req.MintA, state.quoteMint) && mintsEquivalent(req.MintB, state.baseMint))
}

func priceOfMintAInMintB(req Request, snapshot *reserveSnapshot) decimal.Decimal {
	if snapshot == nil || snapshot.baseReserve.IsZero() || snapshot.quoteReserve.IsZero() {
		return decimal.Zero
	}
	switch {
	case mintsEquivalent(req.MintA, snapshot.baseMint) && mintsEquivalent(req.MintB, snapshot.quoteMint):
		return snapshot.quoteReserve.Div(snapshot.baseReserve)
	case mintsEquivalent(req.MintA, snapshot.quoteMint) && mintsEquivalent(req.MintB, snapshot.baseMint):
		return snapshot.baseReserve.Div(snapshot.quoteReserve)
	default:
		return decimal.Zero
	}
}

func liquidityInMintB(req Request, snapshot *reserveSnapshot) decimal.Decimal {
	if snapshot == nil {
		return decimal.Zero
	}
	switch {
	case mintsEquivalent(req.MintB, snapshot.quoteMint):
		return snapshot.quoteReserve.Mul(decimal.NewFromInt(2))
	case mintsEquivalent(req.MintB, snapshot.baseMint):
		return snapshot.baseReserve.Mul(decimal.NewFromInt(2))
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
