package launchpad

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
	getAccountFn func(context.Context, solana.PublicKey) (*rpc.AccountInfo, error)
}

func (m *mockRPC) GetAccount(ctx context.Context, key solana.PublicKey) (*rpc.AccountInfo, error) {
	if m.getAccountFn == nil {
		return nil, nil
	}
	return m.getAccountFn(ctx, key)
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

func buildPoolData(baseMint, quoteMint, config, platformCfg, baseVault, quoteVault, creator solana.PublicKey) []byte {
	data := make([]byte, poolStateLength)
	copy(data[:8], poolStateDiscriminator)
	binary.LittleEndian.PutUint64(data[poolEpochOffset:poolEpochOffset+8], 957)
	data[poolAuthBumpOffset] = 250
	data[poolStatusOffset] = 0
	data[poolBaseDecimalsOffset] = 6
	data[poolQuoteDecimalsOffset] = 9
	data[poolMigrateTypeOffset] = 0
	binary.LittleEndian.PutUint64(data[poolSupplyOffset:poolSupplyOffset+8], 1_000_000_000_000_000)
	binary.LittleEndian.PutUint64(data[poolTotalSellBaseOffset:poolTotalSellBaseOffset+8], 793_100_000_000_000)
	binary.LittleEndian.PutUint64(data[poolVirtualBaseOffset:poolVirtualBaseOffset+8], 1_073_025_605_596_382)
	binary.LittleEndian.PutUint64(data[poolVirtualQuoteOffset:poolVirtualQuoteOffset+8], 30_000_852_951)
	binary.LittleEndian.PutUint64(data[poolRealBaseOffset:poolRealBaseOffset+8], 727_865_254_549_778)
	binary.LittleEndian.PutUint64(data[poolRealQuoteOffset:poolRealQuoteOffset+8], 63_265_025_701)
	binary.LittleEndian.PutUint64(data[poolTotalRaiseOffset:poolTotalRaiseOffset+8], 85_000_000_000)
	binary.LittleEndian.PutUint64(data[poolProtocolFeeOffset:poolProtocolFeeOffset+8], 186_997_449)
	binary.LittleEndian.PutUint64(data[poolPlatformFeeOffset:poolPlatformFeeOffset+8], 100_000_000)
	binary.LittleEndian.PutUint64(data[poolMigrateFeeOffset:poolMigrateFeeOffset+8], 0)
	binary.LittleEndian.PutUint64(data[poolVestingTotalLockedOffset:poolVestingTotalLockedOffset+8], 5_000_000_000)
	binary.LittleEndian.PutUint64(data[poolVestingCliffOffset:poolVestingCliffOffset+8], 120)
	binary.LittleEndian.PutUint64(data[poolVestingUnlockOffset:poolVestingUnlockOffset+8], 360)
	binary.LittleEndian.PutUint64(data[poolVestingStartOffset:poolVestingStartOffset+8], 1_700_000_000)
	binary.LittleEndian.PutUint64(data[poolVestingAllocatedOffset:poolVestingAllocatedOffset+8], 100_000_000)
	copy(data[poolConfigOffset:poolConfigOffset+32], config.Bytes())
	copy(data[poolPlatformCfgOffset:poolPlatformCfgOffset+32], platformCfg.Bytes())
	copy(data[poolBaseMintOffset:poolBaseMintOffset+32], baseMint.Bytes())
	copy(data[poolQuoteMintOffset:poolQuoteMintOffset+32], quoteMint.Bytes())
	copy(data[poolBaseVaultOffset:poolBaseVaultOffset+32], baseVault.Bytes())
	copy(data[poolQuoteVaultOffset:poolQuoteVaultOffset+32], quoteVault.Bytes())
	copy(data[poolCreatorOffset:poolCreatorOffset+32], creator.Bytes())
	data[poolTokenProgramOffset] = 1
	data[poolCpmmFeeOnOffset] = 0
	return data
}

func buildConfigData(curveType uint8, quoteMint solana.PublicKey) []byte {
	data := make([]byte, configStateLength)
	copy(data[:8], globalConfigDiscriminator)
	data[configCurveTypeOffset] = curveType
	binary.LittleEndian.PutUint16(data[configIndexOffset:configIndexOffset+2], 7)
	binary.LittleEndian.PutUint64(data[configMigrateFeeOffset:configMigrateFeeOffset+8], 1_000_000_000)
	binary.LittleEndian.PutUint64(data[configTradeFeeRateOffset:configTradeFeeRateOffset+8], 10_000)
	binary.LittleEndian.PutUint64(data[configMaxShareFeeRate:configMaxShareFeeRate+8], 20_000)
	binary.LittleEndian.PutUint64(data[configMinSupplyOffset:configMinSupplyOffset+8], 1_000_000_000)
	binary.LittleEndian.PutUint64(data[configMaxLockRateOffset:configMaxLockRateOffset+8], 900_000)
	binary.LittleEndian.PutUint64(data[configMinSellRateOffset:configMinSellRateOffset+8], 700_000)
	binary.LittleEndian.PutUint64(data[configMinMigrateRateOffset:configMinMigrateRateOffset+8], 100_000)
	binary.LittleEndian.PutUint64(data[configMinRaiseOffset:configMinRaiseOffset+8], 10_000_000_000)
	copy(data[configQuoteMintOffset:configQuoteMintOffset+32], quoteMint.Bytes())
	return data
}

func TestComputeConstantProductSuccess(t *testing.T) {
	pool := testPubkey(1)
	config := testPubkey(2)
	baseMint := testPubkey(3)
	quoteMint := solana.SolMint
	platformCfg := testPubkey(4)
	baseVault := testPubkey(5)
	quoteVault := testPubkey(6)
	creator := testPubkey(7)

	poolData := buildPoolData(baseMint, quoteMint, config, platformCfg, baseVault, quoteVault, creator)
	configData := buildConfigData(curveTypeConstantProduct, quoteMint)
	calc := NewCalculator(&mockRPC{
		getAccountFn: func(_ context.Context, key solana.PublicKey) (*rpc.AccountInfo, error) {
			switch {
			case key.Equals(pool):
				return &rpc.AccountInfo{Exists: true, Owner: launchpadProgramID, Data: poolData}, nil
			case key.Equals(config):
				return &rpc.AccountInfo{Exists: true, Owner: launchpadProgramID, Data: configData}, nil
			default:
				return &rpc.AccountInfo{Exists: false}, nil
			}
		},
	}, nil, &mockSupply{
		total:  decimal.NewFromInt(1_000_000),
		circ:   decimal.NewFromInt(900_000),
		method: "mock_supply",
	})

	resp, err := calc.Compute(context.Background(), Request{
		PoolAddress: pool,
		MintA:       baseMint,
		MintB:       quoteMint,
	})
	if err != nil {
		t.Fatalf("compute failed: %v", err)
	}

	state, _ := decodePoolState(poolData)
	cfg, _ := decodeConfigState(configData)
	basePrice, _ := priceBaseInQuoteByCurve(state, cfg.curveType)
	expectedLiq := liquidityInMintB(Request{MintB: quoteMint}, reserveForPoolState(state), basePrice)
	if !resp.PriceOfAInB.Equal(basePrice) {
		t.Fatalf("unexpected price: got=%s want=%s", resp.PriceOfAInB, basePrice)
	}
	if !resp.LiquidityInB.Equal(expectedLiq) {
		t.Fatalf("unexpected liquidity: got=%s want=%s", resp.LiquidityInB, expectedLiq)
	}
	if resp.Metadata["pool_curve_type"] != "constant_product" {
		t.Fatalf("expected constant_product curve metadata, got %#v", resp.Metadata["pool_curve_type"])
	}
	if !resp.FDVInSOL.GreaterThan(resp.MarketCapInSOL) {
		t.Fatalf("expected fdv > market cap, got fdv=%s market=%s", resp.FDVInSOL, resp.MarketCapInSOL)
	}
}

func TestComputeReversePairSuccess(t *testing.T) {
	pool := testPubkey(11)
	config := testPubkey(12)
	baseMint := testPubkey(13)
	quoteMint := solana.SolMint

	poolData := buildPoolData(baseMint, quoteMint, config, testPubkey(14), testPubkey(15), testPubkey(16), testPubkey(17))
	configData := buildConfigData(curveTypeFixedPrice, quoteMint)
	calc := NewCalculator(&mockRPC{
		getAccountFn: func(_ context.Context, key solana.PublicKey) (*rpc.AccountInfo, error) {
			if key.Equals(pool) {
				return &rpc.AccountInfo{Exists: true, Owner: launchpadProgramID, Data: poolData}, nil
			}
			if key.Equals(config) {
				return &rpc.AccountInfo{Exists: true, Owner: launchpadProgramID, Data: configData}, nil
			}
			return &rpc.AccountInfo{Exists: false}, nil
		},
	}, nil, &mockSupply{
		total:  decimal.NewFromInt(500_000),
		circ:   decimal.NewFromInt(400_000),
		method: "mock",
	})

	resp, err := calc.Compute(context.Background(), Request{
		PoolAddress: pool,
		MintA:       quoteMint,
		MintB:       baseMint,
	})
	if err != nil {
		t.Fatalf("compute failed: %v", err)
	}
	if resp.PriceOfAInB.IsZero() {
		t.Fatal("expected reverse price to be non-zero")
	}
	if resp.Metadata["pool_curve_type"] != "fixed_price" {
		t.Fatalf("unexpected curve metadata: %#v", resp.Metadata["pool_curve_type"])
	}
}

func TestComputeValidationAndPoolErrors(t *testing.T) {
	if _, err := NewCalculator(nil, nil, &mockSupply{}).Compute(context.Background(), Request{}); err == nil {
		t.Fatal("expected rpc required error")
	}
	if _, err := NewCalculator(&mockRPC{}, nil, nil).Compute(context.Background(), Request{}); err == nil {
		t.Fatal("expected supply required error")
	}
	if _, err := NewCalculator(&mockRPC{}, nil, &mockSupply{}).Compute(context.Background(), Request{}); err == nil {
		t.Fatal("expected pool address required error")
	}
	if _, err := NewCalculator(&mockRPC{}, nil, &mockSupply{}).Compute(context.Background(), Request{PoolAddress: testPubkey(20)}); err == nil {
		t.Fatal("expected mints required error")
	}

	calc := NewCalculator(&mockRPC{
		getAccountFn: func(context.Context, solana.PublicKey) (*rpc.AccountInfo, error) {
			return nil, errors.New("rpc failed")
		},
	}, nil, &mockSupply{})
	if _, err := calc.Compute(context.Background(), Request{PoolAddress: testPubkey(21), MintA: testPubkey(22), MintB: testPubkey(23)}); err == nil {
		t.Fatal("expected rpc error")
	}

	calc = NewCalculator(&mockRPC{
		getAccountFn: func(context.Context, solana.PublicKey) (*rpc.AccountInfo, error) {
			return &rpc.AccountInfo{Exists: false}, nil
		},
	}, nil, &mockSupply{})
	if _, err := calc.Compute(context.Background(), Request{PoolAddress: testPubkey(21), MintA: testPubkey(22), MintB: testPubkey(23)}); err == nil {
		t.Fatal("expected pool not found")
	}

	calc = NewCalculator(&mockRPC{
		getAccountFn: func(context.Context, solana.PublicKey) (*rpc.AccountInfo, error) {
			return &rpc.AccountInfo{Exists: true, Owner: testPubkey(99), Data: make([]byte, poolStateLength)}, nil
		},
	}, nil, &mockSupply{})
	if _, err := calc.Compute(context.Background(), Request{PoolAddress: testPubkey(21), MintA: testPubkey(22), MintB: testPubkey(23)}); err == nil {
		t.Fatal("expected invalid owner error")
	}

	shortData := make([]byte, poolStateLength-1)
	copy(shortData[:8], poolStateDiscriminator)
	calc = NewCalculator(&mockRPC{
		getAccountFn: func(context.Context, solana.PublicKey) (*rpc.AccountInfo, error) {
			return &rpc.AccountInfo{Exists: true, Owner: launchpadProgramID, Data: shortData}, nil
		},
	}, nil, &mockSupply{})
	if _, err := calc.Compute(context.Background(), Request{PoolAddress: testPubkey(21), MintA: testPubkey(22), MintB: testPubkey(23)}); err == nil {
		t.Fatal("expected short data decode error")
	}

	invalidDisc := make([]byte, poolStateLength)
	calc = NewCalculator(&mockRPC{
		getAccountFn: func(context.Context, solana.PublicKey) (*rpc.AccountInfo, error) {
			return &rpc.AccountInfo{Exists: true, Owner: launchpadProgramID, Data: invalidDisc}, nil
		},
	}, nil, &mockSupply{})
	if _, err := calc.Compute(context.Background(), Request{PoolAddress: testPubkey(21), MintA: testPubkey(22), MintB: testPubkey(23)}); err == nil {
		t.Fatal("expected invalid discriminator error")
	}
}

func TestComputeConfigAndCurveErrors(t *testing.T) {
	pool := testPubkey(31)
	config := testPubkey(32)
	baseMint := testPubkey(33)
	quoteMint := solana.SolMint
	poolData := buildPoolData(baseMint, quoteMint, config, testPubkey(34), testPubkey(35), testPubkey(36), testPubkey(37))

	baseReq := Request{PoolAddress: pool, MintA: baseMint, MintB: quoteMint}
	makeCalculator := func(configInfo *rpc.AccountInfo) *Calculator {
		return NewCalculator(&mockRPC{
			getAccountFn: func(_ context.Context, key solana.PublicKey) (*rpc.AccountInfo, error) {
				if key.Equals(pool) {
					return &rpc.AccountInfo{Exists: true, Owner: launchpadProgramID, Data: poolData}, nil
				}
				if key.Equals(config) {
					return configInfo, nil
				}
				return &rpc.AccountInfo{Exists: false}, nil
			},
		}, nil, &mockSupply{total: decimal.NewFromInt(1), circ: decimal.NewFromInt(1), method: "m"})
	}
	calcConfigRPCError := NewCalculator(&mockRPC{
		getAccountFn: func(_ context.Context, key solana.PublicKey) (*rpc.AccountInfo, error) {
			if key.Equals(pool) {
				return &rpc.AccountInfo{Exists: true, Owner: launchpadProgramID, Data: poolData}, nil
			}
			if key.Equals(config) {
				return nil, errors.New("config rpc failure")
			}
			return &rpc.AccountInfo{Exists: false}, nil
		},
	}, nil, &mockSupply{total: decimal.NewFromInt(1), circ: decimal.NewFromInt(1), method: "m"})
	if _, err := calcConfigRPCError.Compute(context.Background(), baseReq); err == nil {
		t.Fatal("expected config rpc error")
	}

	if _, err := makeCalculator(&rpc.AccountInfo{Exists: false}).Compute(context.Background(), baseReq); err == nil {
		t.Fatal("expected missing config error")
	}
	if _, err := makeCalculator(&rpc.AccountInfo{Exists: true, Owner: testPubkey(88), Data: buildConfigData(curveTypeConstantProduct, quoteMint)}).Compute(context.Background(), baseReq); err == nil {
		t.Fatal("expected invalid config owner error")
	}
	badConfigLen := make([]byte, configStateLength-1)
	copy(badConfigLen[:8], globalConfigDiscriminator)
	if _, err := makeCalculator(&rpc.AccountInfo{Exists: true, Owner: launchpadProgramID, Data: badConfigLen}).Compute(context.Background(), baseReq); err == nil {
		t.Fatal("expected short config decode error")
	}
	if _, err := makeCalculator(&rpc.AccountInfo{Exists: true, Owner: launchpadProgramID, Data: make([]byte, configStateLength)}).Compute(context.Background(), baseReq); err == nil {
		t.Fatal("expected bad config discriminator error")
	}
	if _, err := makeCalculator(&rpc.AccountInfo{Exists: true, Owner: launchpadProgramID, Data: buildConfigData(curveTypeConstantProduct, testPubkey(90))}).Compute(context.Background(), baseReq); err == nil {
		t.Fatal("expected quote mint mismatch")
	}
	if _, err := makeCalculator(&rpc.AccountInfo{Exists: true, Owner: launchpadProgramID, Data: buildConfigData(99, quoteMint)}).Compute(context.Background(), baseReq); err == nil {
		t.Fatal("expected unsupported curve error")
	}

	calcMismatch := makeCalculator(&rpc.AccountInfo{Exists: true, Owner: launchpadProgramID, Data: buildConfigData(curveTypeConstantProduct, quoteMint)})
	if _, err := calcMismatch.Compute(context.Background(), Request{
		PoolAddress: pool,
		MintA:       testPubkey(111),
		MintB:       quoteMint,
	}); err == nil {
		t.Fatal("expected pool/request mint mismatch")
	}
}

func TestComputeQuoteAndSupplyErrors(t *testing.T) {
	pool := testPubkey(41)
	config := testPubkey(42)
	baseMint := testPubkey(43)
	quoteMint := testPubkey(44)
	poolData := buildPoolData(baseMint, quoteMint, config, testPubkey(45), testPubkey(46), testPubkey(47), testPubkey(48))
	configData := buildConfigData(curveTypeLinearPrice, quoteMint)

	makeRPC := func() *mockRPC {
		return &mockRPC{
			getAccountFn: func(_ context.Context, key solana.PublicKey) (*rpc.AccountInfo, error) {
				if key.Equals(pool) {
					return &rpc.AccountInfo{Exists: true, Owner: launchpadProgramID, Data: poolData}, nil
				}
				if key.Equals(config) {
					return &rpc.AccountInfo{Exists: true, Owner: launchpadProgramID, Data: configData}, nil
				}
				return &rpc.AccountInfo{Exists: false}, nil
			},
		}
	}
	req := Request{PoolAddress: pool, MintA: baseMint, MintB: quoteMint}

	calc := NewCalculator(makeRPC(), &mockQuote{err: errors.New("quote fail")}, &mockSupply{total: decimal.NewFromInt(1), circ: decimal.NewFromInt(1), method: "m"})
	if _, err := calc.Compute(context.Background(), req); err == nil {
		t.Fatal("expected quote conversion error")
	}

	calc = NewCalculator(makeRPC(), &sequenceQuote{
		values: []decimal.Decimal{decimal.NewFromInt(2)},
		errors: []error{nil, errors.New("liquidity quote fail")},
	}, &mockSupply{total: decimal.NewFromInt(1), circ: decimal.NewFromInt(1), method: "m"})
	if _, err := calc.Compute(context.Background(), req); err == nil {
		t.Fatal("expected liquidity quote conversion error")
	}

	calc = NewCalculator(makeRPC(), &mockQuote{value: decimal.NewFromInt(1)}, &mockSupply{err: errors.New("supply fail")})
	if _, err := calc.Compute(context.Background(), req); err == nil {
		t.Fatal("expected supply error")
	}
}

func TestCurveAndHelperFunctions(t *testing.T) {
	state := poolState{
		baseDecimals:     6,
		quoteDecimals:    9,
		virtualBaseRaw:   1_000_000_000,
		virtualQuoteRaw:  3_000_000_000,
		realBaseRaw:      500_000_000,
		realQuoteRaw:     2_000_000_000,
		totalSellBaseRaw: 900_000_000,
		baseMint:         testPubkey(51),
		quoteMint:        solana.SolMint,
	}

	if got, err := priceBaseInQuoteByCurve(state, curveTypeConstantProduct); err != nil || got.IsZero() {
		t.Fatalf("expected constant product price > 0, got %s err=%v", got, err)
	}
	if got, err := priceBaseInQuoteByCurve(state, curveTypeFixedPrice); err != nil || got.IsZero() {
		t.Fatalf("expected fixed price > 0, got %s err=%v", got, err)
	}
	if got, err := priceBaseInQuoteByCurve(state, curveTypeLinearPrice); err != nil || got.IsZero() {
		t.Fatalf("expected linear price > 0, got %s err=%v", got, err)
	}
	if _, err := priceBaseInQuoteByCurve(state, 99); err == nil {
		t.Fatal("expected unsupported curve error")
	}

	state.virtualBaseRaw = state.realBaseRaw
	if got, _ := priceBaseInQuoteByCurve(state, curveTypeConstantProduct); !got.IsZero() {
		t.Fatalf("expected zero cp price when virtual<=real, got %s", got)
	}
	state.virtualBaseRaw = 0
	if got, _ := priceBaseInQuoteByCurve(state, curveTypeFixedPrice); !got.IsZero() {
		t.Fatalf("expected zero fixed price when virtual base is zero, got %s", got)
	}
	state.virtualBaseRaw = 1
	state.realBaseRaw = 0
	if got, _ := priceBaseInQuoteByCurve(state, curveTypeLinearPrice); !got.IsZero() {
		t.Fatalf("expected zero linear price when real base is zero, got %s", got)
	}
	state.virtualBaseRaw = 0
	state.realBaseRaw = 1
	if got, _ := priceBaseInQuoteByCurve(state, curveTypeLinearPrice); !got.IsZero() {
		t.Fatalf("expected zero linear price when virtual base is zero, got %s", got)
	}

	snap := &reserveSnapshot{
		baseMint:     testPubkey(61),
		quoteMint:    solana.SolMint,
		baseReserve:  decimal.NewFromInt(10),
		quoteReserve: decimal.NewFromInt(5),
	}
	price := decimal.RequireFromString("0.5")
	if got := priceOfMintAInMintB(Request{MintA: snap.baseMint, MintB: snap.quoteMint}, snap, price); !got.Equal(price) {
		t.Fatalf("expected direct price, got %s", got)
	}
	if got := priceOfMintAInMintB(Request{MintA: snap.quoteMint, MintB: snap.baseMint}, snap, price); !got.Equal(decimal.NewFromInt(2)) {
		t.Fatalf("expected inverse price, got %s", got)
	}
	if got := priceOfMintAInMintB(Request{MintA: testPubkey(62), MintB: testPubkey(63)}, snap, price); !got.IsZero() {
		t.Fatalf("expected zero unmatched price, got %s", got)
	}
	if got := priceOfMintAInMintB(Request{MintA: snap.baseMint, MintB: snap.quoteMint}, nil, price); !got.IsZero() {
		t.Fatalf("expected zero nil snapshot price, got %s", got)
	}
	if got := priceOfMintAInMintB(Request{MintA: snap.baseMint, MintB: snap.quoteMint}, snap, decimal.Zero); !got.IsZero() {
		t.Fatalf("expected zero when base price is zero, got %s", got)
	}

	if got := liquidityInMintB(Request{MintB: snap.quoteMint}, snap, price); !got.Equal(decimal.NewFromInt(10)) {
		t.Fatalf("expected quote liquidity 10, got %s", got)
	}
	if got := liquidityInMintB(Request{MintB: snap.baseMint}, snap, price); !got.Equal(decimal.NewFromInt(20)) {
		t.Fatalf("expected base liquidity 20, got %s", got)
	}
	if got := liquidityInMintB(Request{MintB: snap.baseMint}, snap, decimal.Zero); !got.Equal(decimal.NewFromInt(10)) {
		t.Fatalf("expected fallback base reserve on zero price, got %s", got)
	}
	if got := liquidityInMintB(Request{MintB: testPubkey(64)}, snap, price); !got.IsZero() {
		t.Fatalf("expected zero unmatched liquidity, got %s", got)
	}
	if got := liquidityInMintB(Request{MintB: testPubkey(64)}, nil, price); !got.IsZero() {
		t.Fatalf("expected zero nil snapshot liquidity, got %s", got)
	}

	if curveTypeName(0) != "constant_product" || curveTypeName(1) != "fixed_price" || curveTypeName(2) != "linear_price" || curveTypeName(99) != "unknown" {
		t.Fatal("unexpected curveTypeName mapping")
	}
}

func TestDecodeAndConversionHelpers(t *testing.T) {
	quoteMint := solana.SolMint
	poolData := buildPoolData(testPubkey(71), quoteMint, testPubkey(72), testPubkey(73), testPubkey(74), testPubkey(75), testPubkey(76))
	configData := buildConfigData(curveTypeConstantProduct, quoteMint)

	state, err := decodePoolState(poolData)
	if err != nil {
		t.Fatalf("decodePoolState failed: %v", err)
	}
	cfg, err := decodeConfigState(configData)
	if err != nil {
		t.Fatalf("decodeConfigState failed: %v", err)
	}
	if state.quoteMint != quoteMint || cfg.quoteMint != quoteMint {
		t.Fatal("decoded mints mismatch")
	}
	if _, err := decodePoolState(make([]byte, poolStateLength-1)); err == nil {
		t.Fatal("expected short pool decode error")
	}
	if _, err := decodePoolState(make([]byte, poolStateLength)); err == nil {
		t.Fatal("expected invalid pool discriminator error")
	}
	if _, err := decodeConfigState(make([]byte, configStateLength-1)); err == nil {
		t.Fatal("expected short config decode error")
	}
	if _, err := decodeConfigState(make([]byte, configStateLength)); err == nil {
		t.Fatal("expected invalid config discriminator error")
	}

	calc := NewCalculator(&mockRPC{}, nil, &mockSupply{})
	if got, err := calc.priceOfMintAInSOL(context.Background(), Request{MintA: solana.SolMint, MintB: testPubkey(80)}, decimal.NewFromInt(9)); err != nil || !got.Equal(decimal.NewFromInt(1)) {
		t.Fatalf("mintA SOL conversion mismatch, got=%s err=%v", got, err)
	}
	if got, err := calc.priceOfMintAInSOL(context.Background(), Request{MintA: testPubkey(81), MintB: solana.SolMint}, decimal.NewFromInt(9)); err != nil || !got.Equal(decimal.NewFromInt(9)) {
		t.Fatalf("mintB SOL conversion mismatch, got=%s err=%v", got, err)
	}
	if got, err := calc.priceOfMintAInSOL(context.Background(), Request{MintA: testPubkey(82), MintB: testPubkey(83)}, decimal.NewFromInt(9)); err != nil || !got.IsZero() {
		t.Fatalf("expected zero without quote bridge, got=%s err=%v", got, err)
	}
	calc.quotes = &mockQuote{value: decimal.Zero}
	if got, err := calc.priceOfMintAInSOL(context.Background(), Request{MintA: testPubkey(84), MintB: testPubkey(85)}, decimal.NewFromInt(9)); err != nil || !got.IsZero() {
		t.Fatalf("expected zero when quote bridge returns zero, got=%s err=%v", got, err)
	}
	calc.quotes = &mockQuote{value: decimal.NewFromInt(2)}
	if got, err := calc.priceOfMintAInSOL(context.Background(), Request{MintA: testPubkey(86), MintB: testPubkey(87)}, decimal.NewFromInt(9)); err != nil || !got.Equal(decimal.NewFromInt(18)) {
		t.Fatalf("expected bridged conversion, got=%s err=%v", got, err)
	}

	calc.quotes = nil
	if got, err := calc.liquidityInSOL(context.Background(), Request{MintB: solana.SolMint}, decimal.NewFromInt(5)); err != nil || !got.Equal(decimal.NewFromInt(5)) {
		t.Fatalf("expected liquidity passthrough for SOL, got=%s err=%v", got, err)
	}
	if got, err := calc.liquidityInSOL(context.Background(), Request{MintB: testPubkey(88)}, decimal.NewFromInt(5)); err != nil || !got.IsZero() {
		t.Fatalf("expected zero without quote bridge, got=%s err=%v", got, err)
	}
	calc.quotes = &mockQuote{value: decimal.NewFromInt(3)}
	if got, err := calc.liquidityInSOL(context.Background(), Request{MintB: testPubkey(88)}, decimal.NewFromInt(5)); err != nil || !got.Equal(decimal.NewFromInt(3)) {
		t.Fatalf("expected bridged liquidity conversion, got=%s err=%v", got, err)
	}
	calc.quotes = &mockQuote{err: errors.New("bridge fail")}
	if _, err := calc.liquidityInSOL(context.Background(), Request{MintB: testPubkey(89)}, decimal.NewFromInt(5)); err == nil {
		t.Fatal("expected liquidity bridge error")
	}

	if !mintsEquivalent(solana.SolMint, solana.SolMint) {
		t.Fatal("expected identical mint equivalence")
	}
	if mintsEquivalent(testPubkey(90), testPubkey(91)) {
		t.Fatal("unexpected equivalence for different mints")
	}
	if !poolMatchesRequest(Request{MintA: state.baseMint, MintB: state.quoteMint}, state.baseMint, state.quoteMint) {
		t.Fatal("expected direct pool match")
	}
	if !poolMatchesRequest(Request{MintA: state.quoteMint, MintB: state.baseMint}, state.baseMint, state.quoteMint) {
		t.Fatal("expected reversed pool match")
	}
	if poolMatchesRequest(Request{MintA: testPubkey(92), MintB: state.baseMint}, state.baseMint, state.quoteMint) {
		t.Fatal("unexpected pool match")
	}
}
