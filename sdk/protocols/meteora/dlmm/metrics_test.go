package dlmm

import (
	"context"
	"encoding/binary"
	"errors"
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

func makePoolData(activeID int32, binStep uint16, tokenXMint, tokenYMint, reserveX, reserveY solana.PublicKey) []byte {
	data := make([]byte, poolMinDataSize)
	binary.LittleEndian.PutUint32(data[activeIDOffset:activeIDOffset+4], uint32(activeID))
	binary.LittleEndian.PutUint16(data[binStepOffset:binStepOffset+2], binStep)
	data[statusOffset] = 1
	copy(data[tokenXMintOffset:tokenXMintOffset+32], tokenXMint.Bytes())
	copy(data[tokenYMintOffset:tokenYMintOffset+32], tokenYMint.Bytes())
	copy(data[reserveXOffset:reserveXOffset+32], reserveX.Bytes())
	copy(data[reserveYOffset:reserveYOffset+32], reserveY.Bytes())
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
	mintA := solana.MustPublicKeyFromBase58("2nCeHpECQvnMfzjU5fDMAKws1vBxMzxvWr6qqLpApump")
	poolQuoteMint := solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")
	reserveX := testPubkey(2)
	reserveY := testPubkey(3)

	poolData := makePoolData(0, 80, mintA, poolQuoteMint, reserveX, reserveY)
	mockRPCClient := &mockRPC{
		accounts: map[string]*rpc.AccountInfo{
			pool.String():          {Address: pool, Owner: dlmmProgramID, Exists: true, Data: poolData},
			reserveX.String():      {Address: reserveX, Exists: true, Data: makeTokenAccountData(2_000_000)},
			reserveY.String():      {Address: reserveY, Exists: true, Data: makeTokenAccountData(5_000_000_000)},
			mintA.String():         {Address: mintA, Exists: true, Data: makeMintData(6)},
			poolQuoteMint.String(): {Address: poolQuoteMint, Exists: true, Data: makeMintData(9)},
		},
	}
	calc := NewCalculator(mockRPCClient, nil, &mockSupply{
		total:  decimal.NewFromInt(1_000_000),
		circ:   decimal.NewFromInt(800_000),
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
	expectedPrice := decimal.RequireFromString("0.001")
	if !resp.PriceOfAInB.Equal(expectedPrice) {
		t.Fatalf("unexpected price_a_in_b: %s", resp.PriceOfAInB)
	}
	if got := resp.LiquidityInB.String(); got != "10" {
		t.Fatalf("unexpected liquidity_in_b: %s", got)
	}
	expectedMarketCap := expectedPrice.Mul(decimal.NewFromInt(800_000))
	if !resp.MarketCapInSOL.Equal(expectedMarketCap) {
		t.Fatalf("unexpected market cap: %s", resp.MarketCapInSOL)
	}
	expectedFDV := expectedPrice.Mul(decimal.NewFromInt(1_000_000))
	if !resp.FDVInSOL.Equal(expectedFDV) {
		t.Fatalf("unexpected fdv: %s", resp.FDVInSOL)
	}
	if resp.Metadata["pool_active_id"] != int32(0) {
		t.Fatalf("unexpected active id metadata: %#v", resp.Metadata["pool_active_id"])
	}
	if mockRPCClient.getAccountCalls != 2 || mockRPCClient.getMultipleCalls != 1 {
		t.Fatalf("expected two getAccount calls and one batch call, got account=%d multiple=%d", mockRPCClient.getAccountCalls, mockRPCClient.getMultipleCalls)
	}
}

func TestCompute_UsesPumpCurveTotalSupplyForFDV(t *testing.T) {
	pool := testPubkey(11)
	mintA := solana.MustPublicKeyFromBase58("2nCeHpECQvnMfzjU5fDMAKws1vBxMzxvWr6qqLpApump")
	poolQuoteMint := solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")
	reserveX := testPubkey(12)
	reserveY := testPubkey(13)

	poolData := makePoolData(0, 80, mintA, poolQuoteMint, reserveX, reserveY)
	mockRPCClient := &mockRPC{
		accounts: map[string]*rpc.AccountInfo{
			pool.String():          {Address: pool, Owner: dlmmProgramID, Exists: true, Data: poolData},
			reserveX.String():      {Address: reserveX, Exists: true, Data: makeTokenAccountData(2_000_000)},
			reserveY.String():      {Address: reserveY, Exists: true, Data: makeTokenAccountData(5_000_000_000)},
			mintA.String():         {Address: mintA, Exists: true, Data: makeMintData(6)},
			poolQuoteMint.String(): {Address: poolQuoteMint, Exists: true, Data: makeMintData(9)},
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
			solana.SolMint.String(): {Exists: true, Owner: dlmmProgramID, Data: []byte{1}},
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
	pool := testPubkey(21)
	mintA := solana.MustPublicKeyFromBase58("2nCeHpECQvnMfzjU5fDMAKws1vBxMzxvWr6qqLpApump")
	mintB := solana.MustPublicKeyFromBase58("Es9vMFrzaCER7HhMN7vY4sLhM1cBh73PvvrLpzjAUjWQ")
	reserveX := testPubkey(22)
	reserveY := testPubkey(23)
	poolData := makePoolData(0, 80, mintA, mintB, reserveX, reserveY)

	calc := NewCalculator(&mockRPC{
		accounts: map[string]*rpc.AccountInfo{
			pool.String(): {Exists: true, Owner: dlmmProgramID, Data: poolData},
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
			pool.String(): {Exists: true, Owner: dlmmProgramID, Data: poolData},
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
			pool.String(): {Exists: true, Owner: dlmmProgramID, Data: poolData},
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
			pool.String(): {Exists: true, Owner: dlmmProgramID, Data: poolData},
		},
		getMultipleFn: func([]solana.PublicKey) ([]*rpc.AccountInfo, error) {
			return []*rpc.AccountInfo{
				{Exists: true, Data: makeTokenAccountData(1)},
				nil,
				{Exists: true, Data: makeMintData(6)},
				{Exists: true, Data: makeMintData(6)},
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
		acc2 []byte
		acc3 []byte
	}{
		{name: "decode reserveX", acc0: []byte{1}, acc1: makeTokenAccountData(1), acc2: makeMintData(6), acc3: makeMintData(6)},
		{name: "decode reserveY", acc0: makeTokenAccountData(1), acc1: []byte{1}, acc2: makeMintData(6), acc3: makeMintData(6)},
		{name: "decode mintX", acc0: makeTokenAccountData(1), acc1: makeTokenAccountData(1), acc2: []byte{1}, acc3: makeMintData(6)},
		{name: "decode mintY", acc0: makeTokenAccountData(1), acc1: makeTokenAccountData(1), acc2: makeMintData(6), acc3: []byte{1}},
	}
	for _, tc := range cases {
		calc = NewCalculator(&mockRPC{
			accounts: map[string]*rpc.AccountInfo{
				pool.String(): {Exists: true, Owner: dlmmProgramID, Data: poolData},
			},
			getMultipleFn: func([]solana.PublicKey) ([]*rpc.AccountInfo, error) {
				return []*rpc.AccountInfo{
					{Exists: true, Data: tc.acc0},
					{Exists: true, Data: tc.acc1},
					{Exists: true, Data: tc.acc2},
					{Exists: true, Data: tc.acc3},
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
		accounts: map[string]*rpc.AccountInfo{
			pool.String():     {Exists: true, Owner: dlmmProgramID, Data: poolData},
			reserveX.String(): {Exists: true, Data: makeTokenAccountData(0)},
			reserveY.String(): {Exists: true, Data: makeTokenAccountData(0)},
			mintA.String():    {Exists: true, Data: makeMintData(6)},
			mintB.String():    {Exists: true, Data: makeMintData(6)},
		},
	}, nil, &mockSupply{})
	if _, err := calc.Compute(context.Background(), Request{
		PoolAddress: pool,
		MintA:       mintA,
		MintB:       mintB,
	}); err == nil {
		t.Fatal("expected zero reserve error")
	}

	quoteErrCalc := NewCalculator(&mockRPC{
		accounts: map[string]*rpc.AccountInfo{
			pool.String():     {Exists: true, Owner: dlmmProgramID, Data: makePoolData(0, 80, mintA, mintB, reserveX, reserveY)},
			reserveX.String(): {Exists: true, Data: makeTokenAccountData(1_000_000)},
			reserveY.String(): {Exists: true, Data: makeTokenAccountData(2_000_000)},
			mintA.String():    {Exists: true, Data: makeMintData(6)},
			mintB.String():    {Exists: true, Data: makeMintData(6)},
		},
	}, &mockQuote{err: errors.New("quote error")}, &mockSupply{})
	if _, err := quoteErrCalc.Compute(context.Background(), Request{
		PoolAddress: pool,
		MintA:       mintA,
		MintB:       mintB,
	}); err == nil {
		t.Fatal("expected quote conversion error")
	}

	liqErrCalc := NewCalculator(&mockRPC{
		accounts: map[string]*rpc.AccountInfo{
			pool.String():     {Exists: true, Owner: dlmmProgramID, Data: makePoolData(0, 80, mintA, mintB, reserveX, reserveY)},
			reserveX.String(): {Exists: true, Data: makeTokenAccountData(1_000_000)},
			reserveY.String(): {Exists: true, Data: makeTokenAccountData(2_000_000)},
			mintA.String():    {Exists: true, Data: makeMintData(6)},
			mintB.String():    {Exists: true, Data: makeMintData(6)},
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
			pool.String():     {Exists: true, Owner: dlmmProgramID, Data: makePoolData(0, 80, mintA, mintB, reserveX, reserveY)},
			reserveX.String(): {Exists: true, Data: makeTokenAccountData(1_000_000)},
			reserveY.String(): {Exists: true, Data: makeTokenAccountData(2_000_000)},
			mintA.String():    {Exists: true, Data: makeMintData(6)},
			mintB.String():    {Exists: true, Data: makeMintData(6)},
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
	if _, err := decodeMintDecimals([]byte{1}); err == nil {
		t.Fatal("expected decode mint decimals short data error")
	}

	if got := priceTokenXInTokenY(0, 80, 6, 9); !got.Equal(decimal.RequireFromString("0.001")) {
		t.Fatalf("unexpected price for active=0: %s", got)
	}
	if got := priceTokenXInTokenY(2_000_000_000, 65535, 6, 6); !got.IsZero() {
		t.Fatalf("expected zero on inf/NaN price, got %s", got)
	}

	req := Request{
		MintA: solana.SolMint,
		MintB: solana.MustPublicKeyFromBase58("11111111111111111111111111111111"),
	}
	state := poolState{
		tokenXMint: req.MintA,
		tokenYMint: req.MintB,
	}
	if got := priceOfMintAInMintB(req, state, decimal.NewFromInt(2)); !got.Equal(decimal.NewFromInt(2)) {
		t.Fatalf("unexpected direct price: %s", got)
	}
	if got := priceOfMintAInMintB(Request{MintA: req.MintB, MintB: req.MintA}, state, decimal.NewFromInt(2)); !got.Equal(decimal.RequireFromString("0.5")) {
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
		tokenXMint:    req.MintA,
		tokenYMint:    req.MintB,
		tokenXReserve: decimal.NewFromInt(4),
		tokenYReserve: decimal.NewFromInt(8),
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
