package whirlpool

import (
	"context"
	"encoding/binary"
	"errors"
	"math/big"
	"testing"

	"github.com/TokensHive/solana-token-market-go/sdk/rpc"
	"github.com/gagliardetto/solana-go"
	rpcclient "github.com/gagliardetto/solana-go/rpc"
	"github.com/shopspring/decimal"
)

const pumpProgramIDString = "6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P"

type mockRPC struct {
	accounts       map[string]*rpc.AccountInfo
	getAccountErr  error
	getMultipleErr error
	getMultipleFn  func([]solana.PublicKey) ([]*rpc.AccountInfo, error)
}

func (m *mockRPC) GetAccount(_ context.Context, address solana.PublicKey) (*rpc.AccountInfo, error) {
	if m.getAccountErr != nil {
		return nil, m.getAccountErr
	}
	acc := m.accounts[address.String()]
	if acc == nil {
		return &rpc.AccountInfo{Address: address, Exists: false}, nil
	}
	return acc, nil
}

func (m *mockRPC) GetMultipleAccounts(_ context.Context, addresses []solana.PublicKey) ([]*rpc.AccountInfo, error) {
	if m.getMultipleErr != nil {
		return nil, m.getMultipleErr
	}
	if m.getMultipleFn != nil {
		return m.getMultipleFn(addresses)
	}
	out := make([]*rpc.AccountInfo, 0, len(addresses))
	for _, address := range addresses {
		acc := m.accounts[address.String()]
		if acc == nil {
			out = append(out, &rpc.AccountInfo{Address: address, Exists: false})
			continue
		}
		out = append(out, acc)
	}
	return out, nil
}

func (m *mockRPC) GetTokenSupply(context.Context, solana.PublicKey) (decimal.Decimal, uint8, error) {
	return decimal.Zero, 0, nil
}

func (m *mockRPC) GetSignaturesForAddress(context.Context, solana.PublicKey, *rpc.SignaturesForAddressOptions) ([]*rpcclient.TransactionSignature, error) {
	return nil, nil
}

func (m *mockRPC) GetTransaction(context.Context, solana.Signature) (*rpcclient.GetTransactionResult, error) {
	return nil, nil
}

func (m *mockRPC) GetTransactionRaw(context.Context, solana.Signature) ([]byte, error) {
	return nil, nil
}

type mockSupply struct {
	total  decimal.Decimal
	circ   decimal.Decimal
	method string
	err    error
}

func (m *mockSupply) GetSupply(context.Context, solana.PublicKey) (decimal.Decimal, decimal.Decimal, string, error) {
	return m.total, m.circ, m.method, m.err
}

type mockQuote struct {
	value decimal.Decimal
	err   error
}

func (m *mockQuote) ToSOL(context.Context, solana.PublicKey, decimal.Decimal) (decimal.Decimal, error) {
	return m.value, m.err
}

func testPubkey(seed byte) solana.PublicKey {
	out := make([]byte, 32)
	for i := range out {
		out[i] = seed
	}
	return solana.PublicKeyFromBytes(out)
}

func putU128(dst []byte, offset int, value uint64) {
	binary.LittleEndian.PutUint64(dst[offset:offset+8], value)
	binary.LittleEndian.PutUint64(dst[offset+8:offset+16], 0)
}

func putU128Parts(dst []byte, offset int, low uint64, high uint64) {
	binary.LittleEndian.PutUint64(dst[offset:offset+8], low)
	binary.LittleEndian.PutUint64(dst[offset+8:offset+16], high)
}

