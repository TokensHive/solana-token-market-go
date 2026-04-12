package raydium

import (
"context"
"time"

"github.com/TokensHive/solana-token-market-go/sdk/discovery"
"github.com/TokensHive/solana-token-market-go/sdk/market"
"github.com/TokensHive/solana-token-market-go/sdk/parser"
"github.com/TokensHive/solana-token-market-go/sdk/rpc"
"github.com/gagliardetto/solana-go"
)

type baseAdapter struct {
rpc      rpc.Client
parser   parser.Adapter
protocol market.Protocol
market   market.MarketType
}

func (a *baseAdapter) Protocol() market.Protocol { return a.protocol }

func (a *baseAdapter) Discover(ctx context.Context, req discovery.Request) ([]*market.Pool, error) {
edges, err := a.parser.FindPoolsByMint(req.Mint.String())
if err != nil {
return nil, err
}
out := make([]*market.Pool, 0, len(edges))
for _, e := range edges {
if e.Protocol != string(a.protocol) {
continue
}
pk, err := solana.PublicKeyFromBase58(e.PoolAddress)
if err != nil {
continue
}
acc, err := a.rpc.GetAccount(ctx, pk)
if err != nil || acc == nil || !acc.Exists {
continue
}
p := DecodeConstantProductPool(pk, req.Mint, a.protocol, a.market, acc.Data)
p.LastUpdatedAt = time.Now().UTC()
out = append(out, p)
}
if len(out) == 0 {
return nil, parser.ErrNoEvidence
}
return out, nil
}

func (a *baseAdapter) GetByAddress(ctx context.Context, pool solana.PublicKey) (*market.Pool, error) {
acc, err := a.rpc.GetAccount(ctx, pool)
if err != nil || acc == nil || !acc.Exists {
return nil, err
}
p := DecodeConstantProductPool(pool, solana.PublicKey{}, a.protocol, a.market, acc.Data)
p.LastUpdatedAt = time.Now().UTC()
return p, nil
}

func NewV4Adapter(c rpc.Client, p parser.Adapter) discovery.Adapter {
return &baseAdapter{rpc: c, parser: p, protocol: market.ProtocolRaydiumV4, market: market.MarketTypeConstantProduct}
}

func NewCPMMAdapter(c rpc.Client, p parser.Adapter) discovery.Adapter {
return &baseAdapter{rpc: c, parser: p, protocol: market.ProtocolRaydiumCPMM, market: market.MarketTypeConstantProduct}
}

func NewCLMMAdapter(c rpc.Client, p parser.Adapter) discovery.Adapter {
return &clmmAdapter{baseAdapter: baseAdapter{rpc: c, parser: p, protocol: market.ProtocolRaydiumCLMM, market: market.MarketTypeConcentratedLiquidity}}
}

func NewLaunchLabAdapter(c rpc.Client, p parser.Adapter) discovery.Adapter {
return &baseAdapter{rpc: c, parser: p, protocol: market.ProtocolRaydiumLaunchLab, market: market.MarketTypeLaunchpad}
}
