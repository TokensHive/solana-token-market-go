package damm_v1

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
	poolMinDataSize = 892

	lpMintOffset = 8

	tokenAMintOffset = 40
	tokenBMintOffset = 72

	aVaultOffset = 104
	bVaultOffset = 136

	aVaultLPOffset = 168
	bVaultLPOffset = 200

	aVaultLPBumpOffset = 232
	enabledOffset      = 233

	protocolTokenAFeeOffset = 234
	protocolTokenBFeeOffset = 266

	feeLastUpdatedAtOffset = 298
	poolTypeOffset         = 378

	stakeOffset         = 380
	totalLockedLPOffset = 412
	curveTypeTagOffset  = 891

	vaultMinDataSize = 1227

	vaultTotalAmountOffset = 11
	vaultTokenVaultOffset  = 19
	vaultLPMintOffset      = 115

	vaultLastLockedProfitOffset = 1203
	vaultLastReportOffset       = 1211
	vaultProfitDegradeOffset    = 1219

	tokenAccountMintOffset = 0
	tokenAmountOffset      = 64
	tokenAccountMinLength  = 72

	mintSupplyOffset   = 36
	mintDecimalsOffset = 44
	mintAccountMinSize = 45

	clockUnixTimestampOffset = 32
	clockMinDataSize         = 40

	lockedProfitDenominator = uint64(1_000_000_000_000)
)

var dammV1ProgramID = solana.MustPublicKeyFromBase58("Eo7WjKq67rjJQSZxS6z3YkapzY3eMj6Xy8X5EQVn5UaB")
var vaultProgramID = solana.MustPublicKeyFromBase58("24Uqj9JCLxUeoC3hGfh5W3s9FM9uCHDS2SG3LYwBpyTi")

var poolDiscriminator = []byte{241, 154, 109, 4, 17, 177, 109, 188}
var clockSysvarID = solana.MustPublicKeyFromBase58("SysvarC1ock11111111111111111111111111111111")

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
	lpMint            solana.PublicKey
	tokenAMint        solana.PublicKey
	tokenBMint        solana.PublicKey
	aVault            solana.PublicKey
	bVault            solana.PublicKey
	aVaultLPToken     solana.PublicKey
	bVaultLPToken     solana.PublicKey
	protocolTokenAFee solana.PublicKey
	protocolTokenBFee solana.PublicKey
	stake             solana.PublicKey
	aVaultLPBump      uint8
	enabled           bool
	feeLastUpdatedAt  uint64
	poolType          uint8
	totalLockedLP     uint64
	curveTypeTag      uint8
}