func makePoolData(tokenMintA, tokenMintB, tokenVaultA, tokenVaultB solana.PublicKey) []byte {
	data := make([]byte, poolMinDataSize)
	copy(data[:8], whirlpoolDiscriminator)
	copy(data[whirlpoolsConfigOffset:whirlpoolsConfigOffset+32], testPubkey(10).Bytes())
	data[whirlpoolBumpOffset] = 7
	binary.LittleEndian.PutUint16(data[tickSpacingOffset:tickSpacingOffset+2], 64)
	binary.LittleEndian.PutUint16(data[feeTierIndexSeedOffset:feeTierIndexSeedOffset+2], 64)
	binary.LittleEndian.PutUint16(data[feeRateOffset:feeRateOffset+2], 300)
	binary.LittleEndian.PutUint16(data[protocolFeeRateOffset:protocolFeeRateOffset+2], 2500)
	putU128(data, liquidityOffset, 123456)
	putU128Parts(data, sqrtPriceOffset, 0, 1)
	binary.LittleEndian.PutUint32(data[tickCurrentIndexOffset:tickCurrentIndexOffset+4], uint32(12))
	binary.LittleEndian.PutUint64(data[protocolFeeOwedAOffset:protocolFeeOwedAOffset+8], 10)
	binary.LittleEndian.PutUint64(data[protocolFeeOwedBOffset:protocolFeeOwedBOffset+8], 20)
	copy(data[tokenMintAOffset:tokenMintAOffset+32], tokenMintA.Bytes())
	copy(data[tokenVaultAOffset:tokenVaultAOffset+32], tokenVaultA.Bytes())
	putU128(data, feeGrowthGlobalAOffset, 1)
	copy(data[tokenMintBOffset:tokenMintBOffset+32], tokenMintB.Bytes())
	copy(data[tokenVaultBOffset:tokenVaultBOffset+32], tokenVaultB.Bytes())
	putU128(data, feeGrowthGlobalBOffset, 2)
	binary.LittleEndian.PutUint64(data[rewardLastUpdatedAt:rewardLastUpdatedAt+8], 55)
	return data
}

func makeTokenAccountData(amount uint64) []byte {
	data := make([]byte, tokenAccountMinLength)
	binary.LittleEndian.PutUint64(data[tokenAmountOffset:tokenAmountOffset+8], amount)
	return data
}

func makeMintData(decimals uint8) []byte {
	data := make([]byte, mintAccountMinSize)
	data[mintDecimalsOffset] = decimals
	return data
}

func TestCompute_UsesPoolAndReserves(t *testing.T) {
	pool := testPubkey(1)
	mintA := solana.MustPublicKeyFromBase58("EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")
	mintB := solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")
	vaultA := testPubkey(2)
	vaultB := testPubkey(3)

	mockRPCClient := &mockRPC{
		accounts: map[string]*rpc.AccountInfo{
			pool.String():   {Address: pool, Owner: whirlpoolProgramID, Exists: true, Data: makePoolData(mintA, mintB, vaultA, vaultB)},
			vaultA.String(): {Address: vaultA, Exists: true, Data: makeTokenAccountData(2_000_000)},
			vaultB.String(): {Address: vaultB, Exists: true, Data: makeTokenAccountData(5_000_000_000)},
			mintA.String():  {Address: mintA, Exists: true, Data: makeMintData(6)},
			mintB.String():  {Address: mintB, Exists: true, Data: makeMintData(9)},
		},
	}

	calc := NewCalculator(mockRPCClient, nil, &mockSupply{
		total:  decimal.NewFromInt(1_000_000),
		circ:   decimal.NewFromInt(900_000),
		method: "mock_supply",
	})
	resp, err := calc.Compute(context.Background(), Request{
		PoolAddress: pool,
		MintA:       mintA,
		MintB:       solana.SolMint,
	})
	if err != nil {
		t.Fatalf("compute failed: %v", err)
	}

	if got := resp.PriceOfAInB.String(); got != "0.001" {
		t.Fatalf("unexpected price_a_in_b: %s", got)
	}
	if got := resp.LiquidityInB.String(); got != "5.00199997" {
		t.Fatalf("unexpected liquidity_in_b: %s", got)
	}
	if got := resp.PriceOfAInSOL.String(); got != "0.001" {
		t.Fatalf("unexpected price_a_in_sol: %s", got)
	}
	if got := resp.MarketCapInSOL.String(); got != "900" {
		t.Fatalf("unexpected market cap: %s", got)
	}
	if got := resp.FDVInSOL.String(); got != "1000" {
		t.Fatalf("unexpected fdv: %s", got)
	}
	if resp.Metadata["pool_version"] != "whirlpool" {
		t.Fatalf("unexpected metadata pool_version: %#v", resp.Metadata["pool_version"])
	}
}

