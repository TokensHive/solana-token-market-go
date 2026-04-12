package rpc

import (
"context"

"github.com/gagliardetto/solana-go"
"github.com/shopspring/decimal"
)

func GetTotalSupply(ctx context.Context, c Client, mint solana.PublicKey) (decimal.Decimal, uint8, error) {
return c.GetTokenSupply(ctx, mint)
}