type vaultState struct {
	tokenVault       solana.PublicKey
	lpMint           solana.PublicKey
	totalAmount      uint64
	lastLockedProfit uint64
	lastReport       uint64
	profitDegrade    uint64
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
	if !poolInfo.Owner.Equals(dammV1ProgramID) {
		return nil, fmt.Errorf("invalid meteora damm v1 owner: %s", poolInfo.Owner.String())
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
		state.aVaultLPToken,
		state.bVaultLPToken,
		state.aVault,
		state.bVault,
		state.tokenAMint,
		state.tokenBMint,
		clockSysvarID,
	})
	if err != nil {
		return nil, err
	}
	if len(accounts) != 7 {
		return nil, fmt.Errorf("unexpected account batch size: %d", len(accounts))
	}
	for idx, acc := range accounts {
		if acc == nil || !acc.Exists {
			return nil, fmt.Errorf("required account missing at index %d", idx)
		}
	}

	poolVaultALPRaw, aVaultLPMint, err := decodeTokenAccountAmountAndMint(accounts[0].Data)
	if err != nil {
		return nil, fmt.Errorf("decode pool a vault lp token account: %w", err)
	}
	poolVaultBLPRaw, bVaultLPMint, err := decodeTokenAccountAmountAndMint(accounts[1].Data)
	if err != nil {
		return nil, fmt.Errorf("decode pool b vault lp token account: %w", err)
	}
	aVaultState, err := decodeVaultState(accounts[2].Data)
	if err != nil {
		return nil, fmt.Errorf("decode vaultA state: %w", err)
	}
	bVaultState, err := decodeVaultState(accounts[3].Data)
	if err != nil {
		return nil, fmt.Errorf("decode vaultB state: %w", err)
	}
	tokenADecimals, err := decodeMintDecimals(accounts[4].Data)
	if err != nil {
		return nil, fmt.Errorf("decode tokenA decimals: %w", err)
	}
	tokenBDecimals, err := decodeMintDecimals(accounts[5].Data)
	if err != nil {
		return nil, fmt.Errorf("decode tokenB decimals: %w", err)
	}
	clockUnixTimestamp, err := decodeClockUnixTimestamp(accounts[6].Data)
	if err != nil {
		return nil, fmt.Errorf("decode clock sysvar: %w", err)
	}

	lpMints, err := c.rpc.GetMultipleAccounts(ctx, []solana.PublicKey{
		aVaultLPMint,
		bVaultLPMint,
	})
	if err != nil {
		return nil, err
	}
	if len(lpMints) != 2 {
		return nil, fmt.Errorf("unexpected lp mint batch size: %d", len(lpMints))
	}
	for idx, acc := range lpMints {
		if acc == nil || !acc.Exists {
			return nil, fmt.Errorf("required lp mint missing at index %d", idx)
		}
	}

	aVaultLPSupplyRaw, err := decodeMintSupply(lpMints[0].Data)
	if err != nil {
		return nil, fmt.Errorf("decode vaultA lp mint supply: %w", err)
	}
	bVaultLPSupplyRaw, err := decodeMintSupply(lpMints[1].Data)
	if err != nil {
		return nil, fmt.Errorf("decode vaultB lp mint supply: %w", err)
	}

	aVaultWithdrawableRaw, aVaultLockedProfitRaw := vaultWithdrawableAmount(clockUnixTimestamp, aVaultState)
	bVaultWithdrawableRaw, bVaultLockedProfitRaw := vaultWithdrawableAmount(clockUnixTimestamp, bVaultState)
	tokenAReserveRaw := amountByShare(poolVaultALPRaw, aVaultWithdrawableRaw, aVaultLPSupplyRaw)
	tokenBReserveRaw := amountByShare(poolVaultBLPRaw, bVaultWithdrawableRaw, bVaultLPSupplyRaw)
	snapshot := &reserveSnapshot{
		tokenAMint:    state.tokenAMint,
		tokenBMint:    state.tokenBMint,
		tokenAReserve: decimalFromU64(tokenAReserveRaw, tokenADecimals),
		tokenBReserve: decimalFromU64(tokenBReserveRaw, tokenBDecimals),
	}
	if snapshot.tokenAReserve.IsZero() || snapshot.tokenBReserve.IsZero() {
		return nil, fmt.Errorf("pool reserves are zero")
	}

	priceAInB := priceOfMintAInMintB(req, snapshot)
	liquidityInB := liquidityInMintB(req, snapshot, priceAInB)

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
			"dex":                             "meteora",
			"pool_version":                    "damm_v1",
			"source":                          "meteora_damm_v1_pool_account",
			"pool_program_id":                 dammV1ProgramID.String(),
			"vault_program_id":                vaultProgramID.String(),
			"pool_enabled":                    state.enabled,
			"pool_type":                       state.poolType,
			"pool_curve_type":                 curveTypeString(state.curveTypeTag),
			"pool_curve_type_tag":             state.curveTypeTag,
			"pool_fee_last_updated_at":        state.feeLastUpdatedAt,
			"pool_total_locked_lp":            state.totalLockedLP,
			"pool_a_vault_lp_bump":            state.aVaultLPBump,
			"pool_lp_mint":                    state.lpMint.String(),
			"pool_stake":                      state.stake.String(),
			"pool_token0_mint":                state.tokenAMint.String(),
			"pool_token1_mint":                state.tokenBMint.String(),
			"pool_token0_vault":               state.aVault.String(),
			"pool_token1_vault":               state.bVault.String(),
			"pool_token0_vault_lp_account":    state.aVaultLPToken.String(),
			"pool_token1_vault_lp_account":    state.bVaultLPToken.String(),
			"pool_protocol_token0_fee":        state.protocolTokenAFee.String(),
			"pool_protocol_token1_fee":        state.protocolTokenBFee.String(),
			"pool_token0_decimals":            tokenADecimals,
			"pool_token1_decimals":            tokenBDecimals,
			"pool_token0_reserve":             snapshot.tokenAReserve.String(),
			"pool_token1_reserve":             snapshot.tokenBReserve.String(),
			"pool_token0_amount_raw":          tokenAReserveRaw,
			"pool_token1_amount_raw":          tokenBReserveRaw,
			"pool_token0_vault_lp_balance":    poolVaultALPRaw,
			"pool_token1_vault_lp_balance":    poolVaultBLPRaw,
			"pool_token0_vault_lp_mint":       aVaultLPMint.String(),
			"pool_token1_vault_lp_mint":       bVaultLPMint.String(),
			"pool_token0_vault_lp_supply":     aVaultLPSupplyRaw,
			"pool_token1_vault_lp_supply":     bVaultLPSupplyRaw,
			"pool_token0_vault_total_amount":  aVaultState.totalAmount,
			"pool_token1_vault_total_amount":  bVaultState.totalAmount,
			"pool_token0_vault_token_vault":   aVaultState.tokenVault.String(),
			"pool_token1_vault_token_vault":   bVaultState.tokenVault.String(),
			"pool_token0_vault_locked_profit": aVaultLockedProfitRaw,
			"pool_token1_vault_locked_profit": bVaultLockedProfitRaw,
			"pool_clock_unix_timestamp":       clockUnixTimestamp,
			"fdv_supply":                      fdvSupply.String(),
			"fdv_method":                      fdvMethod,
		},
	}, nil
}

