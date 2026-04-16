package bonding_curve

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/TokensHive/solana-token-market-go/sdk/internal/pubkeyx"
	"github.com/TokensHive/solana-token-market-go/sdk/quote"
	"github.com/TokensHive/solana-token-market-go/sdk/rpc"
	"github.com/gagliardetto/solana-go"
	"github.com/shopspring/decimal"
)

type Calculator struct {
	rpc    rpc.Client
	quotes quote.Bridge
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

func NewCalculator(rpcClient rpc.Client, quoteBridge quote.Bridge) *Calculator {
	return &Calculator{rpc: rpcClient, quotes: quoteBridge}
}

type curveState struct {
	virtualTokenReserve decimal.Decimal
	virtualSOLReserve   decimal.Decimal
	realTokenReserve    decimal.Decimal
	realSOLReserve      decimal.Decimal
	tokenTotalSupply    decimal.Decimal
	complete            bool
}

func (c *Calculator) Compute(ctx context.Context, req Request) (*Result, error) {
	if c.rpc == nil {
		return nil, fmt.Errorf("rpc client is required")
	}
	if req.PoolAddress.IsZero() {
		return nil, fmt.Errorf("pool address is required")
	}

	info, err := c.rpc.GetAccount(ctx, req.PoolAddress)
	if err != nil {
		return nil, err
	}
	if info == nil || !info.Exists {
		return nil, fmt.Errorf("pool account not found")
	}

	state := decodeCurveState(info.Data)
	priceTokenInSOL, liquiditySOL := computeCurvePriceAndLiquidity(state)
	totalSupply, circulatingSupply := computeCurveSupplies(state)

	priceOfAInSOL, priceOfAInB, liquidityInB := c.computePairMetrics(ctx, req.MintA, req.MintB, priceTokenInSOL, liquiditySOL)

	marketCapInSOL := priceOfAInSOL.Mul(totalSupply)
	fdvInSOL := priceOfAInSOL.Mul(totalSupply)
	return &Result{
		PriceOfAInB:       priceOfAInB,
		PriceOfAInSOL:     priceOfAInSOL,
		LiquidityInB:      liquidityInB,
		LiquidityInSOL:    liquiditySOL,
		MarketCapInSOL:    marketCapInSOL,
		FDVInSOL:          fdvInSOL,
		TotalSupply:       totalSupply,
		CirculatingSupply: circulatingSupply,
		SupplyMethod:      "pumpfun_curve_state",
		Metadata: map[string]any{
			"dex":                   "pumpfun",
			"pool_version":          "bonding_curve",
			"curve_complete":        state.complete,
			"virtual_token_reserve": state.virtualTokenReserve.String(),
			"virtual_sol_reserve":   state.virtualSOLReserve.String(),
			"real_token_reserve":    state.realTokenReserve.String(),
			"real_sol_reserve":      state.realSOLReserve.String(),
			"fdv_supply":            totalSupply.String(),
			"fdv_method":            "pumpfun_curve_token_total_supply",
		},
	}, nil
}

func decodeCurveState(data []byte) curveState {
	if len(data) < 49 {
		return curveState{}
	}
	return curveState{
		virtualTokenReserve: decimal.NewFromUint64(readU64(data, 8)).Shift(-6),
		virtualSOLReserve:   decimal.NewFromUint64(readU64(data, 16)).Shift(-9),
		realTokenReserve:    decimal.NewFromUint64(readU64(data, 24)).Shift(-6),
		realSOLReserve:      decimal.NewFromUint64(readU64(data, 32)).Shift(-9),
		tokenTotalSupply:    decimal.NewFromUint64(readU64(data, 40)).Shift(-6),
		complete:            data[48] == 1,
	}
}

func computeCurvePriceAndLiquidity(state curveState) (decimal.Decimal, decimal.Decimal) {
	if state.virtualTokenReserve.IsZero() {
		return decimal.Zero, state.realSOLReserve
	}
	return state.virtualSOLReserve.Div(state.virtualTokenReserve), state.realSOLReserve
}

func computeCurveSupplies(state curveState) (decimal.Decimal, decimal.Decimal) {
	total := state.tokenTotalSupply
	circulating := total.Sub(state.realTokenReserve)
	if circulating.IsNegative() {
		circulating = decimal.Zero
	}
	return total, circulating
}

func (c *Calculator) computePairMetrics(
	ctx context.Context,
	mintA solana.PublicKey,
	mintB solana.PublicKey,
	priceTokenInSOL decimal.Decimal,
	liquiditySOL decimal.Decimal,
) (priceOfAInSOL decimal.Decimal, priceOfAInB decimal.Decimal, liquidityInB decimal.Decimal) {
	if pubkeyx.IsSOLMint(mintA) {
		priceOfAInSOL = decimal.NewFromInt(1)
		if priceTokenInSOL.GreaterThan(decimal.Zero) {
			priceOfAInB = decimal.NewFromInt(1).Div(priceTokenInSOL)
		}
		return priceOfAInSOL, priceOfAInB, liquiditySOL
	}

	priceOfAInSOL = priceTokenInSOL
	if pubkeyx.IsSOLMint(mintB) {
		return priceOfAInSOL, priceTokenInSOL, liquiditySOL
	}
	if c.quotes == nil {
		return priceOfAInSOL, decimal.Zero, decimal.Zero
	}
	oneBInSOL, err := c.quotes.ToSOL(ctx, mintB, decimal.NewFromInt(1))
	if err != nil || oneBInSOL.IsZero() {
		return priceOfAInSOL, decimal.Zero, decimal.Zero
	}
	return priceOfAInSOL, priceTokenInSOL.Div(oneBInSOL), liquiditySOL.Div(oneBInSOL)
}

func readU64(data []byte, offset int) uint64 {
	if len(data) < offset+8 {
		return 0
	}
	return binary.LittleEndian.Uint64(data[offset : offset+8])
}
