package cpmm

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
	poolExpectedMinDataSize = 637

	token0VaultOffset = 72
	token1VaultOffset = 104
	token0MintOffset  = 168
	token1MintOffset  = 200

	statusOffset          = 329
	lpMintDecimalsOffset  = 330
	mint0DecimalsOffset   = 331
	mint1DecimalsOffset   = 332
	lpSupplyOffset        = 333
	protocolFees0Offset   = 341
	protocolFees1Offset   = 349
	fundFees0Offset       = 357
	fundFees1Offset       = 365
	openTimeOffset        = 373
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
	token0Mint      solana.PublicKey
	token1Mint      solana.PublicKey
	token0Vault     solana.PublicKey
	token1Vault     solana.PublicKey
	status          uint8
	lpMintDecimals  uint8
	mint0Decimals   uint8
	mint1Decimals   uint8
	lpSupplyRaw     uint64
	protocolFees0   uint64
	protocolFees1   uint64
	fundFees0       uint64
	fundFees1       uint64
	openTimeUnixSec uint64
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

	priceAInB := priceOfMintAInMintB(resolvedReq, snapshot)
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
			"dex":                   "raydium",
			"pool_version":          "cpmm",
			"source":                "raydium_cpmm_pool_account",
			"pool_status":           state.status,
			"pool_open_time":        state.openTimeUnixSec,
			"pool_lp_mint_decimals": state.lpMintDecimals,
			"pool_lp_supply":        decimalFromU64(state.lpSupplyRaw, state.lpMintDecimals).String(),
			"pool_token0_mint":      state.token0Mint.String(),
			"pool_token1_mint":      state.token1Mint.String(),
			"pool_token0_vault":     state.token0Vault.String(),
			"pool_token1_vault":     state.token1Vault.String(),
			"pool_token0_decimals":  state.mint0Decimals,
			"pool_token1_decimals":  state.mint1Decimals,
			"pool_protocol_fees_0":  decimalFromU64(state.protocolFees0, state.mint0Decimals).String(),
			"pool_protocol_fees_1":  decimalFromU64(state.protocolFees1, state.mint1Decimals).String(),
			"pool_fund_fees_0":      decimalFromU64(state.fundFees0, state.mint0Decimals).String(),
			"pool_fund_fees_1":      decimalFromU64(state.fundFees1, state.mint1Decimals).String(),
			"pool_token0_reserve":   snapshot.token0Reserve.String(),
			"pool_token1_reserve":   snapshot.token1Reserve.String(),
			"fdv_supply":            fdvSupply.String(),
			"fdv_method":            fdvMethod,
		},
	}, nil
}

func decodePoolState(data []byte) (poolState, error) {
	if len(data) < poolExpectedMinDataSize {
		return poolState{}, fmt.Errorf("invalid raydium cpmm pool data length: %d", len(data))
	}
	return poolState{
		token0Mint:      solana.PublicKeyFromBytes(data[token0MintOffset : token0MintOffset+32]),
		token1Mint:      solana.PublicKeyFromBytes(data[token1MintOffset : token1MintOffset+32]),
		token0Vault:     solana.PublicKeyFromBytes(data[token0VaultOffset : token0VaultOffset+32]),
		token1Vault:     solana.PublicKeyFromBytes(data[token1VaultOffset : token1VaultOffset+32]),
		status:          data[statusOffset],
		lpMintDecimals:  data[lpMintDecimalsOffset],
		mint0Decimals:   data[mint0DecimalsOffset],
		mint1Decimals:   data[mint1DecimalsOffset],
		lpSupplyRaw:     binary.LittleEndian.Uint64(data[lpSupplyOffset : lpSupplyOffset+8]),
		protocolFees0:   binary.LittleEndian.Uint64(data[protocolFees0Offset : protocolFees0Offset+8]),
		protocolFees1:   binary.LittleEndian.Uint64(data[protocolFees1Offset : protocolFees1Offset+8]),
		fundFees0:       binary.LittleEndian.Uint64(data[fundFees0Offset : fundFees0Offset+8]),
		fundFees1:       binary.LittleEndian.Uint64(data[fundFees1Offset : fundFees1Offset+8]),
		openTimeUnixSec: binary.LittleEndian.Uint64(data[openTimeOffset : openTimeOffset+8]),
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

func priceOfMintAInMintB(req Request, snapshot *reserveSnapshot) decimal.Decimal {
	if snapshot == nil || snapshot.token0Reserve.IsZero() || snapshot.token1Reserve.IsZero() {
		return decimal.Zero
	}
	switch {
	case mintsEquivalent(req.MintA, snapshot.token0Mint) && mintsEquivalent(req.MintB, snapshot.token1Mint):
		return snapshot.token1Reserve.Div(snapshot.token0Reserve)
	case mintsEquivalent(req.MintA, snapshot.token1Mint) && mintsEquivalent(req.MintB, snapshot.token0Mint):
		return snapshot.token0Reserve.Div(snapshot.token1Reserve)
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
