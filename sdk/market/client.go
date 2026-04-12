package market

import (
"github.com/TokensHive/solana-token-market-go/sdk/discovery"
"github.com/TokensHive/solana-token-market-go/sdk/parser"
"github.com/TokensHive/solana-token-market-go/sdk/quote"
"github.com/TokensHive/solana-token-market-go/sdk/rpc"
"github.com/TokensHive/solana-token-market-go/sdk/supply"
)

type Client struct {
cfg     Config
service *Service
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
cfg.DiscoveryEngine = discovery.NewEngine(cfg.RPCClient, cfg.Parser)
}
c := &Client{cfg: cfg}
c.service = NewService(cfg)
return c, nil
}
