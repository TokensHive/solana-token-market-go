package dbc

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
	poolMinDataSize = 424

	configOffset       = 72
	baseMintOffset     = 136
	baseVaultOffset    = 168
	quoteVaultOffset   = 200
	baseReserveOffset  = 232
	quoteReserveOffset = 240

	sqrtPriceOffset = 280

	isMigratedOffset        = 305
	migrationProgressOffset = 308

	protocolBaseFeeOffset  = 248
	protocolQuoteFeeOffset = 256
	partnerBaseFeeOffset   = 264
	partnerQuoteFeeOffset  = 272
	creatorBaseFeeOffset   = 352
	creatorQuoteFeeOffset  = 360

	tokenMintOffset           = 0
	tokenAccountMinDataLength = 32
	mintDecimalsOffset        = 44
	mintAccountMinDataLength  = 45
)

var dbcProgramID = solana.MustPublicKeyFromBase58("dbcij3LWUppWqq96dh6gJWwBifmcGfLSB5D4DuSMaqN")
var virtualPoolDiscriminator = []byte{213, 224, 5, 209, 98, 69, 119, 92}

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
	config            solana.PublicKey
	baseMint          solana.PublicKey
	baseVault         solana.PublicKey
	quoteVault        solana.PublicKey
	baseReserveRaw    uint64
	quoteReserveRaw   uint64
	sqrtPriceRaw      *big.Int
	isMigrated        bool
	migrationProgress uint8

	protocolBaseFeeRaw  uint64
	protocolQuoteFeeRaw uint64
	partnerBaseFeeRaw   uint64
	partnerQuoteFeeRaw  uint64
	creatorBaseFeeRaw   uint64
	creatorQuoteFeeRaw  uint64
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
	if !poolInfo.Owner.Equals(dbcProgramID) {
		return nil, fmt.Errorf("invalid meteora dbc owner: %s", poolInfo.Owner.String())
	}

	state, err := decodePoolState(poolInfo.Data)
	if err != nil {
		return nil, err
	}

	accounts, err := c.rpc.GetMultipleAccounts(ctx, []solana.PublicKey{
		state.quoteVault,
		state.baseMint,
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

	quoteMint, err := decodeTokenAccountMint(accounts[0].Data)
	if err != nil {
		return nil, fmt.Errorf("decode quote vault mint: %w", err)
	}
	baseDecimals, err := decodeMintDecimals(accounts[1].Data)
	if err != nil {
		return nil, fmt.Errorf("decode base mint decimals: %w", err)
	}

	if !poolMatchesRequest(req, state.baseMint, quoteMint) {
		return nil, fmt.Errorf(
			"pool mint mismatch: request=(%s,%s) pool=(%s,%s)",
			req.MintA.String(),
			req.MintB.String(),
			state.baseMint.String(),
			quoteMint.String(),
		)
	}

	quoteMintInfo, err := c.rpc.GetAccount(ctx, quoteMint)
	if err != nil {
		return nil, err
	}
	if quoteMintInfo == nil || !quoteMintInfo.Exists {
		return nil, fmt.Errorf("quote mint account not found")
	}
	quoteDecimals, err := decodeMintDecimals(quoteMintInfo.Data)
	if err != nil {
		return nil, fmt.Errorf("decode quote mint decimals: %w", err)
	}

	snapshot := &reserveSnapshot{
		baseMint:     state.baseMint,
		quoteMint:    quoteMint,
		baseReserve:  decimalFromU64(state.baseReserveRaw, baseDecimals),
		quoteReserve: decimalFromU64(state.quoteReserveRaw, quoteDecimals),
	}
	if snapshot.baseReserve.IsZero() || snapshot.quoteReserve.IsZero() {
		return nil, fmt.Errorf("pool reserves are zero")
	}

	priceBaseInQuote := priceBaseInQuoteFromSqrt(state.sqrtPriceRaw, baseDecimals, quoteDecimals)
	priceAInB := priceOfMintAInMintB(req, snapshot, priceBaseInQuote)
	liquidityInB := liquidityInMintB(req, snapshot, priceBaseInQuote)

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
			"dex":                      "meteora",
			"pool_version":             "dbc",
			"source":                   "meteora_dbc_virtual_pool_account",
			"pool_program_id":          dbcProgramID.String(),
			"pool_config":              state.config.String(),
			"pool_base_mint":           snapshot.baseMint.String(),
			"pool_quote_mint":          snapshot.quoteMint.String(),
			"pool_base_vault":          state.baseVault.String(),
			"pool_quote_vault":         state.quoteVault.String(),
			"pool_base_decimals":       baseDecimals,
			"pool_quote_decimals":      quoteDecimals,
			"pool_base_reserve":        snapshot.baseReserve.String(),
			"pool_quote_reserve":       snapshot.quoteReserve.String(),
			"pool_sqrt_price":          state.sqrtPriceRaw.String(),
			"pool_price_base_in_quote": priceBaseInQuote.String(),
			"pool_is_migrated":         state.isMigrated,
			"pool_migration_progress":  state.migrationProgress,
			"pool_protocol_base_fee":   decimalFromU64(state.protocolBaseFeeRaw, baseDecimals).String(),
			"pool_protocol_quote_fee":  decimalFromU64(state.protocolQuoteFeeRaw, quoteDecimals).String(),
			"pool_partner_base_fee":    decimalFromU64(state.partnerBaseFeeRaw, baseDecimals).String(),
			"pool_partner_quote_fee":   decimalFromU64(state.partnerQuoteFeeRaw, quoteDecimals).String(),
			"pool_creator_base_fee":    decimalFromU64(state.creatorBaseFeeRaw, baseDecimals).String(),
			"pool_creator_quote_fee":   decimalFromU64(state.creatorQuoteFeeRaw, quoteDecimals).String(),
			"fdv_supply":               fdvSupply.String(),
			"fdv_method":               fdvMethod,
		},
	}, nil
}

