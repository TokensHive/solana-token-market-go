package quote

import (
"context"

"github.com/gagliardetto/solana-go"
"github.com/shopspring/decimal"
)

type Bridge interface {
ToSOL(ctx context.Context, quoteMint solana.PublicKey, value decimal.Decimal) (decimal.Decimal, error)
}

type noopBridge struct{}

func NewNoopBridge() Bridge { return &noopBridge{} }

func (n *noopBridge) ToSOL(_ context.Context, quoteMint solana.PublicKey, value decimal.Decimal) (decimal.Decimal, error) {
if quoteMint == solana.SolMint {
return value, nil
}
return decimal.Zero, nil
}
