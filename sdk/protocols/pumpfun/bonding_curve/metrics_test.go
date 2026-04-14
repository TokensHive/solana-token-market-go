package bonding_curve

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
	account *rpc.AccountInfo
	err     error
}

func (m *mockRPC) GetAccount(context.Context, solana.PublicKey) (*rpc.AccountInfo, error) {
	return m.account, m.err
}
func (m *mockRPC) GetMultipleAccounts(context.Context, []solana.PublicKey) ([]*rpc.AccountInfo, error) {
	return nil, nil
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

type mockQuote struct {
	value decimal.Decimal
	err   error
}

func (m *mockQuote) ToSOL(context.Context, solana.PublicKey, decimal.Decimal) (decimal.Decimal, error) {
	return m.value, m.err
}

func TestComputeBondingCurveMetrics(t *testing.T) {
	data := make([]byte, 64)
	binary.LittleEndian.PutUint64(data[8:16], 1063770573068395)
	binary.LittleEndian.PutUint64(data[16:24], 30260284408)
	binary.LittleEndian.PutUint64(data[24:32], 783870573068395)
	binary.LittleEndian.PutUint64(data[32:40], 260284408)
	binary.LittleEndian.PutUint64(data[40:48], 1000000000000000)
	data[48] = 0

	calc := NewCalculator(&mockRPC{
		account: &rpc.AccountInfo{
			Address: solana.SolMint,
			Data:    data,
			Exists:  true,
		},
	}, nil)

	resp, err := calc.Compute(context.Background(), Request{
		PoolAddress: solana.SolMint,
		MintA:       solana.MustPublicKeyFromBase58("9BHt7aq3DFCb74kZjPY5epgVtsWKCeYX1tUWxYwDpump"),
		MintB:       solana.SolMint,
	})
	if err != nil {
		t.Fatalf("compute failed: %v", err)
	}
	if got := resp.PriceOfAInSOL.String(); got != "0.0000000284462507" {
		t.Fatalf("unexpected price in SOL: %s", got)
	}
	if got := resp.LiquidityInSOL.String(); got != "0.260284408" {
		t.Fatalf("unexpected liquidity in SOL: %s", got)
	}
	if got := resp.TotalSupply.String(); got != "1000000000" {
		t.Fatalf("unexpected total supply: %s", got)
	}
	if got := resp.CirculatingSupply.String(); got != "216129426.931605" {
		t.Fatalf("unexpected circulating supply: %s", got)
	}
}

func TestComputeValidationAndErrors(t *testing.T) {
	calc := NewCalculator(nil, nil)
	if _, err := calc.Compute(context.Background(), Request{}); err == nil {
		t.Fatal("expected rpc required validation error")
	}

	calc = NewCalculator(&mockRPC{}, nil)
	if _, err := calc.Compute(context.Background(), Request{}); err == nil {
		t.Fatal("expected pool address required validation error")
	}

	calc = NewCalculator(&mockRPC{err: errors.New("rpc error")}, nil)
	if _, err := calc.Compute(context.Background(), Request{PoolAddress: solana.SolMint}); err == nil {
		t.Fatal("expected rpc error")
	}

	calc = NewCalculator(&mockRPC{account: &rpc.AccountInfo{Exists: false}}, nil)
	if _, err := calc.Compute(context.Background(), Request{PoolAddress: solana.SolMint}); err == nil {
		t.Fatal("expected not found error")
	}
}

func TestDecodeAndHelperFunctions(t *testing.T) {
	state := decodeCurveState([]byte{1, 2, 3})
	if !state.virtualTokenReserve.IsZero() {
		t.Fatalf("expected empty decoded state on short input, got %#v", state)
	}

	price, liq := computeCurvePriceAndLiquidity(curveState{
		virtualTokenReserve: decimal.Zero,
		realSOLReserve:      decimal.NewFromInt(2),
	})
	if !price.IsZero() || !liq.Equal(decimal.NewFromInt(2)) {
		t.Fatalf("unexpected price/liquidity on zero reserve: price=%s liq=%s", price, liq)
	}

	total, circ := computeCurveSupplies(curveState{
		tokenTotalSupply: decimal.NewFromInt(10),
		realTokenReserve: decimal.NewFromInt(20),
	})
	if !total.Equal(decimal.NewFromInt(10)) || !circ.IsZero() {
		t.Fatalf("unexpected supply computation total=%s circ=%s", total, circ)
	}

	if got := readU64([]byte{1}, 8); got != 0 {
		t.Fatalf("expected zero for short read, got %d", got)
	}
}

func TestComputePairMetricsBranches(t *testing.T) {
	calc := NewCalculator(&mockRPC{}, &mockQuote{value: decimal.NewFromInt(2)})

	priceSOL, priceB, liqB := calc.computePairMetrics(context.Background(), solana.SolMint, solana.MustPublicKeyFromBase58("11111111111111111111111111111111"), decimal.NewFromInt(4), decimal.NewFromInt(8))
	if !priceSOL.Equal(decimal.NewFromInt(1)) || !priceB.Equal(decimal.RequireFromString("0.25")) || !liqB.Equal(decimal.NewFromInt(8)) {
		t.Fatalf("unexpected SOL-as-base metrics: priceSOL=%s priceB=%s liqB=%s", priceSOL, priceB, liqB)
	}

	priceSOL, priceB, liqB = calc.computePairMetrics(context.Background(), solana.MustPublicKeyFromBase58("11111111111111111111111111111111"), solana.SolMint, decimal.NewFromInt(4), decimal.NewFromInt(8))
	if !priceSOL.Equal(decimal.NewFromInt(4)) || !priceB.Equal(decimal.NewFromInt(4)) || !liqB.Equal(decimal.NewFromInt(8)) {
		t.Fatalf("unexpected SOL-as-quote metrics: priceSOL=%s priceB=%s liqB=%s", priceSOL, priceB, liqB)
	}

	calc = NewCalculator(&mockRPC{}, nil)
	_, priceB, liqB = calc.computePairMetrics(context.Background(), solana.MustPublicKeyFromBase58("11111111111111111111111111111111"), solana.MustPublicKeyFromBase58("SysvarRent111111111111111111111111111111111"), decimal.NewFromInt(4), decimal.NewFromInt(8))
	if !priceB.IsZero() || !liqB.IsZero() {
		t.Fatalf("expected zero quote conversion when bridge nil, priceB=%s liqB=%s", priceB, liqB)
	}

	calc = NewCalculator(&mockRPC{}, &mockQuote{err: errors.New("quote error")})
	_, priceB, liqB = calc.computePairMetrics(context.Background(), solana.MustPublicKeyFromBase58("11111111111111111111111111111111"), solana.MustPublicKeyFromBase58("SysvarRent111111111111111111111111111111111"), decimal.NewFromInt(4), decimal.NewFromInt(8))
	if !priceB.IsZero() || !liqB.IsZero() {
		t.Fatalf("expected zero conversion on quote error, priceB=%s liqB=%s", priceB, liqB)
	}

	calc = NewCalculator(&mockRPC{}, &mockQuote{value: decimal.Zero})
	_, priceB, liqB = calc.computePairMetrics(context.Background(), solana.MustPublicKeyFromBase58("11111111111111111111111111111111"), solana.MustPublicKeyFromBase58("SysvarRent111111111111111111111111111111111"), decimal.NewFromInt(4), decimal.NewFromInt(8))
	if !priceB.IsZero() || !liqB.IsZero() {
		t.Fatalf("expected zero conversion on zero quote price, priceB=%s liqB=%s", priceB, liqB)
	}

	calc = NewCalculator(&mockRPC{}, &mockQuote{value: decimal.NewFromInt(2)})
	priceSOL, priceB, liqB = calc.computePairMetrics(context.Background(), solana.MustPublicKeyFromBase58("11111111111111111111111111111111"), solana.MustPublicKeyFromBase58("SysvarRent111111111111111111111111111111111"), decimal.NewFromInt(4), decimal.NewFromInt(8))
	if !priceSOL.Equal(decimal.NewFromInt(4)) || !priceB.Equal(decimal.NewFromInt(2)) || !liqB.Equal(decimal.NewFromInt(4)) {
		t.Fatalf("unexpected quote conversion metrics: priceSOL=%s priceB=%s liqB=%s", priceSOL, priceB, liqB)
	}

	priceSOL, priceB, liqB = calc.computePairMetrics(context.Background(), solana.SolMint, solana.MustPublicKeyFromBase58("11111111111111111111111111111111"), decimal.Zero, decimal.NewFromInt(8))
	if !priceSOL.Equal(decimal.NewFromInt(1)) || !priceB.IsZero() || !liqB.Equal(decimal.NewFromInt(8)) {
		t.Fatalf("unexpected SOL base zero-price metrics: priceSOL=%s priceB=%s liqB=%s", priceSOL, priceB, liqB)
	}
}