func decodePoolState(data []byte) (poolState, error) {
	if len(data) < poolMinDataSize {
		return poolState{}, fmt.Errorf("invalid meteora damm v1 pool data length: %d", len(data))
	}
	if !bytes.Equal(data[:8], poolDiscriminator) {
		return poolState{}, fmt.Errorf("invalid meteora damm v1 pool discriminator")
	}
	return poolState{
		lpMint:            solana.PublicKeyFromBytes(data[lpMintOffset : lpMintOffset+32]),
		tokenAMint:        solana.PublicKeyFromBytes(data[tokenAMintOffset : tokenAMintOffset+32]),
		tokenBMint:        solana.PublicKeyFromBytes(data[tokenBMintOffset : tokenBMintOffset+32]),
		aVault:            solana.PublicKeyFromBytes(data[aVaultOffset : aVaultOffset+32]),
		bVault:            solana.PublicKeyFromBytes(data[bVaultOffset : bVaultOffset+32]),
		aVaultLPToken:     solana.PublicKeyFromBytes(data[aVaultLPOffset : aVaultLPOffset+32]),
		bVaultLPToken:     solana.PublicKeyFromBytes(data[bVaultLPOffset : bVaultLPOffset+32]),
		protocolTokenAFee: solana.PublicKeyFromBytes(data[protocolTokenAFeeOffset : protocolTokenAFeeOffset+32]),
		protocolTokenBFee: solana.PublicKeyFromBytes(data[protocolTokenBFeeOffset : protocolTokenBFeeOffset+32]),
		stake:             solana.PublicKeyFromBytes(data[stakeOffset : stakeOffset+32]),
		aVaultLPBump:      data[aVaultLPBumpOffset],
		enabled:           data[enabledOffset] != 0,
		feeLastUpdatedAt:  binary.LittleEndian.Uint64(data[feeLastUpdatedAtOffset : feeLastUpdatedAtOffset+8]),
		poolType:          data[poolTypeOffset],
		totalLockedLP:     binary.LittleEndian.Uint64(data[totalLockedLPOffset : totalLockedLPOffset+8]),
		curveTypeTag:      data[curveTypeTagOffset],
	}, nil
}

func decodeTokenAccountAmountAndMint(data []byte) (uint64, solana.PublicKey, error) {
	if len(data) < tokenAccountMinLength {
		return 0, solana.PublicKey{}, fmt.Errorf("invalid token account data length: %d", len(data))
	}
	mint := solana.PublicKeyFromBytes(data[tokenAccountMintOffset : tokenAccountMintOffset+32])
	amount := binary.LittleEndian.Uint64(data[tokenAmountOffset : tokenAmountOffset+8])
	return amount, mint, nil
}

func decodeVaultState(data []byte) (vaultState, error) {
	if len(data) < vaultMinDataSize {
		return vaultState{}, fmt.Errorf("invalid vault data length: %d", len(data))
	}
	return vaultState{
		totalAmount:      binary.LittleEndian.Uint64(data[vaultTotalAmountOffset : vaultTotalAmountOffset+8]),
		tokenVault:       solana.PublicKeyFromBytes(data[vaultTokenVaultOffset : vaultTokenVaultOffset+32]),
		lpMint:           solana.PublicKeyFromBytes(data[vaultLPMintOffset : vaultLPMintOffset+32]),
		lastLockedProfit: binary.LittleEndian.Uint64(data[vaultLastLockedProfitOffset : vaultLastLockedProfitOffset+8]),
		lastReport:       binary.LittleEndian.Uint64(data[vaultLastReportOffset : vaultLastReportOffset+8]),
		profitDegrade:    binary.LittleEndian.Uint64(data[vaultProfitDegradeOffset : vaultProfitDegradeOffset+8]),
	}, nil
}