func decodePoolState(data []byte) (poolState, error) {
	if len(data) < poolMinDataSize {
		return poolState{}, fmt.Errorf("invalid meteora dbc pool data length: %d", len(data))
	}
	if !bytes.Equal(data[:8], virtualPoolDiscriminator) {
		return poolState{}, fmt.Errorf("invalid meteora dbc virtual pool discriminator")
	}

	return poolState{
		config:              solana.PublicKeyFromBytes(data[configOffset : configOffset+32]),
		baseMint:            solana.PublicKeyFromBytes(data[baseMintOffset : baseMintOffset+32]),
		baseVault:           solana.PublicKeyFromBytes(data[baseVaultOffset : baseVaultOffset+32]),
		quoteVault:          solana.PublicKeyFromBytes(data[quoteVaultOffset : quoteVaultOffset+32]),
		baseReserveRaw:      binary.LittleEndian.Uint64(data[baseReserveOffset : baseReserveOffset+8]),
		quoteReserveRaw:     binary.LittleEndian.Uint64(data[quoteReserveOffset : quoteReserveOffset+8]),
		sqrtPriceRaw:        readU128(data[sqrtPriceOffset : sqrtPriceOffset+16]),
		isMigrated:          data[isMigratedOffset] == 1,
		migrationProgress:   data[migrationProgressOffset],
		protocolBaseFeeRaw:  binary.LittleEndian.Uint64(data[protocolBaseFeeOffset : protocolBaseFeeOffset+8]),
		protocolQuoteFeeRaw: binary.LittleEndian.Uint64(data[protocolQuoteFeeOffset : protocolQuoteFeeOffset+8]),
		partnerBaseFeeRaw:   binary.LittleEndian.Uint64(data[partnerBaseFeeOffset : partnerBaseFeeOffset+8]),
		partnerQuoteFeeRaw:  binary.LittleEndian.Uint64(data[partnerQuoteFeeOffset : partnerQuoteFeeOffset+8]),
		creatorBaseFeeRaw:   binary.LittleEndian.Uint64(data[creatorBaseFeeOffset : creatorBaseFeeOffset+8]),
		creatorQuoteFeeRaw:  binary.LittleEndian.Uint64(data[creatorQuoteFeeOffset : creatorQuoteFeeOffset+8]),
	}, nil
}

func decodeTokenAccountMint(data []byte) (solana.PublicKey, error) {
	if len(data) < tokenAccountMinDataLength {
		return solana.PublicKey{}, fmt.Errorf("invalid token account data length: %d", len(data))
	}
	return solana.PublicKeyFromBytes(data[tokenMintOffset : tokenMintOffset+32]), nil
}

func decodeMintDecimals(data []byte) (uint8, error) {
	if len(data) < mintAccountMinDataLength {
		return 0, fmt.Errorf("invalid mint account data length: %d", len(data))
	}
	return data[mintDecimalsOffset], nil
}

func poolMatchesRequest(req Request, baseMint, quoteMint solana.PublicKey) bool {
	return (mintsEquivalent(req.MintA, baseMint) && mintsEquivalent(req.MintB, quoteMint)) ||
		(mintsEquivalent(req.MintA, quoteMint) && mintsEquivalent(req.MintB, baseMint))
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

func priceBaseInQuoteFromSqrt(sqrtPriceRaw *big.Int, baseDecimals uint8, quoteDecimals uint8) decimal.Decimal {
	if sqrtPriceRaw == nil || sqrtPriceRaw.Sign() <= 0 {
		return decimal.Zero
	}
	sqrtPrice := decimal.NewFromBigInt(sqrtPriceRaw, 0)
	twoPow128 := decimal.NewFromBigInt(new(big.Int).Lsh(big.NewInt(1), 128), 0)
	price := sqrtPrice.Mul(sqrtPrice).Div(twoPow128)
	return price.Shift(int32(baseDecimals) - int32(quoteDecimals))
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

func decimalFromU64(v uint64, decimals uint8) decimal.Decimal {
	n := new(big.Int).SetUint64(v)
	return decimal.NewFromBigInt(n, -int32(decimals))
}

func mintsEquivalent(a, b solana.PublicKey) bool {
	return a.Equals(b) || (pubkeyx.IsSOLMint(a) && pubkeyx.IsSOLMint(b))
}