func TestCompute_UsesPumpCurveTotalSupplyForFDV(t *testing.T) {
	pool := testPubkey(21)
	mintA := solana.MustPublicKeyFromBase58("2nCeHpECQvnMfzjU5fDMAKws1vBxMzxvWr6qqLpApump")
	mintB := solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")
	vaultA := testPubkey(22)
	vaultB := testPubkey(23)

	mockRPCClient := &mockRPC{
		accounts: map[string]*rpc.AccountInfo{
			pool.String():   {Address: pool, Owner: whirlpoolProgramID, Exists: true, Data: makePoolData(mintA, mintB, vaultA, vaultB)},
			vaultA.String(): {Address: vaultA, Exists: true, Data: makeTokenAccountData(2_000_000)},
			vaultB.String(): {Address: vaultB, Exists: true, Data: makeTokenAccountData(5_000_000_000)},
			mintA.String():  {Address: mintA, Exists: true, Data: makeMintData(6)},
			mintB.String():  {Address: mintB, Exists: true, Data: makeMintData(9)},
		},
	}

	pumpProgramID := solana.MustPublicKeyFromBase58(pumpProgramIDString)
	bondingCurve, _, err := solana.FindProgramAddress([][]byte{
		[]byte("bonding-curve"),
		mintA.Bytes(),
	}, pumpProgramID)
	if err != nil {
		t.Fatalf("derive bonding curve pda: %v", err)
	}
	bondingCurveData := make([]byte, 48)
	binary.LittleEndian.PutUint64(bondingCurveData[40:48], 1_000_000_000_000_000)
	mockRPCClient.accounts[bondingCurve.String()] = &rpc.AccountInfo{
		Address: bondingCurve,
		Exists:  true,
		Owner:   pumpProgramID,
		Data:    bondingCurveData,
	}

	calc := NewCalculator(mockRPCClient, nil, &mockSupply{
		total:  decimal.RequireFromString("465147407.683957"),
		circ:   decimal.RequireFromString("465147407.683957"),
		method: "mint_total_equals_circulating_default",
	})
	resp, err := calc.Compute(context.Background(), Request{
		PoolAddress: pool,
		MintA:       mintA,
		MintB:       solana.SolMint,
	})
	if err != nil {
		t.Fatalf("compute failed: %v", err)
	}
	if !resp.FDVInSOL.GreaterThan(resp.MarketCapInSOL) {
		t.Fatalf("expected fdv (%s) > market cap (%s)", resp.FDVInSOL, resp.MarketCapInSOL)
	}
	if resp.Metadata["fdv_method"] != "pumpfun_curve_token_total_supply" {
		t.Fatalf("unexpected fdv_method metadata: %#v", resp.Metadata["fdv_method"])
	}
}

