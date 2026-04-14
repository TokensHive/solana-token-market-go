package rpc

import (
	"context"
	"errors"
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/shopspring/decimal"
)

func TestGetTotalSupply(t *testing.T) {
	client := &testClient{
		getTokenSupplyFn: func(context.Context, solana.PublicKey) (decimal.Decimal, uint8, error) {
			return decimal.RequireFromString("123.45"), 6, nil
		},
	}
	got, decimals, err := GetTotalSupply(context.Background(), client, solana.SolMint)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Equal(decimal.RequireFromString("123.45")) || decimals != 6 {
		t.Fatalf("unexpected supply=%s decimals=%d", got, decimals)
	}

	client.getTokenSupplyFn = func(context.Context, solana.PublicKey) (decimal.Decimal, uint8, error) {
		return decimal.Zero, 0, errors.New("supply error")
	}
	if _, _, err := GetTotalSupply(context.Background(), client, solana.SolMint); err == nil {
		t.Fatal("expected propagated error")
	}
}
