package pumpswap_amm

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

type mockRPC struct {
	accounts         map[string]*rpc.AccountInfo
	getAccountCalls  int
	getMultipleCalls int
	getAccountErr    error
	getMultipleErr   error
	getMultipleFn    func(addresses []solana.PublicKey) ([]*rpc.AccountInfo, error)
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

func TestCompute_UsesDirectPoolAndVaultAccounts(t *testing.T) {
	pool := solana.MustPublicKeyFromBase58("HnhpJPJgBG2KwniMTNW8cVBHvk1hFog3RC3kjnyc23tD")
	mintA := solana.MustPublicKeyFromBase58("9BHt7aq3DFCb74kZjPY5epgVtsWKCeYX1tUWxYwDpump")
	poolQuoteMint := solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")
	mintB := solana.SolMint
	baseVault := solana.MustPublicKeyFromBase58("9B8xXGabWeEfCLzwHAwwbjZjQamiFsh7WC8Y5bkvPbqP")
	quoteVault := solana.MustPublicKeyFromBase58("53WjyeT21YNnyP6ZNuMemvBaYvZfvijDdV6RmLb8Te9c")

	poolData := make([]byte, poolExpectedMinDataSize)
	copy(poolData[poolBaseMintOffset:poolBaseMintOffset+32], mintA.Bytes())
	copy(poolData[poolQuoteMintOffset:poolQuoteMintOffset+32], poolQuoteMint.Bytes())
	copy(poolData[poolBaseVaultOffset:poolBaseVaultOffset+32], baseVault.Bytes())
	copy(poolData[poolQuoteVaultOffset:poolQuoteVaultOffset+32], quoteVault.Bytes())

	baseVaultData := make([]byte, tokenAccountMinDataSize)
	quoteVaultData := make([]byte, tokenAccountMinDataSize)
	binary.LittleEndian.PutUint64(baseVaultData[tokenAmountOffset:tokenAmountOffset+8], 1_000_000_000)
	binary.LittleEndian.PutUint64(quoteVaultData[tokenAmountOffset:tokenAmountOffset+8], 5_000_000_000)

	baseMintData := make([]byte, mintAccountMinDataSize)
	quoteMintData := make([]byte, mintAccountMinDataSize)
	baseMintData[mintDecimalsOffset] = 6
	quoteMintData[mintDecimalsOffset] = 9

	mockRPCClient := &mockRPC{
		accounts: map[string]*rpc.AccountInfo{
			pool.String():          {Address: pool, Exists: true, Data: poolData},
			baseVault.String():     {Address: baseVault, Exists: true, Data: baseVaultData},
			quoteVault.String():    {Address: quoteVault, Exists: true, Data: quoteVaultData},
			mintA.String():         {Address: mintA, Exists: true, Data: baseMintData},
			poolQuoteMint.String(): {Address: poolQuoteMint, Exists: true, Data: quoteMintData},
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
		MintB:       mintB,
	})
	if err != nil {
		t.Fatalf("compute failed: %v", err)
	}
	if got := resp.PriceOfAInB.String(); got != "0.005" {
		t.Fatalf("unexpected price_a_in_b: %s", got)
	}
	if got := resp.PriceOfAInSOL.String(); got != "0.005" {
		t.Fatalf("unexpected price_a_in_sol: %s", got)
	}
	if got := resp.LiquidityInB.String(); got != "10" {
		t.Fatalf("unexpected liquidity_in_b: %s", got)
	}
	if got := resp.LiquidityInSOL.String(); got != "10" {
		t.Fatalf("unexpected liquidity_in_sol: %s", got)
	}
	if got := resp.MarketCapInSOL.String(); got != "4000" {
		t.Fatalf("unexpected market_cap_in_sol: %s", got)
	}
	if got := resp.FDVInSOL.String(); got != "5000" {
		t.Fatalf("unexpected fdv_in_sol: %s", got)
	}
	if mockRPCClient.getAccountCalls != 2 {
		t.Fatalf("expected two getAccount calls (pool + fdv lookup), got %d", mockRPCClient.getAccountCalls)
	}
	if mockRPCClient.getMultipleCalls != 1 {
		t.Fatalf("expected one getMultipleAccounts call, got %d", mockRPCClient.getMultipleCalls)
	}
}

func TestCompute_FailsWhenRequestPairDoesNotMatchPool(t *testing.T) {
	pool := solana.MustPublicKeyFromBase58("HnhpJPJgBG2KwniMTNW8cVBHvk1hFog3RC3kjnyc23tD")
	mintA := solana.MustPublicKeyFromBase58("9BHt7aq3DFCb74kZjPY5epgVtsWKCeYX1tUWxYwDpump")
	mintB := solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")
	poolData := make([]byte, poolExpectedMinDataSize)
	copy(poolData[poolBaseMintOffset:poolBaseMintOffset+32], mintA.Bytes())
	copy(poolData[poolQuoteMintOffset:poolQuoteMintOffset+32], mintB.Bytes())

	mockRPCClient := &mockRPC{
		accounts: map[string]*rpc.AccountInfo{
			pool.String(): {Address: pool, Exists: true, Data: poolData},
		},
	}
	calc := NewCalculator(mockRPCClient, nil, &mockSupply{
		total: decimal.NewFromInt(1),
		circ:  decimal.NewFromInt(1),
	})

	_, err := calc.Compute(context.Background(), Request{
		PoolAddress: pool,
		MintA:       solana.MustPublicKeyFromBase58("6iA73gWCKkLWKbVr8rgibV57MMRxzsaqS9cWpgKBpump"),
		MintB:       mintB,
	})
	if err == nil {
		t.Fatal("expected pair mismatch error")
	}
}

func TestDecodePoolState_InvalidLength(t *testing.T) {
	_, err := decodePoolState([]byte{1, 2, 3})
	if err == nil {
		t.Fatal("expected decodePoolState to fail for short data")
	}
}

func TestComputeValidationAndErrors(t *testing.T) {
	if _, err := NewCalculator(nil, nil, &mockSupply{}).Compute(context.Background(), Request{}); err == nil {
		t.Fatal("expected rpc required validation error")
	}
	if _, err := NewCalculator(&mockRPC{}, nil, nil).Compute(context.Background(), Request{}); err == nil {
		t.Fatal("expected supply required validation error")
	}
	if _, err := NewCalculator(&mockRPC{}, nil, &mockSupply{}).Compute(context.Background(), Request{}); err == nil {
		t.Fatal("expected pool address required validation error")
	}
	if _, err := NewCalculator(&mockRPC{}, nil, &mockSupply{}).Compute(context.Background(), Request{
		PoolAddress: solana.SolMint,
	}); err == nil {
		t.Fatal("expected mint validation error")
	}

	calc := NewCalculator(&mockRPC{getAccountErr: errors.New("rpc error")}, nil, &mockSupply{})
	if _, err := calc.Compute(context.Background(), Request{
		PoolAddress: solana.SolMint,
		MintA:       solana.SolMint,
		MintB:       solana.SolMint,
	}); err == nil {
		t.Fatal("expected rpc error")
	}

	calc = NewCalculator(&mockRPC{accounts: map[string]*rpc.AccountInfo{}}, nil, &mockSupply{})
	if _, err := calc.Compute(context.Background(), Request{
		PoolAddress: solana.SolMint,
		MintA:       solana.SolMint,
		MintB:       solana.SolMint,
	}); err == nil {
		t.Fatal("expected missing pool account error")
	}
}

func TestComputeAccountDecodeAndSupplyErrors(t *testing.T) {
	pool := solana.MustPublicKeyFromBase58("HnhpJPJgBG2KwniMTNW8cVBHvk1hFog3RC3kjnyc23tD")
	mint := solana.MustPublicKeyFromBase58("9BHt7aq3DFCb74kZjPY5epgVtsWKCeYX1tUWxYwDpump")
	poolData := make([]byte, poolExpectedMinDataSize)
	copy(poolData[poolBaseMintOffset:poolBaseMintOffset+32], mint.Bytes())
	copy(poolData[poolQuoteMintOffset:poolQuoteMintOffset+32], solana.SolMint.Bytes())

	calc := NewCalculator(&mockRPC{
		accounts: map[string]*rpc.AccountInfo{
			pool.String(): {Exists: true, Data: poolData},
		},
		getMultipleErr: errors.New("batch fail"),
	}, nil, &mockSupply{})
	_, err := calc.Compute(context.Background(), Request{
		PoolAddress: pool,
		MintA:       mint,
		MintB:       solana.SolMint,
	})
	if err == nil {
		t.Fatal("expected get multiple accounts error")
	}

	calc = NewCalculator(&mockRPC{
		accounts: map[string]*rpc.AccountInfo{
			pool.String(): {Exists: true, Data: poolData},
		},
		getMultipleFn: func([]solana.PublicKey) ([]*rpc.AccountInfo, error) {
			return []*rpc.AccountInfo{
				{Exists: true, Data: []byte{1}},
				{Exists: true, Data: []byte{1}},
				{Exists: true, Data: []byte{1}},
			}, nil
		},
	}, nil, &mockSupply{})
	_, err = calc.Compute(context.Background(), Request{
		PoolAddress: pool,
		MintA:       mint,
		MintB:       solana.SolMint,
	})
	if err == nil {
		t.Fatal("expected account batch shape error")
	}

	baseVault := solana.MustPublicKeyFromBase58("9B8xXGabWeEfCLzwHAwwbjZjQamiFsh7WC8Y5bkvPbqP")
	quoteVault := solana.MustPublicKeyFromBase58("53WjyeT21YNnyP6ZNuMemvBaYvZfvijDdV6RmLb8Te9c")
	copy(poolData[poolBaseVaultOffset:poolBaseVaultOffset+32], baseVault.Bytes())
	copy(poolData[poolQuoteVaultOffset:poolQuoteVaultOffset+32], quoteVault.Bytes())
	baseVaultData := make([]byte, tokenAccountMinDataSize)
	quoteVaultData := make([]byte, tokenAccountMinDataSize)
	baseMintData := make([]byte, mintAccountMinDataSize)
	quoteMintData := make([]byte, mintAccountMinDataSize)
	baseMintData[mintDecimalsOffset] = 6
	quoteMintData[mintDecimalsOffset] = 9

	calc = NewCalculator(&mockRPC{
		accounts: map[string]*rpc.AccountInfo{
			pool.String():           {Exists: true, Data: poolData},
			baseVault.String():      {Exists: true, Data: baseVaultData},
			quoteVault.String():     {Exists: true, Data: quoteVaultData},
			mint.String():           {Exists: true, Data: baseMintData},
			solana.SolMint.String(): {Exists: true, Data: quoteMintData},
		},
	}, nil, &mockSupply{})
	_, err = calc.Compute(context.Background(), Request{
		PoolAddress: pool,
		MintA:       mint,
		MintB:       solana.SolMint,
	})
	if err == nil {
		t.Fatal("expected zero reserves error")
	}

	binary.LittleEndian.PutUint64(baseVaultData[tokenAmountOffset:tokenAmountOffset+8], 10)
	binary.LittleEndian.PutUint64(quoteVaultData[tokenAmountOffset:tokenAmountOffset+8], 20)
	calc = NewCalculator(&mockRPC{
		accounts: map[string]*rpc.AccountInfo{
			pool.String():           {Exists: true, Data: poolData},
			baseVault.String():      {Exists: true, Data: baseVaultData},
			quoteVault.String():     {Exists: true, Data: quoteVaultData},
			mint.String():           {Exists: true, Data: baseMintData},
			solana.SolMint.String(): {Exists: true, Data: quoteMintData},
		},
	}, nil, &mockSupply{err: errors.New("supply error")})
	_, err = calc.Compute(context.Background(), Request{
		PoolAddress: pool,
		MintA:       mint,
		MintB:       solana.SolMint,
	})
	if err == nil {
		t.Fatal("expected supply error")
	}
}

func TestHelperFunctions(t *testing.T) {
	if _, err := decodeTokenAmount([]byte{1, 2, 3}); err == nil {
		t.Fatal("expected decode token amount length error")
	}
	if _, err := decodeMintDecimals([]byte{1, 2, 3}); err == nil {
		t.Fatal("expected decode mint decimals length error")
	}

	req := Request{
		MintA: solana.SolMint,
		MintB: solana.MustPublicKeyFromBase58("11111111111111111111111111111111"),
	}
	snapshot := &reserveSnapshot{
		BaseMint:     req.MintA.String(),
		BaseReserve:  decimal.NewFromInt(4),
		QuoteMint:    req.MintB.String(),
		QuoteReserve: decimal.NewFromInt(8),
	}
	if got := priceOfMintAInMintB(req, snapshot); !got.Equal(decimal.NewFromInt(2)) {
		t.Fatalf("unexpected price: %s", got)
	}
	if got := liquidityInMintB(req, snapshot); !got.Equal(decimal.NewFromInt(16)) {
		t.Fatalf("unexpected liquidity in quote: %s", got)
	}

	otherReq := Request{
		MintA: solana.MustPublicKeyFromBase58("SysvarRent111111111111111111111111111111111"),
		MintB: solana.MustPublicKeyFromBase58("SysvarC1ock11111111111111111111111111111111"),
	}
	if got := priceOfMintAInMintB(otherReq, snapshot); !got.IsZero() {
		t.Fatalf("expected unmatched price to be zero, got %s", got)
	}
	if got := liquidityInMintB(otherReq, snapshot); !got.IsZero() {
		t.Fatalf("expected unmatched liquidity to be zero, got %s", got)
	}
	if got := priceOfMintAInMintB(req, nil); !got.IsZero() {
		t.Fatalf("expected zero price for nil snapshot, got %s", got)
	}
	if got := liquidityInMintB(req, nil); !got.IsZero() {
		t.Fatalf("expected zero liquidity for nil snapshot, got %s", got)
	}

	reversedReq := Request{MintA: req.MintB, MintB: req.MintA}
	if got := priceOfMintAInMintB(reversedReq, snapshot); !got.Equal(decimal.RequireFromString("0.5")) {
		t.Fatalf("expected reversed pair price 0.5, got %s", got)
	}
	if got := liquidityInMintB(reversedReq, snapshot); !got.Equal(decimal.NewFromInt(8)) {
		t.Fatalf("expected reversed liquidity in base reserve x2, got %s", got)
	}

	if got := decimalFromU64(12345, 2); got.String() != "123.45" {
		t.Fatalf("unexpected decimal conversion: %s", got)
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
		t.Fatalf("expected SOL mintB passthrough price, got %s err=%v", priceInSOL, err)
	}

	priceInSOL, err = calc.priceOfMintAInSOL(context.Background(), Request{
		MintA: solana.MustPublicKeyFromBase58("11111111111111111111111111111111"),
		MintB: solana.MustPublicKeyFromBase58("SysvarRent111111111111111111111111111111111"),
	}, decimal.NewFromInt(5))
	if err != nil || !priceInSOL.IsZero() {
		t.Fatalf("expected zero without quote bridge, got %s err=%v", priceInSOL, err)
	}

	calc = NewCalculator(&mockRPC{}, &mockQuote{err: errors.New("quote error")}, &mockSupply{})
	if _, err = calc.priceOfMintAInSOL(context.Background(), Request{
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
		t.Fatalf("expected zero on zero quote conversion, got %s err=%v", priceInSOL, err)
	}

	calc = NewCalculator(&mockRPC{}, &mockQuote{value: decimal.NewFromInt(3)}, &mockSupply{})
	priceInSOL, err = calc.priceOfMintAInSOL(context.Background(), Request{
		MintA: solana.MustPublicKeyFromBase58("11111111111111111111111111111111"),
		MintB: solana.MustPublicKeyFromBase58("SysvarRent111111111111111111111111111111111"),
	}, decimal.NewFromInt(5))
	if err != nil || !priceInSOL.Equal(decimal.NewFromInt(15)) {
		t.Fatalf("expected quote conversion price, got %s err=%v", priceInSOL, err)
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
		t.Fatalf("expected quote bridge conversion, got %s err=%v", liqSOL, err)
	}

	calc = NewCalculator(&mockRPC{}, nil, &mockSupply{})
	liqSOL, err = calc.liquidityInSOL(context.Background(), Request{
		MintB: solana.MustPublicKeyFromBase58("SysvarRent111111111111111111111111111111111"),
	}, decimal.NewFromInt(7))
	if err != nil || !liqSOL.IsZero() {
		t.Fatalf("expected zero without bridge, got %s err=%v", liqSOL, err)
	}

	calc = NewCalculator(&mockRPC{}, &mockQuote{err: errors.New("quote error")}, &mockSupply{})
	if _, err = calc.liquidityInSOL(context.Background(), Request{
		MintB: solana.MustPublicKeyFromBase58("SysvarRent111111111111111111111111111111111"),
	}, decimal.NewFromInt(7)); err == nil {
		t.Fatal("expected quote bridge liquidity conversion error")
	}
}

func TestMintComparisonHelpers(t *testing.T) {
	if !mintsEquivalent(solana.SolMint, solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")) {
		t.Fatal("expected SOL and wrapped SOL to be equivalent")
	}
	if mintsEquivalent(solana.SolMint, solana.MustPublicKeyFromBase58("11111111111111111111111111111111")) {
		t.Fatal("expected SOL and non-SOL to be non-equivalent")
	}
	if mintStringMatchesReq("not-a-pubkey", solana.SolMint) {
		t.Fatal("expected invalid mint string match to be false")
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

func TestComputeDecodeAndAccountStateErrors(t *testing.T) {
	pool := solana.MustPublicKeyFromBase58("HnhpJPJgBG2KwniMTNW8cVBHvk1hFog3RC3kjnyc23tD")
	mint := solana.MustPublicKeyFromBase58("9BHt7aq3DFCb74kZjPY5epgVtsWKCeYX1tUWxYwDpump")
	calc := NewCalculator(&mockRPC{
		accounts: map[string]*rpc.AccountInfo{
			pool.String(): {Exists: true, Data: []byte{1, 2, 3}},
		},
	}, nil, &mockSupply{})
	_, err := calc.Compute(context.Background(), Request{
		PoolAddress: pool,
		MintA:       mint,
		MintB:       solana.SolMint,
	})
	if err == nil {
		t.Fatal("expected decode pool state error")
	}

	poolData := make([]byte, poolExpectedMinDataSize)
	copy(poolData[poolBaseMintOffset:poolBaseMintOffset+32], mint.Bytes())
	copy(poolData[poolQuoteMintOffset:poolQuoteMintOffset+32], solana.SolMint.Bytes())
	baseVault := solana.MustPublicKeyFromBase58("9B8xXGabWeEfCLzwHAwwbjZjQamiFsh7WC8Y5bkvPbqP")
	quoteVault := solana.MustPublicKeyFromBase58("53WjyeT21YNnyP6ZNuMemvBaYvZfvijDdV6RmLb8Te9c")
	copy(poolData[poolBaseVaultOffset:poolBaseVaultOffset+32], baseVault.Bytes())
	copy(poolData[poolQuoteVaultOffset:poolQuoteVaultOffset+32], quoteVault.Bytes())

	calc = NewCalculator(&mockRPC{
		accounts: map[string]*rpc.AccountInfo{
			pool.String():           {Exists: true, Data: poolData},
			baseVault.String():      {Exists: false},
			quoteVault.String():     {Exists: true, Data: make([]byte, tokenAccountMinDataSize)},
			mint.String():           {Exists: true, Data: make([]byte, mintAccountMinDataSize)},
			solana.SolMint.String(): {Exists: true, Data: make([]byte, mintAccountMinDataSize)},
		},
	}, nil, &mockSupply{})
	_, err = calc.Compute(context.Background(), Request{
		PoolAddress: pool,
		MintA:       mint,
		MintB:       solana.SolMint,
	})
	if err == nil {
		t.Fatal("expected required account missing error")
	}
}

func TestComputeDecodeSubstepErrorsAndQuoteFailures(t *testing.T) {
	pool := solana.MustPublicKeyFromBase58("HnhpJPJgBG2KwniMTNW8cVBHvk1hFog3RC3kjnyc23tD")
	mintA := solana.MustPublicKeyFromBase58("9BHt7aq3DFCb74kZjPY5epgVtsWKCeYX1tUWxYwDpump")
	nonSOLQuote := solana.MustPublicKeyFromBase58("SysvarRent111111111111111111111111111111111")
	baseVault := solana.MustPublicKeyFromBase58("9B8xXGabWeEfCLzwHAwwbjZjQamiFsh7WC8Y5bkvPbqP")
	quoteVault := solana.MustPublicKeyFromBase58("53WjyeT21YNnyP6ZNuMemvBaYvZfvijDdV6RmLb8Te9c")

	poolData := make([]byte, poolExpectedMinDataSize)
	copy(poolData[poolBaseMintOffset:poolBaseMintOffset+32], mintA.Bytes())
	copy(poolData[poolQuoteMintOffset:poolQuoteMintOffset+32], nonSOLQuote.Bytes())
	copy(poolData[poolBaseVaultOffset:poolBaseVaultOffset+32], baseVault.Bytes())
	copy(poolData[poolQuoteVaultOffset:poolQuoteVaultOffset+32], quoteVault.Bytes())

	baseVaultData := make([]byte, tokenAccountMinDataSize)
	quoteVaultData := make([]byte, tokenAccountMinDataSize)
	binary.LittleEndian.PutUint64(baseVaultData[tokenAmountOffset:tokenAmountOffset+8], 10_000_000)
	binary.LittleEndian.PutUint64(quoteVaultData[tokenAmountOffset:tokenAmountOffset+8], 20_000_000)
	baseMintData := make([]byte, mintAccountMinDataSize)
	quoteMintData := make([]byte, mintAccountMinDataSize)
	baseMintData[mintDecimalsOffset] = 6
	quoteMintData[mintDecimalsOffset] = 6

	req := Request{
		PoolAddress: pool,
		MintA:       mintA,
		MintB:       nonSOLQuote,
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
				{Exists: true, Data: baseMintData},
				{Exists: true, Data: quoteMintData},
			},
		},
		{
			name: "quote vault decode",
			accounts: []*rpc.AccountInfo{
				{Exists: true, Data: baseVaultData},
				{Exists: true, Data: []byte{1}},
				{Exists: true, Data: baseMintData},
				{Exists: true, Data: quoteMintData},
			},
		},
		{
			name: "base mint decode",
			accounts: []*rpc.AccountInfo{
				{Exists: true, Data: baseVaultData},
				{Exists: true, Data: quoteVaultData},
				{Exists: true, Data: []byte{1}},
				{Exists: true, Data: quoteMintData},
			},
		},
		{
			name: "quote mint decode",
			accounts: []*rpc.AccountInfo{
				{Exists: true, Data: baseVaultData},
				{Exists: true, Data: quoteVaultData},
				{Exists: true, Data: baseMintData},
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

	priceErrCalc := NewCalculator(&mockRPC{
		accounts: map[string]*rpc.AccountInfo{
			pool.String():        {Exists: true, Data: poolData},
			baseVault.String():   {Exists: true, Data: baseVaultData},
			quoteVault.String():  {Exists: true, Data: quoteVaultData},
			mintA.String():       {Exists: true, Data: baseMintData},
			nonSOLQuote.String(): {Exists: true, Data: quoteMintData},
		},
	}, &mockQuote{err: errors.New("price quote error")}, &mockSupply{})
	if _, err := priceErrCalc.Compute(context.Background(), req); err == nil {
		t.Fatal("expected price quote conversion error")
	}

	liqErrCalc := NewCalculator(&mockRPC{
		accounts: map[string]*rpc.AccountInfo{
			pool.String():        {Exists: true, Data: poolData},
			baseVault.String():   {Exists: true, Data: baseVaultData},
			quoteVault.String():  {Exists: true, Data: quoteVaultData},
			mintA.String():       {Exists: true, Data: baseMintData},
			nonSOLQuote.String(): {Exists: true, Data: quoteMintData},
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
		t.Fatal("expected liquidity quote conversion error")
	}
}
