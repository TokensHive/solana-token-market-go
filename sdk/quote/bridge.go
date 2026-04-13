package quote

import (
	"context"

	"github.com/TokensHive/solana-token-market-go/sdk/internal/pubkeyx"
	"github.com/gagliardetto/solana-go"
	"github.com/shopspring/decimal"
)

type Bridge interface {
	ToSOL(ctx context.Context, quoteMint solana.PublicKey, value decimal.Decimal) (decimal.Decimal, error)
}

type noopBridge struct{}

func NewNoopBridge() Bridge { return &noopBridge{} }

func (n *noopBridge) ToSOL(_ context.Context, quoteMint solana.PublicKey, value decimal.Decimal) (decimal.Decimal, error) {
	if pubkeyx.IsSOLMint(quoteMint) {
		return value, nil
	}
	return decimal.Zero, nil
}
