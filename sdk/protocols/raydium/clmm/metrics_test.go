package clmm

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
	accounts         map[string]*rpc.AccountInfo
	getAccountErr    error
	getMultipleErr   error
	getMultipleFn    func([]solana.PublicKey) ([]*rpc.AccountInfo, error)
	getAccountCalls  int
	getMultipleCalls int
}

func (m *mockRPC) GetAccount(_ context.Context, address solana.PublicKey) (*rpc.AccountInfo, error) {
	m.getAccountCalls++
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
	m.getMultipleCalls++
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

type sequenceQuote struct {
	values []decimal.Decimal
	errors []error
	idx    int
}

func (s *sequenceQuote) ToSOL(context.Context, solana.PublicKey, decimal.Decimal) (decimal.Decimal, error) {
	i := s.idx
	s.idx++
	if i < len(s.errors) && s.errors[i] != nil {
		return decimal.Zero, s.errors[i]
	}
	if i < len(s.values) {
		return s.values[i], nil
	}
	return decimal.Zero, nil
}

func testPubkey(seed byte) solana.PublicKey {
	out := make([]byte, 32)
	for i := range out {
		out[i] = seed
	}
	return solana.PublicKeyFromBytes(out)
}

func putU128(data []byte, offset int, n *big.Int) {
	if len(data) < offset+16 || n == nil || n.Sign() < 0 {
		return
	}
	lowMask := new(big.Int).SetUint64(^uint64(0))
	low := new(big.Int).And(new(big.Int).Set(n), lowMask).Uint64()
	high := new(big.Int).Rsh(new(big.Int).Set(n), 64).Uint64()
	binary.LittleEndian.PutUint64(data[offset:offset+8], low)
	binary.LittleEndian.PutUint64(data[offset+8:offset+16], high)
}

func makePoolData(token0Mint solana.PublicKey, token1Mint solana.PublicKey, token0Vault solana.PublicKey, token1Vault solana.PublicKey, dec0 uint8, dec1 uint8, sqrtPrice *big.Int) []byte {
	poolData := make([]byte, poolExpectedMinDataSize)
	copy(poolData[token0MintOffset:token0MintOffset+32], token0Mint.Bytes())
	copy(poolData[token1MintOffset:token1MintOffset+32], token1Mint.Bytes())
	copy(poolData[token0VaultOffset:token0VaultOffset+32], token0Vault.Bytes())
	copy(poolData[token1VaultOffset:token1VaultOffset+32], token1Vault.Bytes())
	poolData[mint0DecimalsOffset] = dec0
	poolData[mint1DecimalsOffset] = dec1
	putU128(poolData, sqrtPriceX64Offset, sqrtPrice)
	return poolData
}

func makeVaultData(amount uint64) []byte {
	data := make([]byte, tokenAccountMinLength)
	binary.LittleEndian.PutUint64(data[tokenAmountOffset:tokenAmountOffset+8], amount)
	return data
}

func TestCompute_UsesPoolVaultAndSqrtPrice(t *testing.T) {
	pool := testPubkey(1)
	tokenMint := solana.MustPublicKeyFromBase58("9BHt7aq3DFCb74kZjPY5epgVtsWKCeYX1tUWxYwDpump")
	poolToken0Mint := solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")
	token0Vault := testPubkey(2)
	token1Vault := testPubkey(3)

	poolData := makePoolData(poolToken0Mint, tokenMint, token0Vault, token1Vault, 9, 6, new(big.Int).Lsh(big.NewInt(1), 64))
	binary.LittleEndian.PutUint64(poolData[protocolFees0Offset:protocolFees0Offset+8], 1_000_000_000)
	binary.LittleEndian.PutUint64(poolData[protocolFees1Offset:protocolFees1Offset+8], 100_000_000)
	binary.LittleEndian.PutUint64(poolData[fundFees1Offset:fundFees1Offset+8], 50_000_000)
	binary.LittleEndian.PutUint64(poolData[totalFees0Offset:totalFees0Offset+8], 2_000_000_000)
	binary.LittleEndian.PutUint64(poolData[totalFees1Offset:totalFees1Offset+8], 400_000_000)
	poolData[statusOffset] = 1

	mockRPCClient := &mockRPC{
		accounts: map[string]*rpc.AccountInfo{
			pool.String():        {Address: pool, Exists: true, Data: poolData},
			token0Vault.String(): {Address: token0Vault, Exists: true, Data: makeVaultData(11_000_000_000)},
			token1Vault.String(): {Address: token1Vault, Exists: true, Data: makeVaultData(2_000_000_000)},
		},
	}
	calc := NewCalculator(mockRPCClient, nil, &mockSupply{
		total:  decimal.NewFromInt(1_000_000),
		circ:   decimal.NewFromInt(800_000),
		method: "mock_supply",
	})

	resp, err := calc.Compute(context.Background(), Request{
		PoolAddress: pool,
		MintA:       tokenMint,
		MintB:       solana.SolMint,
	})
	if err != nil {
		t.Fatalf("compute failed: %v", err)
	}
	expectedPrice := decimal.RequireFromString("0.001")
	if !resp.PriceOfAInB.Equal(expectedPrice) {
		t.Fatalf("unexpected price_a_in_b: %s", resp.PriceOfAInB)
	}
	if got := resp.LiquidityInB.String(); got != "20" {
		t.Fatalf("unexpected liquidity_in_b: %s", got)
	}
	if !resp.MarketCapInSOL.Equal(expectedPrice.Mul(decimal.NewFromInt(800_000))) {
		t.Fatalf("unexpected market cap: %s", resp.MarketCapInSOL)
	}
	if !resp.FDVInSOL.Equal(expectedPrice.Mul(decimal.NewFromInt(1_000_000))) {
		t.Fatalf("unexpected fdv: %s", resp.FDVInSOL)
	}
	if resp.Metadata["pool_status"] != uint8(1) {
		t.Fatalf("unexpected pool status metadata: %#v", resp.Metadata["pool_status"])
	}
	if resp.Metadata["pool_price_token1_in_token0"] != "0.001" {
		t.Fatalf("unexpected token1/token0 price metadata: %#v", resp.Metadata["pool_price_token1_in_token0"])
	}
	if mockRPCClient.getAccountCalls != 2 || mockRPCClient.getMultipleCalls != 1 {
		t.Fatalf("expected two getAccount calls and one batch call, got account=%d multiple=%d", mockRPCClient.getAccountCalls, mockRPCClient.getMultipleCalls)
	}
}

func TestCompute_UsesPumpCurveTotalSupplyForFDV(t *testing.T) {
	pool := testPubkey(31)
	mintA := solana.MustPublicKeyFromBase58("2nCeHpECQvnMfzjU5fDMAKws1vBxMzxvWr6qqLpApump")
	poolToken0Mint := solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")
	token0Vault := testPubkey(32)
	token1Vault := testPubkey(33)

	poolData := makePoolData(poolToken0Mint, mintA, token0Vault, token1Vault, 9, 6, new(big.Int).Lsh(big.NewInt(1), 64))
	mockRPCClient := &mockRPC{
		accounts: map[string]*rpc.AccountInfo{
			pool.String():        {Address: pool, Exists: true, Data: poolData},
			token0Vault.String(): {Address: token0Vault, Exists: true, Data: makeVaultData(11_000_000_000)},
			token1Vault.String(): {Address: token1Vault, Exists: true, Data: makeVaultData(2_000_000_000)},
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
	mockRPCClient.accounts[bondingCurve.String()] = &rpc.AccountInfo{Address: bondingCurve, Exists: true, Owner: pumpProgramID, Data: bondingCurveData}

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
			solana.SolMint.String(): {Exists: true, Data: []byte{1}},
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

func TestCompute_BatchDecodeZeroAndDownstreamErrors(t *testing.T) {
	pool := testPubkey(11)
	mintA := solana.MustPublicKeyFromBase58("2nCeHpECQvnMfzjU5fDMAKws1vBxMzxvWr6qqLpApump")
	mintB := solana.MustPublicKeyFromBase58("Es9vMFrzaCER7HhMN7vY4sLhM1cBh73PvvrLpzjAUjWQ")
	token0Vault := testPubkey(12)
	token1Vault := testPubkey(13)
	poolData := makePoolData(mintA, mintB, token0Vault, token1Vault, 6, 6, new(big.Int).Lsh(big.NewInt(1), 64))

	calc := NewCalculator(&mockRPC{
		accounts: map[string]*rpc.AccountInfo{
			pool.String(): {Exists: true, Data: poolData},
		},
	}, nil, &mockSupply{})
	if _, err := calc.Compute(context.Background(), Request{
		PoolAddress: pool,
		MintA:       testPubkey(99),
		MintB:       mintB,
	}); err == nil {
		t.Fatal("expected pool mismatch error")
	}

	calc = NewCalculator(&mockRPC{
		accounts: map[string]*rpc.AccountInfo{
			pool.String(): {Exists: true, Data: poolData},
		},
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
		accounts: map[string]*rpc.AccountInfo{
			pool.String(): {Exists: true, Data: poolData},
		},
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
		accounts: map[string]*rpc.AccountInfo{
			pool.String(): {Exists: true, Data: poolData},
		},
		getMultipleFn: func([]solana.PublicKey) ([]*rpc.AccountInfo, error) {
			return []*rpc.AccountInfo{
				{Exists: true, Data: makeVaultData(1)},
				nil,
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
		acc0 []byte
		acc1 []byte
	}{
		{name: "decode token0", acc0: []byte{1}, acc1: makeVaultData(1)},
		{name: "decode token1", acc0: makeVaultData(1), acc1: []byte{1}},
	}
	for _, tc := range cases {
		calc = NewCalculator(&mockRPC{
			accounts: map[string]*rpc.AccountInfo{
				pool.String(): {Exists: true, Data: poolData},
			},
			getMultipleFn: func([]solana.PublicKey) ([]*rpc.AccountInfo, error) {
				return []*rpc.AccountInfo{{Exists: true, Data: tc.acc0}, {Exists: true, Data: tc.acc1}}, nil
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

	binary.LittleEndian.PutUint64(poolData[protocolFees0Offset:protocolFees0Offset+8], 10_000_000)
	binary.LittleEndian.PutUint64(poolData[protocolFees1Offset:protocolFees1Offset+8], 20_000_000)
	zeroReservesRPC := &mockRPC{
		accounts: map[string]*rpc.AccountInfo{
			pool.String():        {Exists: true, Data: poolData},
			token0Vault.String(): {Exists: true, Data: makeVaultData(10_000_000)},
			token1Vault.String(): {Exists: true, Data: makeVaultData(20_000_000)},
		},
	}
	calc = NewCalculator(zeroReservesRPC, nil, &mockSupply{})
	if _, err := calc.Compute(context.Background(), Request{
		PoolAddress: pool,
		MintA:       mintA,
		MintB:       mintB,
	}); err == nil {
		t.Fatal("expected zero reserve error")
	}

	binary.LittleEndian.PutUint64(poolData[protocolFees0Offset:protocolFees0Offset+8], 0)
	binary.LittleEndian.PutUint64(poolData[protocolFees1Offset:protocolFees1Offset+8], 0)
	priceErrCalc := NewCalculator(&mockRPC{
		accounts: map[string]*rpc.AccountInfo{
			pool.String():        {Exists: true, Data: poolData},
			token0Vault.String(): {Exists: true, Data: makeVaultData(10_000_000)},
			token1Vault.String(): {Exists: true, Data: makeVaultData(20_000_000)},
		},
	}, &mockQuote{err: errors.New("quote error")}, &mockSupply{})
	if _, err := priceErrCalc.Compute(context.Background(), Request{
		PoolAddress: pool,
		MintA:       mintA,
		MintB:       mintB,
	}); err == nil {
		t.Fatal("expected quote conversion error")
	}

	liqErrCalc := NewCalculator(&mockRPC{
		accounts: map[string]*rpc.AccountInfo{
			pool.String():        {Exists: true, Data: poolData},
			token0Vault.String(): {Exists: true, Data: makeVaultData(10_000_000)},
			token1Vault.String(): {Exists: true, Data: makeVaultData(20_000_000)},
		},
	}, &sequenceQuote{
		values: []decimal.Decimal{decimal.NewFromInt(1)},
		errors: []error{nil, errors.New("liquidity quote error")},
	}, &mockSupply{total: decimal.NewFromInt(1), circ: decimal.NewFromInt(1), method: "ok"})
	if _, err := liqErrCalc.Compute(context.Background(), Request{
		PoolAddress: pool,
		MintA:       mintA,
		MintB:       mintB,
	}); err == nil {
		t.Fatal("expected liquidity quote conversion error")
	}

	supplyErrCalc := NewCalculator(&mockRPC{
		accounts: map[string]*rpc.AccountInfo{
			pool.String():        {Exists: true, Data: poolData},
			token0Vault.String(): {Exists: true, Data: makeVaultData(10_000_000)},
			token1Vault.String(): {Exists: true, Data: makeVaultData(20_000_000)},
		},
	}, &mockQuote{value: decimal.NewFromInt(1)}, &mockSupply{err: errors.New("supply error")})
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
	if _, err := decodeTokenAmount([]byte{1}); err == nil {
		t.Fatal("expected decode token amount short data error")
	}
	if got := readU128([]byte{1, 2, 3}); got.Sign() != 0 {
		t.Fatalf("expected short u128 decode to be zero, got %s", got.String())
	}
	if got := readU128([]byte{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}); got.Uint64() != 1 {
		t.Fatalf("unexpected decoded u128 value: %s", got.String())
	}
	if got := subtractFees(10, 3, 4); got != 3 {
		t.Fatalf("unexpected subtract fees result: %d", got)
	}
	if got := subtractFees(10, 7, 4); got != 0 {
		t.Fatalf("expected clamped zero from subtractFees, got %d", got)
	}
	if got := subtractFees(10, 11, 0); got != 0 {
		t.Fatalf("expected clamped zero when protocol fee exceeds vault, got %d", got)
	}
	if got := subtractFees(10, 3, ^uint64(0)); got != 0 {
		t.Fatalf("expected clamped zero on huge fund fee, got %d", got)
	}

	price := priceToken1InToken0FromSqrt(new(big.Int).Lsh(big.NewInt(1), 64), 9, 6)
	if !price.Equal(decimal.RequireFromString("0.001")) {
		t.Fatalf("unexpected price from sqrt: %s", price)
	}
	if got := priceToken1InToken0FromSqrt(nil, 9, 6); !got.IsZero() {
		t.Fatalf("expected zero price for nil sqrt, got %s", got)
	}
	if got := priceToken1InToken0FromSqrt(big.NewInt(0), 9, 6); !got.IsZero() {
		t.Fatalf("expected zero price for zero sqrt, got %s", got)
	}

	req := Request{
		MintA: solana.SolMint,
		MintB: solana.MustPublicKeyFromBase58("11111111111111111111111111111111"),
	}
	state := poolState{
		token0Mint: solana.SolMint,
		token1Mint: req.MintB,
	}
	if got := priceOfMintAInMintB(req, state, decimal.NewFromInt(2)); !got.Equal(decimal.RequireFromString("0.5")) {
		t.Fatalf("unexpected direct price: %s", got)
	}
	if got := priceOfMintAInMintB(Request{MintA: req.MintB, MintB: req.MintA}, state, decimal.NewFromInt(2)); !got.Equal(decimal.NewFromInt(2)) {
		t.Fatalf("unexpected inverse price: %s", got)
	}
	if got := priceOfMintAInMintB(req, state, decimal.Zero); !got.IsZero() {
		t.Fatalf("expected zero price when base price is zero, got %s", got)
	}
	if got := priceOfMintAInMintB(Request{
		MintA: solana.MustPublicKeyFromBase58("SysvarRent111111111111111111111111111111111"),
		MintB: solana.MustPublicKeyFromBase58("SysvarC1ock11111111111111111111111111111111"),
	}, state, decimal.NewFromInt(2)); !got.IsZero() {
		t.Fatalf("expected zero price for unmatched pair, got %s", got)
	}

	snapshot := &reserveSnapshot{
		token0Mint:    req.MintA,
		token1Mint:    req.MintB,
		token0Reserve: decimal.NewFromInt(4),
		token1Reserve: decimal.NewFromInt(8),
	}
	if got := liquidityInMintB(req, snapshot); !got.Equal(decimal.NewFromInt(16)) {
		t.Fatalf("unexpected liquidity in quote: %s", got)
	}
	if got := liquidityInMintB(Request{MintB: req.MintA}, snapshot); !got.Equal(decimal.NewFromInt(8)) {
		t.Fatalf("unexpected liquidity in base: %s", got)
	}
	if got := liquidityInMintB(req, nil); !got.IsZero() {
		t.Fatalf("expected zero liquidity for nil snapshot, got %s", got)
	}
	if got := liquidityInMintB(Request{
		MintB: solana.MustPublicKeyFromBase58("SysvarRent111111111111111111111111111111111"),
	}, snapshot); !got.IsZero() {
		t.Fatalf("expected zero liquidity for unmatched pair, got %s", got)
	}

	if got := decimalFromU64(12345, 2); got.String() != "123.45" {
		t.Fatalf("unexpected decimal conversion: %s", got)
	}
	if !mintsEquivalent(solana.SolMint, solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")) {
		t.Fatal("expected native and wrapped SOL equivalence")
	}
	if mintsEquivalent(solana.SolMint, solana.MustPublicKeyFromBase58("11111111111111111111111111111111")) {
		t.Fatal("expected non-sol mismatch")
	}
}

func TestSOLAndQuoteConversions(t *testing.T) {
	calc := NewCalculator(&mockRPC{}, nil, &mockSupply{})
	priceInSOL, err := calc.priceOfMintAInSOL(context.Background(), Request{
		MintA: solana.SolMint,
		MintB: solana.MustPublicKeyFromBase58("11111111111111111111111111111111"),
	}, decimal.NewFromInt(5))
	if err != nil || !priceInSOL.Equal(decimal.NewFromInt(1)) {
		t.Fatalf("expected SOL mintA price to be 1, got %s err=%v", priceInSOL, err)
	}

	priceInSOL, err = calc.priceOfMintAInSOL(context.Background(), Request{
		MintA: solana.MustPublicKeyFromBase58("11111111111111111111111111111111"),
		MintB: solana.SolMint,
	}, decimal.NewFromInt(5))
	if err != nil || !priceInSOL.Equal(decimal.NewFromInt(5)) {
		t.Fatalf("expected passthrough for SOL mintB, got %s err=%v", priceInSOL, err)
	}

	priceInSOL, err = calc.priceOfMintAInSOL(context.Background(), Request{
		MintA: solana.MustPublicKeyFromBase58("11111111111111111111111111111111"),
		MintB: solana.MustPublicKeyFromBase58("SysvarRent111111111111111111111111111111111"),
	}, decimal.NewFromInt(5))
	if err != nil || !priceInSOL.IsZero() {
		t.Fatalf("expected zero without quote bridge, got %s err=%v", priceInSOL, err)
	}

	calc = NewCalculator(&mockRPC{}, &mockQuote{err: errors.New("quote error")}, &mockSupply{})
	if _, err := calc.priceOfMintAInSOL(context.Background(), Request{
		MintA: solana.MustPublicKeyFromBase58("11111111111111111111111111111111"),
		MintB: solana.MustPublicKeyFromBase58("SysvarRent111111111111111111111111111111111"),
	}, decimal.NewFromInt(5)); err == nil {
		t.Fatal("expected quote conversion error")
	}

	calc = NewCalculator(&mockRPC{}, &mockQuote{value: decimal.Zero}, &mockSupply{})
	priceInSOL, err = calc.priceOfMintAInSOL(context.Background(), Request{
		MintA: solana.MustPublicKeyFromBase58("11111111111111111111111111111111"),
		MintB: solana.MustPublicKeyFromBase58("SysvarRent111111111111111111111111111111111"),
	}, decimal.NewFromInt(5))
	if err != nil || !priceInSOL.IsZero() {
		t.Fatalf("expected zero on zero conversion, got %s err=%v", priceInSOL, err)
	}

	calc = NewCalculator(&mockRPC{}, &mockQuote{value: decimal.NewFromInt(3)}, &mockSupply{})
	priceInSOL, err = calc.priceOfMintAInSOL(context.Background(), Request{
		MintA: solana.MustPublicKeyFromBase58("11111111111111111111111111111111"),
		MintB: solana.MustPublicKeyFromBase58("SysvarRent111111111111111111111111111111111"),
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
		MintB: solana.MustPublicKeyFromBase58("SysvarRent111111111111111111111111111111111"),
	}, decimal.NewFromInt(7))
	if err != nil || !liqSOL.Equal(decimal.NewFromInt(3)) {
		t.Fatalf("expected quote converted liquidity, got %s err=%v", liqSOL, err)
	}

	calc = NewCalculator(&mockRPC{}, nil, &mockSupply{})
	liqSOL, err = calc.liquidityInSOL(context.Background(), Request{
		MintB: solana.MustPublicKeyFromBase58("SysvarRent111111111111111111111111111111111"),
	}, decimal.NewFromInt(7))
	if err != nil || !liqSOL.IsZero() {
		t.Fatalf("expected zero without quote bridge, got %s err=%v", liqSOL, err)
	}

	calc = NewCalculator(&mockRPC{}, &mockQuote{err: errors.New("quote error")}, &mockSupply{})
	if _, err := calc.liquidityInSOL(context.Background(), Request{
		MintB: solana.MustPublicKeyFromBase58("SysvarRent111111111111111111111111111111111"),
	}, decimal.NewFromInt(7)); err == nil {
		t.Fatal("expected quote liquidity conversion error")
	}
}
