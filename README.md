# solana-token-market-go

On-chain-first Go SDK for Solana token market discovery and pricing.

## Architecture

- `sdk/market`: public client API, normalized models, typed requests/responses
- `sdk/discovery`: layered discovery (deterministic PDA -> parser evidence -> rpc fallback -> optional api)
- `sdk/protocols`: protocol adapters for Pumpfun, PumpSwap, Raydium, Orca, Meteora
- `sdk/rpc`: bounded RPC abstraction and account/supply helpers
- `sdk/parser`: adapter hooks for `github.com/DefaultPerson/solana-dex-parser-go`
- `sdk/supply`: total/circulating supply providers
- `sdk/quote`: SOL/stable quote bridging abstraction

## Supported Protocols

- Pumpfun bonding curve
- PumpSwap
- Raydium V4
- Raydium CPMM
- Raydium CLMM
- Raydium LaunchLab
- Orca Whirlpool
- Meteora DLMM
- Meteora DAMM
- Meteora DBC

## On-chain vs API-first

Default is `onchain`.

`DiscoveryMode` values:
- `onchain` (default)
- `api-first` (optional, must still verify on-chain)
- `hybrid`

`PreferRaydiumAPI` and `PreferMeteoraAPI` are optional hints only.
When `PreferRaydiumAPI` is enabled and on-chain discovery is empty (or has no usable SOL price),
the SDK applies API fallback discovery (Raydium first, then DexScreener enrichment) to recover
real pool pairs/prices.

## Public API

- `NewClient(opts ...Option)`
- `ResolvePools(ctx, req)`
- `GetPool(ctx, req)`
- `GetTokenMarket(ctx, req)`
- `FindPoolsByMint`, `FindPoolsByPair`, `FindPoolsByProtocol`
- `ComputePoolMetrics`, `ComputeTokenMetricsFromPool`, `SelectPrimaryPool`

## Ranking

Deterministic ranking is centralized and stores:
- `SelectionScore`
- `SelectionReason`

Policy:
- prefer SOL quote pools
- prefer verified pools
- prefer higher liquidity/fresher pools
- penalize stale/empty pools
- for completed Pumpfun curves, migrated funded AMMs are preferred as primary

## Usage

Run the unified examples CLI:

```bash
go run ./examples -cmd interactive
go run ./examples -cmd batch-all -debug
```

Direct commands:

```bash
go run ./examples -cmd resolve-pools -mint 3y6kjbdG3ULceQMuJh3RWz68bGoEZ3U1YBeJyXbJpump
go run ./examples -cmd get-token-market -mint 9BHt7aq3DFCb74kZjPY5epgVtsWKCeYX1tUWxYwDpump
go run ./examples -cmd all-methods -mint Dfh5DzRgSvvCFDoYc2ciTkMrbDfRKybA4SoFbPmApump
go run ./examples -cmd batch-all
```

Legacy entry points are still available:
- `go run ./examples/resolve_pools`
- `go run ./examples/get_token_market`
- `go run ./examples/get_pool`

SDK option for request debug telemetry:

```go
client, err := market.NewClient(
    market.WithDebugRequests(true),
)
```

When enabled, each public API call records categorized request usage in debug metadata
(`rpc` by operation type, `api` by source + operation type), and the examples CLI prints it.

To enable default parser registrations explicitly (opt-in):

```go
import _ "github.com/TokensHive/solana-token-market-go/sdk/parser/all"
```

## Protocol limitations / approximations

- CLMM/Whirlpool/DLMM liquidity is currently best-effort and marked via metadata.
- Full tick/bin depth traversal is not yet implemented.
- Parser adapter defaults to noop unless parser-backed evidence source is injected.

## TODO (clear gaps)

- Implement parser-backed production adapter using transaction/yellowstone ingestion.
- Implement full on-chain decoders for each protocol state layout.
- Implement stable->SOL bridge source for non-SOL quote pools.
- Add optional API bootstrap integrations with strict on-chain verification.
- Expand golden fixtures with real mainnet snapshots.
