package discovery

import (
"github.com/TokensHive/solana-token-market-go/sdk/market"
"github.com/gagliardetto/solana-go"
)

type FilterOptions struct {
Protocols         []market.Protocol
MarketTypes       []market.MarketType
QuoteMint         *solana.PublicKey
IncludeInactive   bool
IncludeUnverified bool
PoolAddresses     []solana.PublicKey
DirectSOLOnly     bool
}

func ApplyFilters(pools []*market.Pool, opts FilterOptions) []*market.Pool {
protocolSet := make(map[market.Protocol]struct{}, len(opts.Protocols))
for _, p := range opts.Protocols {
protocolSet[p] = struct{}{}
}
marketTypeSet := make(map[market.MarketType]struct{}, len(opts.MarketTypes))
for _, m := range opts.MarketTypes {
marketTypeSet[m] = struct{}{}
}
poolSet := make(map[string]struct{}, len(opts.PoolAddresses))
for _, p := range opts.PoolAddresses {
poolSet[p.String()] = struct{}{}
}

out := make([]*market.Pool, 0, len(pools))
for _, p := range pools {
if len(protocolSet) > 0 {
if _, ok := protocolSet[p.Protocol]; !ok {
continue
}
}
if len(marketTypeSet) > 0 {
if _, ok := marketTypeSet[p.MarketType]; !ok {
continue
}
}
if opts.QuoteMint != nil && p.QuoteMint != opts.QuoteMint.String() {
continue
}
if !opts.IncludeInactive && !p.IsActive {
continue
}
if !opts.IncludeUnverified && !p.IsVerified {
continue
}
if len(poolSet) > 0 {
if _, ok := poolSet[p.Address]; !ok {
continue
}
}
if opts.DirectSOLOnly && p.QuoteMint != solana.SolMint.String() {
continue
}
out = append(out, p)
}
return out
}