func TestCompute_ValidationAndPoolErrors(t *testing.T) {
	if _, err := NewCalculator(nil, nil, &mockSupply{}).Compute(context.Background(), Request{}); err == nil {
		t.Fatal("expected rpc required error")
	}
	if _, err := NewCalculator(&mockRPC{}, nil, nil).Compute(context.Background(), Request{}); err == nil {
		t.Fatal("expected supply required error")
	}
	if _, err := NewCalculator(&mockRPC{}, nil, &mockSupply{}).Compute(context.Background(), Request{}); err == nil {
		t.Fatal("expected pool address required error")
	}
	if _, err := NewCalculator(&mockRPC{}, nil, &mockSupply{}).Compute(context.Background(), Request{PoolAddress: solana.SolMint}); err == nil {
		t.Fatal("expected mint required error")
	}

	calc := NewCalculator(&mockRPC{getAccountErr: errors.New("rpc failed")}, nil, &mockSupply{})
	if _, err := calc.Compute(context.Background(), Request{
		PoolAddress: solana.SolMint,
		MintA:       solana.SolMint,
		MintB:       solana.SolMint,
	}); err == nil {
		t.Fatal("expected get account error")
	}

	calc = NewCalculator(&mockRPC{accounts: map[string]*rpc.AccountInfo{}}, nil, &mockSupply{})
	if _, err := calc.Compute(context.Background(), Request{
		PoolAddress: solana.SolMint,
		MintA:       solana.SolMint,
		MintB:       solana.SolMint,
	}); err == nil {
		t.Fatal("expected pool not found")
	}

	calc = NewCalculator(&mockRPC{
		accounts: map[string]*rpc.AccountInfo{
			solana.SolMint.String(): {Exists: true, Owner: solana.SolMint, Data: make([]byte, poolMinDataSize)},
		},
	}, nil, &mockSupply{})
	if _, err := calc.Compute(context.Background(), Request{
		PoolAddress: solana.SolMint,
		MintA:       solana.SolMint,
		MintB:       solana.SolMint,
	}); err == nil {
		t.Fatal("expected invalid owner error")
	}

	calc = NewCalculator(&mockRPC{
		accounts: map[string]*rpc.AccountInfo{
			solana.SolMint.String(): {Exists: true, Owner: whirlpoolProgramID, Data: []byte{1}},
		},
	}, nil, &mockSupply{})
	if _, err := calc.Compute(context.Background(), Request{
		PoolAddress: solana.SolMint,
		MintA:       solana.SolMint,
		MintB:       solana.SolMint,
	}); err == nil {
		t.Fatal("expected decode pool error")
	}
}

