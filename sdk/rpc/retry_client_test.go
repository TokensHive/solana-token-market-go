package rpc

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/gagliardetto/solana-go"
	rpcclient "github.com/gagliardetto/solana-go/rpc"
	"github.com/shopspring/decimal"
)

func TestClassifyErrorKinds(t *testing.T) {
	ce := &ClientError{Operation: "x", Kind: ErrorKindRateLimited, Err: errors.New("wrapped")}
	if got := ClassifyError(ce); got != ErrorKindRateLimited {
		t.Fatalf("expected wrapped client error classification, got %s", got)
	}
	if got := ClassifyError(errors.New("HTTP 429 RateLimitExceeded")); got != ErrorKindRateLimited {
		t.Fatalf("expected rate limited classification, got %s", got)
	}
	if got := ClassifyError(context.DeadlineExceeded); got != ErrorKindTimeout {
		t.Fatalf("expected timeout classification, got %s", got)
	}
	if got := ClassifyError(errors.New("connection refused")); got != ErrorKindRPCUnavailable {
		t.Fatalf("expected rpc unavailable classification, got %s", got)
	}
	if got := ClassifyError(errors.New("service unavailable")); got != ErrorKindRPCUnavailable {
		t.Fatalf("expected service unavailable classification, got %s", got)
	}
	if got := ClassifyError(errors.New("deadline exceeded")); got != ErrorKindTimeout {
		t.Fatalf("expected timeout string classification, got %s", got)
	}
	if got := ClassifyError(errors.New("unknown")); got != ErrorKindUnknown {
		t.Fatalf("expected unknown classification, got %s", got)
	}
	if got := ClassifyError(nil); got != ErrorKindUnknown {
		t.Fatalf("expected nil classification to be unknown, got %s", got)
	}
	timeoutErr := timeoutNetError{}
	if got := ClassifyError(timeoutErr); got != ErrorKindTimeout {
		t.Fatalf("expected net timeout classification, got %s", got)
	}
}

func TestClientErrorFormattingAndUnwrap(t *testing.T) {
	inner := errors.New("inner")
	err := &ClientError{Operation: "get_account", Kind: ErrorKindRateLimited, Err: inner}
	if err.Error() == "" {
		t.Fatal("expected formatted client error string")
	}
	if !errors.Is(err, inner) {
		t.Fatal("expected unwrap to expose inner error")
	}
	noInner := &ClientError{Operation: "get_account", Kind: ErrorKindUnknown}
	if noInner.Error() == "" {
		t.Fatal("expected non-empty error string without inner error")
	}
}

func TestNewRetryClientDisabled(t *testing.T) {
	base := &testClient{}
	if got := NewRetryClient(base, RetryConfig{MaxRetries: 0}); got != base {
		t.Fatalf("expected disabled retry to return original client, got %T", got)
	}
	if got := NewRetryClient(nil, RetryConfig{MaxRetries: 1}); got != nil {
		t.Fatalf("expected nil client passthrough, got %T", got)
	}
	if got := NewRetryClient(base, RetryConfig{MaxRetries: 1}); got == base {
		t.Fatalf("expected wrapped retry client for positive retries, got %T", got)
	}
	if got := NewRetryClient(base, RetryConfig{MaxRetries: 1, InitialBackoff: -1, MaxBackoff: -1, JitterFraction: -1}); got == nil {
		t.Fatal("expected wrapped retry client with normalized defaults")
	}
}

func TestRetryClientRetriesRateLimitedGetMultiple(t *testing.T) {
	calls := 0
	client := NewRetryClient(&testClient{
		getMultipleAccountsFn: func(context.Context, []solana.PublicKey) ([]*AccountInfo, error) {
			calls++
			if calls == 1 {
				return nil, errors.New("HTTP 429 RateLimitExceeded")
			}
			return []*AccountInfo{{Address: solana.SolMint, Exists: true}}, nil
		},
	}, RetryConfig{
		MaxRetries:     1,
		InitialBackoff: time.Millisecond,
		MaxBackoff:     time.Millisecond,
		JitterFraction: 0,
	})

	infos, err := client.GetMultipleAccounts(context.Background(), []solana.PublicKey{solana.SolMint})
	if err != nil {
		t.Fatalf("expected retry success, got %v", err)
	}
	if len(infos) != 1 || !infos[0].Exists {
		t.Fatalf("unexpected infos after retry: %#v", infos)
	}
	if calls != 2 {
		t.Fatalf("expected 2 calls with one retry, got %d", calls)
	}
}

