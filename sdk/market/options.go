package market

import (
	"time"

	"github.com/TokensHive/solana-token-market-go/sdk/internal/pubkeyx"
	"github.com/TokensHive/solana-token-market-go/sdk/parser"
	"github.com/TokensHive/solana-token-market-go/sdk/quote"
	"github.com/TokensHive/solana-token-market-go/sdk/rpc"
	"github.com/TokensHive/solana-token-market-go/sdk/supply"
	"github.com/gagliardetto/solana-go"
)

type Cache interface {
	Get(key string, dst any) (bool, error)
	Set(key string, value any, ttl time.Duration) error
	Delete(key string) error
}

type Config struct {
	RPCClient         rpc.Client
	Parser            parser.Adapter
	DiscoveryEngine   DiscoveryEngine
	QuoteBridge       quote.Bridge
	SupplyProvider    supply.Provider
	Cache             Cache
	PreferRaydiumAPI  bool
	PreferMeteoraAPI  bool
	DefaultMode       DiscoveryMode
	RequestTimeout    time.Duration
	RetryCount        int
	StableQuoteMints  []solana.PublicKey
	DefaultQuoteMints []solana.PublicKey
}

type Option func(*Config) error

func defaultConfig() Config {
	return Config{
		DefaultMode:    DiscoveryModeOnChain,
		RequestTimeout: 8 * time.Second,
		RetryCount:     2,
		StableQuoteMints: []solana.PublicKey{
			solana.MustPublicKeyFromBase58("EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"), // USDC
			solana.MustPublicKeyFromBase58("Es9vMFrzaCERmJfrF4H2ZQ8QYfV7kRXKuX3sX5Yucs5b"), // USDT
		},
		DefaultQuoteMints: []solana.PublicKey{pubkeyx.WrappedSOLMint, solana.SolMint},
	}
}

func WithRPCClient(c rpc.Client) Option {
	return func(cfg *Config) error { cfg.RPCClient = c; return nil }
}

func WithParserAdapter(p parser.Adapter) Option {
	return func(cfg *Config) error { cfg.Parser = p; return nil }
}

func WithDiscoveryEngine(e DiscoveryEngine) Option {
	return func(cfg *Config) error { cfg.DiscoveryEngine = e; return nil }
}

func WithQuoteBridge(b quote.Bridge) Option {
	return func(cfg *Config) error { cfg.QuoteBridge = b; return nil }
}

func WithSupplyProvider(p supply.Provider) Option {
	return func(cfg *Config) error { cfg.SupplyProvider = p; return nil }
}

func WithCache(cache Cache) Option {
	return func(cfg *Config) error { cfg.Cache = cache; return nil }
}

func WithDiscoveryMode(mode DiscoveryMode) Option {
	return func(cfg *Config) error { cfg.DefaultMode = mode; return nil }
}

func WithAPIHints(preferRaydiumAPI, preferMeteoraAPI bool) Option {
	return func(cfg *Config) error {
		cfg.PreferRaydiumAPI = preferRaydiumAPI
		cfg.PreferMeteoraAPI = preferMeteoraAPI
		return nil
	}
}
