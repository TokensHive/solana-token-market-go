package damm_v2

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
	poolMinDataSize = 1112

	tokenAMintOffset  = 168
	tokenBMintOffset  = 200
	tokenAVaultOffset = 232
	tokenBVaultOffset = 264

	protocolAFeeOffset = 392
	protocolBFeeOffset = 400
	sqrtPriceX64Offset = 456

	poolStatusOffset     = 481
	collectFeeModeOffset = 484
	poolTypeOffset       = 485
	feeVersionOffset     = 486

	tokenAAmountOffset  = 680
	tokenBAmountOffset  = 688
	layoutVersionOffset = 696

	tokenAmountOffset     = 64
	tokenAccountMinLength = 72

	mintDecimalsOffset = 44
	mintAccountMinSize = 45
)

var dammV2ProgramID = solana.MustPublicKeyFromBase58("cpamdpZCGKUy5JxQXB4dcpGPiikHawvSWAd6mEn1sGG")

var poolDiscriminator = []byte{241, 154, 109, 4, 17, 177, 109, 188}

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
	tokenAMint   solana.PublicKey
	tokenBMint   solana.PublicKey
	tokenAVault  solana.PublicKey
	tokenBVault  solana.PublicKey
	protocolAFee uint64
	protocolBFee uint64
	sqrtPriceX64 *big.Int
	poolStatus   uint8
	feeMode      uint8
	poolType     uint8
	feeVersion   uint8
	layout       uint8
	tokenAAmount uint64
	tokenBAmount uint64
}

type reserveSnapshot struct {
	tokenAMint    solana.PublicKey
	tokenBMint    solana.PublicKey
	tokenAReserve decimal.Decimal
	tokenBReserve decimal.Decimal
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
	if !poolInfo.Owner.Equals(dammV2ProgramID) {
		return nil, fmt.Errorf("invalid meteora damm v2 owner: %s", poolInfo.Owner.String())
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
			state.tokenAMint.String(),
			state.tokenBMint.String(),
		)
	}

	accounts, err := c.rpc.GetMultipleAccounts(ctx, []solana.PublicKey{
		state.tokenAVault,
		state.tokenBVault,
		state.tokenAMint,
		state.tokenBMint,
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

	tokenAVaultRaw, err := decodeTokenAmount(accounts[0].Data)
	if err != nil {
		return nil, fmt.Errorf("decode tokenA vault amount: %w", err)
	}
	tokenBVaultRaw, err := decodeTokenAmount(accounts[1].Data)
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

	tokenANetRaw := subtractProtocolFee(tokenAVaultRaw, state.protocolAFee)
	tokenBNetRaw := subtractProtocolFee(tokenBVaultRaw, state.protocolBFee)
	snapshot := &reserveSnapshot{
		tokenAMint:    state.tokenAMint,
		tokenBMint:    state.tokenBMint,
		tokenAReserve: decimalFromU64(tokenANetRaw, tokenADecimals),
		tokenBReserve: decimalFromU64(tokenBNetRaw, tokenBDecimals),
	}
	if snapshot.tokenAReserve.IsZero() || snapshot.tokenBReserve.IsZero() {
		return nil, fmt.Errorf("pool reserves are zero")
	}

	priceTokenAInTokenB := priceTokenAInTokenBFromSqrt(state.sqrtPriceX64, tokenADecimals, tokenBDecimals)
	priceAInB := priceOfMintAInMintB(req, state, priceTokenAInTokenB)
	liquidityInB := liquidityInMintB(req, state, snapshot, priceTokenAInTokenB)

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
			"pool_version":           "damm_v2",
			"source":                 "meteora_damm_v2_pool_account",
			"pool_program_id":        dammV2ProgramID.String(),
			"pool_status":            state.poolStatus,
			"pool_collect_fee_mode":  state.feeMode,
			"pool_type":              state.poolType,
			"pool_fee_version":       state.feeVersion,
			"pool_layout_version":    state.layout,
			"pool_token0_mint":       state.tokenAMint.String(),
			"pool_token1_mint":       state.tokenBMint.String(),
			"pool_token0_vault":      state.tokenAVault.String(),
			"pool_token1_vault":      state.tokenBVault.String(),
			"pool_token0_decimals":   tokenADecimals,
			"pool_token1_decimals":   tokenBDecimals,
			"pool_token0_reserve":    snapshot.tokenAReserve.String(),
			"pool_token1_reserve":    snapshot.tokenBReserve.String(),
			"pool_protocol_fees_0":   decimalFromU64(state.protocolAFee, tokenADecimals).String(),
			"pool_protocol_fees_1":   decimalFromU64(state.protocolBFee, tokenBDecimals).String(),
			"pool_token0_amount":     decimalFromU64(state.tokenAAmount, tokenADecimals).String(),
			"pool_token1_amount":     decimalFromU64(state.tokenBAmount, tokenBDecimals).String(),
			"pool_sqrt_price_x64":    state.sqrtPriceX64.String(),
			"pool_price_token0_in_1": priceTokenAInTokenB.String(),
			"fdv_supply":             fdvSupply.String(),
			"fdv_method":             fdvMethod,
		},
	}, nil
}