func TestCompute_BatchDecodeAndDownstreamErrors(t *testing.T) {
	pool := testPubkey(41)
	mintA := solana.MustPublicKeyFromBase58("EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")
	mintB := solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")
	vaultA := testPubkey(42)
	vaultB := testPubkey(43)

	baseAccounts := map[string]*rpc.AccountInfo{
		pool.String():   {Address: pool, Owner: whirlpoolProgramID, Exists: true, Data: makePoolData(mintA, mintB, vaultA, vaultB)},
		vaultA.String(): {Address: vaultA, Exists: true, Data: makeTokenAccountData(2_000_000)},
		vaultB.String(): {Address: vaultB, Exists: true, Data: makeTokenAccountData(5_000_000_000)},
		mintA.String():  {Address: mintA, Exists: true, Data: makeMintData(6)},
		mintB.String():  {Address: mintB, Exists: true, Data: makeMintData(9)},
	}

	calc := NewCalculator(&mockRPC{accounts: baseAccounts}, nil, &mockSupply{})
	if _, err := calc.Compute(context.Background(), Request{
		PoolAddress: pool,
		MintA:       testPubkey(99),
		MintB:       mintB,
	}); err == nil {
		t.Fatal("expected pool mismatch error")
	}

	calc = NewCalculator(&mockRPC{
		accounts:       baseAccounts,
		getMultipleErr: errors.New("batch failed"),
	}, nil, &mockSupply{})
	if _, err := calc.Compute(context.Background(), Request{
		PoolAddress: pool,
		MintA:       mintA,
		MintB:       mintB,
	}); err == nil {
		t.Fatal("expected batch rpc error")
	}

	calc = NewCalculator(&mockRPC{
		accounts: baseAccounts,
		getMultipleFn: func([]solana.PublicKey) ([]*rpc.AccountInfo, error) {
			return []*rpc.AccountInfo{}, nil
		},
	}, nil, &mockSupply{})
	if _, err := calc.Compute(context.Background(), Request{
		PoolAddress: pool,
		MintA:       mintA,
		MintB:       mintB,
	}); err == nil {
		t.Fatal("expected unexpected batch size error")
	}

	calc = NewCalculator(&mockRPC{
		accounts: baseAccounts,
		getMultipleFn: func([]solana.PublicKey) ([]*rpc.AccountInfo, error) {
			return []*rpc.AccountInfo{
				nil,
				{Exists: true, Data: makeTokenAccountData(1)},
				{Exists: true, Data: makeMintData(6)},
				{Exists: true, Data: makeMintData(9)},
			}, nil
		},
	}, nil, &mockSupply{})
	if _, err := calc.Compute(context.Background(), Request{
		PoolAddress: pool,
		MintA:       mintA,
		MintB:       mintB,
	}); err == nil {
		t.Fatal("expected required account missing error")
	}

	cases := []struct {
		name string
		a0   []byte
		a1   []byte
		a2   []byte
		a3   []byte
	}{
		{name: "decode token vault a", a0: []byte{1}, a1: makeTokenAccountData(1), a2: makeMintData(6), a3: makeMintData(9)},
		{name: "decode token vault b", a0: makeTokenAccountData(1), a1: []byte{1}, a2: makeMintData(6), a3: makeMintData(9)},
		{name: "decode mint a", a0: makeTokenAccountData(1), a1: makeTokenAccountData(1), a2: []byte{1}, a3: makeMintData(9)},
		{name: "decode mint b", a0: makeTokenAccountData(1), a1: makeTokenAccountData(1), a2: makeMintData(6), a3: []byte{1}},
	}
	for _, tc := range cases {
		calc = NewCalculator(&mockRPC{
			accounts: baseAccounts,
			getMultipleFn: func([]solana.PublicKey) ([]*rpc.AccountInfo, error) {
				return []*rpc.AccountInfo{
					{Exists: true, Data: tc.a0},
					{Exists: true, Data: tc.a1},
					{Exists: true, Data: tc.a2},
					{Exists: true, Data: tc.a3},
				}, nil
			},
		}, nil, &mockSupply{})
		if _, err := calc.Compute(context.Background(), Request{
			PoolAddress: pool,
			MintA:       mintA,
			MintB:       mintB,
		}); err == nil {
			t.Fatalf("expected error for case %s", tc.name)
		}
	}

	calc = NewCalculator(&mockRPC{
		accounts: baseAccounts,
		getMultipleFn: func([]solana.PublicKey) ([]*rpc.AccountInfo, error) {
			return []*rpc.AccountInfo{
				{Exists: true, Data: makeTokenAccountData(0)},
				{Exists: true, Data: makeTokenAccountData(0)},
				{Exists: true, Data: makeMintData(6)},
				{Exists: true, Data: makeMintData(9)},
			}, nil
		},
	}, nil, &mockSupply{})
	if _, err := calc.Compute(context.Background(), Request{
		PoolAddress: pool,
		MintA:       mintA,
		MintB:       mintB,
	}); err == nil {
		t.Fatal("expected zero reserve error")
	}

	quoteErrCalc := NewCalculator(&mockRPC{accounts: baseAccounts}, &mockQuote{err: errors.New("quote error")}, &mockSupply{
		total: decimal.NewFromInt(1), circ: decimal.NewFromInt(1), method: "ok",
	})
	if _, err := quoteErrCalc.Compute(context.Background(), Request{
		PoolAddress: pool,
		MintA:       mintA,
		MintB:       testPubkey(77),
	}); err == nil {
		t.Fatal("expected quote conversion error")
	}

	supplyErrCalc := NewCalculator(&mockRPC{accounts: baseAccounts}, &mockQuote{value: decimal.NewFromInt(1)}, &mockSupply{err: errors.New("supply error")})
	if _, err := supplyErrCalc.Compute(context.Background(), Request{
		PoolAddress: pool,
		MintA:       mintA,
		MintB:       mintB,
	}); err == nil {
		t.Fatal("expected supply error")
	}
}

