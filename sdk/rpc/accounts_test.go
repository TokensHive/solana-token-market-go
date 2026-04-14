package rpc

import (
	"context"
	"errors"
	"testing"

	"github.com/gagliardetto/solana-go"
)

func TestExists(t *testing.T) {
	addr := solana.SolMint
	client := &testClient{
		getAccountFn: func(context.Context, solana.PublicKey) (*AccountInfo, error) {
			return &AccountInfo{Address: addr, Exists: true}, nil
		},
	}
	ok, err := Exists(context.Background(), client, addr)
	if err != nil || !ok {
		t.Fatalf("expected account exists, ok=%v err=%v", ok, err)
	}

	client.getAccountFn = func(context.Context, solana.PublicKey) (*AccountInfo, error) {
		return nil, nil
	}
	ok, err = Exists(context.Background(), client, addr)
	if err != nil || ok {
		t.Fatalf("expected missing account, ok=%v err=%v", ok, err)
	}

	client.getAccountFn = func(context.Context, solana.PublicKey) (*AccountInfo, error) {
		return nil, errors.New("rpc error")
	}
	if _, err = Exists(context.Background(), client, addr); err == nil {
		t.Fatal("expected propagated error")
	}
}
