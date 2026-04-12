package meteora

import (
"context"
"time"

"github.com/TokensHive/solana-token-market-go/sdk/discovery"
"github.com/TokensHive/solana-token-market-go/sdk/market"
"github.com/TokensHive/solana-token-market-go/sdk/parser"
"github.com/TokensHive/solana-token-market-go/sdk/rpc"
"github.com/gagliardetto/solana-go"
)

type adapter struct {
rpc      rpc.Client
parser   parser.Adapter
protocol market.Protocol
mtype    market.MarketType
}

func (a *adapter) Protocol() market.Protocol { return a.protocol }

func (a *adapter) Discover(ctx context.Context, req discovery.Request) ([]*market.Pool, error) {
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
p := DecodePool(a.protocol, a.mtype, pk, req.Mint, acc.Data)
p.LastUpdatedAt = time.Now().UTC()
out = append(out, p)
}
if len(out) == 0 {
return nil, parser.ErrNoEvidence
}
return out, nil
}

func (a *adapter) GetByAddress(ctx context.Context, pool solana.PublicKey) (*market.Pool, error) {
acc, err := a.rpc.GetAccount(ctx, pool)
if err != nil || acc == nil || !acc.Exists {
return nil, err
}
p := DecodePool(a.protocol, a.mtype, pool, solana.PublicKey{}, acc.Data)
p.LastUpdatedAt = time.Now().UTC()
return p, nil
}

func NewDLMMAdapter(c rpc.Client, p parser.Adapter) discovery.Adapter {
return &adapter{rpc: c, parser: p, protocol: market.ProtocolMeteoraDLMM, mtype: market.MarketTypeDynamicLiquidity}
}
func NewDAMMAdapter(c rpc.Client, p parser.Adapter) discovery.Adapter {
return &adapter{rpc: c, parser: p, protocol: market.ProtocolMeteoraDAMM, mtype: market.MarketTypeConstantProduct}
}
func NewDBCAdapter(c rpc.Client, p parser.Adapter) discovery.Adapter {
return &adapter{rpc: c, parser: p, protocol: market.ProtocolMeteoraDBC, mtype: market.MarketTypeLaunchpad}
}