func TestWrapClientErrorPassthrough(t *testing.T) {
	base := &ClientError{Operation: "x", Kind: ErrorKindUnknown, Err: errors.New("y")}
	if got := WrapClientError("ignored", base); got != base {
		t.Fatalf("expected existing client error passthrough, got %#v", got)
	}
	if WrapClientError("x", nil) != nil {
		t.Fatal("expected nil input to stay nil")
	}
}

func TestRetryClientDoesNotRetryDecodeLikeErrors(t *testing.T) {
	calls := 0
	client := NewRetryClient(&testClient{
		getAccountFn: func(context.Context, solana.PublicKey) (*AccountInfo, error) {
			calls++
			return nil, errors.New("decode failed")
		},
	}, RetryConfig{
		MaxRetries:     2,
		InitialBackoff: time.Millisecond,
		MaxBackoff:     time.Millisecond,
		JitterFraction: 0,
	})

	_, err := client.GetAccount(context.Background(), solana.SolMint)
	if err == nil {
		t.Fatal("expected decode-like failure")
	}
	if calls != 1 {
		t.Fatalf("expected no retries for non-retryable error, got %d calls", calls)
	}
}

func TestRetryClientMethodsAndContextCancellation(t *testing.T) {
	callCount := 0
	client := NewRetryClient(&testClient{
		getTokenSupplyFn: func(context.Context, solana.PublicKey) (decimal.Decimal, uint8, error) {
			callCount++
			return decimal.Zero, 0, errors.New("HTTP 429")
		},
		getSignaturesForAddrFn: func(context.Context, solana.PublicKey, *SignaturesForAddressOptions) ([]*rpcclient.TransactionSignature, error) {
			return []*rpcclient.TransactionSignature{}, nil
		},
		getTransactionFn: func(context.Context, solana.Signature) (*rpcclient.GetTransactionResult, error) {
			return &rpcclient.GetTransactionResult{}, nil
		},
		getTransactionRawFn: func(context.Context, solana.Signature) ([]byte, error) {
			return []byte("ok"), nil
		},
	}, RetryConfig{
		MaxRetries:     1,
		InitialBackoff: 50 * time.Millisecond,
		MaxBackoff:     50 * time.Millisecond,
		JitterFraction: 0,
	})

	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := client.GetTokenSupply(cancelCtx, solana.SolMint); err == nil {
		t.Fatal("expected canceled context retry wait error")
	}
	if callCount != 1 {
		t.Fatalf("expected one token supply call before cancellation, got %d", callCount)
	}
	if _, err := client.GetSignaturesForAddress(context.Background(), solana.SolMint, nil); err != nil {
		t.Fatalf("expected signatures path success, got %v", err)
	}
	if _, err := client.GetTransaction(context.Background(), solana.Signature{}); err != nil {
		t.Fatalf("expected transaction path success, got %v", err)
	}
	if _, err := client.GetTransactionRaw(context.Background(), solana.Signature{}); err != nil {
		t.Fatalf("expected transaction raw path success, got %v", err)
	}
}

func TestRetryBackoffHelpers(t *testing.T) {
	if got := nextBackoff(100*time.Millisecond, 150*time.Millisecond); got != 150*time.Millisecond {
		t.Fatalf("expected max-capped backoff, got %v", got)
	}
	if got := nextBackoff(100*time.Millisecond, 500*time.Millisecond); got != 200*time.Millisecond {
		t.Fatalf("expected doubled backoff, got %v", got)
	}
	if got := applyJitter(0, 0.5); got != 0 {
		t.Fatalf("expected zero jitter for zero backoff, got %v", got)
	}
	if got := applyJitter(100*time.Millisecond, 0); got != 100*time.Millisecond {
		t.Fatalf("expected unchanged backoff with zero jitter, got %v", got)
	}
	if got := applyJitter(100*time.Millisecond, 0.5); got <= 0 {
		t.Fatalf("expected positive jittered backoff, got %v", got)
	}
	if got := applyJitter(100*time.Millisecond, 2.0); got < 0 {
		t.Fatalf("expected clamped non-negative jittered backoff, got %v", got)
	}
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()
	if sleepWithBackoff(cancelCtx, 100*time.Millisecond) {
		t.Fatal("expected canceled context to stop backoff sleep")
	}
	if !sleepWithBackoff(context.Background(), 0) {
		t.Fatal("expected zero backoff to return true")
	}
}