func decodePoolState(data []byte) (poolState, error) {
	if len(data) < poolMinDataSize {
		return poolState{}, fmt.Errorf("invalid meteora damm v2 pool data length: %d", len(data))
	}
	if !bytes.Equal(data[:8], poolDiscriminator) {
		return poolState{}, fmt.Errorf("invalid meteora damm v2 pool discriminator")
	}
	return poolState{
		tokenAMint:   solana.PublicKeyFromBytes(data[tokenAMintOffset : tokenAMintOffset+32]),
		tokenBMint:   solana.PublicKeyFromBytes(data[tokenBMintOffset : tokenBMintOffset+32]),
		tokenAVault:  solana.PublicKeyFromBytes(data[tokenAVaultOffset : tokenAVaultOffset+32]),
		tokenBVault:  solana.PublicKeyFromBytes(data[tokenBVaultOffset : tokenBVaultOffset+32]),
		protocolAFee: binary.LittleEndian.Uint64(data[protocolAFeeOffset : protocolAFeeOffset+8]),
		protocolBFee: binary.LittleEndian.Uint64(data[protocolBFeeOffset : protocolBFeeOffset+8]),
		sqrtPriceX64: readU128(data[sqrtPriceX64Offset : sqrtPriceX64Offset+16]),
		poolStatus:   data[poolStatusOffset],
		feeMode:      data[collectFeeModeOffset],
		poolType:     data[poolTypeOffset],
		feeVersion:   data[feeVersionOffset],
		layout:       data[layoutVersionOffset],
		tokenAAmount: binary.LittleEndian.Uint64(data[tokenAAmountOffset : tokenAAmountOffset+8]),
		tokenBAmount: binary.LittleEndian.Uint64(data[tokenBAmountOffset : tokenBAmountOffset+8]),
	}, nil
}

func poolMatchesRequest(req Request, state poolState) bool {
	return (mintsEquivalent(req.MintA, state.tokenAMint) && mintsEquivalent(req.MintB, state.tokenBMint)) ||
		(mintsEquivalent(req.MintA, state.tokenBMint) && mintsEquivalent(req.MintB, state.tokenAMint))
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

func priceTokenAInTokenBFromSqrt(sqrtPriceX64Raw *big.Int, tokenADecimals uint8, tokenBDecimals uint8) decimal.Decimal {
	if sqrtPriceX64Raw == nil || sqrtPriceX64Raw.Sign() <= 0 {
		return decimal.Zero
	}
	sqrtPrice := decimal.NewFromBigInt(sqrtPriceX64Raw, 0)
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
	case mintsEquivalent(req.MintA, state.tokenAMint) && mintsEquivalent(req.MintB, state.tokenBMint):
		return priceTokenAInTokenB
	case mintsEquivalent(req.MintA, state.tokenBMint) && mintsEquivalent(req.MintB, state.tokenAMint):
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
	case mintsEquivalent(req.MintB, state.tokenBMint):
		return snapshot.tokenBReserve.Add(snapshot.tokenAReserve.Mul(priceTokenAInTokenB))
	case mintsEquivalent(req.MintB, state.tokenAMint):
		if priceTokenAInTokenB.IsZero() {
			return snapshot.tokenAReserve
		}
		return snapshot.tokenAReserve.Add(snapshot.tokenBReserve.Div(priceTokenAInTokenB))
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
