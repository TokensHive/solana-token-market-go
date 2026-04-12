package market

import (
	"context"
	"sync"

	"github.com/TokensHive/solana-token-market-go/sdk/internal/reqdebug"
	"github.com/TokensHive/solana-token-market-go/sdk/parser"
	"github.com/TokensHive/solana-token-market-go/sdk/quote"
	"github.com/TokensHive/solana-token-market-go/sdk/rpc"
	"github.com/TokensHive/solana-token-market-go/sdk/supply"
)

type Client struct {
	cfg       Config
	service   *Service
	mu        sync.RWMutex
	lastDebug map[string]any
}

func NewClient(opts ...Option) (*Client, error) {
	cfg := defaultConfig()
	for _, opt := range opts {
		if err := opt(&cfg); err != nil {
			return nil, NewError(ErrCodeInvalidArgument, "invalid option", err)
		}
	}
	if cfg.RPCClient == nil {
		cfg.RPCClient = rpc.NewNoopClient()
	}
	if cfg.Parser == nil {
		cfg.Parser = parser.NewNoopAdapter()
	}
	if cfg.QuoteBridge == nil {
		cfg.QuoteBridge = quote.NewNoopBridge()
	}
	if cfg.SupplyProvider == nil {
		cfg.SupplyProvider = supply.NewDefaultProvider(cfg.RPCClient)
	}
	if cfg.DiscoveryEngine == nil {
		cfg.DiscoveryEngine = newNoopDiscoveryEngine()
	}
	c := &Client{cfg: cfg}
	c.service = NewService(cfg)
	return c, nil
}

func (c *Client) LastRequestDebug() map[string]any {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.lastDebug == nil {
		return nil
	}
	out := make(map[string]any, len(c.lastDebug))
	for k, v := range c.lastDebug {
		out[k] = v
	}
	return out
}

func (c *Client) startDebug(ctx context.Context, operation string) (context.Context, *reqdebug.Recorder) {
	if !c.cfg.DebugRequests {
		return ctx, nil
	}
	recorder := reqdebug.NewRecorder(operation)
	return reqdebug.WithRecorder(ctx, recorder), recorder
}

func (c *Client) finishDebug(recorder *reqdebug.Recorder) {
	if recorder == nil {
		return
	}
	c.mu.Lock()
	c.lastDebug = recorder.SnapshotMap()
	c.mu.Unlock()
}
