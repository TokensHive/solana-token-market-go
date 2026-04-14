package rpc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gagliardetto/solana-go"
	rpcclient "github.com/gagliardetto/solana-go/rpc"
	"github.com/shopspring/decimal"
)

func TestNewFailoverClientFallbackLogic(t *testing.T) {
	primary := &testClient{}
	fallback := &testClient{}

	if got := NewFailoverClient(nil, fallback, 0, 0); got != fallback {
		t.Fatal("expected fallback client when primary is nil")
	}
	if got := NewFailoverClient(primary, nil, 0, 0); got != primary {
		t.Fatal("expected primary client when fallback is nil")
	}
	got := NewFailoverClient(primary, fallback, 0, 0)
	if _, ok := got.(*failoverClient); !ok {
		t.Fatalf("expected failoverClient, got %T", got)
	}
}

func TestFailoverClientMethods(t *testing.T) {
	addr := solana.SolMint
	sig := solana.Signature{}
	primaryErr := errors.New("primary error")

	primary := &testClient{
		getAccountFn: func(context.Context, solana.PublicKey) (*AccountInfo, error) {
			return nil, primaryErr
		},
		getMultipleAccountsFn: func(context.Context, []solana.PublicKey) ([]*AccountInfo, error) {
			return nil, primaryErr
		},
		getTokenSupplyFn: func(context.Context, solana.PublicKey) (decimal.Decimal, uint8, error) {
			return decimal.Zero, 0, primaryErr
		},
		getSignaturesForAddrFn: func(context.Context, solana.PublicKey, *SignaturesForAddressOptions) ([]*rpcclient.TransactionSignature, error) {
			return nil, primaryErr
		},
		getTransactionFn: func(context.Context, solana.Signature) (*rpcclient.GetTransactionResult, error) {
			return nil, primaryErr
		},
		getTransactionRawFn: func(context.Context, solana.Signature) ([]byte, error) {
			return nil, primaryErr
		},
	}
	fallback := &testClient{
		getAccountFn: func(context.Context, solana.PublicKey) (*AccountInfo, error) {
			return &AccountInfo{Address: addr, Exists: true}, nil
		},
		getMultipleAccountsFn: func(context.Context, []solana.PublicKey) ([]*AccountInfo, error) {
			return []*AccountInfo{{Address: addr, Exists: true}}, nil
		},
		getTokenSupplyFn: func(context.Context, solana.PublicKey) (decimal.Decimal, uint8, error) {
			return decimal.NewFromInt(1), 9, nil
		},
		getSignaturesForAddrFn: func(context.Context, solana.PublicKey, *SignaturesForAddressOptions) ([]*rpcclient.TransactionSignature, error) {
			return []*rpcclient.TransactionSignature{{Signature: sig}}, nil
		},
		getTransactionFn: func(context.Context, solana.Signature) (*rpcclient.GetTransactionResult, error) {
			return &rpcclient.GetTransactionResult{}, nil
		},
		getTransactionRawFn: func(context.Context, solana.Signature) ([]byte, error) {
			return []byte(`{"slot":1}`), nil
		},
	}

	client := NewFailoverClient(primary, fallback, time.Millisecond, time.Millisecond)

	if _, err := client.GetAccount(context.Background(), addr); err != nil {
		t.Fatalf("expected fallback account success: %v", err)
	}
	if _, err := client.GetMultipleAccounts(context.Background(), []solana.PublicKey{addr}); err != nil {
		t.Fatalf("expected fallback multiple accounts success: %v", err)
	}
	if _, _, err := client.GetTokenSupply(context.Background(), addr); err != nil {
		t.Fatalf("expected fallback token supply success: %v", err)
	}
	if _, err := client.GetSignaturesForAddress(context.Background(), addr, nil); err != nil {
		t.Fatalf("expected fallback signatures success: %v", err)
	}
	if _, err := client.GetTransaction(context.Background(), sig); err != nil {
		t.Fatalf("expected fallback transaction success: %v", err)
	}
	if _, err := client.GetTransactionRaw(context.Background(), sig); err != nil {
		t.Fatalf("expected fallback transaction raw success: %v", err)
	}
}

func TestFallbackHelpersReturnPrimaryErrorWhenBothFail(t *testing.T) {
	expected := errors.New("primary failed")
	fallbackErr := errors.New("fallback failed")

	_, err := withFallback(context.Background(), time.Millisecond, time.Millisecond,
		func(context.Context, int) (string, error) { return "", expected },
		func(context.Context, int) (string, error) { return "", fallbackErr },
		1,
	)
	if !errors.Is(err, expected) {
		t.Fatalf("expected primary error, got %v", err)
	}

	_, err = withFallbackPair(context.Background(), time.Millisecond, time.Millisecond,
		func(context.Context, int, int) (string, error) { return "", expected },
		func(context.Context, int, int) (string, error) { return "", fallbackErr },
		1, 2,
	)
	if !errors.Is(err, expected) {
		t.Fatalf("expected primary error, got %v", err)
	}

	_, _, err = withFallbackTuple(context.Background(), time.Millisecond, time.Millisecond,
		func(context.Context, solana.PublicKey) (decimal.Decimal, uint8, error) {
			return decimal.Zero, 0, expected
		},
		func(context.Context, solana.PublicKey) (decimal.Decimal, uint8, error) {
			return decimal.Zero, 0, fallbackErr
		},
		solana.SolMint,
	)
	if !errors.Is(err, expected) {
		t.Fatalf("expected primary error, got %v", err)
	}
}

func TestFallbackHelpersReturnPrimarySuccess(t *testing.T) {
	out, err := withFallback(context.Background(), time.Millisecond, time.Millisecond,
		func(context.Context, int) (string, error) { return "primary", nil },
		func(context.Context, int) (string, error) { return "fallback", nil },
		1,
	)
	if err != nil || out != "primary" {
		t.Fatalf("expected primary success, out=%s err=%v", out, err)
	}

	out, err = withFallbackPair(context.Background(), time.Millisecond, time.Millisecond,
		func(context.Context, int, int) (string, error) { return "primary", nil },
		func(context.Context, int, int) (string, error) { return "fallback", nil },
		1, 2,
	)
	if err != nil || out != "primary" {
		t.Fatalf("expected pair primary success, out=%s err=%v", out, err)
	}

	value, decimals, err := withFallbackTuple(context.Background(), time.Millisecond, time.Millisecond,
		func(context.Context, solana.PublicKey) (decimal.Decimal, uint8, error) {
			return decimal.NewFromInt(5), 9, nil
		},
		func(context.Context, solana.PublicKey) (decimal.Decimal, uint8, error) {
			return decimal.NewFromInt(1), 0, nil
		},
		solana.SolMint,
	)
	if err != nil || !value.Equal(decimal.NewFromInt(5)) || decimals != 9 {
		t.Fatalf("expected tuple primary success, value=%s decimals=%d err=%v", value, decimals, err)
	}
}
