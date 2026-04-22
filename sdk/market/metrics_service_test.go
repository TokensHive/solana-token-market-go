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

func marketTestPubkey(seed byte) solana.PublicKey {
	data := make([]byte, 32)
	for i := range data {
		data[i] = seed
	}
	return solana.PublicKeyFromBytes(data)
}

func (m *marketMockSupply) GetSupply(context.Context, solana.PublicKey) (decimal.Decimal, decimal.Decimal, string, error) {
	return m.total, m.circ, m.method, m.err
}

func TestValidateMetricsRequest(t *testing.T) {
	valid := GetMetricsByPoolRequest{
		Pool: PoolIdentifier{
			Dex:         DexPumpfun,
			PoolVersion: PoolVersionPumpfunBondingCurve,
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

func TestGetMetricsByPumpfunBondingCurve(t *testing.T) {
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

	resp, err := service.GetMetricsByPumpfunBondingCurve(context.Background(), GetMetricsByPumpfunBondingCurveRequest{
		MintA: solana.MustPublicKeyFromBase58("9BHt7aq3DFCb74kZjPY5epgVtsWKCeYX1tUWxYwDpump"),
		MintB: solana.SolMint,
	})
	if err != nil {
		t.Fatalf("expected successful bonding curve response, got %v", err)
	}
	if resp.SupplyMethod != "pumpfun_curve_state" {
		t.Fatalf("unexpected supply method: %s", resp.SupplyMethod)
	}
	if resp.FDVInSOL.IsZero() {
		t.Fatal("expected non-zero FDV for bonding curve route")
	}
	if resp.Pool.PoolAddress.IsZero() {
		t.Fatal("expected derived bonding curve pool address")
	}
}

func TestGetMetricsByPool_RejectsBondingCurveRoute(t *testing.T) {
	service := NewService(defaultConfig())
	_, err := service.GetMetricsByPool(context.Background(), GetMetricsByPoolRequest{
		Pool: PoolIdentifier{
			Dex:         DexPumpfun,
			PoolVersion: PoolVersionPumpfunBondingCurve,
			PoolAddress: solana.SolMint,
		},
	})
	if err == nil {
		t.Fatal("expected bonding curve rejection for GetMetricsByPool")
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
			PoolAddress: pool,
		},
	})
	if err != nil {
		t.Fatalf("expected successful amm response, got %v", err)
	}
	if resp.PriceOfAInSOL.IsZero() {
		t.Fatal("expected non-zero AMM price in SOL")
	}
	if !resp.FDVInSOL.GreaterThan(resp.MarketCapInSOL) {
		t.Fatalf("expected fdv > market cap for amm route, fdv=%s marketcap=%s", resp.FDVInSOL, resp.MarketCapInSOL)
	}
}

func TestDefaultRaydiumLiquidityV4Route(t *testing.T) {
	pool := marketTestPubkey(1)
	mintA := solana.MustPublicKeyFromBase58("EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")
	baseVault := marketTestPubkey(2)
	quoteVault := marketTestPubkey(3)
	openOrders := marketTestPubkey(4)

	poolData := make([]byte, 752)
	binary.LittleEndian.PutUint64(poolData[32:40], 6)
	binary.LittleEndian.PutUint64(poolData[40:48], 9)
	copy(poolData[400:432], mintA.Bytes())
	copy(poolData[432:464], solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112").Bytes())
	copy(poolData[336:368], baseVault.Bytes())
	copy(poolData[368:400], quoteVault.Bytes())
	copy(poolData[496:528], openOrders.Bytes())

	baseVaultData := make([]byte, 72)
	quoteVaultData := make([]byte, 72)
	openOrdersData := make([]byte, 109)
	binary.LittleEndian.PutUint64(baseVaultData[64:72], 100_000_000)
	binary.LittleEndian.PutUint64(quoteVaultData[64:72], 3_000_000_000)
	binary.LittleEndian.PutUint64(openOrdersData[85:93], 20_000_000)
	binary.LittleEndian.PutUint64(openOrdersData[101:109], 1_000_000_000)

	service := NewService(Config{
		RPCClient: &marketMockRPC{
			getAccountFn: func(context.Context, solana.PublicKey) (*rpc.AccountInfo, error) {
				return &rpc.AccountInfo{Exists: true, Data: poolData}, nil
			},
			getMultipleAccountsFn: func(context.Context, []solana.PublicKey) ([]*rpc.AccountInfo, error) {
				return []*rpc.AccountInfo{
					{Exists: true, Data: baseVaultData},
					{Exists: true, Data: quoteVaultData},
					{Exists: true, Data: openOrdersData},
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
			Dex:         DexRaydium,
			PoolVersion: PoolVersionRaydiumLiquidityV4,
			PoolAddress: pool,
		},
	})
	if err != nil {
		t.Fatalf("expected successful raydium v4 response, got %v", err)
	}
	if resp.PriceOfAInSOL.IsZero() {
		t.Fatal("expected non-zero raydium v4 price in SOL")
	}
	if !resp.FDVInSOL.GreaterThan(resp.MarketCapInSOL) {
		t.Fatalf("expected fdv > market cap for raydium v4 route, fdv=%s marketcap=%s", resp.FDVInSOL, resp.MarketCapInSOL)
	}
}

func TestDefaultRaydiumCPMMRoute(t *testing.T) {
	pool := marketTestPubkey(21)
	mintA := solana.MustPublicKeyFromBase58("2nCeHpECQvnMfzjU5fDMAKws1vBxMzxvWr6qqLpApump")
	token0Vault := marketTestPubkey(22)
	token1Vault := marketTestPubkey(23)

	poolData := make([]byte, 637)
	copy(poolData[168:200], mintA.Bytes())
	copy(poolData[200:232], solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112").Bytes())
	copy(poolData[72:104], token0Vault.Bytes())
	copy(poolData[104:136], token1Vault.Bytes())
	poolData[331] = 6
	poolData[332] = 9
	binary.LittleEndian.PutUint64(poolData[341:349], 100_000_000)
	binary.LittleEndian.PutUint64(poolData[349:357], 500_000_000)

	token0VaultData := make([]byte, 72)
	token1VaultData := make([]byte, 72)
	binary.LittleEndian.PutUint64(token0VaultData[64:72], 1_000_000_000)
	binary.LittleEndian.PutUint64(token1VaultData[64:72], 5_000_000_000)

	service := NewService(Config{
		RPCClient: &marketMockRPC{
			getAccountFn: func(_ context.Context, key solana.PublicKey) (*rpc.AccountInfo, error) {
				if key.Equals(pool) {
					return &rpc.AccountInfo{Exists: true, Data: poolData}, nil
				}
				return &rpc.AccountInfo{Exists: false}, nil
			},
			getMultipleAccountsFn: func(context.Context, []solana.PublicKey) ([]*rpc.AccountInfo, error) {
				return []*rpc.AccountInfo{
					{Exists: true, Data: token0VaultData},
					{Exists: true, Data: token1VaultData},
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
			Dex:         DexRaydium,
			PoolVersion: PoolVersionRaydiumCPMM,
			PoolAddress: pool,
		},
	})
	if err != nil {
		t.Fatalf("expected successful raydium cpmm response, got %v", err)
	}
	if resp.PriceOfAInSOL.IsZero() {
		t.Fatal("expected non-zero raydium cpmm price in SOL")
	}
}

func TestDefaultRaydiumCLMMRoute(t *testing.T) {
	pool := marketTestPubkey(41)
	mintA := solana.MustPublicKeyFromBase58("9BHt7aq3DFCb74kZjPY5epgVtsWKCeYX1tUWxYwDpump")
	token0Vault := marketTestPubkey(42)
	token1Vault := marketTestPubkey(43)

	poolData := make([]byte, 1544)
	copy(poolData[73:105], solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112").Bytes())
	copy(poolData[105:137], mintA.Bytes())
	copy(poolData[137:169], token0Vault.Bytes())
	copy(poolData[169:201], token1Vault.Bytes())
	poolData[233] = 9
	poolData[234] = 6
	binary.LittleEndian.PutUint64(poolData[253:261], 0)
	binary.LittleEndian.PutUint64(poolData[261:269], 1)
	binary.LittleEndian.PutUint64(poolData[309:317], 1_000_000_000)
	binary.LittleEndian.PutUint64(poolData[317:325], 100_000_000)

	token0VaultData := make([]byte, 72)
	token1VaultData := make([]byte, 72)
	binary.LittleEndian.PutUint64(token0VaultData[64:72], 11_000_000_000)
	binary.LittleEndian.PutUint64(token1VaultData[64:72], 2_000_000_000)

	service := NewService(Config{
		RPCClient: &marketMockRPC{
			getAccountFn: func(_ context.Context, key solana.PublicKey) (*rpc.AccountInfo, error) {
				if key.Equals(pool) {
					return &rpc.AccountInfo{Exists: true, Data: poolData}, nil
				}
				return &rpc.AccountInfo{Exists: false}, nil
			},
			getMultipleAccountsFn: func(context.Context, []solana.PublicKey) ([]*rpc.AccountInfo, error) {
				return []*rpc.AccountInfo{
					{Exists: true, Data: token0VaultData},
					{Exists: true, Data: token1VaultData},
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
			Dex:         DexRaydium,
			PoolVersion: PoolVersionRaydiumCLMM,
			PoolAddress: pool,
		},
	})
	if err != nil {
		t.Fatalf("expected successful raydium clmm response, got %v", err)
	}
	if resp.PriceOfAInSOL.IsZero() {
		t.Fatal("expected non-zero raydium clmm price in SOL")
	}
}

func TestDefaultRaydiumLaunchpadRoute(t *testing.T) {
	pool := marketTestPubkey(49)
	config := marketTestPubkey(50)
	baseMint := solana.MustPublicKeyFromBase58("9BHt7aq3DFCb74kZjPY5epgVtsWKCeYX1tUWxYwDpump")
	quoteMint := solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")

	poolData := make([]byte, 429)
	copy(poolData[:8], []byte{247, 237, 227, 245, 215, 195, 222, 70})
	poolData[18] = 6
	poolData[19] = 9
	binary.LittleEndian.PutUint64(poolData[29:37], 793_100_000_000_000)
	binary.LittleEndian.PutUint64(poolData[37:45], 1_073_025_605_596_382)
	binary.LittleEndian.PutUint64(poolData[45:53], 30_000_852_951)
	binary.LittleEndian.PutUint64(poolData[53:61], 727_865_254_549_778)
	binary.LittleEndian.PutUint64(poolData[61:69], 63_265_025_701)
	copy(poolData[141:173], config.Bytes())
	copy(poolData[205:237], baseMint.Bytes())
	copy(poolData[237:269], quoteMint.Bytes())

	configData := make([]byte, 371)
	copy(configData[:8], []byte{149, 8, 156, 202, 160, 252, 176, 217})
	configData[16] = 0
	binary.LittleEndian.PutUint64(configData[27:35], 10_000)
	copy(configData[83:115], quoteMint.Bytes())

	launchpadProgram := solana.MustPublicKeyFromBase58("LanMV9sAd7wArD4vJFi2qDdfnVhFxYSUg6eADduJ3uj")
	service := NewService(Config{
		RPCClient: &marketMockRPC{
			getAccountFn: func(_ context.Context, key solana.PublicKey) (*rpc.AccountInfo, error) {
				switch {
				case key.Equals(pool):
					return &rpc.AccountInfo{Exists: true, Owner: launchpadProgram, Data: poolData}, nil
				case key.Equals(config):
					return &rpc.AccountInfo{Exists: true, Owner: launchpadProgram, Data: configData}, nil
				default:
					return &rpc.AccountInfo{Exists: false}, nil
				}
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
			Dex:         DexRaydium,
			PoolVersion: PoolVersionRaydiumLaunchpad,
			PoolAddress: pool,
		},
	})
	if err != nil {
		t.Fatalf("expected successful raydium launchpad response, got %v", err)
	}
	if resp.PriceOfAInSOL.IsZero() {
		t.Fatal("expected non-zero raydium launchpad price in SOL")
	}
}

func TestDefaultMeteoraDLMMRoute(t *testing.T) {
	pool := marketTestPubkey(51)
	mintA := solana.MustPublicKeyFromBase58("2nCeHpECQvnMfzjU5fDMAKws1vBxMzxvWr6qqLpApump")
	reserveX := marketTestPubkey(52)
	reserveY := marketTestPubkey(53)

	poolData := make([]byte, 216)
	binary.LittleEndian.PutUint32(poolData[76:80], uint32(0))
	binary.LittleEndian.PutUint16(poolData[80:82], 80)
	copy(poolData[88:120], mintA.Bytes())
	copy(poolData[120:152], solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112").Bytes())
	copy(poolData[152:184], reserveX.Bytes())
	copy(poolData[184:216], reserveY.Bytes())

	reserveXData := make([]byte, 72)
	reserveYData := make([]byte, 72)
	mintAData := make([]byte, 45)
	mintBData := make([]byte, 45)
	binary.LittleEndian.PutUint64(reserveXData[64:72], 2_000_000)
	binary.LittleEndian.PutUint64(reserveYData[64:72], 5_000_000_000)
	mintAData[44] = 6
	mintBData[44] = 9

	dlmmProgram := solana.MustPublicKeyFromBase58("LBUZKhRxPF3XUpBCjp4YzTKgLccjZhTSDM9YuVaPwxo")
	service := NewService(Config{
		RPCClient: &marketMockRPC{
			getAccountFn: func(_ context.Context, key solana.PublicKey) (*rpc.AccountInfo, error) {
				if key.Equals(pool) {
					return &rpc.AccountInfo{Exists: true, Owner: dlmmProgram, Data: poolData}, nil
				}
				return &rpc.AccountInfo{Exists: false}, nil
			},
			getMultipleAccountsFn: func(context.Context, []solana.PublicKey) ([]*rpc.AccountInfo, error) {
				return []*rpc.AccountInfo{
					{Exists: true, Data: reserveXData},
					{Exists: true, Data: reserveYData},
					{Exists: true, Data: mintAData},
					{Exists: true, Data: mintBData},
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
			Dex:         DexMeteora,
			PoolVersion: PoolVersionMeteoraDLMM,
			PoolAddress: pool,
		},
	})
	if err != nil {
		t.Fatalf("expected successful meteora dlmm response, got %v", err)
	}
	if resp.PriceOfAInSOL.IsZero() {
		t.Fatal("expected non-zero meteora dlmm price in SOL")
	}
}

func TestDefaultMeteoraDBCRoute(t *testing.T) {
	pool := marketTestPubkey(54)
	mintA := solana.MustPublicKeyFromBase58("2nCeHpECQvnMfzjU5fDMAKws1vBxMzxvWr6qqLpApump")
	quoteMint := solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")
	baseVault := marketTestPubkey(55)
	quoteVault := marketTestPubkey(56)
	config := marketTestPubkey(57)

	poolData := make([]byte, 424)
	copy(poolData[:8], []byte{213, 224, 5, 209, 98, 69, 119, 92})
	copy(poolData[72:104], config.Bytes())
	copy(poolData[136:168], mintA.Bytes())
	copy(poolData[168:200], baseVault.Bytes())
	copy(poolData[200:232], quoteVault.Bytes())
	binary.LittleEndian.PutUint64(poolData[232:240], 2_000_000)
	binary.LittleEndian.PutUint64(poolData[240:248], 5_000_000_000)
	binary.LittleEndian.PutUint64(poolData[280:288], 0)
	binary.LittleEndian.PutUint64(poolData[288:296], 1)

	quoteVaultData := make([]byte, 32)
	copy(quoteVaultData[:32], quoteMint.Bytes())
	mintAData := make([]byte, 45)
	quoteMintData := make([]byte, 45)
	mintAData[44] = 6
	quoteMintData[44] = 9

	dbcProgram := solana.MustPublicKeyFromBase58("dbcij3LWUppWqq96dh6gJWwBifmcGfLSB5D4DuSMaqN")
	service := NewService(Config{
		RPCClient: &marketMockRPC{
			getAccountFn: func(_ context.Context, key solana.PublicKey) (*rpc.AccountInfo, error) {
				switch {
				case key.Equals(pool):
					return &rpc.AccountInfo{Exists: true, Owner: dbcProgram, Data: poolData}, nil
				case key.Equals(quoteMint):
					return &rpc.AccountInfo{Exists: true, Data: quoteMintData}, nil
				default:
					return &rpc.AccountInfo{Exists: false}, nil
				}
			},
			getMultipleAccountsFn: func(context.Context, []solana.PublicKey) ([]*rpc.AccountInfo, error) {
				return []*rpc.AccountInfo{
					{Exists: true, Data: quoteVaultData},
					{Exists: true, Data: mintAData},
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
			Dex:         DexMeteora,
			PoolVersion: PoolVersionMeteoraDBC,
			PoolAddress: pool,
		},
	})
	if err != nil {
		t.Fatalf("expected successful meteora dbc response, got %v", err)
	}
	if resp.PriceOfAInSOL.IsZero() {
		t.Fatal("expected non-zero meteora dbc price in SOL")
	}
}

func TestDefaultMeteoraDAMMV1Route(t *testing.T) {
	pool := marketTestPubkey(55)
	lpMint := marketTestPubkey(56)
	mintA := solana.MustPublicKeyFromBase58("EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")
	tokenAMint := mintA
	tokenBMint := solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")
	tokenAVault := marketTestPubkey(57)
	tokenBVault := marketTestPubkey(58)
	tokenAVaultLP := marketTestPubkey(59)
	tokenBVaultLP := marketTestPubkey(60)
	tokenAVaultLPMint := marketTestPubkey(61)
	tokenBVaultLPMint := marketTestPubkey(62)

	poolData := make([]byte, 892)
	copy(poolData[:8], []byte{241, 154, 109, 4, 17, 177, 109, 188})
	copy(poolData[8:40], lpMint.Bytes())
	copy(poolData[40:72], tokenAMint.Bytes())
	copy(poolData[72:104], tokenBMint.Bytes())
	copy(poolData[104:136], tokenAVault.Bytes())
	copy(poolData[136:168], tokenBVault.Bytes())
	copy(poolData[168:200], tokenAVaultLP.Bytes())
	copy(poolData[200:232], tokenBVaultLP.Bytes())
	poolData[233] = 1
	poolData[891] = 0

	tokenAVaultLPData := make([]byte, 72)
	tokenBVaultLPData := make([]byte, 72)
	copy(tokenAVaultLPData[:32], tokenAVaultLPMint.Bytes())
	copy(tokenBVaultLPData[:32], tokenBVaultLPMint.Bytes())
	binary.LittleEndian.PutUint64(tokenAVaultLPData[64:72], 20_000)
	binary.LittleEndian.PutUint64(tokenBVaultLPData[64:72], 50_000)

	tokenAMintData := make([]byte, 45)
	tokenBMintData := make([]byte, 45)
	tokenAVaultLPMintData := make([]byte, 45)
	tokenBVaultLPMintData := make([]byte, 45)
	tokenAMintData[44] = 6
	tokenBMintData[44] = 9
	binary.LittleEndian.PutUint64(tokenAVaultLPMintData[36:44], 100_000)
	tokenAVaultLPMintData[44] = 6
	binary.LittleEndian.PutUint64(tokenBVaultLPMintData[36:44], 100_000)
	tokenBVaultLPMintData[44] = 9

	now := uint64(1_700_000_000)
	tokenAVaultData := make([]byte, 1227)
	tokenBVaultData := make([]byte, 1227)
	copy(tokenAVaultData[115:147], tokenAVaultLPMint.Bytes())
	copy(tokenBVaultData[115:147], tokenBVaultLPMint.Bytes())
	binary.LittleEndian.PutUint64(tokenAVaultData[11:19], 10_000_000)
	binary.LittleEndian.PutUint64(tokenBVaultData[11:19], 10_000_000_000)
	binary.LittleEndian.PutUint64(tokenAVaultData[1211:1219], now)
	binary.LittleEndian.PutUint64(tokenBVaultData[1211:1219], now)

	clockData := make([]byte, 40)
	binary.LittleEndian.PutUint64(clockData[32:40], now)

	dammV1Program := solana.MustPublicKeyFromBase58("Eo7WjKq67rjJQSZxS6z3YkapzY3eMj6Xy8X5EQVn5UaB")
	service := NewService(Config{
		RPCClient: &marketMockRPC{
			getAccountFn: func(_ context.Context, key solana.PublicKey) (*rpc.AccountInfo, error) {
				if key.Equals(pool) {
					return &rpc.AccountInfo{Exists: true, Owner: dammV1Program, Data: poolData}, nil
				}
				return &rpc.AccountInfo{Exists: false}, nil
			},
			getMultipleAccountsFn: func(_ context.Context, keys []solana.PublicKey) ([]*rpc.AccountInfo, error) {
				if len(keys) == 7 {
					return []*rpc.AccountInfo{
						{Exists: true, Data: tokenAVaultLPData},
						{Exists: true, Data: tokenBVaultLPData},
						{Exists: true, Data: tokenAVaultData},
						{Exists: true, Data: tokenBVaultData},
						{Exists: true, Data: tokenAMintData},
						{Exists: true, Data: tokenBMintData},
						{Exists: true, Data: clockData},
					}, nil
				}
				return []*rpc.AccountInfo{
					{Exists: true, Data: tokenAVaultLPMintData},
					{Exists: true, Data: tokenBVaultLPMintData},
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
			Dex:         DexMeteora,
			PoolVersion: PoolVersionMeteoraDAMMV1,
			PoolAddress: pool,
		},
	})
	if err != nil {
		t.Fatalf("expected successful meteora damm v1 response, got %v", err)
	}
	if resp.PriceOfAInSOL.IsZero() {
		t.Fatal("expected non-zero meteora damm v1 price in SOL")
	}
}

func TestDefaultMeteoraDAMMV2Route(t *testing.T) {
	pool := marketTestPubkey(61)
	mintA := solana.MustPublicKeyFromBase58("2nCeHpECQvnMfzjU5fDMAKws1vBxMzxvWr6qqLpApump")
	tokenAMint := mintA
	tokenBMint := solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")
	tokenAVault := marketTestPubkey(62)
	tokenBVault := marketTestPubkey(63)

	poolData := make([]byte, 1112)
	copy(poolData[:8], []byte{241, 154, 109, 4, 17, 177, 109, 188})
	copy(poolData[168:200], tokenAMint.Bytes())
	copy(poolData[200:232], tokenBMint.Bytes())
	copy(poolData[232:264], tokenAVault.Bytes())
	copy(poolData[264:296], tokenBVault.Bytes())
	binary.LittleEndian.PutUint64(poolData[392:400], 100_000)
	binary.LittleEndian.PutUint64(poolData[400:408], 200_000_000)
	binary.LittleEndian.PutUint64(poolData[464:472], 1)

	tokenAVaultData := make([]byte, 72)
	tokenBVaultData := make([]byte, 72)
	mintAData := make([]byte, 45)
	mintBData := make([]byte, 45)
	binary.LittleEndian.PutUint64(tokenAVaultData[64:72], 2_000_000)
	binary.LittleEndian.PutUint64(tokenBVaultData[64:72], 5_000_000_000)
	mintAData[44] = 6
	mintBData[44] = 9

	dammV2Program := solana.MustPublicKeyFromBase58("cpamdpZCGKUy5JxQXB4dcpGPiikHawvSWAd6mEn1sGG")
	service := NewService(Config{
		RPCClient: &marketMockRPC{
			getAccountFn: func(_ context.Context, key solana.PublicKey) (*rpc.AccountInfo, error) {
				if key.Equals(pool) {
					return &rpc.AccountInfo{Exists: true, Owner: dammV2Program, Data: poolData}, nil
				}
				return &rpc.AccountInfo{Exists: false}, nil
			},
			getMultipleAccountsFn: func(context.Context, []solana.PublicKey) ([]*rpc.AccountInfo, error) {
				return []*rpc.AccountInfo{
					{Exists: true, Data: tokenAVaultData},
					{Exists: true, Data: tokenBVaultData},
					{Exists: true, Data: mintAData},
					{Exists: true, Data: mintBData},
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
			Dex:         DexMeteora,
			PoolVersion: PoolVersionMeteoraDAMMV2,
			PoolAddress: pool,
		},
	})
	if err != nil {
		t.Fatalf("expected successful meteora damm v2 response, got %v", err)
	}
	if resp.PriceOfAInSOL.IsZero() {
		t.Fatal("expected non-zero meteora damm v2 price in SOL")
	}
}

func TestDefaultOrcaWhirlpoolRoute(t *testing.T) {
	pool := marketTestPubkey(71)
	mintA := solana.MustPublicKeyFromBase58("EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")
	tokenAMint := mintA
	tokenBMint := solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")
	tokenAVault := marketTestPubkey(72)
	tokenBVault := marketTestPubkey(73)

	poolData := make([]byte, 653)
	copy(poolData[:8], []byte{63, 149, 209, 12, 225, 128, 99, 9})
	copy(poolData[101:133], tokenAMint.Bytes())
	copy(poolData[133:165], tokenAVault.Bytes())
	copy(poolData[181:213], tokenBMint.Bytes())
	copy(poolData[213:245], tokenBVault.Bytes())
	binary.LittleEndian.PutUint64(poolData[85:93], 100)
	binary.LittleEndian.PutUint64(poolData[93:101], 200)
	binary.LittleEndian.PutUint64(poolData[65:73], 0)
	binary.LittleEndian.PutUint64(poolData[73:81], 1)

	tokenAVaultData := make([]byte, 72)
	tokenBVaultData := make([]byte, 72)
	mintAData := make([]byte, 45)
	mintBData := make([]byte, 45)
	binary.LittleEndian.PutUint64(tokenAVaultData[64:72], 2_000_000)
	binary.LittleEndian.PutUint64(tokenBVaultData[64:72], 5_000_000_000)
	mintAData[44] = 6
	mintBData[44] = 9

	orcaProgram := solana.MustPublicKeyFromBase58("whirLbMiicVdio4qvUfM5KAg6Ct8VwpYzGff3uctyCc")
	service := NewService(Config{
		RPCClient: &marketMockRPC{
			getAccountFn: func(_ context.Context, key solana.PublicKey) (*rpc.AccountInfo, error) {
				if key.Equals(pool) {
					return &rpc.AccountInfo{Exists: true, Owner: orcaProgram, Data: poolData}, nil
				}
				return &rpc.AccountInfo{Exists: false}, nil
			},
			getMultipleAccountsFn: func(context.Context, []solana.PublicKey) ([]*rpc.AccountInfo, error) {
				return []*rpc.AccountInfo{
					{Exists: true, Data: tokenAVaultData},
					{Exists: true, Data: tokenBVaultData},
					{Exists: true, Data: mintAData},
					{Exists: true, Data: mintBData},
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
			Dex:         DexOrca,
			PoolVersion: PoolVersionOrcaWhirlpool,
			PoolAddress: pool,
		},
	})
	if err != nil {
		t.Fatalf("expected successful orca whirlpool response, got %v", err)
	}
	if resp.PriceOfAInSOL.IsZero() {
		t.Fatal("expected non-zero orca whirlpool price in SOL")
	}
}

func TestCustomFactoryOverridesDefaultRoute(t *testing.T) {
	var called bool
	client, err := NewClient(
		WithPoolCalculatorFactory(PoolRoute{
			Dex:         DexPumpfun,
			PoolVersion: PoolVersionPumpfunAmm,
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
			PoolVersion: PoolVersionPumpfunAmm,
			PoolAddress: solana.SolMint,
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
		solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112"),
		solana.MustPublicKeyFromBase58("11111111111111111111111111111111"),
		decimal.NewFromInt(1),
		decimal.NewFromInt(2),
		decimal.NewFromInt(3),
		decimal.NewFromInt(4),
		decimal.NewFromInt(5),
		decimal.NewFromInt(6),
		decimal.NewFromInt(7),
		decimal.NewFromInt(8),
		"supply",
		map[string]any{"k": "v"},
	)
	if resp.SupplyMethod != "supply" || resp.Metadata["k"] != "v" || !resp.FDVInSOL.Equal(decimal.NewFromInt(6)) {
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
			PoolVersion: PoolVersionPumpfunAmm,
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
			PoolAddress: solana.SolMint,
		},
	})
	if err == nil {
		t.Fatal("expected wrapped amm error")
	}
}

func TestDefaultRaydiumV4FactoryErrorMapping(t *testing.T) {
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
			Dex:         DexRaydium,
			PoolVersion: PoolVersionRaydiumLiquidityV4,
			PoolAddress: solana.SolMint,
		},
	})
	if err == nil {
		t.Fatal("expected wrapped raydium v4 error")
	}
}

func TestDefaultRaydiumCPMMFactoryErrorMapping(t *testing.T) {
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
			Dex:         DexRaydium,
			PoolVersion: PoolVersionRaydiumCPMM,
			PoolAddress: solana.SolMint,
		},
	})
	if err == nil {
		t.Fatal("expected wrapped raydium cpmm error")
	}
}

func TestDefaultRaydiumCLMMFactoryErrorMapping(t *testing.T) {
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
			Dex:         DexRaydium,
			PoolVersion: PoolVersionRaydiumCLMM,
			PoolAddress: solana.SolMint,
		},
	})
	if err == nil {
		t.Fatal("expected wrapped raydium clmm error")
	}
}

func TestDefaultRaydiumLaunchpadFactoryErrorMapping(t *testing.T) {
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
			Dex:         DexRaydium,
			PoolVersion: PoolVersionRaydiumLaunchpad,
			PoolAddress: solana.SolMint,
		},
	})
	if err == nil {
		t.Fatal("expected wrapped raydium launchpad error")
	}
}

func TestDefaultMeteoraDLMMFactoryErrorMapping(t *testing.T) {
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
			Dex:         DexMeteora,
			PoolVersion: PoolVersionMeteoraDLMM,
			PoolAddress: solana.SolMint,
		},
	})
	if err == nil {
		t.Fatal("expected wrapped meteora dlmm error")
	}
}

func TestDefaultMeteoraDBCFactoryErrorMapping(t *testing.T) {
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
			Dex:         DexMeteora,
			PoolVersion: PoolVersionMeteoraDBC,
			PoolAddress: solana.SolMint,
		},
	})
	if err == nil {
		t.Fatal("expected wrapped meteora dbc error")
	}
}

func TestDefaultMeteoraDAMMV1FactoryErrorMapping(t *testing.T) {
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
			Dex:         DexMeteora,
			PoolVersion: PoolVersionMeteoraDAMMV1,
			PoolAddress: solana.SolMint,
		},
	})
	if err == nil {
		t.Fatal("expected wrapped meteora damm v1 error")
	}
}

func TestDefaultMeteoraDAMMV2FactoryErrorMapping(t *testing.T) {
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
			Dex:         DexMeteora,
			PoolVersion: PoolVersionMeteoraDAMMV2,
			PoolAddress: solana.SolMint,
		},
	})
	if err == nil {
		t.Fatal("expected wrapped meteora damm v2 error")
	}
}

func TestDefaultOrcaWhirlpoolFactoryErrorMapping(t *testing.T) {
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
			Dex:         DexOrca,
			PoolVersion: PoolVersionOrcaWhirlpool,
			PoolAddress: solana.SolMint,
		},
	})
	if err == nil {
		t.Fatal("expected wrapped orca whirlpool error")
	}
}
