package damm_v1

import (
	"context"
	"encoding/binary"
	"errors"
	"math"
	"testing"
	"time"

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

func testPubkey(seed byte) solana.PublicKey {
	out := make([]byte, 32)
	for i := range out {
		out[i] = seed
	}
	return solana.PublicKeyFromBytes(out)
}

func makePoolData(
	lpMint solana.PublicKey,
	tokenAMint solana.PublicKey,
	tokenBMint solana.PublicKey,
	aVault solana.PublicKey,
	bVault solana.PublicKey,
	aVaultLPToken solana.PublicKey,
	bVaultLPToken solana.PublicKey,
) []byte {
	data := make([]byte, poolMinDataSize)
	copy(data[:8], poolDiscriminator)
	copy(data[lpMintOffset:lpMintOffset+32], lpMint.Bytes())
	copy(data[tokenAMintOffset:tokenAMintOffset+32], tokenAMint.Bytes())
	copy(data[tokenBMintOffset:tokenBMintOffset+32], tokenBMint.Bytes())
	copy(data[aVaultOffset:aVaultOffset+32], aVault.Bytes())
	copy(data[bVaultOffset:bVaultOffset+32], bVault.Bytes())
	copy(data[aVaultLPOffset:aVaultLPOffset+32], aVaultLPToken.Bytes())
	copy(data[bVaultLPOffset:bVaultLPOffset+32], bVaultLPToken.Bytes())
	copy(data[protocolTokenAFeeOffset:protocolTokenAFeeOffset+32], testPubkey(81).Bytes())
	copy(data[protocolTokenBFeeOffset:protocolTokenBFeeOffset+32], testPubkey(82).Bytes())
	copy(data[stakeOffset:stakeOffset+32], testPubkey(83).Bytes())
	data[aVaultLPBumpOffset] = 255
	data[enabledOffset] = 1
	binary.LittleEndian.PutUint64(data[feeLastUpdatedAtOffset:feeLastUpdatedAtOffset+8], 123)
	data[poolTypeOffset] = 0
	binary.LittleEndian.PutUint64(data[totalLockedLPOffset:totalLockedLPOffset+8], 456)
	data[curveTypeTagOffset] = 0
	return data
}

func makeTokenAccountData(mint solana.PublicKey, amount uint64) []byte {
	data := make([]byte, tokenAccountMinLength)
	copy(data[tokenAccountMintOffset:tokenAccountMintOffset+32], mint.Bytes())
	binary.LittleEndian.PutUint64(data[tokenAmountOffset:tokenAmountOffset+8], amount)
	return data
}

func makeMintData(decimals uint8, supply uint64) []byte {
	data := make([]byte, mintAccountMinSize)
	binary.LittleEndian.PutUint64(data[mintSupplyOffset:mintSupplyOffset+8], supply)
	data[mintDecimalsOffset] = decimals
	return data
}

func makeVaultData(
	tokenVault solana.PublicKey,
	lpMint solana.PublicKey,
	totalAmount uint64,
	lastLockedProfit uint64,
	lastReport uint64,
	profitDegrade uint64,
) []byte {
	data := make([]byte, vaultMinDataSize)
	binary.LittleEndian.PutUint64(data[vaultTotalAmountOffset:vaultTotalAmountOffset+8], totalAmount)
	copy(data[vaultTokenVaultOffset:vaultTokenVaultOffset+32], tokenVault.Bytes())
	copy(data[vaultLPMintOffset:vaultLPMintOffset+32], lpMint.Bytes())
	binary.LittleEndian.PutUint64(data[vaultLastLockedProfitOffset:vaultLastLockedProfitOffset+8], lastLockedProfit)
	binary.LittleEndian.PutUint64(data[vaultLastReportOffset:vaultLastReportOffset+8], lastReport)
	binary.LittleEndian.PutUint64(data[vaultProfitDegradeOffset:vaultProfitDegradeOffset+8], profitDegrade)
	return data
}

func makeClockData(unixTimestamp int64) []byte {
	data := make([]byte, clockMinDataSize)
	binary.LittleEndian.PutUint64(data[clockUnixTimestampOffset:clockUnixTimestampOffset+8], uint64(unixTimestamp))
	return data
}

func TestCompute_UsesPoolAndReserves(t *testing.T) {
	pool := testPubkey(1)
	lpMint := testPubkey(2)
	mintA := solana.MustPublicKeyFromBase58("EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")
	mintB := solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")
	aVault := testPubkey(3)
	bVault := testPubkey(4)
	aVaultLPToken := testPubkey(5)
	bVaultLPToken := testPubkey(6)
	aVaultLPMint := testPubkey(7)
	bVaultLPMint := testPubkey(8)
	clockTimestamp := time.Now().Unix()

	poolData := makePoolData(lpMint, mintA, mintB, aVault, bVault, aVaultLPToken, bVaultLPToken)
	mockRPCClient := &mockRPC{
		accounts: map[string]*rpc.AccountInfo{
			pool.String():          {Address: pool, Owner: dammV1ProgramID, Exists: true, Data: poolData},
			aVaultLPToken.String(): {Address: aVaultLPToken, Exists: true, Data: makeTokenAccountData(aVaultLPMint, 20_000)},
			bVaultLPToken.String(): {Address: bVaultLPToken, Exists: true, Data: makeTokenAccountData(bVaultLPMint, 50_000)},
			aVault.String():        {Address: aVault, Owner: vaultProgramID, Exists: true, Data: makeVaultData(testPubkey(11), aVaultLPMint, 10_000_000, 0, uint64(clockTimestamp), 0)},
			bVault.String():        {Address: bVault, Owner: vaultProgramID, Exists: true, Data: makeVaultData(testPubkey(12), bVaultLPMint, 10_000_000_000, 0, uint64(clockTimestamp), 0)},
			mintA.String():         {Address: mintA, Exists: true, Data: makeMintData(6, 9_094_499_663)},
			mintB.String():         {Address: mintB, Exists: true, Data: makeMintData(9, 0)},
			clockSysvarID.String(): {Address: clockSysvarID, Exists: true, Data: makeClockData(clockTimestamp)},
			aVaultLPMint.String():  {Address: aVaultLPMint, Exists: true, Data: makeMintData(6, 100_000)},
			bVaultLPMint.String():  {Address: bVaultLPMint, Exists: true, Data: makeMintData(9, 100_000)},
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

	if got := resp.PriceOfAInB.String(); got != "2.5" {
		t.Fatalf("unexpected price_a_in_b: %s", got)
	}
	if got := resp.LiquidityInB.String(); got != "10" {
		t.Fatalf("unexpected liquidity_in_b: %s", got)
	}
	if got := resp.PriceOfAInSOL.String(); got != "2.5" {
		t.Fatalf("unexpected price_a_in_sol: %s", got)
	}
	if got := resp.LiquidityInSOL.String(); got != "10" {
		t.Fatalf("unexpected liquidity_in_sol: %s", got)
	}
	if got := resp.MarketCapInSOL.String(); got != "2000000" {
		t.Fatalf("unexpected market cap: %s", got)
	}
	if got := resp.FDVInSOL.String(); got != "2500000" {
		t.Fatalf("unexpected fdv: %s", got)
	}
	if resp.Metadata["pool_curve_type"] != "constant_product" {
		t.Fatalf("unexpected curve type metadata: %#v", resp.Metadata["pool_curve_type"])
	}
	if mockRPCClient.getAccountCalls != 2 || mockRPCClient.getMultipleCalls != 2 {
		t.Fatalf("expected two getAccount calls and two batch calls, got account=%d multiple=%d", mockRPCClient.getAccountCalls, mockRPCClient.getMultipleCalls)
	}
}

func TestCompute_ReversedPoolMintOrder(t *testing.T) {
	pool := testPubkey(111)
	lpMint := testPubkey(112)
	mintToken := solana.MustPublicKeyFromBase58("EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")
	mintSOL := solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")
	aVault := testPubkey(113)
	bVault := testPubkey(114)
	aVaultLPToken := testPubkey(115)
	bVaultLPToken := testPubkey(116)
	aVaultLPMint := testPubkey(117)
	bVaultLPMint := testPubkey(118)
	clockTimestamp := time.Now().Unix()

	// Pool token order is reversed relative to request: tokenA=SOL, tokenB=USDC.
	poolData := makePoolData(lpMint, mintSOL, mintToken, aVault, bVault, aVaultLPToken, bVaultLPToken)
	mockRPCClient := &mockRPC{
		accounts: map[string]*rpc.AccountInfo{
			pool.String():          {Address: pool, Owner: dammV1ProgramID, Exists: true, Data: poolData},
			aVaultLPToken.String(): {Address: aVaultLPToken, Exists: true, Data: makeTokenAccountData(aVaultLPMint, 50_000)},
			bVaultLPToken.String(): {Address: bVaultLPToken, Exists: true, Data: makeTokenAccountData(bVaultLPMint, 20_000)},
			aVault.String():        {Address: aVault, Owner: vaultProgramID, Exists: true, Data: makeVaultData(testPubkey(121), aVaultLPMint, 10_000_000_000, 0, uint64(clockTimestamp), 0)},
			bVault.String():        {Address: bVault, Owner: vaultProgramID, Exists: true, Data: makeVaultData(testPubkey(122), bVaultLPMint, 10_000_000, 0, uint64(clockTimestamp), 0)},
			mintSOL.String():       {Address: mintSOL, Exists: true, Data: makeMintData(9, 0)},
			mintToken.String():     {Address: mintToken, Exists: true, Data: makeMintData(6, 9_094_499_663)},
			clockSysvarID.String(): {Address: clockSysvarID, Exists: true, Data: makeClockData(clockTimestamp)},
			aVaultLPMint.String():  {Address: aVaultLPMint, Exists: true, Data: makeMintData(9, 100_000)},
			bVaultLPMint.String():  {Address: bVaultLPMint, Exists: true, Data: makeMintData(6, 100_000)},
		},
	}

	calc := NewCalculator(mockRPCClient, nil, &mockSupply{
		total:  decimal.NewFromInt(1_000_000),
		circ:   decimal.NewFromInt(800_000),
		method: "mock_supply",
	})
	resp, err := calc.Compute(context.Background(), Request{
		PoolAddress: pool,
		MintA:       mintToken,
		MintB:       solana.SolMint,
	})
	if err != nil {
		t.Fatalf("compute failed: %v", err)
	}
	if got := resp.PriceOfAInB.String(); got != "0.4" {
		t.Fatalf("unexpected reversed-order price_a_in_b: %s", got)
	}
	if got := resp.LiquidityInB.String(); got != "4" {
		t.Fatalf("unexpected reversed-order liquidity_in_b: %s", got)
	}
	if got := resp.LiquidityInSOL.String(); got != "0" {
		t.Fatalf("unexpected reversed-order liquidity_in_sol: %s", got)
	}
}

func TestCompute_UsesPumpCurveTotalSupplyForFDV(t *testing.T) {
	pool := testPubkey(21)
	lpMint := testPubkey(22)
	mintA := solana.MustPublicKeyFromBase58("2nCeHpECQvnMfzjU5fDMAKws1vBxMzxvWr6qqLpApump")
	mintB := solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")
	aVault := testPubkey(23)
	bVault := testPubkey(24)
	aVaultLPToken := testPubkey(25)
	bVaultLPToken := testPubkey(26)
	aVaultLPMint := testPubkey(27)
	bVaultLPMint := testPubkey(28)
	clockTimestamp := time.Now().Unix()

	mockRPCClient := &mockRPC{
		accounts: map[string]*rpc.AccountInfo{
			pool.String():          {Address: pool, Owner: dammV1ProgramID, Exists: true, Data: makePoolData(lpMint, mintA, mintB, aVault, bVault, aVaultLPToken, bVaultLPToken)},
			aVaultLPToken.String(): {Address: aVaultLPToken, Exists: true, Data: makeTokenAccountData(aVaultLPMint, 20_000)},
			bVaultLPToken.String(): {Address: bVaultLPToken, Exists: true, Data: makeTokenAccountData(bVaultLPMint, 50_000)},
			aVault.String():        {Address: aVault, Owner: vaultProgramID, Exists: true, Data: makeVaultData(testPubkey(31), aVaultLPMint, 10_000_000, 0, uint64(clockTimestamp), 0)},
			bVault.String():        {Address: bVault, Owner: vaultProgramID, Exists: true, Data: makeVaultData(testPubkey(32), bVaultLPMint, 10_000_000_000, 0, uint64(clockTimestamp), 0)},
			mintA.String():         {Address: mintA, Exists: true, Data: makeMintData(6, 0)},
			mintB.String():         {Address: mintB, Exists: true, Data: makeMintData(9, 0)},
			clockSysvarID.String(): {Address: clockSysvarID, Exists: true, Data: makeClockData(clockTimestamp)},
			aVaultLPMint.String():  {Address: aVaultLPMint, Exists: true, Data: makeMintData(6, 100_000)},
			bVaultLPMint.String():  {Address: bVaultLPMint, Exists: true, Data: makeMintData(9, 100_000)},
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
		t.Fatal("expected pool not found")
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
			solana.SolMint.String(): {Exists: true, Owner: dammV1ProgramID, Data: []byte{1}},
		},
	}, nil, &mockSupply{})
	if _, err := calc.Compute(context.Background(), Request{
		PoolAddress: solana.SolMint,
		MintA:       solana.SolMint,
		MintB:       solana.SolMint,
	}); err == nil {
		t.Fatal("expected decode pool error")
	}

	invalidDiscriminator := make([]byte, poolMinDataSize)
	calc = NewCalculator(&mockRPC{
		accounts: map[string]*rpc.AccountInfo{
			solana.SolMint.String(): {Exists: true, Owner: dammV1ProgramID, Data: invalidDiscriminator},
		},
	}, nil, &mockSupply{})
	if _, err := calc.Compute(context.Background(), Request{
		PoolAddress: solana.SolMint,
		MintA:       solana.SolMint,
		MintB:       solana.SolMint,
	}); err == nil {
		t.Fatal("expected invalid discriminator error")
	}
}

func TestCompute_BatchDecodeAndDownstreamErrors(t *testing.T) {
	pool := testPubkey(41)
	lpMint := testPubkey(42)
	mintA := solana.MustPublicKeyFromBase58("9BHt7aq3DFCb74kZjPY5epgVtsWKCeYX1tUWxYwDpump")
	mintB := solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")
	aVault := testPubkey(43)
	bVault := testPubkey(44)
	aVaultLPToken := testPubkey(45)
	bVaultLPToken := testPubkey(46)
	aVaultLPMint := testPubkey(47)
	bVaultLPMint := testPubkey(48)
	clockTimestamp := time.Now().Unix()

	baseAccounts := map[string]*rpc.AccountInfo{
		pool.String():          {Address: pool, Owner: dammV1ProgramID, Exists: true, Data: makePoolData(lpMint, mintA, mintB, aVault, bVault, aVaultLPToken, bVaultLPToken)},
		aVaultLPToken.String(): {Address: aVaultLPToken, Exists: true, Data: makeTokenAccountData(aVaultLPMint, 20_000)},
		bVaultLPToken.String(): {Address: bVaultLPToken, Exists: true, Data: makeTokenAccountData(bVaultLPMint, 50_000)},
		aVault.String():        {Address: aVault, Owner: vaultProgramID, Exists: true, Data: makeVaultData(testPubkey(51), aVaultLPMint, 10_000_000, 0, uint64(clockTimestamp), 0)},
		bVault.String():        {Address: bVault, Owner: vaultProgramID, Exists: true, Data: makeVaultData(testPubkey(52), bVaultLPMint, 10_000_000_000, 0, uint64(clockTimestamp), 0)},
		mintA.String():         {Address: mintA, Exists: true, Data: makeMintData(6, 0)},
		mintB.String():         {Address: mintB, Exists: true, Data: makeMintData(9, 0)},
		clockSysvarID.String(): {Address: clockSysvarID, Exists: true, Data: makeClockData(clockTimestamp)},
		aVaultLPMint.String():  {Address: aVaultLPMint, Exists: true, Data: makeMintData(6, 100_000)},
		bVaultLPMint.String():  {Address: bVaultLPMint, Exists: true, Data: makeMintData(9, 100_000)},
	}

	calc := NewCalculator(&mockRPC{
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
		getMultipleFn: func(addresses []solana.PublicKey) ([]*rpc.AccountInfo, error) {
			if len(addresses) == 7 {
				out := make([]*rpc.AccountInfo, 0, len(addresses))
				for _, address := range addresses {
					out = append(out, baseAccounts[address.String()])
				}
				return out, nil
			}
			return nil, errors.New("lp mint batch failed")
		},
	}, nil, &mockSupply{})
	if _, err := calc.Compute(context.Background(), Request{
		PoolAddress: pool,
		MintA:       mintA,
		MintB:       mintB,
	}); err == nil {
		t.Fatal("expected lp mint batch rpc error")
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
		getMultipleFn: func(addresses []solana.PublicKey) ([]*rpc.AccountInfo, error) {
			if len(addresses) == 7 {
				return []*rpc.AccountInfo{
					{Exists: true, Data: makeTokenAccountData(aVaultLPMint, 1)},
					nil,
					{Exists: true, Data: makeVaultData(testPubkey(1), aVaultLPMint, 1, 0, 1, 0)},
					{Exists: true, Data: makeVaultData(testPubkey(2), bVaultLPMint, 1, 0, 1, 0)},
					{Exists: true, Data: makeMintData(6, 1)},
					{Exists: true, Data: makeMintData(9, 1)},
					{Exists: true, Data: makeClockData(clockTimestamp)},
				}, nil
			}
			return []*rpc.AccountInfo{
				{Exists: true, Data: makeMintData(6, 1)},
				{Exists: true, Data: makeMintData(9, 1)},
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
		acc4 []byte
		acc5 []byte
		acc6 []byte
	}{
		{name: "decode pool vault a lp", acc0: []byte{1}, acc1: makeTokenAccountData(bVaultLPMint, 1), acc2: makeVaultData(testPubkey(1), aVaultLPMint, 1, 0, 1, 0), acc3: makeVaultData(testPubkey(2), bVaultLPMint, 1, 0, 1, 0), acc4: makeMintData(6, 1), acc5: makeMintData(9, 1), acc6: makeClockData(clockTimestamp)},
		{name: "decode pool vault b lp", acc0: makeTokenAccountData(aVaultLPMint, 1), acc1: []byte{1}, acc2: makeVaultData(testPubkey(1), aVaultLPMint, 1, 0, 1, 0), acc3: makeVaultData(testPubkey(2), bVaultLPMint, 1, 0, 1, 0), acc4: makeMintData(6, 1), acc5: makeMintData(9, 1), acc6: makeClockData(clockTimestamp)},
		{name: "decode vault a", acc0: makeTokenAccountData(aVaultLPMint, 1), acc1: makeTokenAccountData(bVaultLPMint, 1), acc2: []byte{1}, acc3: makeVaultData(testPubkey(2), bVaultLPMint, 1, 0, 1, 0), acc4: makeMintData(6, 1), acc5: makeMintData(9, 1), acc6: makeClockData(clockTimestamp)},
		{name: "decode vault b", acc0: makeTokenAccountData(aVaultLPMint, 1), acc1: makeTokenAccountData(bVaultLPMint, 1), acc2: makeVaultData(testPubkey(1), aVaultLPMint, 1, 0, 1, 0), acc3: []byte{1}, acc4: makeMintData(6, 1), acc5: makeMintData(9, 1), acc6: makeClockData(clockTimestamp)},
		{name: "decode mint a", acc0: makeTokenAccountData(aVaultLPMint, 1), acc1: makeTokenAccountData(bVaultLPMint, 1), acc2: makeVaultData(testPubkey(1), aVaultLPMint, 1, 0, 1, 0), acc3: makeVaultData(testPubkey(2), bVaultLPMint, 1, 0, 1, 0), acc4: []byte{1}, acc5: makeMintData(9, 1), acc6: makeClockData(clockTimestamp)},
		{name: "decode mint b", acc0: makeTokenAccountData(aVaultLPMint, 1), acc1: makeTokenAccountData(bVaultLPMint, 1), acc2: makeVaultData(testPubkey(1), aVaultLPMint, 1, 0, 1, 0), acc3: makeVaultData(testPubkey(2), bVaultLPMint, 1, 0, 1, 0), acc4: makeMintData(6, 1), acc5: []byte{1}, acc6: makeClockData(clockTimestamp)},
		{name: "decode clock", acc0: makeTokenAccountData(aVaultLPMint, 1), acc1: makeTokenAccountData(bVaultLPMint, 1), acc2: makeVaultData(testPubkey(1), aVaultLPMint, 1, 0, 1, 0), acc3: makeVaultData(testPubkey(2), bVaultLPMint, 1, 0, 1, 0), acc4: makeMintData(6, 1), acc5: makeMintData(9, 1), acc6: []byte{1}},
	}
	for _, tc := range cases {
		calc = NewCalculator(&mockRPC{
			accounts: baseAccounts,
			getMultipleFn: func(addresses []solana.PublicKey) ([]*rpc.AccountInfo, error) {
				if len(addresses) == 7 {
					return []*rpc.AccountInfo{
						{Exists: true, Data: tc.acc0},
						{Exists: true, Data: tc.acc1},
						{Exists: true, Data: tc.acc2},
						{Exists: true, Data: tc.acc3},
						{Exists: true, Data: tc.acc4},
						{Exists: true, Data: tc.acc5},
						{Exists: true, Data: tc.acc6},
					}, nil
				}
				return []*rpc.AccountInfo{
					{Exists: true, Data: makeMintData(6, 1)},
					{Exists: true, Data: makeMintData(9, 1)},
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
		getMultipleFn: func(addresses []solana.PublicKey) ([]*rpc.AccountInfo, error) {
			if len(addresses) == 7 {
				return []*rpc.AccountInfo{
					{Exists: true, Data: makeTokenAccountData(aVaultLPMint, 0)},
					{Exists: true, Data: makeTokenAccountData(bVaultLPMint, 0)},
					{Exists: true, Data: makeVaultData(testPubkey(1), aVaultLPMint, 10_000_000, 0, 1, 0)},
					{Exists: true, Data: makeVaultData(testPubkey(2), bVaultLPMint, 10_000_000, 0, 1, 0)},
					{Exists: true, Data: makeMintData(6, 1)},
					{Exists: true, Data: makeMintData(9, 1)},
					{Exists: true, Data: makeClockData(clockTimestamp)},
				}, nil
			}
			return []*rpc.AccountInfo{
				{Exists: true, Data: makeMintData(6, 100_000)},
				{Exists: true, Data: makeMintData(9, 100_000)},
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

	calc = NewCalculator(&mockRPC{
		accounts: baseAccounts,
		getMultipleFn: func(addresses []solana.PublicKey) ([]*rpc.AccountInfo, error) {
			if len(addresses) == 7 {
				return []*rpc.AccountInfo{
					{Exists: true, Data: makeTokenAccountData(aVaultLPMint, 20_000)},
					{Exists: true, Data: makeTokenAccountData(bVaultLPMint, 50_000)},
					{Exists: true, Data: makeVaultData(testPubkey(1), aVaultLPMint, 10_000_000, 0, 1, 0)},
					{Exists: true, Data: makeVaultData(testPubkey(2), bVaultLPMint, 10_000_000_000, 0, 1, 0)},
					{Exists: true, Data: makeMintData(6, 1)},
					{Exists: true, Data: makeMintData(9, 1)},
					{Exists: true, Data: makeClockData(clockTimestamp)},
				}, nil
			}
			return []*rpc.AccountInfo{}, nil
		},
	}, nil, &mockSupply{})
	if _, err := calc.Compute(context.Background(), Request{
		PoolAddress: pool,
		MintA:       mintA,
		MintB:       mintB,
	}); err == nil {
		t.Fatal("expected unexpected lp mint batch size error")
	}

	calc = NewCalculator(&mockRPC{
		accounts: baseAccounts,
		getMultipleFn: func(addresses []solana.PublicKey) ([]*rpc.AccountInfo, error) {
			if len(addresses) == 7 {
				return []*rpc.AccountInfo{
					{Exists: true, Data: makeTokenAccountData(aVaultLPMint, 20_000)},
					{Exists: true, Data: makeTokenAccountData(bVaultLPMint, 50_000)},
					{Exists: true, Data: makeVaultData(testPubkey(1), aVaultLPMint, 10_000_000, 0, 1, 0)},
					{Exists: true, Data: makeVaultData(testPubkey(2), bVaultLPMint, 10_000_000_000, 0, 1, 0)},
					{Exists: true, Data: makeMintData(6, 1)},
					{Exists: true, Data: makeMintData(9, 1)},
					{Exists: true, Data: makeClockData(clockTimestamp)},
				}, nil
			}
			return []*rpc.AccountInfo{
				nil,
				{Exists: true, Data: makeMintData(9, 100_000)},
			}, nil
		},
	}, nil, &mockSupply{})
	if _, err := calc.Compute(context.Background(), Request{
		PoolAddress: pool,
		MintA:       mintA,
		MintB:       mintB,
	}); err == nil {
		t.Fatal("expected required lp mint missing error")
	}

	lpDecodeCases := []struct {
		name string
		m0   []byte
		m1   []byte
	}{
		{name: "decode vaultA lp supply", m0: []byte{1}, m1: makeMintData(9, 100_000)},
		{name: "decode vaultB lp supply", m0: makeMintData(6, 100_000), m1: []byte{1}},
	}
	for _, tc := range lpDecodeCases {
		calc = NewCalculator(&mockRPC{
			accounts: baseAccounts,
			getMultipleFn: func(addresses []solana.PublicKey) ([]*rpc.AccountInfo, error) {
				if len(addresses) == 7 {
					return []*rpc.AccountInfo{
						{Exists: true, Data: makeTokenAccountData(aVaultLPMint, 20_000)},
						{Exists: true, Data: makeTokenAccountData(bVaultLPMint, 50_000)},
						{Exists: true, Data: makeVaultData(testPubkey(1), aVaultLPMint, 10_000_000, 0, 1, 0)},
						{Exists: true, Data: makeVaultData(testPubkey(2), bVaultLPMint, 10_000_000_000, 0, 1, 0)},
						{Exists: true, Data: makeMintData(6, 1)},
						{Exists: true, Data: makeMintData(9, 1)},
						{Exists: true, Data: makeClockData(clockTimestamp)},
					}, nil
				}
				return []*rpc.AccountInfo{
					{Exists: true, Data: tc.m0},
					{Exists: true, Data: tc.m1},
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

	supplyErrCalc := NewCalculator(&mockRPC{accounts: baseAccounts}, &mockQuote{value: decimal.NewFromInt(1)}, &mockSupply{err: errors.New("supply error")})
	if _, err := supplyErrCalc.Compute(context.Background(), Request{
		PoolAddress: pool,
		MintA:       mintA,
		MintB:       mintB,
	}); err == nil {
		t.Fatal("expected supply error")
	}
}

func TestCompute_QuoteConversionError(t *testing.T) {
	pool := testPubkey(141)
	lpMint := testPubkey(142)
	mintA := solana.MustPublicKeyFromBase58("9BHt7aq3DFCb74kZjPY5epgVtsWKCeYX1tUWxYwDpump")
	mintB := solana.MustPublicKeyFromBase58("EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")
	aVault := testPubkey(143)
	bVault := testPubkey(144)
	aVaultLPToken := testPubkey(145)
	bVaultLPToken := testPubkey(146)
	aVaultLPMint := testPubkey(147)
	bVaultLPMint := testPubkey(148)
	clockTimestamp := time.Now().Unix()

	mockRPCClient := &mockRPC{
		accounts: map[string]*rpc.AccountInfo{
			pool.String():          {Address: pool, Owner: dammV1ProgramID, Exists: true, Data: makePoolData(lpMint, mintA, mintB, aVault, bVault, aVaultLPToken, bVaultLPToken)},
			aVaultLPToken.String(): {Address: aVaultLPToken, Exists: true, Data: makeTokenAccountData(aVaultLPMint, 20_000)},
			bVaultLPToken.String(): {Address: bVaultLPToken, Exists: true, Data: makeTokenAccountData(bVaultLPMint, 50_000)},
			aVault.String():        {Address: aVault, Owner: vaultProgramID, Exists: true, Data: makeVaultData(testPubkey(149), aVaultLPMint, 10_000_000, 0, uint64(clockTimestamp), 0)},
			bVault.String():        {Address: bVault, Owner: vaultProgramID, Exists: true, Data: makeVaultData(testPubkey(150), bVaultLPMint, 10_000_000_000, 0, uint64(clockTimestamp), 0)},
			mintA.String():         {Address: mintA, Exists: true, Data: makeMintData(6, 0)},
			mintB.String():         {Address: mintB, Exists: true, Data: makeMintData(6, 0)},
			clockSysvarID.String(): {Address: clockSysvarID, Exists: true, Data: makeClockData(clockTimestamp)},
			aVaultLPMint.String():  {Address: aVaultLPMint, Exists: true, Data: makeMintData(6, 100_000)},
			bVaultLPMint.String():  {Address: bVaultLPMint, Exists: true, Data: makeMintData(6, 100_000)},
		},
	}

	calc := NewCalculator(mockRPCClient, &mockQuote{err: errors.New("quote error")}, &mockSupply{
		total:  decimal.NewFromInt(1_000_000),
		circ:   decimal.NewFromInt(800_000),
		method: "mock_supply",
	})
	if _, err := calc.Compute(context.Background(), Request{
		PoolAddress: pool,
		MintA:       mintA,
		MintB:       mintB,
	}); err == nil {
		t.Fatal("expected quote conversion error")
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
	if _, _, err := decodeTokenAccountAmountAndMint([]byte{1}); err == nil {
		t.Fatal("expected token account short data error")
	}
	if _, err := decodeVaultState([]byte{1}); err == nil {
		t.Fatal("expected decode vault short data error")
	}
	if _, err := decodeMintDecimals([]byte{1}); err == nil {
		t.Fatal("expected decode mint decimals short data error")
	}
	if _, err := decodeMintSupply([]byte{1}); err == nil {
		t.Fatal("expected decode mint supply short data error")
	}
	if _, err := decodeClockUnixTimestamp([]byte{1}); err == nil {
		t.Fatal("expected decode clock short data error")
	}

	if got := lockedProfit(100, 1000, 200, 1); got != 1000 {
		t.Fatalf("expected full locked profit when current < last report, got %d", got)
	}
	if got := lockedProfit(200, 1000, 100, 0); got != 0 {
		t.Fatalf("expected zero locked profit when degrade is zero, got %d", got)
	}
	if got := lockedProfit(300, 1000, 100, lockedProfitDenominator); got != 0 {
		t.Fatalf("expected zero locked profit when degradation ratio exceeded, got %d", got)
	}
	if got := lockedProfit(200, 1000, 100, 1); got == 0 {
		t.Fatalf("expected non-zero locked profit, got %d", got)
	}

	withdrawable, locked := vaultWithdrawableAmount(200, vaultState{
		totalAmount:      1000,
		lastLockedProfit: 400,
		lastReport:       100,
		profitDegrade:    1,
	})
	if withdrawable == 0 || locked == 0 {
		t.Fatalf("expected positive withdrawable and locked values, got w=%d l=%d", withdrawable, locked)
	}
	withdrawable, locked = vaultWithdrawableAmount(200, vaultState{
		totalAmount:      100,
		lastLockedProfit: 1000,
		lastReport:       100,
		profitDegrade:    1,
	})
	if withdrawable != 0 || locked == 0 {
		t.Fatalf("expected clamped withdrawable branch, got w=%d l=%d", withdrawable, locked)
	}

	if got := amountByShare(0, 100, 100); got != 0 {
		t.Fatalf("expected zero for zero share, got %d", got)
	}
	if got := amountByShare(10, 1000, 100); got != 100 {
		t.Fatalf("unexpected amount by share: %d", got)
	}
	if got := amountByShare(math.MaxUint64, math.MaxUint64, 1); got != 0 {
		t.Fatalf("expected overflow guard branch to return zero, got %d", got)
	}

	if got := curveTypeString(0); got != "constant_product" {
		t.Fatalf("unexpected curve type for 0: %s", got)
	}
	if got := curveTypeString(1); got != "stable" {
		t.Fatalf("unexpected curve type for 1: %s", got)
	}
	if got := curveTypeString(9); got != "unknown" {
		t.Fatalf("unexpected curve type for unknown tag: %s", got)
	}

	req := Request{
		MintA: solana.SolMint,
		MintB: solana.MustPublicKeyFromBase58("11111111111111111111111111111111"),
	}
	state := poolState{
		tokenAMint: req.MintA,
		tokenBMint: req.MintB,
	}
	if !poolMatchesRequest(req, state) {
		t.Fatal("expected pool match")
	}
	if poolMatchesRequest(Request{
		MintA: testPubkey(90),
		MintB: req.MintB,
	}, state) {
		t.Fatal("did not expect pool match")
	}

	snapshot := &reserveSnapshot{
		tokenAMint:    req.MintA,
		tokenBMint:    req.MintB,
		tokenAReserve: decimal.NewFromInt(4),
		tokenBReserve: decimal.NewFromInt(8),
	}
	if got := priceOfMintAInMintB(req, snapshot); !got.Equal(decimal.NewFromInt(2)) {
		t.Fatalf("unexpected direct price: %s", got)
	}
	if got := priceOfMintAInMintB(Request{MintA: req.MintB, MintB: req.MintA}, snapshot); !got.Equal(decimal.RequireFromString("0.5")) {
		t.Fatalf("unexpected inverse price: %s", got)
	}
	if got := priceOfMintAInMintB(req, nil); !got.IsZero() {
		t.Fatalf("expected zero price for nil snapshot, got %s", got)
	}
	if got := priceOfMintAInMintB(Request{
		MintA: testPubkey(91),
		MintB: testPubkey(92),
	}, snapshot); !got.IsZero() {
		t.Fatalf("expected zero price for unmatched pair, got %s", got)
	}

	if got := liquidityInMintB(req, snapshot, decimal.NewFromInt(2)); !got.Equal(decimal.NewFromInt(16)) {
		t.Fatalf("unexpected liquidity in quote: %s", got)
	}
	if got := liquidityInMintB(Request{MintB: req.MintA}, snapshot, decimal.NewFromInt(2)); !got.Equal(decimal.NewFromInt(20)) {
		t.Fatalf("unexpected liquidity in base: %s", got)
	}
	if got := liquidityInMintB(Request{MintB: req.MintA}, snapshot, decimal.Zero); !got.Equal(decimal.NewFromInt(4)) {
		t.Fatalf("unexpected liquidity for zero-price branch: %s", got)
	}
	if got := liquidityInMintB(req, nil, decimal.NewFromInt(2)); !got.IsZero() {
		t.Fatalf("expected zero liquidity for nil snapshot, got %s", got)
	}
	if got := liquidityInMintB(Request{MintB: testPubkey(93)}, snapshot, decimal.NewFromInt(2)); !got.IsZero() {
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
