package rpc

import (
	"context"
	"errors"
	"testing"

	"github.com/gagliardetto/solana-go"
	rpcclient "github.com/gagliardetto/solana-go/rpc"
	"github.com/shopspring/decimal"
)

func TestChunkPubkeys(t *testing.T) {
	keys := []solana.PublicKey{
		solana.SolMint,
		solana.MustPublicKeyFromBase58("11111111111111111111111111111111"),
		solana.MustPublicKeyFromBase58("SysvarRent111111111111111111111111111111111"),
	}

	chunks := ChunkPubkeys(keys, 2)
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
	if len(chunks[0]) != 2 || len(chunks[1]) != 1 {
		t.Fatalf("unexpected chunk sizes: %d and %d", len(chunks[0]), len(chunks[1]))
	}

	defaultSized := ChunkPubkeys(keys, 0)
	if len(defaultSized) != 1 || len(defaultSized[0]) != len(keys) {
		t.Fatalf("expected single default-sized chunk, got %#v", defaultSized)
	}
}

func TestNewChunkingClient(t *testing.T) {
	base := &testClient{}
	if got := NewChunkingClient(nil, 100); got != nil {
		t.Fatalf("expected nil when base client is nil, got %T", got)
	}
	if got := NewChunkingClient(base, 0); got != base {
		t.Fatalf("expected passthrough client for invalid chunk size, got %T", got)
	}
	if _, ok := NewChunkingClient(base, 2).(*chunkingClient); !ok {
		t.Fatal("expected chunking client wrapper for valid chunk size")
	}
}

func TestChunkingClientGetMultipleAccountsChunkingAndDedup(t *testing.T) {
	var calls int
	base := &testClient{
		getMultipleAccountsFn: func(_ context.Context, addresses []solana.PublicKey) ([]*AccountInfo, error) {
			calls++
			out := make([]*AccountInfo, len(addresses))
			for i := range addresses {
				out[i] = &AccountInfo{
					Address: addresses[i],
					Exists:  true,
				}
			}
			return out, nil
		},
	}

	client := NewChunkingClient(base, 2)
	addresses := []solana.PublicKey{
		solana.SolMint,
		solana.MustPublicKeyFromBase58("11111111111111111111111111111111"),
		solana.SolMint,
		solana.MustPublicKeyFromBase58("SysvarRent111111111111111111111111111111111"),
	}
	out, err := client.GetMultipleAccounts(context.Background(), addresses)
	if err != nil {
		t.Fatalf("expected chunked getMultipleAccounts success, got %v", err)
	}
	if len(out) != len(addresses) {
		t.Fatalf("expected %d output accounts, got %d", len(addresses), len(out))
	}
	if calls != 2 {
		t.Fatalf("expected 2 chunked calls for 3 unique addresses, got %d", calls)
	}
	if !out[0].Address.Equals(solana.SolMint) || !out[2].Address.Equals(solana.SolMint) {
		t.Fatalf("expected duplicate addresses preserved in output order, got %#v", out)
	}

	emptyOut, err := client.GetMultipleAccounts(context.Background(), nil)
	if err != nil {
		t.Fatalf("expected empty address list to succeed, got %v", err)
	}
	if len(emptyOut) != 0 {
		t.Fatalf("expected empty output for empty address list, got %#v", emptyOut)
	}
}

func TestChunkingClientGetMultipleAccountsContextOverrideAndError(t *testing.T) {
	var calls int
	base := &testClient{
		getMultipleAccountsFn: func(_ context.Context, addresses []solana.PublicKey) ([]*AccountInfo, error) {
			calls++
			if calls == 2 {
				return nil, errors.New("boom")
			}
			out := make([]*AccountInfo, len(addresses))
			for i := range addresses {
				out[i] = &AccountInfo{Address: addresses[i], Exists: true}
			}
			return out, nil
		},
	}

	client := NewChunkingClient(base, 100)
	addresses := []solana.PublicKey{
		solana.SolMint,
		solana.MustPublicKeyFromBase58("11111111111111111111111111111111"),
		solana.MustPublicKeyFromBase58("SysvarRent111111111111111111111111111111111"),
	}
	ctx := WithGetMultipleAccountsChunkSize(context.Background(), 1)
	if _, err := client.GetMultipleAccounts(ctx, addresses); err == nil {
		t.Fatal("expected context-override chunking error to propagate")
	}
	if calls != 2 {
		t.Fatalf("expected two calls before error, got %d", calls)
	}
}

func TestWithGetMultipleAccountsChunkSizeInvalidInput(t *testing.T) {
	ctx := context.Background()
	if got := WithGetMultipleAccountsChunkSize(ctx, 0); got != ctx {
		t.Fatal("expected invalid chunk size to return original context")
	}
}

