# Solana Token Market Go SDK

Metrics-only Solana SDK focused on deterministic market metrics for explicit pool identifiers.

## Install

```bash
go get github.com/TokensHive/solana-token-market-go
```

## Supported in Phase 2

- `dex`: `pumpfun`
- `poolVersion`:
  - `bonding_curve`
  - `pumpswap_amm`

## Public API

The active public SDK surface is:

- `market.NewClient(...)`
- `client.GetMetricsByPool(ctx, request)`
- `client.LastRequestDebug()`
- `market.WithPoolCalculatorFactory(route, factory)` (for adding new market routes)

## Metrics Output

`GetMetricsByPool` returns normalized values for:

- `PriceOfAInB`
- `PriceOfAInSOL`
- `LiquidityInB`
- `LiquidityInSOL`
- `TotalSupply`
- `CirculatingSupply`
- `MarketCapInSOL`
- `SupplyMethod`

## Quick Example

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/TokensHive/solana-token-market-go/sdk/market"
	"github.com/TokensHive/solana-token-market-go/sdk/rpc"
	"github.com/gagliardetto/solana-go"
)

func main() {
	client, err := market.NewClient(
		market.WithRPCClient(rpc.NewSolanaRPCClient("https://api.mainnet-beta.solana.com")),
		market.WithDebugRequests(true),
	)
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	mintA := solana.MustPublicKeyFromBase58("9BHt7aq3DFCb74kZjPY5epgVtsWKCeYX1tUWxYwDpump")
	mintB := solana.SolMint

	resp, err := client.GetMetricsByPool(ctx, market.GetMetricsByPoolRequest{
		Pool: market.PoolIdentifier{
			Dex:         market.DexPumpfun,
			PoolVersion: market.PoolVersionPumpfunBondingCurve,
			MintA:       mintA,
			MintB:       mintB,
			PoolAddress: solana.MustPublicKeyFromBase58("2ebnjcJ5f8V6NhvWQfagfo6f6Mpn8jL42x8UxKM4Gz8H"),
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("price_a_in_sol:", resp.PriceOfAInSOL)
	fmt.Println("liquidity_in_sol:", resp.LiquidityInSOL)
	fmt.Println("market_cap_in_sol:", resp.MarketCapInSOL)
}
```

## Example Runner

The `examples` folder is intentionally minimal and now contains a single interactive CLI:

```bash
go run ./examples
```

### Interactive mode (default)

`go run ./examples` opens a menu to test all public methods in this SDK surface:

- `market.NewClient(...)`
- `client.GetMetricsByPool(...)`
- `client.LastRequestDebug()`

You can:

- run default Pumpfun bonding-curve metrics request
- run default PumpSwap AMM metrics request
- run both defaults

### Non-interactive mode (automation/CI)

```bash
go run ./examples -interactive=false
```

Optional flags:

- `-rpc` custom RPC endpoint
- `-timeout` per-request timeout (example: `60s`)
- `-debug` print or hide `LastRequestDebug()` output

## Extending With New Markets

The SDK now routes metrics calculators through a `Dex + PoolVersion` registry.

- built-in routes are registered by default in `market.NewService(...)`
- you can register/override routes with `market.WithPoolCalculatorFactory(...)`

High-level pattern:

```go
client, err := market.NewClient(
	market.WithRPCClient(rpc.NewSolanaRPCClient("https://api.mainnet-beta.solana.com")),
	market.WithPoolCalculatorFactory(
		market.PoolRoute{Dex: "my_dex", PoolVersion: "my_pool_v1"},
		func(cfg market.Config) market.PoolCalculator {
			return myCalculator{cfg: cfg}
		},
	),
)

type myCalculator struct {
	cfg market.Config
}

func (m myCalculator) Compute(ctx context.Context, pool market.PoolIdentifier) (*market.GetMetricsByPoolResponse, error) {
	// call your on-chain calculator and return normalized response
	return &market.GetMetricsByPoolResponse{Pool: pool}, nil
}
```
