package market

import (
	"fmt"

	"github.com/TokensHive/solana-token-market-go/sdk/quote"
	"github.com/TokensHive/solana-token-market-go/sdk/rpc"
	"github.com/TokensHive/solana-token-market-go/sdk/supply"
)

type Config struct {
	RPCClient               rpc.Client
	QuoteBridge             quote.Bridge
	SupplyProvider          supply.Provider
	DebugRequests           bool
	MaxTxSignatures         int
	PoolCalculatorFactories map[PoolRoute]PoolCalculatorFactory
}

type Option func(*Config) error

func defaultConfig() Config {
	return Config{
		MaxTxSignatures:         120,
		PoolCalculatorFactories: map[PoolRoute]PoolCalculatorFactory{},
	}
}

func WithRPCClient(c rpc.Client) Option {
	return func(cfg *Config) error {
		cfg.RPCClient = c
		return nil
	}
}

func WithQuoteBridge(b quote.Bridge) Option {
	return func(cfg *Config) error {
		cfg.QuoteBridge = b
		return nil
	}
}

func WithSupplyProvider(p supply.Provider) Option {
	return func(cfg *Config) error {
		cfg.SupplyProvider = p
		return nil
	}
}

func WithDebugRequests(enabled bool) Option {
	return func(cfg *Config) error {
		cfg.DebugRequests = enabled
		return nil
	}
}

func WithMaxTxSignatures(limit int) Option {
	return func(cfg *Config) error {
		if limit > 0 {
			cfg.MaxTxSignatures = limit
		}
		return nil
	}
}

func WithPoolCalculatorFactory(route PoolRoute, factory PoolCalculatorFactory) Option {
	return func(cfg *Config) error {
		if route.Dex == "" || route.PoolVersion == "" {
			return fmt.Errorf("pool route dex and pool version are required")
		}
		if factory == nil {
			return fmt.Errorf("pool calculator factory is required")
		}
		if cfg.PoolCalculatorFactories == nil {
			cfg.PoolCalculatorFactories = map[PoolRoute]PoolCalculatorFactory{}
		}
		cfg.PoolCalculatorFactories[route] = factory
		return nil
	}
}
