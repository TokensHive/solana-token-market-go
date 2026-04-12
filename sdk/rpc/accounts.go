package rpc

import (
	"context"

	"github.com/gagliardetto/solana-go"
)

func Exists(ctx context.Context, c Client, address solana.PublicKey) (bool, error) {
	info, err := c.GetAccount(ctx, address)
	if err != nil {
		return false, err
	}
	return info != nil && info.Exists, nil
}