func TestChunkingClientDelegatesOtherMethods(t *testing.T) {
	called := map[string]bool{}
	base := &testClient{
		getAccountFn: func(context.Context, solana.PublicKey) (*AccountInfo, error) {
			called["get_account"] = true
			return &AccountInfo{Address: solana.SolMint, Exists: true}, nil
		},
		getTokenSupplyFn: func(context.Context, solana.PublicKey) (decimal.Decimal, uint8, error) {
			called["get_token_supply"] = true
			return decimal.NewFromInt(1), 9, nil
		},
		getSignaturesForAddrFn: func(context.Context, solana.PublicKey, *SignaturesForAddressOptions) ([]*rpcclient.TransactionSignature, error) {
			called["get_signatures"] = true
			return nil, nil
		},
		getTransactionFn: func(context.Context, solana.Signature) (*rpcclient.GetTransactionResult, error) {
			called["get_transaction"] = true
			return &rpcclient.GetTransactionResult{}, nil
		},
		getTransactionRawFn: func(context.Context, solana.Signature) ([]byte, error) {
			called["get_transaction_raw"] = true
			return []byte("{}"), nil
		},
		getMultipleAccountsFn: func(context.Context, []solana.PublicKey) ([]*AccountInfo, error) {
			return []*AccountInfo{
				nil, // exercise nil account fallback mapping branch
			}, nil
		},
	}
	client := NewChunkingClient(base, 2)

	if _, err := client.GetAccount(context.Background(), solana.SolMint); err != nil {
		t.Fatalf("get account delegation failed: %v", err)
	}
	if _, _, err := client.GetTokenSupply(context.Background(), solana.SolMint); err != nil {
		t.Fatalf("get token supply delegation failed: %v", err)
	}
	if _, err := client.GetSignaturesForAddress(context.Background(), solana.SolMint, nil); err != nil {
		t.Fatalf("get signatures delegation failed: %v", err)
	}
	if _, err := client.GetTransaction(context.Background(), solana.Signature{}); err != nil {
		t.Fatalf("get transaction delegation failed: %v", err)
	}
	if _, err := client.GetTransactionRaw(context.Background(), solana.Signature{}); err != nil {
		t.Fatalf("get transaction raw delegation failed: %v", err)
	}
	out, err := client.GetMultipleAccounts(context.Background(), []solana.PublicKey{solana.SolMint})
	if err != nil {
		t.Fatalf("get multiple accounts delegation failed: %v", err)
	}
	if len(out) != 1 || out[0] == nil || out[0].Exists {
		t.Fatalf("expected nil account in batch to map to missing account, got %#v", out)
	}

	for _, key := range []string{"get_account", "get_token_supply", "get_signatures", "get_transaction", "get_transaction_raw"} {
		if !called[key] {
			t.Fatalf("expected delegated call %q to be invoked", key)
		}
	}
}

func TestChunkingClientGetMultipleAccountsBranchCoverage(t *testing.T) {
	baseChunked := &testClient{
		getMultipleAccountsFn: func(_ context.Context, _ []solana.PublicKey) ([]*AccountInfo, error) {
			return []*AccountInfo{}, nil // intentionally shorter than requested
		},
	}
	client := NewChunkingClient(baseChunked, 2)
	addresses := []solana.PublicKey{
		solana.SolMint,
		solana.MustPublicKeyFromBase58("11111111111111111111111111111111"),
	}

	out, err := client.GetMultipleAccounts(context.Background(), addresses)
	if err != nil {
		t.Fatalf("expected branch-coverage call success, got %v", err)
	}
	if len(out) != len(addresses) {
		t.Fatalf("expected output length to match input, got %#v", out)
	}
	for i := range out {
		if out[i] == nil || out[i].Exists {
			t.Fatalf("expected missing-account fallback for index %d, got %#v", i, out[i])
		}
	}

	baseUnchunked := &testClient{
		getMultipleAccountsFn: func(_ context.Context, keys []solana.PublicKey) ([]*AccountInfo, error) {
			out := make([]*AccountInfo, len(keys))
			for i := range keys {
				out[i] = &AccountInfo{Address: keys[i], Exists: true}
			}
			return out, nil
		},
	}
	client = NewChunkingClient(baseUnchunked, 2)
	// Force chunkSize<=0 branch via internal context key.
	ctx := context.WithValue(context.Background(), chunkSizeContextKey{}, 0)
	out, err = client.GetMultipleAccounts(ctx, addresses)
	if err != nil {
		t.Fatalf("expected unchunked branch call success, got %v", err)
	}
	if len(out) != len(addresses) || !out[0].Exists || !out[1].Exists {
		t.Fatalf("expected passthrough unchunked output, got %#v", out)
	}
}