func TestRetryPairRetryableSuccess(t *testing.T) {
	calls := 0
	client := NewRetryClient(&testClient{
		getTokenSupplyFn: func(context.Context, solana.PublicKey) (decimal.Decimal, uint8, error) {
			calls++
			if calls == 1 {
				return decimal.Zero, 0, errors.New("HTTP 429")
			}
			return decimal.NewFromInt(3), 6, nil
		},
	}, RetryConfig{
		MaxRetries:     1,
		InitialBackoff: time.Millisecond,
		MaxBackoff:     time.Millisecond,
		JitterFraction: 0,
	})
	value, decimals, err := client.GetTokenSupply(context.Background(), solana.SolMint)
	if err != nil {
		t.Fatalf("expected retry pair success, got %v", err)
	}
	if !value.Equal(decimal.NewFromInt(3)) || decimals != 6 || calls != 2 {
		t.Fatalf("unexpected retry pair output value=%s decimals=%d calls=%d", value, decimals, calls)
	}
}

func TestRetryPairNonRetryableAndExhausted(t *testing.T) {
	nonRetryClient := NewRetryClient(&testClient{
		getTokenSupplyFn: func(context.Context, solana.PublicKey) (decimal.Decimal, uint8, error) {
			return decimal.Zero, 0, errors.New("decode failed")
		},
	}, RetryConfig{
		MaxRetries:     2,
		InitialBackoff: time.Millisecond,
		MaxBackoff:     time.Millisecond,
		JitterFraction: 0,
	})
	if _, _, err := nonRetryClient.GetTokenSupply(context.Background(), solana.SolMint); err == nil {
		t.Fatal("expected non-retryable token supply failure")
	}

	retryExhaustedCalls := 0
	retryExhausted := NewRetryClient(&testClient{
		getAccountFn: func(context.Context, solana.PublicKey) (*AccountInfo, error) {
			retryExhaustedCalls++
			return nil, errors.New("HTTP 429")
		},
	}, RetryConfig{
		MaxRetries:     1,
		InitialBackoff: time.Millisecond,
		MaxBackoff:     time.Millisecond,
		JitterFraction: 0,
	})
	if _, err := retryExhausted.GetAccount(context.Background(), solana.SolMint); err == nil {
		t.Fatal("expected retry-exhausted account failure")
	}
	if retryExhaustedCalls != 2 {
		t.Fatalf("expected retries to exhaust at 2 calls, got %d", retryExhaustedCalls)
	}
}

func TestRetryGetAccountCanceledBeforeBackoff(t *testing.T) {
	client := NewRetryClient(&testClient{
		getAccountFn: func(context.Context, solana.PublicKey) (*AccountInfo, error) {
			return nil, errors.New("HTTP 429")
		},
	}, RetryConfig{
		MaxRetries:     2,
		InitialBackoff: 50 * time.Millisecond,
		MaxBackoff:     50 * time.Millisecond,
		JitterFraction: 0,
	})
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.GetAccount(cancelCtx, solana.SolMint); err == nil {
		t.Fatal("expected canceled retry path error")
	}
}

func TestPreloadedClientCachesAndStats(t *testing.T) {
	innerCalls := 0
	preloaded := NewPreloadedClient(&testClient{
		getMultipleAccountsFn: func(_ context.Context, keys []solana.PublicKey) ([]*AccountInfo, error) {
			innerCalls++
			out := make([]*AccountInfo, len(keys))
			for i := range keys {
				out[i] = &AccountInfo{Address: keys[i], Exists: true}
			}
			return out, nil
		},
	})

	addresses := []solana.PublicKey{solana.SolMint, solana.SolMint}
	if err := preloaded.PrimeAccounts(context.Background(), addresses); err != nil {
		t.Fatalf("prime failed: %v", err)
	}
	if _, err := preloaded.GetAccount(context.Background(), solana.SolMint); err != nil {
		t.Fatalf("cached get account failed: %v", err)
	}
	if _, err := preloaded.GetMultipleAccounts(context.Background(), addresses); err != nil {
		t.Fatalf("cached get multiple failed: %v", err)
	}
	if innerCalls != 1 {
		t.Fatalf("expected one inner batch call due cache reuse, got %d", innerCalls)
	}
	stats := preloaded.Stats()
	if stats.GetAccountCalls != 1 || stats.GetMultipleAccountsCalls != 1 || stats.TotalAccountsRequested != 3 {
		t.Fatalf("unexpected preloaded stats: %#v", stats)
	}
}

