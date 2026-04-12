package discovery

import (
"context"
"errors"
"sort"
"time"

"github.com/TokensHive/solana-token-market-go/sdk/market"
"github.com/TokensHive/solana-token-market-go/sdk/parser"
"github.com/TokensHive/solana-token-market-go/sdk/protocols/meteora"
"github.com/TokensHive/solana-token-market-go/sdk/protocols/orca"
"github.com/TokensHive/solana-token-market-go/sdk/protocols/pumpfun"
"github.com/TokensHive/solana-token-market-go/sdk/protocols/pumpswap"
"github.com/TokensHive/solana-token-market-go/sdk/protocols/raydium"
"github.com/TokensHive/solana-token-market-go/sdk/rpc"
"github.com/gagliardetto/solana-go"
)

type Request struct {
Mint              solana.PublicKey
QuoteMint         *solana.PublicKey
Protocols         []market.Protocol
MarketTypes       []market.MarketType
IncludeInactive   bool
IncludeUnverified bool
DiscoveryMode     market.DiscoveryMode
PoolAddresses     []solana.PublicKey
DirectSOLOnly     bool
}

type Engine struct {
rpc      rpc.Client
parser   parser.Adapter
registry *Registry
}

func NewEngine(c rpc.Client, p parser.Adapter) *Engine {
return &Engine{rpc: c, parser: p, registry: NewRegistryWithDefaults(c, p)}
}

func NewRegistryWithDefaults(c rpc.Client, p parser.Adapter) *Registry {
r := NewRegistry()
r.Register(pumpfun.NewAdapter(c, p))
r.Register(pumpswap.NewAdapter(c, p))
r.Register(raydium.NewV4Adapter(c, p))
r.Register(raydium.NewCPMMAdapter(c, p))
r.Register(raydium.NewCLMMAdapter(c, p))
r.Register(raydium.NewLaunchLabAdapter(c, p))
r.Register(orca.NewAdapter(c, p))
r.Register(meteora.NewDLMMAdapter(c, p))
r.Register(meteora.NewDAMMAdapter(c, p))
r.Register(meteora.NewDBCAdapter(c, p))
return r
}

func (e *Engine) Discover(ctx context.Context, req Request) ([]*market.Pool, map[string]any, error) {
if req.Mint.IsZero() {
return nil, nil, market.NewError(market.ErrCodeInvalidArgument, "mint is required", nil)
}
meta := map[string]any{"layers": []string{"deterministic", "parser_evidence", "rpc_fallback", "api_optional"}}
adapters := e.registry.List(req.Protocols)
all := make([]*market.Pool, 0, 8)
for _, a := range adapters {
pools, err := a.Discover(ctx, req)
if err != nil && !errors.Is(err, parser.ErrNoEvidence) {
continue
}
all = append(all, pools...)
}
all = DeduplicatePools(all)
all = ApplyFilters(all, FilterOptions{
Protocols:         req.Protocols,
MarketTypes:       req.MarketTypes,
QuoteMint:         req.QuoteMint,
IncludeInactive:   req.IncludeInactive,
IncludeUnverified: req.IncludeUnverified,
PoolAddresses:     req.PoolAddresses,
DirectSOLOnly:     req.DirectSOLOnly,
})
if len(all) > 0 {
for _, p := range all {
if p.LastUpdatedAt.IsZero() {
p.LastUpdatedAt = time.Now().UTC()
}
}
}
var q *string
if req.QuoteMint != nil {
v := req.QuoteMint.String()
q = &v
}
ranked := RankPools(all, req.Mint.String(), q)
return ranked, meta, nil
}

func (e *Engine) FindByPoolAddress(ctx context.Context, addr solana.PublicKey) ([]*market.Pool, map[string]any, error) {
for _, a := range e.registry.List(nil) {
p, err := a.GetByAddress(ctx, addr)
if err == nil && p != nil {
return []*market.Pool{p}, map[string]any{"source": "adapter"}, nil
}
}
return nil, map[string]any{}, nil
}