func TestHelpers(t *testing.T) {
	if _, err := decodePoolState([]byte{1}); err == nil {
		t.Fatal("expected decode pool short data error")
	}
	invalidData := make([]byte, poolMinDataSize)
	if _, err := decodePoolState(invalidData); err == nil {
		t.Fatal("expected decode pool discriminator error")
	}
	if _, err := decodeTokenAmount([]byte{1}); err == nil {
		t.Fatal("expected token account short data error")
	}
	if _, err := decodeMintDecimals([]byte{1}); err == nil {
		t.Fatal("expected decode mint decimals short data error")
	}

	if got := readU128([]byte{1}); got.Sign() != 0 {
		t.Fatalf("expected zero for short u128 data, got %s", got.String())
	}
	if got := subtractProtocolFee(10, 20); got != 0 {
		t.Fatalf("expected clamped fee subtraction, got %d", got)
	}
	if got := subtractProtocolFee(20, 10); got != 10 {
		t.Fatalf("unexpected fee subtraction result: %d", got)
	}

	if got := priceTokenAInTokenBFromSqrt(nil, 6, 9); !got.IsZero() {
		t.Fatalf("expected zero price for nil sqrt, got %s", got)
	}
	if got := priceTokenAInTokenBFromSqrt(big.NewInt(0), 6, 9); !got.IsZero() {
		t.Fatalf("expected zero price for zero sqrt, got %s", got)
	}
	if got := priceTokenAInTokenBFromSqrt(new(big.Int).Lsh(big.NewInt(1), 64), 6, 9); got.String() != "0.001" {
		t.Fatalf("unexpected sqrt-derived price: %s", got)
	}

	state := poolState{
		tokenMintA: solana.SolMint,
		tokenMintB: testPubkey(90),
	}
	req := Request{MintA: state.tokenMintA, MintB: state.tokenMintB}
	if !poolMatchesRequest(req, state) {
		t.Fatal("expected pool match")
	}
	if poolMatchesRequest(Request{MintA: testPubkey(91), MintB: state.tokenMintB}, state) {
		t.Fatal("did not expect pool match")
	}

	if got := priceOfMintAInMintB(req, state, decimal.NewFromInt(2)); !got.Equal(decimal.NewFromInt(2)) {
		t.Fatalf("unexpected direct price: %s", got)
	}
	if got := priceOfMintAInMintB(Request{MintA: state.tokenMintB, MintB: state.tokenMintA}, state, decimal.NewFromInt(2)); !got.Equal(decimal.RequireFromString("0.5")) {
		t.Fatalf("unexpected inverse price: %s", got)
	}
	if got := priceOfMintAInMintB(req, state, decimal.Zero); !got.IsZero() {
		t.Fatalf("expected zero price branch, got %s", got)
	}

	snapshot := &reserveSnapshot{
		tokenMintA: state.tokenMintA,
		tokenMintB: state.tokenMintB,
		reserveA:   decimal.NewFromInt(4),
		reserveB:   decimal.NewFromInt(8),
	}
	if got := liquidityInMintB(Request{MintB: state.tokenMintB}, state, snapshot, decimal.NewFromInt(2)); !got.Equal(decimal.NewFromInt(16)) {
		t.Fatalf("unexpected liquidity tokenB branch: %s", got)
	}
	if got := liquidityInMintB(Request{MintB: state.tokenMintA}, state, snapshot, decimal.NewFromInt(2)); !got.Equal(decimal.NewFromInt(8)) {
		t.Fatalf("unexpected liquidity tokenA branch: %s", got)
	}
	if got := liquidityInMintB(Request{MintB: state.tokenMintA}, state, snapshot, decimal.Zero); !got.Equal(decimal.NewFromInt(4)) {
		t.Fatalf("unexpected liquidity tokenA zero price branch: %s", got)
	}
	if got := liquidityInMintB(Request{MintB: testPubkey(99)}, state, snapshot, decimal.NewFromInt(2)); !got.IsZero() {
		t.Fatalf("expected zero liquidity unmatched branch: %s", got)
	}
	if got := liquidityInMintB(req, state, nil, decimal.NewFromInt(2)); !got.IsZero() {
		t.Fatalf("expected zero liquidity nil snapshot branch: %s", got)
	}

	if got := decimalFromU64(12345, 2); got.String() != "123.45" {
		t.Fatalf("unexpected decimal conversion: %s", got)
	}
	if !mintsEquivalent(solana.SolMint, solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")) {
		t.Fatal("expected native and wrapped SOL equivalence")
	}
	if mintsEquivalent(solana.SolMint, testPubkey(100)) {
		t.Fatal("expected non-sol mismatch")
	}
}