func TestPreloadedClientFallbackAndDelegates(t *testing.T) {
	getAccountCalls := 0
	preloaded := NewPreloadedClient(nil)
	if preloaded == nil {
		t.Fatal("expected preloaded client to handle nil inner by defaulting noop")
	}
	// Replace with an inner client that lacks getMultiple support to trigger fallback path.
	preloaded = NewPreloadedClient(&testClient{
		getMultipleAccountsFn: func(context.Context, []solana.PublicKey) ([]*AccountInfo, error) {
			return nil, nil
		},
		getAccountFn: func(_ context.Context, key solana.PublicKey) (*AccountInfo, error) {
			getAccountCalls++
			return &AccountInfo{Address: key, Exists: true}, nil
		},
		getTokenSupplyFn: func(context.Context, solana.PublicKey) (decimal.Decimal, uint8, error) {
			return decimal.NewFromInt(5), 9, nil
		},
		getSignaturesForAddrFn: func(context.Context, solana.PublicKey, *SignaturesForAddressOptions) ([]*rpcclient.TransactionSignature, error) {
			return []*rpcclient.TransactionSignature{}, nil
		},
		getTransactionFn: func(context.Context, solana.Signature) (*rpcclient.GetTransactionResult, error) {
			return &rpcclient.GetTransactionResult{}, nil
		},
		getTransactionRawFn: func(context.Context, solana.Signature) ([]byte, error) {
			return []byte("ok"), nil
		},
	})
	if _, err := preloaded.GetMultipleAccounts(context.Background(), []solana.PublicKey{solana.SolMint}); err != nil {
		t.Fatalf("expected fallback fetch success, got %v", err)
	}
	if getAccountCalls == 0 {
		t.Fatal("expected GetAccount fallback path to be used")
	}
	if _, _, err := preloaded.GetTokenSupply(context.Background(), solana.SolMint); err != nil {
		t.Fatalf("expected token supply delegate success, got %v", err)
	}
	if _, err := preloaded.GetSignaturesForAddress(context.Background(), solana.SolMint, nil); err != nil {
		t.Fatalf("expected signatures delegate success, got %v", err)
	}
	if _, err := preloaded.GetTransaction(context.Background(), solana.Signature{}); err != nil {
		t.Fatalf("expected transaction delegate success, got %v", err)
	}
	if _, err := preloaded.GetTransactionRaw(context.Background(), solana.Signature{}); err != nil {
		t.Fatalf("expected transaction raw delegate success, got %v", err)
	}
}

func TestPreloadedClientGetAccountMissingFallback(t *testing.T) {
	preloaded := NewPreloadedClient(&testClient{
		getMultipleAccountsFn: func(context.Context, []solana.PublicKey) ([]*AccountInfo, error) {
			return []*AccountInfo{{Address: solana.SolMint, Exists: false}}, nil
		},
		getAccountFn: func(context.Context, solana.PublicKey) (*AccountInfo, error) {
			return &AccountInfo{Address: solana.SolMint, Exists: false}, nil
		},
	})
	info, err := preloaded.GetAccount(context.Background(), solana.SolMint)
	if err != nil {
		t.Fatalf("expected missing account fallback success, got %v", err)
	}
	if info == nil || info.Exists {
		t.Fatalf("expected non-existing account info, got %#v", info)
	}
}

