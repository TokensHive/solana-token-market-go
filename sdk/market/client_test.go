package market

import (
	"context"
	"testing"

	"github.com/TokensHive/solana-token-market-go/sdk/internal/reqdebug"
	"github.com/gagliardetto/solana-go"
	"github.com/shopspring/decimal"
)

func TestLastRequestDebugCopy(t *testing.T) {
	client, err := NewClient()
	if err != nil {
		t.Fatalf("new client failed: %v", err)
	}

	if got := client.LastRequestDebug(); got != nil {
		t.Fatalf("expected nil debug when not set, got %#v", got)
	}

	client.lastDebug = map[string]any{"a": 1}
	snapshot := client.LastRequestDebug()
	snapshot["a"] = 2
	if client.lastDebug["a"] != 1 {
		t.Fatal("expected LastRequestDebug to return a copy")
	}
}

func TestDebugLifecycle(t *testing.T) {
	client, err := NewClient(
		WithDebugRequests(true),
		WithPoolCalculatorFactory(PoolRoute{
			Dex:         Dex("custom"),
			PoolVersion: PoolVersion("v1"),
		}, func(Config) PoolCalculator {
			return poolCalculatorFunc(func(context.Context, PoolIdentifier) (*GetMetricsByPoolResponse, error) {
				return &GetMetricsByPoolResponse{
					PriceOfAInSOL: decimal.NewFromInt(1),
				}, nil
			})
		}),
	)
	if err != nil {
		t.Fatalf("new client failed: %v", err)
	}

	_, err = client.GetMetricsByPool(context.Background(), GetMetricsByPoolRequest{
		Pool: PoolIdentifier{
			Dex:         Dex("custom"),
			PoolVersion: PoolVersion("v1"),
			PoolAddress: solana.SolMint,
			MintA:       solana.SolMint,
			MintB:       solana.SolMint,
		},
	})
	if err != nil {
		t.Fatalf("expected successful custom call, got %v", err)
	}
	if debug := client.LastRequestDebug(); debug == nil || debug["operation"] == nil {
		t.Fatalf("expected request debug snapshot, got %#v", debug)
	}
}

func TestStartFinishDebugNoopWhenDisabledOrNil(t *testing.T) {
	client, err := NewClient(WithDebugRequests(false))
	if err != nil {
		t.Fatalf("new client failed: %v", err)
	}
	ctx, recorder := client.startDebug(context.Background(), "op")
	if recorder != nil || ctx == nil {
		t.Fatalf("expected disabled debug to skip recorder, recorder=%#v", recorder)
	}

	client.finishDebug(nil)
	if client.LastRequestDebug() != nil {
		t.Fatal("expected finishDebug(nil) to keep debug empty")
	}

	rec := reqdebug.NewRecorder("manual")
	client.finishDebug(rec)
	if client.LastRequestDebug() == nil {
		t.Fatal("expected finishDebug to store snapshot")
	}
}