func TestSOLAndQuoteConversions(t *testing.T) {
	calc := NewCalculator(&mockRPC{}, nil, &mockSupply{})
	priceInSOL, err := calc.priceOfMintAInSOL(context.Background(), Request{
		MintA: solana.SolMint,
		MintB: testPubkey(1),
	}, decimal.NewFromInt(5))
	if err != nil || !priceInSOL.Equal(decimal.NewFromInt(1)) {
		t.Fatalf("expected SOL mintA price to be 1, got %s err=%v", priceInSOL, err)
	}

	priceInSOL, err = calc.priceOfMintAInSOL(context.Background(), Request{
		MintA: testPubkey(2),
		MintB: solana.SolMint,
	}, decimal.NewFromInt(5))
	if err != nil || !priceInSOL.Equal(decimal.NewFromInt(5)) {
		t.Fatalf("expected passthrough for SOL mintB, got %s err=%v", priceInSOL, err)
	}

	priceInSOL, err = calc.priceOfMintAInSOL(context.Background(), Request{
		MintA: testPubkey(3),
		MintB: testPubkey(4),
	}, decimal.NewFromInt(5))
	if err != nil || !priceInSOL.IsZero() {
		t.Fatalf("expected zero without quote bridge, got %s err=%v", priceInSOL, err)
	}

	calc = NewCalculator(&mockRPC{}, &mockQuote{err: errors.New("quote error")}, &mockSupply{})
	if _, err := calc.priceOfMintAInSOL(context.Background(), Request{
		MintA: testPubkey(5),
		MintB: testPubkey(6),
	}, decimal.NewFromInt(5)); err == nil {
		t.Fatal("expected quote conversion error")
	}

	calc = NewCalculator(&mockRPC{}, &mockQuote{value: decimal.Zero}, &mockSupply{})
	priceInSOL, err = calc.priceOfMintAInSOL(context.Background(), Request{
		MintA: testPubkey(5),
		MintB: testPubkey(6),
	}, decimal.NewFromInt(5))
	if err != nil || !priceInSOL.IsZero() {
		t.Fatalf("expected zero on zero conversion, got %s err=%v", priceInSOL, err)
	}

	calc = NewCalculator(&mockRPC{}, &mockQuote{value: decimal.NewFromInt(3)}, &mockSupply{})
	priceInSOL, err = calc.priceOfMintAInSOL(context.Background(), Request{
		MintA: testPubkey(5),
		MintB: testPubkey(6),
	}, decimal.NewFromInt(5))
	if err != nil || !priceInSOL.Equal(decimal.NewFromInt(15)) {
		t.Fatalf("expected quote converted price, got %s err=%v", priceInSOL, err)
	}

	liqSOL, err := calc.liquidityInSOL(context.Background(), Request{
		MintB: solana.SolMint,
	}, decimal.NewFromInt(7))
	if err != nil || !liqSOL.Equal(decimal.NewFromInt(7)) {
		t.Fatalf("expected SOL liquidity passthrough, got %s err=%v", liqSOL, err)
	}
	liqSOL, err = calc.liquidityInSOL(context.Background(), Request{
		MintB: testPubkey(7),
	}, decimal.NewFromInt(7))
	if err != nil || !liqSOL.Equal(decimal.NewFromInt(3)) {
		t.Fatalf("expected quote converted liquidity, got %s err=%v", liqSOL, err)
	}

	calc = NewCalculator(&mockRPC{}, nil, &mockSupply{})
	liqSOL, err = calc.liquidityInSOL(context.Background(), Request{
		MintB: testPubkey(8),
	}, decimal.NewFromInt(7))
	if err != nil || !liqSOL.IsZero() {
		t.Fatalf("expected zero without quote bridge, got %s err=%v", liqSOL, err)
	}

	calc = NewCalculator(&mockRPC{}, &mockQuote{err: errors.New("quote error")}, &mockSupply{})
	if _, err := calc.liquidityInSOL(context.Background(), Request{
		MintB: testPubkey(9),
	}, decimal.NewFromInt(7)); err == nil {
		t.Fatal("expected quote liquidity conversion error")
	}
}