func TestPreloadedClientAdditionalBranches(t *testing.T) {
	preloaded := NewPreloadedClient(&testClient{
		getMultipleAccountsFn: func(context.Context, []solana.PublicKey) ([]*AccountInfo, error) {
			// Return fewer infos than requested to cover padding branch.
			return []*AccountInfo{}, nil
		},
		getAccountFn: func(context.Context, solana.PublicKey) (*AccountInfo, error) {
			return &AccountInfo{Address: solana.SolMint, Exists: true}, nil
		},
	})
	if _, err := preloaded.GetMultipleAccounts(context.Background(), nil); err != nil {
		t.Fatalf("expected empty address list to succeed, got %v", err)
	}

	// Cover GetAccount path where lookup result slice is empty/nil.
	info, err := preloaded.GetAccount(context.Background(), solana.SolMint)
	if err != nil {
		t.Fatalf("expected single account fetch success, got %v", err)
	}
	if info == nil {
		t.Fatal("expected non-nil account info")
	}
}

func TestPreloadedClientPaddingBranch(t *testing.T) {
	preloaded := NewPreloadedClient(&testClient{
		getMultipleAccountsFn: func(context.Context, []solana.PublicKey) ([]*AccountInfo, error) {
			// Return fewer than requested to exercise padding path.
			return []*AccountInfo{{Address: solana.SolMint, Exists: true}}, nil
		},
		getAccountFn: func(_ context.Context, key solana.PublicKey) (*AccountInfo, error) {
			return &AccountInfo{Address: key, Exists: true}, nil
		},
	})
	keys := []solana.PublicKey{
		solana.SolMint,
		solana.MustPublicKeyFromBase58("11111111111111111111111111111111"),
	}
	infos, err := preloaded.GetMultipleAccounts(context.Background(), keys)
	if err != nil {
		t.Fatalf("expected padding branch success, got %v", err)
	}
	if len(infos) != 2 || infos[0] == nil || infos[1] == nil {
		t.Fatalf("expected both infos populated after padding fallback, got %#v", infos)
	}
}

func TestPreloadedClientSkipsFallbackWhenBatchHasExistingAccount(t *testing.T) {
	getAccountCalls := 0
	preloaded := NewPreloadedClient(&testClient{
		getMultipleAccountsFn: func(_ context.Context, keys []solana.PublicKey) ([]*AccountInfo, error) {
			out := make([]*AccountInfo, len(keys))
			for i := range keys {
				out[i] = &AccountInfo{Address: keys[i], Exists: true}
			}
			return out, nil
		},
		getAccountFn: func(context.Context, solana.PublicKey) (*AccountInfo, error) {
			getAccountCalls++
			return &AccountInfo{Address: solana.SolMint, Exists: true}, nil
		},
	})

	infos, err := preloaded.GetMultipleAccounts(context.Background(), []solana.PublicKey{solana.SolMint})
	if err != nil {
		t.Fatalf("expected preloaded batch success, got %v", err)
	}
	if len(infos) != 1 || !infos[0].Exists {
		t.Fatalf("unexpected infos: %#v", infos)
	}
	if getAccountCalls != 0 {
		t.Fatalf("expected no single-account fallback when batch data exists, got %d", getAccountCalls)
	}
}

func TestPreloadedClientErrorPaths(t *testing.T) {
	preloaded := NewPreloadedClient(&testClient{
		getMultipleAccountsFn: func(context.Context, []solana.PublicKey) ([]*AccountInfo, error) {
			return nil, errors.New("batch failed")
		},
	})
	if _, err := preloaded.GetMultipleAccounts(context.Background(), []solana.PublicKey{solana.SolMint}); err == nil {
		t.Fatal("expected batch error propagation")
	}

	preloaded = NewPreloadedClient(&testClient{
		getMultipleAccountsFn: func(context.Context, []solana.PublicKey) ([]*AccountInfo, error) {
			return nil, nil
		},
		getAccountFn: func(context.Context, solana.PublicKey) (*AccountInfo, error) {
			return nil, errors.New("single failed")
		},
	})
	if _, err := preloaded.GetAccount(context.Background(), solana.SolMint); err == nil {
		t.Fatal("expected single-account fallback error propagation")
	}
}

type timeoutNetError struct{}

func (timeoutNetError) Error() string   { return "timeout" }
func (timeoutNetError) Timeout() bool   { return true }
func (timeoutNetError) Temporary() bool { return true }

var _ net.Error = timeoutNetError{}
