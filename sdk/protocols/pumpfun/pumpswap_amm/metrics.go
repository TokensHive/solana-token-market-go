package pumpswap_amm

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/TokensHive/solana-token-market-go/sdk/internal/pubkeyx"
	"github.com/TokensHive/solana-token-market-go/sdk/quote"
	"github.com/TokensHive/solana-token-market-go/sdk/rpc"
	"github.com/TokensHive/solana-token-market-go/sdk/supply"
	"github.com/gagliardetto/solana-go"
	"github.com/shopspring/decimal"
)

const (
	poolBaseMintOffset      = 43
	poolQuoteMintOffset     = 75
	poolBaseVaultOffset     = 139
	poolQuoteVaultOffset    = 171
	poolExpectedMinDataSize = 243

	tokenAmountOffset       = 64
	tokenAccountMinDataSize = 72

	mintDecimalsOffset     = 44
	mintAccountMinDataSize = 45
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
	TotalSupply       decimal.Decimal
	CirculatingSupply decimal.Decimal
	SupplyMethod      string
	Metadata          map[string]any
}

type poolState struct {
	baseMint   solana.PublicKey
	quoteMint  solana.PublicKey
	baseVault  solana.PublicKey
	quoteVault solana.PublicKey
}

type reserveSnapshot struct {
	BaseMint     string
	BaseReserve  decimal.Decimal
	QuoteMint    string
	QuoteReserve decimal.Decimal
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
		state.baseMint,
		state.quoteMint,
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

	baseReserveRaw, err := decodeTokenAmount(accounts[0].Data)
	if err != nil {
		return nil, fmt.Errorf("decode base vault amount: %w", err)
	}
	quoteReserveRaw, err := decodeTokenAmount(accounts[1].Data)
	if err != nil {
		return nil, fmt.Errorf("decode quote vault amount: %w", err)
	}
	baseDecimals, err := decodeMintDecimals(accounts[2].Data)
	if err != nil {
		return nil, fmt.Errorf("decode base mint decimals: %w", err)
	}
	quoteDecimals, err := decodeMintDecimals(accounts[3].Data)
	if err != nil {
		return nil, fmt.Errorf("decode quote mint decimals: %w", err)
	}

	snapshot := &reserveSnapshot{
		BaseMint:     state.baseMint.String(),
		BaseReserve:  decimalFromU64(baseReserveRaw, baseDecimals),
		QuoteMint:    state.quoteMint.String(),
		QuoteReserve: decimalFromU64(quoteReserveRaw, quoteDecimals),
	}
	if snapshot.BaseReserve.IsZero() || snapshot.QuoteReserve.IsZero() {
		return nil, fmt.Errorf("pool reserves are zero")
	}

	priceAInB := priceOfMintAInMintB(req, snapshot)
	priceAInSOL, err := c.priceOfMintAInSOL(ctx, req, priceAInB)
	if err != nil {
		return nil, err
	}
	liquidityInB := liquidityInMintB(req, snapshot)
	liquidityInSOL, err := c.liquidityInSOL(ctx, req, liquidityInB)
	if err != nil {
		return nil, err
	}

	totalSupply, circulatingSupply, supplyMethod, err := c.supply.GetSupply(ctx, req.MintA)
	if err != nil {
		return nil, err
	}
	marketCapInSOL := priceAInSOL.Mul(circulatingSupply)

	return &Result{
		PriceOfAInB:       priceAInB,
		PriceOfAInSOL:     priceAInSOL,
		LiquidityInB:      liquidityInB,
		LiquidityInSOL:    liquidityInSOL,
		MarketCapInSOL:    marketCapInSOL,
		TotalSupply:       totalSupply,
		CirculatingSupply: circulatingSupply,
		SupplyMethod:      supplyMethod,
		Metadata: map[string]any{
			"dex":                "pumpfun",
			"pool_version":       "pumpswap_amm",
			"source":             "pumpswap_pool_account",
			"pool_base_mint":     snapshot.BaseMint,
			"pool_quote_mint":    snapshot.QuoteMint,
			"pool_base_reserve":  snapshot.BaseReserve.String(),
			"pool_quote_reserve": snapshot.QuoteReserve.String(),
			"pool_base_vault":    state.baseVault.String(),
			"pool_quote_vault":   state.quoteVault.String(),
		},
	}, nil
}

