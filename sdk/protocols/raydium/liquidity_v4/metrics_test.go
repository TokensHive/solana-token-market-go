package liquidity_v4

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

func testPubkey(seed byte) solana.PublicKey {
	data := make([]byte, 32)
	for i := range data {
		data[i] = seed
	}
	return solana.PublicKeyFromBytes(data)
}

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

func TestCompute_UsesPoolVaultAndOpenOrders(t *testing.T) {
	pool := testPubkey(1)
	mintA := solana.MustPublicKeyFromBase58("EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")
	poolQuoteMint := solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")
	baseVault := testPubkey(2)
	quoteVault := testPubkey(3)
	openOrders := testPubkey(4)
	marketProgram := testPubkey(5)

	poolData := make([]byte, liquidityStateV4MinDataSize)
	binary.LittleEndian.PutUint64(poolData[baseDecimalOffset:baseDecimalOffset+8], 6)
	binary.LittleEndian.PutUint64(poolData[quoteDecimalOffset:quoteDecimalOffset+8], 9)
	binary.LittleEndian.PutUint64(poolData[baseNeedTakePnlOffset:baseNeedTakePnlOffset+8], 5_000_000)
	binary.LittleEndian.PutUint64(poolData[quoteNeedTakePnlOffset:quoteNeedTakePnlOffset+8], 500_000_000)
	copy(poolData[baseMintOffset:baseMintOffset+32], mintA.Bytes())
	copy(poolData[quoteMintOffset:quoteMintOffset+32], poolQuoteMint.Bytes())
	copy(poolData[baseVaultOffset:baseVaultOffset+32], baseVault.Bytes())
	copy(poolData[quoteVaultOffset:quoteVaultOffset+32], quoteVault.Bytes())
	copy(poolData[openOrdersOffset:openOrdersOffset+32], openOrders.Bytes())
	copy(poolData[marketProgramIDOffset:marketProgramIDOffset+32], marketProgram.Bytes())

	baseVaultData := make([]byte, tokenAccountMinDataSize)
	quoteVaultData := make([]byte, tokenAccountMinDataSize)
	openOrdersData := make([]byte, openOrdersMinDataSize)
	binary.LittleEndian.PutUint64(baseVaultData[tokenAccountAmountOffset:tokenAccountAmountOffset+8], 100_000_000)
	binary.LittleEndian.PutUint64(quoteVaultData[tokenAccountAmountOffset:tokenAccountAmountOffset+8], 3_000_000_000)
	binary.LittleEndian.PutUint64(openOrdersData[openOrdersBaseTotalOffset:openOrdersBaseTotalOffset+8], 20_000_000)
	binary.LittleEndian.PutUint64(openOrdersData[openOrdersQuoteTotalOffset:openOrdersQuoteTotalOffset+8], 1_000_000_000)

	mockRPCClient := &mockRPC{
		accounts: map[string]*rpc.AccountInfo{
			pool.String():       {Address: pool, Exists: true, Data: poolData},
			baseVault.String():  {Address: baseVault, Exists: true, Data: baseVaultData},
			quoteVault.String(): {Address: quoteVault, Exists: true, Data: quoteVaultData},
			openOrders.String(): {Address: openOrders, Exists: true, Data: openOrdersData},
		},
	}

	calc := NewCalculator(mockRPCClient, nil, &mockSupply{
		total:  decimal.NewFromInt(1_000_000),
		circ:   decimal.NewFromInt(1_000),
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

	expectedBase := decimal.RequireFromString("115")
	expectedQuote := decimal.RequireFromString("3.5")
	expectedPrice := expectedQuote.Div(expectedBase)
	if !resp.PriceOfAInB.Equal(expectedPrice) {
		t.Fatalf("unexpected price_a_in_b: got=%s want=%s", resp.PriceOfAInB, expectedPrice)
	}
	if !resp.PriceOfAInSOL.Equal(expectedPrice) {
		t.Fatalf("unexpected price_a_in_sol: got=%s want=%s", resp.PriceOfAInSOL, expectedPrice)
	}
	if !resp.LiquidityInB.Equal(decimal.NewFromInt(7)) {
		t.Fatalf("unexpected liquidity_in_b: %s", resp.LiquidityInB)
	}
	if !resp.LiquidityInSOL.Equal(decimal.NewFromInt(7)) {
		t.Fatalf("unexpected liquidity_in_sol: %s", resp.LiquidityInSOL)
	}
	if !resp.MarketCapInSOL.Equal(expectedPrice.Mul(decimal.NewFromInt(1_000))) {
		t.Fatalf("unexpected market_cap_in_sol: %s", resp.MarketCapInSOL)
	}
	if !resp.FDVInSOL.Equal(expectedPrice.Mul(decimal.NewFromInt(1_000_000))) {
		t.Fatalf("unexpected fdv_in_sol: %s", resp.FDVInSOL)
	}
	if resp.Metadata["pool_market_program_id"] != marketProgram.String() {
		t.Fatalf("unexpected market program id metadata: %#v", resp.Metadata["pool_market_program_id"])
	}
	if mockRPCClient.getAccountCalls != 2 || mockRPCClient.getMultipleCalls != 1 {
		t.Fatalf("expected two getAccount calls (pool + fdv lookup) and one batch call, got account=%d multiple=%d", mockRPCClient.getAccountCalls, mockRPCClient.getMultipleCalls)
	}
}

func TestCompute_UsesPumpCurveTotalSupplyForFDV(t *testing.T) {
	pool := testPubkey(31)
	mintA := solana.MustPublicKeyFromBase58("2nCeHpECQvnMfzjU5fDMAKws1vBxMzxvWr6qqLpApump")
	poolQuoteMint := solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")
	baseVault := testPubkey(32)
	quoteVault := testPubkey(33)
	openOrders := testPubkey(34)

	poolData := make([]byte, liquidityStateV4MinDataSize)
	binary.LittleEndian.PutUint64(poolData[baseDecimalOffset:baseDecimalOffset+8], 6)
	binary.LittleEndian.PutUint64(poolData[quoteDecimalOffset:quoteDecimalOffset+8], 9)
	copy(poolData[baseMintOffset:baseMintOffset+32], mintA.Bytes())
	copy(poolData[quoteMintOffset:quoteMintOffset+32], poolQuoteMint.Bytes())
	copy(poolData[baseVaultOffset:baseVaultOffset+32], baseVault.Bytes())
	copy(poolData[quoteVaultOffset:quoteVaultOffset+32], quoteVault.Bytes())
	copy(poolData[openOrdersOffset:openOrdersOffset+32], openOrders.Bytes())

	baseVaultData := make([]byte, tokenAccountMinDataSize)
	quoteVaultData := make([]byte, tokenAccountMinDataSize)
	openOrdersData := make([]byte, openOrdersMinDataSize)
	binary.LittleEndian.PutUint64(baseVaultData[tokenAccountAmountOffset:tokenAccountAmountOffset+8], 100_000_000)
	binary.LittleEndian.PutUint64(quoteVaultData[tokenAccountAmountOffset:tokenAccountAmountOffset+8], 3_000_000_000)
	binary.LittleEndian.PutUint64(openOrdersData[openOrdersBaseTotalOffset:openOrdersBaseTotalOffset+8], 20_000_000)
	binary.LittleEndian.PutUint64(openOrdersData[openOrdersQuoteTotalOffset:openOrdersQuoteTotalOffset+8], 1_000_000_000)

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

	mockRPCClient := &mockRPC{
		accounts: map[string]*rpc.AccountInfo{
			pool.String():         {Address: pool, Exists: true, Data: poolData},
			baseVault.String():    {Address: baseVault, Exists: true, Data: baseVaultData},
			quoteVault.String():   {Address: quoteVault, Exists: true, Data: quoteVaultData},
			openOrders.String():   {Address: openOrders, Exists: true, Data: openOrdersData},
			bondingCurve.String(): {Address: bondingCurve, Exists: true, Owner: pumpProgramID, Data: bondingCurveData},
		},
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

func TestCompute_ValidationAndRequestErrors(t *testing.T) {
	if _, err := NewCalculator(nil, nil, &mockSupply{}).Compute(context.Background(), Request{}); err == nil {
		t.Fatal("expected rpc required validation error")
	}
	if _, err := NewCalculator(&mockRPC{}, nil, nil).Compute(context.Background(), Request{}); err == nil {
		t.Fatal("expected supply required validation error")
	}
	if _, err := NewCalculator(&mockRPC{}, nil, &mockSupply{}).Compute(context.Background(), Request{}); err == nil {
		t.Fatal("expected pool address validation error")
	}
	if _, err := NewCalculator(&mockRPC{}, nil, &mockSupply{}).Compute(context.Background(), Request{
		PoolAddress: solana.SolMint,
	}); err == nil {
		t.Fatal("expected mint validation error")
	}

	calc := NewCalculator(&mockRPC{getAccountErr: errors.New("rpc failed")}, nil, &mockSupply{})
	if _, err := calc.Compute(context.Background(), Request{
		PoolAddress: solana.SolMint,
		MintA:       solana.SolMint,
		MintB:       solana.SolMint,
	}); err == nil {
		t.Fatal("expected get account rpc error")
	}
}

func TestCompute_PoolAndBatchErrorPaths(t *testing.T) {
	pool := testPubkey(11)
	mintA := solana.MustPublicKeyFromBase58("EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")
	poolData := make([]byte, liquidityStateV4MinDataSize)
	binary.LittleEndian.PutUint64(poolData[baseDecimalOffset:baseDecimalOffset+8], 6)
	binary.LittleEndian.PutUint64(poolData[quoteDecimalOffset:quoteDecimalOffset+8], 9)
	copy(poolData[baseMintOffset:baseMintOffset+32], mintA.Bytes())
	copy(poolData[quoteMintOffset:quoteMintOffset+32], solana.SolMint.Bytes())

	calc := NewCalculator(&mockRPC{accounts: map[string]*rpc.AccountInfo{}}, nil, &mockSupply{})
	if _, err := calc.Compute(context.Background(), Request{
		PoolAddress: pool,
		MintA:       mintA,
		MintB:       solana.SolMint,
	}); err == nil {
		t.Fatal("expected missing pool account error")
	}

	calc = NewCalculator(&mockRPC{
		accounts: map[string]*rpc.AccountInfo{
			pool.String(): {Exists: true, Data: []byte{1, 2, 3}},
		},
	}, nil, &mockSupply{})
	if _, err := calc.Compute(context.Background(), Request{
		PoolAddress: pool,
		MintA:       mintA,
		MintB:       solana.SolMint,
	}); err == nil {
		t.Fatal("expected pool decode error")
	}

	calc = NewCalculator(&mockRPC{
		accounts: map[string]*rpc.AccountInfo{
			pool.String(): {Exists: true, Data: poolData},
		},
	}, nil, &mockSupply{})
	if _, err := calc.Compute(context.Background(), Request{
		PoolAddress: pool,
		MintA:       solana.MustPublicKeyFromBase58("Es9vMFrzaCER7HhMN7vY4sLhM1cBh73PvvrLpzjAUjWQ"),
		MintB:       solana.SolMint,
	}); err == nil {
		t.Fatal("expected pool mint mismatch error")
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
		MintB:       solana.SolMint,
	}); err == nil {
		t.Fatal("expected get multiple accounts error")
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
		MintB:       solana.SolMint,
	}); err == nil {
		t.Fatal("expected batch size error")
	}

	calc = NewCalculator(&mockRPC{
		accounts: map[string]*rpc.AccountInfo{
			pool.String(): {Exists: true, Data: poolData},
		},
		getMultipleFn: func([]solana.PublicKey) ([]*rpc.AccountInfo, error) {
			return []*rpc.AccountInfo{
				{Exists: true, Data: make([]byte, tokenAccountMinDataSize)},
				nil,
				{Exists: true, Data: make([]byte, openOrdersMinDataSize)},
			}, nil
		},
	}, nil, &mockSupply{})
	if _, err := calc.Compute(context.Background(), Request{
		PoolAddress: pool,
		MintA:       mintA,
		MintB:       solana.SolMint,
	}); err == nil {
		t.Fatal("expected missing account in batch")
	}
}

func TestCompute_DecodeAndConversionErrors(t *testing.T) {
	pool := testPubkey(21)
	mintA := solana.MustPublicKeyFromBase58("EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")
	mintB := solana.MustPublicKeyFromBase58("Es9vMFrzaCER7HhMN7vY4sLhM1cBh73PvvrLpzjAUjWQ")
	baseVault := testPubkey(22)
	quoteVault := testPubkey(23)
	openOrders := testPubkey(24)

	poolData := make([]byte, liquidityStateV4MinDataSize)
	binary.LittleEndian.PutUint64(poolData[baseDecimalOffset:baseDecimalOffset+8], 6)
	binary.LittleEndian.PutUint64(poolData[quoteDecimalOffset:quoteDecimalOffset+8], 6)
	copy(poolData[baseMintOffset:baseMintOffset+32], mintA.Bytes())
	copy(poolData[quoteMintOffset:quoteMintOffset+32], mintB.Bytes())
	copy(poolData[baseVaultOffset:baseVaultOffset+32], baseVault.Bytes())
	copy(poolData[quoteVaultOffset:quoteVaultOffset+32], quoteVault.Bytes())
	copy(poolData[openOrdersOffset:openOrdersOffset+32], openOrders.Bytes())

	baseVaultData := make([]byte, tokenAccountMinDataSize)
	quoteVaultData := make([]byte, tokenAccountMinDataSize)
	openOrdersData := make([]byte, openOrdersMinDataSize)
	binary.LittleEndian.PutUint64(baseVaultData[tokenAccountAmountOffset:tokenAccountAmountOffset+8], 10_000_000)
	binary.LittleEndian.PutUint64(quoteVaultData[tokenAccountAmountOffset:tokenAccountAmountOffset+8], 20_000_000)
	binary.LittleEndian.PutUint64(openOrdersData[openOrdersBaseTotalOffset:openOrdersBaseTotalOffset+8], 10_000_000)
	binary.LittleEndian.PutUint64(openOrdersData[openOrdersQuoteTotalOffset:openOrdersQuoteTotalOffset+8], 20_000_000)

	req := Request{
		PoolAddress: pool,
		MintA:       mintA,
		MintB:       mintB,
	}

	cases := []struct {
		name     string
		accounts []*rpc.AccountInfo
	}{
		{
			name: "base vault decode",
			accounts: []*rpc.AccountInfo{
				{Exists: true, Data: []byte{1}},
				{Exists: true, Data: quoteVaultData},
				{Exists: true, Data: openOrdersData},
			},
		},
		{
			name: "quote vault decode",
			accounts: []*rpc.AccountInfo{
				{Exists: true, Data: baseVaultData},
				{Exists: true, Data: []byte{1}},
				{Exists: true, Data: openOrdersData},
			},
		},
		{
			name: "open orders decode",
			accounts: []*rpc.AccountInfo{
				{Exists: true, Data: baseVaultData},
				{Exists: true, Data: quoteVaultData},
				{Exists: true, Data: []byte{1}},
			},
		},
	}
	for _, tc := range cases {
		calc := NewCalculator(&mockRPC{
			accounts: map[string]*rpc.AccountInfo{
				pool.String(): {Exists: true, Data: poolData},
			},
			getMultipleFn: func([]solana.PublicKey) ([]*rpc.AccountInfo, error) {
				return tc.accounts, nil
			},
		}, &mockQuote{value: decimal.NewFromInt(1)}, &mockSupply{})
		if _, err := calc.Compute(context.Background(), req); err == nil {
			t.Fatalf("expected error for case %s", tc.name)
		}
	}

	poolDataZero := make([]byte, len(poolData))
	copy(poolDataZero, poolData)
	binary.LittleEndian.PutUint64(poolDataZero[baseNeedTakePnlOffset:baseNeedTakePnlOffset+8], 100_000_000)
	binary.LittleEndian.PutUint64(poolDataZero[quoteNeedTakePnlOffset:quoteNeedTakePnlOffset+8], 200_000_000)
	calc := NewCalculator(&mockRPC{
		accounts: map[string]*rpc.AccountInfo{
			pool.String():       {Exists: true, Data: poolDataZero},
			baseVault.String():  {Exists: true, Data: baseVaultData},
			quoteVault.String(): {Exists: true, Data: quoteVaultData},
			openOrders.String(): {Exists: true, Data: openOrdersData},
		},
	}, nil, &mockSupply{})
	if _, err := calc.Compute(context.Background(), req); err == nil {
		t.Fatal("expected zero reserve error")
	}

	priceErrCalc := NewCalculator(&mockRPC{
		accounts: map[string]*rpc.AccountInfo{
			pool.String():       {Exists: true, Data: poolData},
			baseVault.String():  {Exists: true, Data: baseVaultData},
			quoteVault.String(): {Exists: true, Data: quoteVaultData},
			openOrders.String(): {Exists: true, Data: openOrdersData},
		},
	}, &mockQuote{err: errors.New("price quote error")}, &mockSupply{})
	if _, err := priceErrCalc.Compute(context.Background(), req); err == nil {
		t.Fatal("expected price quote error")
	}

	liqErrCalc := NewCalculator(&mockRPC{
		accounts: map[string]*rpc.AccountInfo{
			pool.String():       {Exists: true, Data: poolData},
			baseVault.String():  {Exists: true, Data: baseVaultData},
			quoteVault.String(): {Exists: true, Data: quoteVaultData},
			openOrders.String(): {Exists: true, Data: openOrdersData},
		},
	}, &sequenceQuote{
		values: []decimal.Decimal{decimal.NewFromInt(1)},
		errors: []error{nil, errors.New("liquidity quote error")},
	}, &mockSupply{
		total:  decimal.NewFromInt(1),
		circ:   decimal.NewFromInt(1),
		method: "ok",
	})
	if _, err := liqErrCalc.Compute(context.Background(), req); err == nil {
		t.Fatal("expected liquidity quote error")
	}

	supplyErrCalc := NewCalculator(&mockRPC{
		accounts: map[string]*rpc.AccountInfo{
			pool.String():       {Exists: true, Data: poolData},
			baseVault.String():  {Exists: true, Data: baseVaultData},
			quoteVault.String(): {Exists: true, Data: quoteVaultData},
			openOrders.String(): {Exists: true, Data: openOrdersData},
		},
	}, &mockQuote{value: decimal.NewFromInt(1)}, &mockSupply{err: errors.New("supply error")})
	if _, err := supplyErrCalc.Compute(context.Background(), req); err == nil {
		t.Fatal("expected supply error")
	}
}

func TestDecodeHelpersAndMathHelpers(t *testing.T) {
	if _, err := decodePoolState([]byte{1, 2, 3}); err == nil {
		t.Fatal("expected decode pool short data error")
	}
	invalidDecimals := make([]byte, liquidityStateV4MinDataSize)
	binary.LittleEndian.PutUint64(invalidDecimals[baseDecimalOffset:baseDecimalOffset+8], 300)
	binary.LittleEndian.PutUint64(invalidDecimals[quoteDecimalOffset:quoteDecimalOffset+8], 6)
	if _, err := decodePoolState(invalidDecimals); err == nil {
		t.Fatal("expected invalid decimals error")
	}
	if _, err := decodeTokenAmount([]byte{1}); err == nil {
		t.Fatal("expected token amount short data error")
	}
	if _, _, err := decodeOpenOrdersTotals([]byte{1}); err == nil {
		t.Fatal("expected open orders short data error")
	}
	openOrdersData := make([]byte, 109)
	binary.LittleEndian.PutUint64(openOrdersData[85:93], 123)
	binary.LittleEndian.PutUint64(openOrdersData[101:109], 456)
	baseTotal, quoteTotal, err := decodeOpenOrdersTotals(openOrdersData)
	if err != nil {
		t.Fatalf("unexpected decode open orders totals error: %v", err)
	}
	if baseTotal != 123 || quoteTotal != 456 {
		t.Fatalf("unexpected open orders totals: base=%d quote=%d", baseTotal, quoteTotal)
	}

	req := Request{
		MintA: solana.SolMint,
		MintB: solana.MustPublicKeyFromBase58("11111111111111111111111111111111"),
	}
	snapshot := &reserveSnapshot{
		baseMint:     req.MintA,
		quoteMint:    req.MintB,
		baseReserve:  decimal.NewFromInt(4),
		quoteReserve: decimal.NewFromInt(8),
	}
	if got := priceOfMintAInMintB(req, snapshot); !got.Equal(decimal.NewFromInt(2)) {
		t.Fatalf("unexpected price: %s", got)
	}
	if got := liquidityInMintB(req, snapshot); !got.Equal(decimal.NewFromInt(16)) {
		t.Fatalf("unexpected liquidity in quote mint: %s", got)
	}

	reversed := Request{MintA: req.MintB, MintB: req.MintA}
	if got := priceOfMintAInMintB(reversed, snapshot); !got.Equal(decimal.RequireFromString("0.5")) {
		t.Fatalf("unexpected reversed price: %s", got)
	}
	if got := liquidityInMintB(reversed, snapshot); !got.Equal(decimal.NewFromInt(8)) {
		t.Fatalf("unexpected reversed liquidity: %s", got)
	}
	if got := priceOfMintAInMintB(req, nil); !got.IsZero() {
		t.Fatalf("expected zero price for nil snapshot, got %s", got)
	}
	if got := liquidityInMintB(req, nil); !got.IsZero() {
		t.Fatalf("expected zero liquidity for nil snapshot, got %s", got)
	}
	if got := priceOfMintAInMintB(Request{
		MintA: solana.MustPublicKeyFromBase58("SysvarRent111111111111111111111111111111111"),
		MintB: solana.MustPublicKeyFromBase58("SysvarC1ock11111111111111111111111111111111"),
	}, snapshot); !got.IsZero() {
		t.Fatalf("expected zero for unmatched pair, got %s", got)
	}
	if got := liquidityInMintB(Request{
		MintA: solana.MustPublicKeyFromBase58("SysvarRent111111111111111111111111111111111"),
		MintB: solana.MustPublicKeyFromBase58("SysvarC1ock11111111111111111111111111111111"),
	}, snapshot); !got.IsZero() {
		t.Fatalf("expected zero liquidity for unmatched pair, got %s", got)
	}

	if got := decimalFromU64(12345, 2); got.String() != "123.45" {
		t.Fatalf("unexpected decimal conversion: %s", got)
	}
}

func TestPoolMatchesRequest(t *testing.T) {
	token := solana.MustPublicKeyFromBase58("11111111111111111111111111111111")
	other := solana.MustPublicKeyFromBase58("SysvarRent111111111111111111111111111111111")
	state := poolState{
		baseMint:  solana.SolMint,
		quoteMint: token,
	}
	if !poolMatchesRequest(Request{MintA: solana.SolMint, MintB: token}, state) {
		t.Fatal("expected direct request orientation to match pool")
	}
	if !poolMatchesRequest(Request{MintA: token, MintB: solana.SolMint}, state) {
		t.Fatal("expected inverted request orientation to match pool")
	}
	if poolMatchesRequest(Request{MintA: other, MintB: token}, state) {
		t.Fatal("expected mismatched mint request to fail")
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

func TestMintEquivalence(t *testing.T) {
	if !mintsEquivalent(solana.SolMint, solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")) {
		t.Fatal("expected native and wrapped SOL mints to be equivalent")
	}
	if mintsEquivalent(solana.SolMint, solana.MustPublicKeyFromBase58("11111111111111111111111111111111")) {
		t.Fatal("expected non-SOL mint to not be equivalent to SOL")
	}
}
