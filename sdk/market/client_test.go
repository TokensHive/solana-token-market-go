package market

import (
	"context"
	"encoding/binary"
	"testing"

	"github.com/TokensHive/solana-token-market-go/sdk/internal/reqdebug"
	"github.com/TokensHive/solana-token-market-go/sdk/rpc"
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

func TestGetMetricsByPumpfunBondingCurveClientDebugLifecycle(t *testing.T) {
	data := make([]byte, 64)
	binary.LittleEndian.PutUint64(data[8:16], 1063770573068395)
	binary.LittleEndian.PutUint64(data[16:24], 30260284408)
	binary.LittleEndian.PutUint64(data[24:32], 783870573068395)
	binary.LittleEndian.PutUint64(data[32:40], 260284408)
	binary.LittleEndian.PutUint64(data[40:48], 1000000000000000)

	client, err := NewClient(
		WithDebugRequests(true),
		WithRPCClient(&marketMockRPC{
			getAccountFn: func(context.Context, solana.PublicKey) (*rpc.AccountInfo, error) {
				return &rpc.AccountInfo{Exists: true, Data: data}, nil
			},
		}),
	)
	if err != nil {
		t.Fatalf("new client failed: %v", err)
	}

	resp, err := client.GetMetricsByPumpfunBondingCurve(context.Background(), GetMetricsByPumpfunBondingCurveRequest{
		MintA: solana.MustPublicKeyFromBase58("9BHt7aq3DFCb74kZjPY5epgVtsWKCeYX1tUWxYwDpump"),
		MintB: solana.SolMint,
	})
	if err != nil {
		t.Fatalf("expected successful bonding curve client call, got %v", err)
	}
	if resp == nil || resp.Pool.PoolAddress.IsZero() {
		t.Fatalf("expected non-empty bonding curve response, got %#v", resp)
	}
	if debug := client.LastRequestDebug(); debug == nil || debug["operation"] == nil {
		t.Fatalf("expected request debug snapshot, got %#v", debug)
	}
}