func decodePoolState(data []byte) (poolState, error) {
	if len(data) < poolExpectedMinDataSize {
		return poolState{}, fmt.Errorf("invalid pumpswap pool data length: %d", len(data))
	}
	return poolState{
		baseMint:   solana.PublicKeyFromBytes(data[poolBaseMintOffset : poolBaseMintOffset+32]),
		quoteMint:  solana.PublicKeyFromBytes(data[poolQuoteMintOffset : poolQuoteMintOffset+32]),
		baseVault:  solana.PublicKeyFromBytes(data[poolBaseVaultOffset : poolBaseVaultOffset+32]),
		quoteVault: solana.PublicKeyFromBytes(data[poolQuoteVaultOffset : poolQuoteVaultOffset+32]),
	}, nil
}

func poolMatchesRequest(req Request, state poolState) bool {
	return (mintsEquivalent(req.MintA, state.baseMint) && mintsEquivalent(req.MintB, state.quoteMint)) ||
		(mintsEquivalent(req.MintA, state.quoteMint) && mintsEquivalent(req.MintB, state.baseMint))
}

func decodeTokenAmount(data []byte) (uint64, error) {
	if len(data) < tokenAccountMinDataSize {
		return 0, fmt.Errorf("invalid token account data length: %d", len(data))
	}
	return binary.LittleEndian.Uint64(data[tokenAmountOffset : tokenAmountOffset+8]), nil
}

func decodeMintDecimals(data []byte) (uint8, error) {
	if len(data) < mintAccountMinDataSize {
		return 0, fmt.Errorf("invalid mint account data length: %d", len(data))
	}
	return data[mintDecimalsOffset], nil
}

func priceOfMintAInMintB(req Request, snapshot *reserveSnapshot) decimal.Decimal {
	if snapshot == nil || snapshot.BaseReserve.IsZero() || snapshot.QuoteReserve.IsZero() {
		return decimal.Zero
	}
	switch {
	case mintStringMatchesReq(snapshot.BaseMint, req.MintA) && mintStringMatchesReq(snapshot.QuoteMint, req.MintB):
		return snapshot.QuoteReserve.Div(snapshot.BaseReserve)
	case mintStringMatchesReq(snapshot.QuoteMint, req.MintA) && mintStringMatchesReq(snapshot.BaseMint, req.MintB):
		return snapshot.BaseReserve.Div(snapshot.QuoteReserve)
	default:
		return decimal.Zero
	}
}

func liquidityInMintB(req Request, snapshot *reserveSnapshot) decimal.Decimal {
	if snapshot == nil {
		return decimal.Zero
	}
	switch {
	case mintStringMatchesReq(snapshot.QuoteMint, req.MintB):
		return snapshot.QuoteReserve.Mul(decimal.NewFromInt(2))
	case mintStringMatchesReq(snapshot.BaseMint, req.MintB):
		return snapshot.BaseReserve.Mul(decimal.NewFromInt(2))
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
	return decimal.NewFromInt(int64(v)).Shift(-int32(decimals))
}

func mintStringMatchesReq(snapshotMint string, reqMint solana.PublicKey) bool {
	pk, err := solana.PublicKeyFromBase58(snapshotMint)
	if err != nil {
		return false
	}
	return mintsEquivalent(reqMint, pk)
}

func mintsEquivalent(a, b solana.PublicKey) bool {
	return a.Equals(b) || (pubkeyx.IsSOLMint(a) && pubkeyx.IsSOLMint(b))
}
