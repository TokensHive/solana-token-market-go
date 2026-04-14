package market

import (
	"context"
	"encoding/binary"
	"errors"
	"testing"

	"github.com/TokensHive/solana-token-market-go/sdk/internal/reqdebug"
	"github.com/TokensHive/solana-token-market-go/sdk/rpc"
	"github.com/gagliardetto/solana-go"
	rpcclient "github.com/gagliardetto/solana-go/rpc"
	"github.com/shopspring/decimal"
)

type marketMockRPC struct {
	getAccountFn          func(context.Context, solana.PublicKey) (*rpc.AccountInfo, error)
	getMultipleAccountsFn func(context.Context, []solana.PublicKey) ([]*rpc.AccountInfo, error)
	getTokenSupplyFn      func(context.Context, solana.PublicKey) (decimal.Decimal, uint8, error)
}

func (m *marketMockRPC) GetAccount(ctx context.Context, key solana.PublicKey) (*rpc.AccountInfo, error) {
	if m.getAccountFn == nil {
		return nil, nil
	}
	return m.getAccountFn(ctx, key)
}

func (m *marketMockRPC) GetMultipleAccounts(ctx context.Context, keys []solana.PublicKey) ([]*rpc.AccountInfo, error) {
	if m.getMultipleAccountsFn == nil {
		return nil, nil
	}
	return m.getMultipleAccountsFn(ctx, keys)
}

func (m *marketMockRPC) GetTokenSupply(ctx context.Context, mint solana.PublicKey) (decimal.Decimal, uint8, error) {
	if m.getTokenSupplyFn == nil {
		return decimal.Zero, 0, nil
	}
	return m.getTokenSupplyFn(ctx, mint)
}

func (m *marketMockRPC) GetSignaturesForAddress(context.Context, solana.PublicKey, *rpc.SignaturesForAddressOptions) ([]*rpcclient.TransactionSignature, error) {
	return nil, nil
}

func (m *marketMockRPC) GetTransaction(context.Context, solana.Signature) (*rpcclient.GetTransactionResult, error) {
	return nil, nil
}

func (m *marketMockRPC) GetTransactionRaw(context.Context, solana.Signature) ([]byte, error) {
	return nil, nil
}

type marketMockSupply struct {
	total  decimal.Decimal
	circ   decimal.Decimal
	method string
	err    error
}

func (m *marketMockSupply) GetSupply(context.Context, solana.PublicKey) (decimal.Decimal, decimal.Decimal, string, error) {
	return m.total, m.circ, m.method, m.err
}

func TestValidateMetricsRequest(t *testing.T) {
	valid := GetMetricsByPoolRequest{
		Pool: PoolIdentifier{
			Dex:         DexPumpfun,
			PoolVersion: PoolVersionPumpfunBondingCurve,
			MintA:       solana.SolMint,
			MintB:       solana.SolMint,
			PoolAddress: solana.SolMint,
		},
	}
	if err := validateMetricsRequest(valid); err != nil {
		t.Fatalf("expected valid request, got %v", err)
	}

	tests := []GetMetricsByPoolRequest{
		{},
		{Pool: PoolIdentifier{Dex: DexPumpfun}},
		{Pool: PoolIdentifier{Dex: DexPumpfun, PoolVersion: PoolVersionPumpfunBondingCurve}},
		{Pool: PoolIdentifier{Dex: DexPumpfun, PoolVersion: PoolVersionPumpfunBondingCurve, PoolAddress: solana.SolMint}},
	}
	for i, req := range tests {
		if err := validateMetricsRequest(req); err == nil {
			t.Fatalf("expected validation error for case %d", i)
		}
	}
}

func TestGetMetricsByPool_UnsupportedRoute(t *testing.T) {
	service := NewService(defaultConfig())
	_, err := service.GetMetricsByPool(context.Background(), GetMetricsByPoolRequest{
		Pool: PoolIdentifier{
			Dex:         Dex("unknown"),
			PoolVersion: PoolVersion("v1"),
			MintA:       solana.SolMint,
			MintB:       solana.SolMint,
			PoolAddress: solana.SolMint,
		},
	})
	if err == nil {
		t.Fatal("expected unsupported route error")
	}

	_, err = service.GetMetricsByPool(context.Background(), GetMetricsByPoolRequest{})
	if err == nil {
		t.Fatal("expected request validation error path from service method")
	}
}

