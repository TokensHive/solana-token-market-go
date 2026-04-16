package supply

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

type fdvMockRPC struct {
	account *rpc.AccountInfo
	err     error
}

func (m *fdvMockRPC) GetAccount(context.Context, solana.PublicKey) (*rpc.AccountInfo, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.account == nil {
		return &rpc.AccountInfo{Exists: false}, nil
	}
	return m.account, nil
}
func (m *fdvMockRPC) GetMultipleAccounts(context.Context, []solana.PublicKey) ([]*rpc.AccountInfo, error) {
	return nil, nil
}
func (m *fdvMockRPC) GetTokenSupply(context.Context, solana.PublicKey) (decimal.Decimal, uint8, error) {
	return decimal.Zero, 0, nil
}
func (m *fdvMockRPC) GetSignaturesForAddress(context.Context, solana.PublicKey, *rpc.SignaturesForAddressOptions) ([]*rpcclient.TransactionSignature, error) {
	return nil, nil
}
func (m *fdvMockRPC) GetTransaction(context.Context, solana.Signature) (*rpcclient.GetTransactionResult, error) {
	return nil, nil
}
func (m *fdvMockRPC) GetTransactionRaw(context.Context, solana.Signature) ([]byte, error) {
	return nil, nil
}

func TestResolveFDVSupply(t *testing.T) {
	total := decimal.NewFromInt(500_000_000)
	mint := solana.MustPublicKeyFromBase58("2nCeHpECQvnMfzjU5fDMAKws1vBxMzxvWr6qqLpApump")

	supply, method := ResolveFDVSupply(context.Background(), nil, mint, total)
	if !supply.Equal(total) || method != fdvMethodMintTotalSupply {
		t.Fatalf("expected fallback for nil rpc, supply=%s method=%s", supply, method)
	}

	supply, method = ResolveFDVSupply(context.Background(), &fdvMockRPC{err: errors.New("rpc fail")}, mint, total)
	if !supply.Equal(total) || method != fdvMethodMintTotalSupply {
		t.Fatalf("expected fallback on rpc error, supply=%s method=%s", supply, method)
	}

	shortData := make([]byte, pumpfunTokenTotalSupplyDataLen-1)
	supply, method = ResolveFDVSupply(context.Background(), &fdvMockRPC{
		account: &rpc.AccountInfo{Exists: true, Owner: pumpfunProgramID, Data: shortData},
	}, mint, total)
	if !supply.Equal(total) || method != fdvMethodMintTotalSupply {
		t.Fatalf("expected fallback for short data, supply=%s method=%s", supply, method)
	}

	notPumpData := make([]byte, pumpfunTokenTotalSupplyDataLen)
	binary.LittleEndian.PutUint64(notPumpData[pumpfunTokenTotalSupplyOffset:pumpfunTokenTotalSupplyOffset+8], 1_000_000_000_000_000)
	supply, method = ResolveFDVSupply(context.Background(), &fdvMockRPC{
		account: &rpc.AccountInfo{Exists: true, Owner: solana.SolMint, Data: notPumpData},
	}, mint, total)
	if !supply.Equal(total) || method != fdvMethodMintTotalSupply {
		t.Fatalf("expected fallback for non-pump owner, supply=%s method=%s", supply, method)
	}

	pumpData := make([]byte, pumpfunTokenTotalSupplyDataLen)
	binary.LittleEndian.PutUint64(pumpData[pumpfunTokenTotalSupplyOffset:pumpfunTokenTotalSupplyOffset+8], 1_000_000_000_000_000)
	supply, method = ResolveFDVSupply(context.Background(), &fdvMockRPC{
		account: &rpc.AccountInfo{Exists: true, Owner: pumpfunProgramID, Data: pumpData},
	}, mint, total)
	if !supply.Equal(decimal.NewFromInt(1_000_000_000)) || method != fdvMethodPumpCurveTotal {
		t.Fatalf("expected pump curve supply, supply=%s method=%s", supply, method)
	}

	lowData := make([]byte, pumpfunTokenTotalSupplyDataLen)
	binary.LittleEndian.PutUint64(lowData[pumpfunTokenTotalSupplyOffset:pumpfunTokenTotalSupplyOffset+8], 100_000_000_000_000)
	supply, method = ResolveFDVSupply(context.Background(), &fdvMockRPC{
		account: &rpc.AccountInfo{Exists: true, Owner: pumpfunProgramID, Data: lowData},
	}, mint, total)
	if !supply.Equal(total) || method != fdvMethodMintTotalSupply {
		t.Fatalf("expected fallback when curve supply <= total, supply=%s method=%s", supply, method)
	}

	supply, method = ResolveFDVSupply(context.Background(), &fdvMockRPC{}, solana.PublicKey{}, total)
	if !supply.Equal(total) || method != fdvMethodMintTotalSupply {
		t.Fatalf("expected fallback for zero mint, supply=%s method=%s", supply, method)
	}
}

var _ rpc.Client = (*fdvMockRPC)(nil)
