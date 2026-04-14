package rpc

import (
	"context"
	"errors"

	"github.com/gagliardetto/solana-go"
	rpcclient "github.com/gagliardetto/solana-go/rpc"
	"github.com/shopspring/decimal"
)

type testClient struct {
	getAccountFn           func(context.Context, solana.PublicKey) (*AccountInfo, error)
	getMultipleAccountsFn  func(context.Context, []solana.PublicKey) ([]*AccountInfo, error)
	getTokenSupplyFn       func(context.Context, solana.PublicKey) (decimal.Decimal, uint8, error)
	getSignaturesForAddrFn func(context.Context, solana.PublicKey, *SignaturesForAddressOptions) ([]*rpcclient.TransactionSignature, error)
	getTransactionFn       func(context.Context, solana.Signature) (*rpcclient.GetTransactionResult, error)
	getTransactionRawFn    func(context.Context, solana.Signature) ([]byte, error)
}

func (c *testClient) GetAccount(ctx context.Context, address solana.PublicKey) (*AccountInfo, error) {
	if c.getAccountFn == nil {
		return nil, errors.New("not implemented")
	}
	return c.getAccountFn(ctx, address)
}

func (c *testClient) GetMultipleAccounts(ctx context.Context, addresses []solana.PublicKey) ([]*AccountInfo, error) {
	if c.getMultipleAccountsFn == nil {
		return nil, errors.New("not implemented")
	}
	return c.getMultipleAccountsFn(ctx, addresses)
}

func (c *testClient) GetTokenSupply(ctx context.Context, mint solana.PublicKey) (decimal.Decimal, uint8, error) {
	if c.getTokenSupplyFn == nil {
		return decimal.Zero, 0, errors.New("not implemented")
	}
	return c.getTokenSupplyFn(ctx, mint)
}

func (c *testClient) GetSignaturesForAddress(ctx context.Context, address solana.PublicKey, opts *SignaturesForAddressOptions) ([]*rpcclient.TransactionSignature, error) {
	if c.getSignaturesForAddrFn == nil {
		return nil, errors.New("not implemented")
	}
	return c.getSignaturesForAddrFn(ctx, address, opts)
}

func (c *testClient) GetTransaction(ctx context.Context, signature solana.Signature) (*rpcclient.GetTransactionResult, error) {
	if c.getTransactionFn == nil {
		return nil, errors.New("not implemented")
	}
	return c.getTransactionFn(ctx, signature)
}

func (c *testClient) GetTransactionRaw(ctx context.Context, signature solana.Signature) ([]byte, error) {
	if c.getTransactionRawFn == nil {
		return nil, errors.New("not implemented")
	}
	return c.getTransactionRawFn(ctx, signature)
}
