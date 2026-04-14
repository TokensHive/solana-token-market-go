package supply

import (
	"context"
	"errors"
	"testing"

	"github.com/TokensHive/solana-token-market-go/sdk/rpc"
	"github.com/gagliardetto/solana-go"
	rpcclient "github.com/gagliardetto/solana-go/rpc"
	"github.com/shopspring/decimal"
)

type mockRPC struct{}

func (m *mockRPC) GetAccount(context.Context, solana.PublicKey) (*rpc.AccountInfo, error) {
	return nil, nil
}
func (m *mockRPC) GetMultipleAccounts(context.Context, []solana.PublicKey) ([]*rpc.AccountInfo, error) {
	return nil, nil
}
func (m *mockRPC) GetTokenSupply(context.Context, solana.PublicKey) (decimal.Decimal, uint8, error) {
	return decimal.NewFromInt(1_000_000), 6, nil
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

func TestApplyAdjustments(t *testing.T) {
	total := decimal.NewFromInt(1000)
	out := ApplyAdjustments(total, []CirculatingAdjustment{{Label: "locked", Amount: decimal.NewFromInt(100)}})
	if !out.Equal(decimal.NewFromInt(900)) {
		t.Fatalf("expected 900, got %s", out)
	}

	out = ApplyAdjustments(total, []CirculatingAdjustment{{Label: "over", Amount: decimal.NewFromInt(2000)}})
	if !out.Equal(decimal.Zero) {
		t.Fatalf("expected clamped zero, got %s", out)
	}
}

func TestDefaultProvider_GetSupply(t *testing.T) {
	p := NewDefaultProvider(&mockRPC{})
	total, circ, method, err := p.GetSupply(context.Background(), solana.SolMint)
	if err != nil {
		t.Fatal(err)
	}
	if !total.Equal(circ) || method == "" {
		t.Fatalf("unexpected values total=%s circ=%s method=%s", total, circ, method)
	}
}

type errMockRPC struct{ mockRPC }

func (m *errMockRPC) GetTokenSupply(context.Context, solana.PublicKey) (decimal.Decimal, uint8, error) {
	return decimal.Zero, 0, errors.New("rpc failure")
}

func TestDefaultProvider_GetSupplyError(t *testing.T) {
	p := NewDefaultProvider(&errMockRPC{})
	_, _, _, err := p.GetSupply(context.Background(), solana.SolMint)
	if err == nil {
		t.Fatal("expected supply error")
	}
}