func TestDefaultPumpfunBondingCurveRoute(t *testing.T) {
	data := make([]byte, 64)
	binary.LittleEndian.PutUint64(data[8:16], 1063770573068395)
	binary.LittleEndian.PutUint64(data[16:24], 30260284408)
	binary.LittleEndian.PutUint64(data[24:32], 783870573068395)
	binary.LittleEndian.PutUint64(data[32:40], 260284408)
	binary.LittleEndian.PutUint64(data[40:48], 1000000000000000)

	service := NewService(Config{
		RPCClient: &marketMockRPC{
			getAccountFn: func(context.Context, solana.PublicKey) (*rpc.AccountInfo, error) {
				return &rpc.AccountInfo{Exists: true, Data: data}, nil
			},
		},
	})

	resp, err := service.GetMetricsByPool(context.Background(), GetMetricsByPoolRequest{
		Pool: PoolIdentifier{
			Dex:         DexPumpfun,
			PoolVersion: PoolVersionPumpfunBondingCurve,
			MintA:       solana.MustPublicKeyFromBase58("9BHt7aq3DFCb74kZjPY5epgVtsWKCeYX1tUWxYwDpump"),
			MintB:       solana.SolMint,
			PoolAddress: solana.SolMint,
		},
	})
	if err != nil {
		t.Fatalf("expected successful bonding curve response, got %v", err)
	}
	if resp.SupplyMethod != "pumpfun_curve_state" {
		t.Fatalf("unexpected supply method: %s", resp.SupplyMethod)
	}
}

func TestDefaultPumpfunAmmRoute(t *testing.T) {
	pool := solana.MustPublicKeyFromBase58("HnhpJPJgBG2KwniMTNW8cVBHvk1hFog3RC3kjnyc23tD")
	mintA := solana.MustPublicKeyFromBase58("9BHt7aq3DFCb74kZjPY5epgVtsWKCeYX1tUWxYwDpump")
	poolQuoteMint := solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")
	baseVault := solana.MustPublicKeyFromBase58("9B8xXGabWeEfCLzwHAwwbjZjQamiFsh7WC8Y5bkvPbqP")
	quoteVault := solana.MustPublicKeyFromBase58("53WjyeT21YNnyP6ZNuMemvBaYvZfvijDdV6RmLb8Te9c")

	poolData := make([]byte, 243)
	copy(poolData[43:75], mintA.Bytes())
	copy(poolData[75:107], poolQuoteMint.Bytes())
	copy(poolData[139:171], baseVault.Bytes())
	copy(poolData[171:203], quoteVault.Bytes())

	baseVaultData := make([]byte, 72)
	quoteVaultData := make([]byte, 72)
	binary.LittleEndian.PutUint64(baseVaultData[64:72], 1_000_000_000)
	binary.LittleEndian.PutUint64(quoteVaultData[64:72], 5_000_000_000)
	baseMintData := make([]byte, 45)
	quoteMintData := make([]byte, 45)
	baseMintData[44] = 6
	quoteMintData[44] = 9

	service := NewService(Config{
		RPCClient: &marketMockRPC{
			getAccountFn: func(context.Context, solana.PublicKey) (*rpc.AccountInfo, error) {
				return &rpc.AccountInfo{Exists: true, Data: poolData}, nil
			},
			getMultipleAccountsFn: func(context.Context, []solana.PublicKey) ([]*rpc.AccountInfo, error) {
				return []*rpc.AccountInfo{
					{Exists: true, Data: baseVaultData},
					{Exists: true, Data: quoteVaultData},
					{Exists: true, Data: baseMintData},
					{Exists: true, Data: quoteMintData},
				}, nil
			},
		},
		SupplyProvider: &marketMockSupply{
			total:  decimal.NewFromInt(1_000_000),
			circ:   decimal.NewFromInt(900_000),
			method: "mock",
		},
	})

	resp, err := service.GetMetricsByPool(context.Background(), GetMetricsByPoolRequest{
		Pool: PoolIdentifier{
			Dex:         DexPumpfun,
			PoolVersion: PoolVersionPumpfunAmm,
			MintA:       mintA,
			MintB:       solana.SolMint,
			PoolAddress: pool,
		},
	})
	if err != nil {
		t.Fatalf("expected successful amm response, got %v", err)
	}
	if resp.PriceOfAInSOL.IsZero() {
		t.Fatal("expected non-zero AMM price in SOL")
	}
}

