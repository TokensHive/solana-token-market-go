package quote

import (
	"context"
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/shopspring/decimal"
)

func TestNoopBridgeToSOL(t *testing.T) {
	b := NewNoopBridge()

	out, err := b.ToSOL(context.Background(), solana.SolMint, decimal.NewFromInt(5))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.Equal(decimal.NewFromInt(5)) {
		t.Fatalf("expected passthrough for SOL mint, got %s", out.String())
	}

	out, err = b.ToSOL(context.Background(), solana.MustPublicKeyFromBase58("11111111111111111111111111111111"), decimal.NewFromInt(5))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.IsZero() {
		t.Fatalf("expected zero for non-SOL mint, got %s", out.String())
	}
}