func decodeMintDecimals(data []byte) (uint8, error) {
	if len(data) < mintAccountMinSize {
		return 0, fmt.Errorf("invalid mint account data length: %d", len(data))
	}
	return data[mintDecimalsOffset], nil
}

func decodeMintSupply(data []byte) (uint64, error) {
	if len(data) < mintAccountMinSize {
		return 0, fmt.Errorf("invalid mint account data length: %d", len(data))
	}
	return binary.LittleEndian.Uint64(data[mintSupplyOffset : mintSupplyOffset+8]), nil
}

func decodeClockUnixTimestamp(data []byte) (int64, error) {
	if len(data) < clockMinDataSize {
		return 0, fmt.Errorf("invalid clock data length: %d", len(data))
	}
	return int64(binary.LittleEndian.Uint64(data[clockUnixTimestampOffset : clockUnixTimestampOffset+8])), nil
}

func poolMatchesRequest(req Request, state poolState) bool {
	return (mintsEquivalent(req.MintA, state.tokenAMint) && mintsEquivalent(req.MintB, state.tokenBMint)) ||
		(mintsEquivalent(req.MintA, state.tokenBMint) && mintsEquivalent(req.MintB, state.tokenAMint))
}

func lockedProfit(currentUnix int64, lastLockedProfit uint64, lastReport uint64, degrade uint64) uint64 {
	if lastLockedProfit == 0 || degrade == 0 {
		return 0
	}
	var duration uint64
	if currentUnix > 0 && uint64(currentUnix) > lastReport {
		duration = uint64(currentUnix) - lastReport
	}
	ratio := new(big.Int).Mul(new(big.Int).SetUint64(duration), new(big.Int).SetUint64(degrade))
	denominator := new(big.Int).SetUint64(lockedProfitDenominator)
	if ratio.Cmp(denominator) > 0 {
		return 0
	}
	remaining := new(big.Int).Sub(denominator, ratio)
	locked := new(big.Int).Mul(new(big.Int).SetUint64(lastLockedProfit), remaining)
	locked.Div(locked, denominator)
	if !locked.IsUint64() {
		return 0
	}
	return locked.Uint64()
}

func vaultWithdrawableAmount(currentUnix int64, state vaultState) (uint64, uint64) {
	locked := lockedProfit(currentUnix, state.lastLockedProfit, state.lastReport, state.profitDegrade)
	if locked > state.totalAmount {
		return 0, locked
	}
	return state.totalAmount - locked, locked
}

func amountByShare(share uint64, totalAmount uint64, totalSupply uint64) uint64 {
	if share == 0 || totalAmount == 0 || totalSupply == 0 {
		return 0
	}
	numerator := new(big.Int).Mul(new(big.Int).SetUint64(share), new(big.Int).SetUint64(totalAmount))
	value := numerator.Div(numerator, new(big.Int).SetUint64(totalSupply))
	if !value.IsUint64() {
		return 0
	}
	return value.Uint64()
}

func curveTypeString(tag uint8) string {
	switch tag {
	case 0:
		return "constant_product"
	case 1:
		return "stable"
	default:
		return "unknown"
	}
}

func priceOfMintAInMintB(req Request, snapshot *reserveSnapshot) decimal.Decimal {
	if snapshot == nil || snapshot.tokenAReserve.IsZero() || snapshot.tokenBReserve.IsZero() {
		return decimal.Zero
	}
	switch {
	case mintsEquivalent(req.MintA, snapshot.tokenAMint) && mintsEquivalent(req.MintB, snapshot.tokenBMint):
		return snapshot.tokenBReserve.Div(snapshot.tokenAReserve)
	case mintsEquivalent(req.MintA, snapshot.tokenBMint) && mintsEquivalent(req.MintB, snapshot.tokenAMint):
		return snapshot.tokenAReserve.Div(snapshot.tokenBReserve)
	default:
		return decimal.Zero
	}
}

func liquidityInMintB(req Request, snapshot *reserveSnapshot, priceTokenAInTokenB decimal.Decimal) decimal.Decimal {
	if snapshot == nil {
		return decimal.Zero
	}
	switch {
	case mintsEquivalent(req.MintB, snapshot.tokenBMint):
		return snapshot.tokenBReserve.Add(snapshot.tokenAReserve.Mul(priceTokenAInTokenB))
	case mintsEquivalent(req.MintB, snapshot.tokenAMint):
		return snapshot.tokenAReserve.Add(snapshot.tokenBReserve.Mul(priceTokenAInTokenB))
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
