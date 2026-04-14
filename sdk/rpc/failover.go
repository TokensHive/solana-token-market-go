package rpc

import (
	"context"
	"time"

	"github.com/gagliardetto/solana-go"
	rpcclient "github.com/gagliardetto/solana-go/rpc"
	"github.com/shopspring/decimal"
)

type failoverClient struct {
	primary         Client
	fallback        Client
	primaryTimeout  time.Duration
	fallbackTimeout time.Duration
}

func NewFailoverClient(primary, fallback Client, primaryTimeout, fallbackTimeout time.Duration) Client {
	if primary == nil {
		return fallback
	}
	if fallback == nil {
		return primary
	}
	if primaryTimeout <= 0 {
		primaryTimeout = 4 * time.Second
	}
	if fallbackTimeout <= 0 {
		fallbackTimeout = 4 * time.Second
	}
	return &failoverClient{
		primary:         primary,
		fallback:        fallback,
		primaryTimeout:  primaryTimeout,
		fallbackTimeout: fallbackTimeout,
	}
}

func (f *failoverClient) GetAccount(ctx context.Context, address solana.PublicKey) (*AccountInfo, error) {
	out, err := withFallback(ctx, f.primaryTimeout, f.fallbackTimeout, f.primary.GetAccount, f.fallback.GetAccount, address)
	return out, err
}

func (f *failoverClient) GetMultipleAccounts(ctx context.Context, addresses []solana.PublicKey) ([]*AccountInfo, error) {
	out, err := withFallback(ctx, f.primaryTimeout, f.fallbackTimeout, f.primary.GetMultipleAccounts, f.fallback.GetMultipleAccounts, addresses)
	return out, err
}

func (f *failoverClient) GetTokenSupply(ctx context.Context, mint solana.PublicKey) (decimal.Decimal, uint8, error) {
	return withFallbackTuple(ctx, f.primaryTimeout, f.fallbackTimeout, f.primary.GetTokenSupply, f.fallback.GetTokenSupply, mint)
}

func (f *failoverClient) GetSignaturesForAddress(ctx context.Context, address solana.PublicKey, opts *SignaturesForAddressOptions) ([]*rpcclient.TransactionSignature, error) {
	return withFallbackPair(ctx, f.primaryTimeout, f.fallbackTimeout, f.primary.GetSignaturesForAddress, f.fallback.GetSignaturesForAddress, address, opts)
}

func (f *failoverClient) GetTransaction(ctx context.Context, signature solana.Signature) (*rpcclient.GetTransactionResult, error) {
	out, err := withFallback(ctx, f.primaryTimeout, f.fallbackTimeout, f.primary.GetTransaction, f.fallback.GetTransaction, signature)
	return out, err
}

func (f *failoverClient) GetTransactionRaw(ctx context.Context, signature solana.Signature) ([]byte, error) {
	return withFallback(ctx, f.primaryTimeout, f.fallbackTimeout, f.primary.GetTransactionRaw, f.fallback.GetTransactionRaw, signature)
}

func withFallback[T any, A any](
	ctx context.Context,
	primaryTimeout time.Duration,
	fallbackTimeout time.Duration,
	primary func(context.Context, A) (T, error),
	fallback func(context.Context, A) (T, error),
	arg A,
) (T, error) {
	var zero T
	pctx, cancel := context.WithTimeout(ctx, primaryTimeout)
	defer cancel()
	out, err := primary(pctx, arg)
	if err == nil {
		return out, nil
	}
	fctx, fcancel := context.WithTimeout(ctx, fallbackTimeout)
	defer fcancel()
	out, ferr := fallback(fctx, arg)
	if ferr == nil {
		return out, nil
	}
	return zero, err
}

func withFallbackPair[T any, A any, B any](
	ctx context.Context,
	primaryTimeout time.Duration,
	fallbackTimeout time.Duration,
	primary func(context.Context, A, B) (T, error),
	fallback func(context.Context, A, B) (T, error),
	argA A,
	argB B,
) (T, error) {
	var zero T
	pctx, cancel := context.WithTimeout(ctx, primaryTimeout)
	defer cancel()
	out, err := primary(pctx, argA, argB)
	if err == nil {
		return out, nil
	}
	fctx, fcancel := context.WithTimeout(ctx, fallbackTimeout)
	defer fcancel()
	out, ferr := fallback(fctx, argA, argB)
	if ferr == nil {
		return out, nil
	}
	return zero, err
}

func withFallbackTuple(
	ctx context.Context,
	primaryTimeout time.Duration,
	fallbackTimeout time.Duration,
	primary func(context.Context, solana.PublicKey) (decimal.Decimal, uint8, error),
	fallback func(context.Context, solana.PublicKey) (decimal.Decimal, uint8, error),
	mint solana.PublicKey,
) (decimal.Decimal, uint8, error) {
	pctx, cancel := context.WithTimeout(ctx, primaryTimeout)
	defer cancel()
	v, d, err := primary(pctx, mint)
	if err == nil {
		return v, d, nil
	}
	fctx, fcancel := context.WithTimeout(ctx, fallbackTimeout)
	defer fcancel()
	v, d, ferr := fallback(fctx, mint)
	if ferr == nil {
		return v, d, nil
	}
	return decimal.Zero, 0, err
}
