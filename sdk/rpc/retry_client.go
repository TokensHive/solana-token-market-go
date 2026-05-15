package rpc

import (
	"context"
	"math/rand"
	"time"

	"github.com/gagliardetto/solana-go"
	rpcclient "github.com/gagliardetto/solana-go/rpc"
	"github.com/shopspring/decimal"
)

type RetryConfig struct {
	MaxRetries     int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	JitterFraction float64
}

type retryClient struct {
	inner Client
	cfg   RetryConfig
}

func NewRetryClient(inner Client, cfg RetryConfig) Client {
	if inner == nil {
		return nil
	}
	if cfg.MaxRetries <= 0 {
		return inner
	}
	if cfg.InitialBackoff <= 0 {
		cfg.InitialBackoff = 100 * time.Millisecond
	}
	if cfg.MaxBackoff <= 0 {
		cfg.MaxBackoff = 2 * time.Second
	}
	if cfg.JitterFraction < 0 {
		cfg.JitterFraction = 0
	}
	return &retryClient{
		inner: inner,
		cfg:   cfg,
	}
}

func (c *retryClient) GetAccount(ctx context.Context, address solana.PublicKey) (*AccountInfo, error) {
	return withRetry(ctx, c.cfg, func(callCtx context.Context) (*AccountInfo, error) {
		info, err := c.inner.GetAccount(callCtx, address)
		return info, WrapClientError("get_account", err)
	})
}

func (c *retryClient) GetMultipleAccounts(ctx context.Context, addresses []solana.PublicKey) ([]*AccountInfo, error) {
	return withRetry(ctx, c.cfg, func(callCtx context.Context) ([]*AccountInfo, error) {
		infos, err := c.inner.GetMultipleAccounts(callCtx, addresses)
		return infos, WrapClientError("get_multiple_accounts", err)
	})
}

func (c *retryClient) GetTokenSupply(ctx context.Context, mint solana.PublicKey) (decimal.Decimal, uint8, error) {
	return withRetryPair(ctx, c.cfg, func(callCtx context.Context) (decimal.Decimal, uint8, error) {
		value, decimals, err := c.inner.GetTokenSupply(callCtx, mint)
		return value, decimals, WrapClientError("get_token_supply", err)
	})
}

func (c *retryClient) GetSignaturesForAddress(ctx context.Context, address solana.PublicKey, opts *SignaturesForAddressOptions) ([]*rpcclient.TransactionSignature, error) {
	return withRetry(ctx, c.cfg, func(callCtx context.Context) ([]*rpcclient.TransactionSignature, error) {
		sigs, err := c.inner.GetSignaturesForAddress(callCtx, address, opts)
		return sigs, WrapClientError("get_signatures_for_address", err)
	})
}

func (c *retryClient) GetTransaction(ctx context.Context, signature solana.Signature) (*rpcclient.GetTransactionResult, error) {
	return withRetry(ctx, c.cfg, func(callCtx context.Context) (*rpcclient.GetTransactionResult, error) {
		result, err := c.inner.GetTransaction(callCtx, signature)
		return result, WrapClientError("get_transaction", err)
	})
}

func (c *retryClient) GetTransactionRaw(ctx context.Context, signature solana.Signature) ([]byte, error) {
	return withRetry(ctx, c.cfg, func(callCtx context.Context) ([]byte, error) {
		raw, err := c.inner.GetTransactionRaw(callCtx, signature)
		return raw, WrapClientError("get_transaction_raw", err)
	})
}

func withRetry[T any](ctx context.Context, cfg RetryConfig, fn func(context.Context) (T, error)) (T, error) {
	var zero T
	backoff := cfg.InitialBackoff
	for attempt := 0; ; attempt++ {
		out, err := fn(ctx)
		if err == nil {
			return out, nil
		}
		if attempt >= cfg.MaxRetries || !IsRetryableError(err) {
			return zero, err
		}
		if !sleepWithBackoff(ctx, applyJitter(backoff, cfg.JitterFraction)) {
			return zero, WrapClientError("retry_wait", ctx.Err())
		}
		backoff = nextBackoff(backoff, cfg.MaxBackoff)
	}
}

func withRetryPair(ctx context.Context, cfg RetryConfig, fn func(context.Context) (decimal.Decimal, uint8, error)) (decimal.Decimal, uint8, error) {
	backoff := cfg.InitialBackoff
	for attempt := 0; ; attempt++ {
		value, decimals, err := fn(ctx)
		if err == nil {
			return value, decimals, nil
		}
		if attempt >= cfg.MaxRetries || !IsRetryableError(err) {
			return decimal.Zero, 0, err
		}
		if !sleepWithBackoff(ctx, applyJitter(backoff, cfg.JitterFraction)) {
			return decimal.Zero, 0, WrapClientError("retry_wait", ctx.Err())
		}
		backoff = nextBackoff(backoff, cfg.MaxBackoff)
	}
}

func nextBackoff(current, max time.Duration) time.Duration {
	next := current * 2
	if next > max {
		return max
	}
	return next
}

func applyJitter(backoff time.Duration, jitterFraction float64) time.Duration {
	if backoff <= 0 || jitterFraction <= 0 {
		return backoff
	}
	delta := float64(backoff) * jitterFraction
	low := float64(backoff) - delta
	high := float64(backoff) + delta
	if low < 0 {
		low = 0
	}
	return time.Duration(low + rand.Float64()*(high-low))
}

func sleepWithBackoff(ctx context.Context, backoff time.Duration) bool {
	if backoff <= 0 {
		return true
	}
	timer := time.NewTimer(backoff)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