func TestCustomFactoryOverridesDefaultRoute(t *testing.T) {
	var called bool
	client, err := NewClient(
		WithPoolCalculatorFactory(PoolRoute{
			Dex:         DexPumpfun,
			PoolVersion: PoolVersionPumpfunBondingCurve,
		}, func(Config) PoolCalculator {
			return poolCalculatorFunc(func(context.Context, PoolIdentifier) (*GetMetricsByPoolResponse, error) {
				called = true
				return &GetMetricsByPoolResponse{
					PriceOfAInSOL: decimal.NewFromInt(1),
				}, nil
			})
		}),
	)
	if err != nil {
		t.Fatalf("new client failed: %v", err)
	}

	_, err = client.GetMetricsByPool(context.Background(), GetMetricsByPoolRequest{
		Pool: PoolIdentifier{
			Dex:         DexPumpfun,
			PoolVersion: PoolVersionPumpfunBondingCurve,
			PoolAddress: solana.SolMint,
			MintA:       solana.SolMint,
			MintB:       solana.SolMint,
		},
	})
	if err != nil {
		t.Fatalf("expected custom calculator success, got %v", err)
	}
	if !called {
		t.Fatal("expected custom calculator to be called")
	}
}

func TestAttachRequestDebug(t *testing.T) {
	meta := map[string]any{"x": 1}
	if out := attachRequestDebug(context.Background(), meta); out["x"] != 1 {
		t.Fatalf("expected metadata passthrough, got %#v", out)
	}

	rec := reqdebug.NewRecorder("op")
	ctx := reqdebug.WithRecorder(context.Background(), rec)
	out := attachRequestDebug(ctx, nil)
	if out == nil || out["requests"] == nil {
		t.Fatalf("expected request debug metadata, got %#v", out)
	}
}

func TestBuildMetricsResponse(t *testing.T) {
	resp := buildMetricsResponse(
		PoolIdentifier{Dex: DexPumpfun},
		decimal.NewFromInt(1),
		decimal.NewFromInt(2),
		decimal.NewFromInt(3),
		decimal.NewFromInt(4),
		decimal.NewFromInt(5),
		decimal.NewFromInt(6),
		decimal.NewFromInt(7),
		"supply",
		map[string]any{"k": "v"},
	)
	if resp.SupplyMethod != "supply" || resp.Metadata["k"] != "v" {
		t.Fatalf("unexpected response mapping: %#v", resp)
	}
}

func TestDefaultCalculatorFactoryErrorMapping(t *testing.T) {
	service := NewService(Config{
		RPCClient: &marketMockRPC{
			getAccountFn: func(context.Context, solana.PublicKey) (*rpc.AccountInfo, error) {
				return nil, errors.New("rpc failure")
			},
		},
		SupplyProvider: &marketMockSupply{},
	})

	_, err := service.GetMetricsByPool(context.Background(), GetMetricsByPoolRequest{
		Pool: PoolIdentifier{
			Dex:         DexPumpfun,
			PoolVersion: PoolVersionPumpfunBondingCurve,
			MintA:       solana.SolMint,
			MintB:       solana.SolMint,
			PoolAddress: solana.SolMint,
		},
	})
	if err == nil {
		t.Fatal("expected wrapped default calculator error")
	}
}

func TestGetMetricsByPool_NilRegisteredCalculator(t *testing.T) {
	service := NewService(defaultConfig())
	service.calculators[PoolRoute{
		Dex:         Dex("nilcalc"),
		PoolVersion: PoolVersion("v1"),
	}] = nil
	_, err := service.GetMetricsByPool(context.Background(), GetMetricsByPoolRequest{
		Pool: PoolIdentifier{
			Dex:         Dex("nilcalc"),
			PoolVersion: PoolVersion("v1"),
			MintA:       solana.SolMint,
			MintB:       solana.SolMint,
			PoolAddress: solana.SolMint,
		},
	})
	if err == nil {
		t.Fatal("expected unsupported route for nil calculator")
	}
}

func TestDefaultAmmFactoryErrorMapping(t *testing.T) {
	service := NewService(Config{
		RPCClient: &marketMockRPC{
			getAccountFn: func(context.Context, solana.PublicKey) (*rpc.AccountInfo, error) {
				return nil, errors.New("rpc failure")
			},
		},
		SupplyProvider: &marketMockSupply{},
	})
	_, err := service.GetMetricsByPool(context.Background(), GetMetricsByPoolRequest{
		Pool: PoolIdentifier{
			Dex:         DexPumpfun,
			PoolVersion: PoolVersionPumpfunAmm,
			MintA:       solana.SolMint,
			MintB:       solana.SolMint,
			PoolAddress: solana.SolMint,
		},
	})
	if err == nil {
		t.Fatal("expected wrapped amm error")
	}
}