func DeduplicatePools(pools []*market.Pool) []*market.Pool {
seen := map[string]struct{}{}
out := make([]*market.Pool, 0, len(pools))
for _, p := range pools {
if p == nil || p.Address == "" {
continue
}
if _, ok := seen[p.Address]; ok {
continue
}
seen[p.Address] = struct{}{}
out = append(out, p)
}
return out
}

func RankPools(pools []*market.Pool, targetMint string, quoteMint *string) []*market.Pool {
scored := make([]*market.Pool, 0, len(pools))
for _, p := range pools {
if p == nil {
continue
}
if !p.IsActive {
continue
}
if p.BaseReserve.IsZero() || p.QuoteReserve.IsZero() {
// allow but heavily penalize
}
scored = append(scored, p)
}
if len(scored) == 0 {
return scored
}
maxLiq := scored[0].LiquidityInSOL
for _, p := range scored[1:] {
if p.LiquidityInSOL.GreaterThan(maxLiq) {
maxLiq = p.LiquidityInSOL
}
}
score := func(p *market.Pool) market.Decimal {
w1 := market.Decimal{}.NewFromFloat(0.35)
w2 := market.Decimal{}.NewFromFloat(0.20)
w3 := market.Decimal{}.NewFromFloat(0.05)
w4 := market.Decimal{}.NewFromFloat(0.15)
w5 := market.Decimal{}.NewFromFloat(0.10)
w6 := market.Decimal{}.NewFromFloat(0.10)
w7 := market.Decimal{}.NewFromFloat(0.03)
w8 := market.Decimal{}.NewFromFloat(0.02)
normLiq := market.Decimal{}.NewFromInt(0)
if !maxLiq.IsZero() {
normLiq = p.LiquidityInSOL.Div(maxLiq)
}
normTVL := normLiq
normVol := market.Decimal{}.NewFromInt(0)
if v, ok := p.Metadata["volume_24h"].(float64); ok {
normVol = market.Decimal{}.NewFromFloat(v).Div(market.Decimal{}.NewFromFloat(1_000_000)).Min(market.Decimal{}.NewFromInt(1))
}
fresh := market.Decimal{}.NewFromFloat(1)
if time.Since(p.LastUpdatedAt) > 30*time.Minute {
fresh = market.Decimal{}.NewFromFloat(0.3)
}
quotePref := market.Decimal{}.NewFromFloat(0.5)
if quoteMint != nil && p.QuoteMint == *quoteMint {
quotePref = market.Decimal{}.NewFromFloat(1)
} else if p.QuoteMint == solana.SolMint.String() {
quotePref = market.Decimal{}.NewFromFloat(0.95)
}
verified := market.Decimal{}.NewFromInt(0)
if p.IsVerified {
verified = market.Decimal{}.NewFromInt(1)
}
stalePenalty := market.Decimal{}.NewFromInt(0)
if time.Since(p.LastUpdatedAt) > 2*time.Hour {
stalePenalty = market.Decimal{}.NewFromInt(1)
}
zeroPenalty := market.Decimal{}.NewFromInt(0)
if p.BaseReserve.IsZero() || p.QuoteReserve.IsZero() {
zeroPenalty = market.Decimal{}.NewFromInt(1)
}
s := w1.Mul(normLiq).
Add(w2.Mul(normTVL)).
Add(w3.Mul(normVol)).
Add(w4.Mul(fresh)).
Add(w5.Mul(quotePref)).
Add(w6.Mul(verified)).
Sub(w7.Mul(stalePenalty)).
Sub(w8.Mul(zeroPenalty))
p.SelectionScore = s
p.SelectionReason = "deterministic weighted ranking"
return s
}
sort.SliceStable(scored, func(i, j int) bool { return score(scored[i]).GreaterThan(score(scored[j])) })
for i := range scored {
scored[i].IsPrimary = i == 0
}
if len(scored) > 1 && scored[0].Protocol == market.ProtocolPumpfun && scored[0].IsComplete {
for _, p := range scored {
if p.Protocol != market.ProtocolPumpfun && p.LiquidityInSOL.GreaterThan(market.Decimal{}.NewFromInt(0)) {
p.IsPrimary = true
scored[0].IsPrimary = false
break
}
}
}
return scored
}
