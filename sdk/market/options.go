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
	MaxBulkConcurrency      int
	BulkChunkSize           int
	RPCRetryMaxRetries      int
	RPCRetryInitialBackoff  int
	RPCRetryMaxBackoff      int
	RPCRetryJitterFraction  float64
	PoolCalculatorFactories map[PoolRoute]PoolCalculatorFactory
}

type Option func(*Config) error

func defaultConfig() Config {
	return Config{
		MaxTxSignatures:         120,
		MaxBulkConcurrency:      8,
		BulkChunkSize:           100,
		RPCRetryMaxRetries:      2,
		RPCRetryInitialBackoff:  100,
		RPCRetryMaxBackoff:      1200,
		RPCRetryJitterFraction:  0.2,
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

func WithMaxBulkConcurrency(limit int) Option {
	return func(cfg *Config) error {
		if limit > 0 {
			cfg.MaxBulkConcurrency = limit
		}
		return nil
	}
}

func WithBulkChunkSize(size int) Option {
	return func(cfg *Config) error {
		if size > 0 {
			cfg.BulkChunkSize = size
		}
		return nil
	}
}

func WithRPCRetryMaxRetries(retries int) Option {
	return func(cfg *Config) error {
		if retries >= 0 {
			cfg.RPCRetryMaxRetries = retries
		}
		return nil
	}
}

func WithRPCRetryInitialBackoffMS(backoffMS int) Option {
	return func(cfg *Config) error {
		if backoffMS > 0 {
			cfg.RPCRetryInitialBackoff = backoffMS
		}
		return nil
	}
}

func WithRPCRetryMaxBackoffMS(backoffMS int) Option {
	return func(cfg *Config) error {
		if backoffMS > 0 {
			cfg.RPCRetryMaxBackoff = backoffMS
		}
		return nil
	}
}

func WithRPCRetryJitterFraction(jitter float64) Option {
	return func(cfg *Config) error {
		if jitter >= 0 {
			cfg.RPCRetryJitterFraction = jitter
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
