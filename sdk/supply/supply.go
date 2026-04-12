package supply

import (
	"context"

	"github.com/TokensHive/solana-token-market-go/sdk/rpc"
	"github.com/gagliardetto/solana-go"
	"github.com/shopspring/decimal"
)

type Provider interface {
	GetSupply(ctx context.Context, mint solana.PublicKey) (total decimal.Decimal, circulating decimal.Decimal, method string, err error)
}

type DefaultProvider struct {
	rpc rpc.Client
}

func NewDefaultProvider(c rpc.Client) Provider {
	return &DefaultProvider{rpc: c}
}

func (p *DefaultProvider) GetSupply(ctx context.Context, mint solana.PublicKey) (decimal.Decimal, decimal.Decimal, string, error) {
	total, _, err := p.rpc.GetTokenSupply(ctx, mint)
	if err != nil {
		return decimal.Zero, decimal.Zero, "", err
	}
	return total, total, "mint_total_equals_circulating_default", nil
}
