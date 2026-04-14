package market

import (
	"context"
	"errors"
	"testing"

	"github.com/TokensHive/solana-token-market-go/sdk/quote"
	"github.com/TokensHive/solana-token-market-go/sdk/rpc"
	"github.com/TokensHive/solana-token-market-go/sdk/supply"
	"github.com/gagliardetto/solana-go"
	"github.com/shopspring/decimal"
)

type noopSupplyProvider struct{}

func (noopSupplyProvider) GetSupply(context.Context, solana.PublicKey) (decimal.Decimal, decimal.Decimal, string, error) {
	return decimal.Zero, decimal.Zero, "noop", nil
}

func TestDefaultConfig(t *testing.T) {
	cfg := defaultConfig()
	if cfg.MaxTxSignatures != 120 {
		t.Fatalf("unexpected default max signatures: %d", cfg.MaxTxSignatures)
	}
	if cfg.PoolCalculatorFactories == nil {
		t.Fatal("expected default calculator factory map to be initialized")
	}
}

func TestOptionsApply(t *testing.T) {
	cfg := defaultConfig()
	rpcClient := rpc.NewNoopClient()
	bridge := quote.NewNoopBridge()
	provider := noopSupplyProvider{}

	options := []Option{
		WithRPCClient(rpcClient),
		WithQuoteBridge(bridge),
		WithSupplyProvider(provider),
		WithDebugRequests(true),
		WithMaxTxSignatures(77),
	}
	for _, opt := range options {
		if err := opt(&cfg); err != nil {
			t.Fatalf("option failed: %v", err)
		}
	}

	if cfg.RPCClient != rpcClient || cfg.QuoteBridge != bridge || cfg.SupplyProvider != provider {
		t.Fatal("expected options to set clients/providers")
	}
	if !cfg.DebugRequests || cfg.MaxTxSignatures != 77 {
		t.Fatalf("unexpected debug/signature values: debug=%v max=%d", cfg.DebugRequests, cfg.MaxTxSignatures)
	}

	_ = WithMaxTxSignatures(0)(&cfg)
	if cfg.MaxTxSignatures != 77 {
		t.Fatalf("expected max signatures unchanged for invalid limit, got %d", cfg.MaxTxSignatures)
	}
}

func TestWithPoolCalculatorFactory(t *testing.T) {
	cfg := defaultConfig()
	cfg.PoolCalculatorFactories = nil
	route := PoolRoute{Dex: Dex("custom"), PoolVersion: PoolVersion("v1")}
	factory := func(Config) PoolCalculator {
		return poolCalculatorFunc(func(context.Context, PoolIdentifier) (*GetMetricsByPoolResponse, error) {
			return &GetMetricsByPoolResponse{}, nil
		})
	}

	if err := WithPoolCalculatorFactory(route, factory)(&cfg); err != nil {
		t.Fatalf("expected valid factory option, got %v", err)
	}
	if cfg.PoolCalculatorFactories[route] == nil {
		t.Fatal("expected calculator factory to be registered")
	}

	if err := WithPoolCalculatorFactory(PoolRoute{}, factory)(&cfg); err == nil {
		t.Fatal("expected validation error for empty route")
	}
	if err := WithPoolCalculatorFactory(route, nil)(&cfg); err == nil {
		t.Fatal("expected validation error for nil factory")
	}
}

func TestNewClientOptionErrorWrap(t *testing.T) {
	badOpt := func(*Config) error { return errors.New("bad option") }
	_, err := NewClient(badOpt)
	if err == nil {
		t.Fatal("expected wrapped option error")
	}
}

func TestNewClientDefaultDependencies(t *testing.T) {
	client, err := NewClient()
	if err != nil {
		t.Fatalf("new client failed: %v", err)
	}
	if client.service == nil || client.cfg.RPCClient == nil || client.cfg.QuoteBridge == nil || client.cfg.SupplyProvider == nil {
		t.Fatalf("expected default dependencies initialized, client=%#v", client)
	}
}

var _ supply.Provider = noopSupplyProvider{}
