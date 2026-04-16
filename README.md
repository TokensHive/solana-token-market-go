# Solana Token Market Go SDK

Metrics-only Solana SDK focused on deterministic market metrics for explicit pool identifiers.

## Install

```bash
go get github.com/TokensHive/solana-token-market-go
```

## Public API

The active public SDK surface is intentionally minimal:

- `market.NewClient(...)`
- `client.GetMetricsByPool(ctx, request)`
- `client.LastRequestDebug()`
- `market.WithPoolCalculatorFactory(route, factory)` (for custom routes)

## Metrics Returned

`GetMetricsByPool` returns normalized values for:

- `PriceOfAInB`
- `PriceOfAInSOL`
- `LiquidityInB`
- `LiquidityInSOL`
- `TotalSupply`
- `CirculatingSupply`
- `MarketCapInSOL`
- `FDVInSOL`
- `SupplyMethod`

## Supported DEX/Protocols

| Dex | Pool Version | Program ID (Mainnet) | IDL / Layout Reference | Operations |
| --- | --- | --- | --- | --- |
| pumpfun | `bonding_curve` | `6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P` | [`pump.json`](https://github.com/pump-fun/pump-public-docs/blob/main/idl/pump.json) | `GetMetricsByPool` (full metrics set) |
| pumpfun | `pumpswap_amm` | `pAMMBay6oceH9fJKBRHGP5D4bD4sWpmSwMn52FMfXEA` | [`pump_amm.json`](https://github.com/pump-fun/pump-public-docs/blob/main/idl/pump_amm.json) | `GetMetricsByPool` (full metrics set) |
| raydium | `liquidity_v4` | `675kPX9MHTjS2zt1qfr1NYHuzeLXfQM9H24wFSUt1Mp8` | [`raydium_amm/idl.json`](https://github.com/raydium-io/raydium-idl/blob/master/raydium_amm/idl.json) | `GetMetricsByPool` (full metrics set) |
| raydium | `cpmm` | `CPMMoo8L3F4NbTegBCKVNunggL7H1ZpdTHKxQB5qKP1C` | [`raydium_cp_swap.json`](https://github.com/raydium-io/raydium-idl/blob/master/raydium_cpmm/raydium_cp_swap.json) | `GetMetricsByPool` (full metrics set) |
| raydium | `clmm` | `CAMMCzo5YL8w4VFF8KVHrK22GGUsp5VTaW7grrKgrWqK` | [`amm_v3.json`](https://github.com/raydium-io/raydium-idl/blob/master/raydium_clmm/amm_v3.json) | `GetMetricsByPool` (full metrics set) |
| raydium | `launchpad` | `LanMV9sAd7wArD4vJFi2qDdfnVhFxYSUg6eADduJ3uj` | [`launchpad/layout.ts`](https://github.com/raydium-io/raydium-sdk-V2/blob/master/src/raydium/launchpad/layout.ts) and [`launch-cpi/states.rs`](https://github.com/raydium-io/raydium-cpi/blob/master/programs/launch-cpi/src/states.rs) | `GetMetricsByPool` (full metrics set) |
| meteora | `dlmm` | `LBUZKhRxPF3XUpBCjp4YzTKgLccjZhTSDM9YuVaPwxo` | [`dlmm.json`](https://github.com/MeteoraAg/dlmm-sdk/blob/main/idls/dlmm.json) | `GetMetricsByPool` (full metrics set) |
| meteora | `dbc` | `dbcij3LWUppWqq96dh6gJWwBifmcGfLSB5D4DuSMaqN` | [`idl.json`](https://github.com/MeteoraAg/dynamic-bonding-curve-sdk/blob/main/packages/dynamic-bonding-curve/src/idl/dynamic-bonding-curve/idl.json) | `GetMetricsByPool` (full metrics set) |
| meteora | `damm_v1` | `Eo7WjKq67rjJQSZxS6z3YkapzY3eMj6Xy8X5EQVn5UaB` | [`ts-client/src/amm/idl.ts`](https://github.com/MeteoraAg/damm-v1-sdk/blob/main/ts-client/src/amm/idl.ts) | `GetMetricsByPool` (full metrics set) |
| meteora | `damm_v2` | `cpamdpZCGKUy5JxQXB4dcpGPiikHawvSWAd6mEn1sGG` | [`cp_amm.json`](https://github.com/MeteoraAg/damm-v2-sdk/blob/main/src/idl/cp_amm.json) | `GetMetricsByPool` (full metrics set) |
| orca | `whirlpool` | `whirLbMiicVdio4qvUfM5KAg6Ct8VwpYzGff3uctyCc` | [Orca Whirlpool IDL docs](https://dev.orca.so/More%20Resources/IDL/) | `GetMetricsByPool` (full metrics set) |

## Quick Start

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

	resp, err := client.GetMetricsByPool(ctx, market.GetMetricsByPoolRequest{
		Pool: market.PoolIdentifier{
			Dex:         market.DexPumpfun,
			PoolVersion: market.PoolVersionPumpfunBondingCurve,
			MintA:       solana.MustPublicKeyFromBase58("9BHt7aq3DFCb74kZjPY5epgVtsWKCeYX1tUWxYwDpump"),
			MintB:       solana.SolMint,
			PoolAddress: solana.MustPublicKeyFromBase58("2ebnjcJ5f8V6NhvWQfagfo6f6Mpn8jL42x8UxKM4Gz8H"),
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("price_a_in_sol:", resp.PriceOfAInSOL)
	fmt.Println("liquidity_in_sol:", resp.LiquidityInSOL)
	fmt.Println("market_cap_in_sol:", resp.MarketCapInSOL)
	fmt.Println("fdv_in_sol:", resp.FDVInSOL)
}
```

## Route Examples By Dex / Pool Version

### Pump.fun

`bonding_curve`:

```go
resp, err := client.GetMetricsByPool(ctx, market.GetMetricsByPoolRequest{
	Pool: market.PoolIdentifier{
		Dex:         market.DexPumpfun,
		PoolVersion: market.PoolVersionPumpfunBondingCurve,
		MintA:       solana.MustPublicKeyFromBase58("9BHt7aq3DFCb74kZjPY5epgVtsWKCeYX1tUWxYwDpump"),
		MintB:       solana.SolMint,
		PoolAddress: solana.MustPublicKeyFromBase58("2ebnjcJ5f8V6NhvWQfagfo6f6Mpn8jL42x8UxKM4Gz8H"),
	},
})
_ = resp
_ = err
```

`pumpswap_amm`:

```go
resp, err := client.GetMetricsByPool(ctx, market.GetMetricsByPoolRequest{
	Pool: market.PoolIdentifier{
		Dex:         market.DexPumpfun,
		PoolVersion: market.PoolVersionPumpfunAmm,
		MintA:       solana.MustPublicKeyFromBase58("9BHt7aq3DFCb74kZjPY5epgVtsWKCeYX1tUWxYwDpump"),
		MintB:       solana.SolMint,
		PoolAddress: solana.MustPublicKeyFromBase58("EQqvZi6mSaQL95wWkP5vGBX6ZsAkVTqZCV88rQU1fbcY"),
	},
})
_ = resp
_ = err
```

### Raydium

`liquidity_v4`:

```go
resp, err := client.GetMetricsByPool(ctx, market.GetMetricsByPoolRequest{
	Pool: market.PoolIdentifier{
		Dex:         market.DexRaydium,
		PoolVersion: market.PoolVersionRaydiumLiquidityV4,
		MintA:       solana.MustPublicKeyFromBase58("2nCeHpECQvnMfzjU5fDMAKws1vBxMzxvWr6qqLpApump"),
		MintB:       solana.SolMint,
		PoolAddress: solana.MustPublicKeyFromBase58("81BTnebmHFZdVMhFKHhQKAnEwgGPTNbMj1fezsbUjtkG"),
	},
})
_ = resp
_ = err
```

`cpmm`:

```go
resp, err := client.GetMetricsByPool(ctx, market.GetMetricsByPoolRequest{
	Pool: market.PoolIdentifier{
		Dex:         market.DexRaydium,
		PoolVersion: market.PoolVersionRaydiumCPMM,
		MintA:       solana.MustPublicKeyFromBase58("3z2tRjNuQjoq6UDcw4zyEPD1Eb5KXMPYb4GWFzVT1DPg"),
		MintB:       solana.SolMint,
		PoolAddress: solana.MustPublicKeyFromBase58("BScfGKZf9YDfpL11hZQnCQPskPrdeyFcvCjSA5qupEH5"),
	},
})
_ = resp
_ = err
```

`clmm`:

```go
resp, err := client.GetMetricsByPool(ctx, market.GetMetricsByPoolRequest{
	Pool: market.PoolIdentifier{
		Dex:         market.DexRaydium,
		PoolVersion: market.PoolVersionRaydiumCLMM,
		MintA:       solana.MustPublicKeyFromBase58("EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"),
		MintB:       solana.SolMint,
		PoolAddress: solana.MustPublicKeyFromBase58("2QdhepnKRTLjjSqPL1PtKNwqrUkoLee5Gqs8bvZhRdMv"),
	},
})
_ = resp
_ = err
```

`launchpad`:

```go
resp, err := client.GetMetricsByPool(ctx, market.GetMetricsByPoolRequest{
	Pool: market.PoolIdentifier{
		Dex:         market.DexRaydium,
		PoolVersion: market.PoolVersionRaydiumLaunchpad,
		MintA:       solana.MustPublicKeyFromBase58("nXU1zKEAyqPJmsjfdSuc4bcFUt8huu6GFUVgziebonk"),
		MintB:       solana.SolMint,
		PoolAddress: solana.MustPublicKeyFromBase58("257urGqFaYq3BjCVrA6GS7MdfyZR4mb11RWEeuG73LYG"),
	},
})
_ = resp
_ = err
```

### Meteora

`dlmm`:

```go
resp, err := client.GetMetricsByPool(ctx, market.GetMetricsByPoolRequest{
	Pool: market.PoolIdentifier{
		Dex:         market.DexMeteora,
		PoolVersion: market.PoolVersionMeteoraDLMM,
		MintA:       solana.MustPublicKeyFromBase58("BFiGUxnidogqcZAPVPDZRCfhx3nXnFLYqpQUaUGpump"),
		MintB:       solana.SolMint,
		PoolAddress: solana.MustPublicKeyFromBase58("6KvXWfjwZ7mfiFALRDmj4YvJw3LxfSnLadj1kZfBykYp"),
	},
})
_ = resp
_ = err
```

`dbc`:

```go
resp, err := client.GetMetricsByPool(ctx, market.GetMetricsByPoolRequest{
	Pool: market.PoolIdentifier{
		Dex:         market.DexMeteora,
		PoolVersion: market.PoolVersionMeteoraDBC,
		MintA:       solana.MustPublicKeyFromBase58("8pmn9W36uuJDACuw4wVTtrnw4rGDhmiP9Kgsne5Hbrrr"),
		MintB:       solana.SolMint,
		PoolAddress: solana.MustPublicKeyFromBase58("Fc2rywpnDPrb4ik2V31tKTdo4EEWTZaHQJCWcLjAUvFD"),
	},
})
_ = resp
_ = err
```

`damm_v1`:

```go
resp, err := client.GetMetricsByPool(ctx, market.GetMetricsByPoolRequest{
	Pool: market.PoolIdentifier{
		Dex:         market.DexMeteora,
		PoolVersion: market.PoolVersionMeteoraDAMMV1,
		MintA:       solana.MustPublicKeyFromBase58("C29ebrgYjYoJPMGPnPSGY1q3mMGk4iDSqnQeQQA7moon"),
		MintB:       solana.SolMint,
		PoolAddress: solana.MustPublicKeyFromBase58("7rQd8FhC1rimV3v9edCRZ6RNFsJN1puXM9UmjaURJRNj"),
	},
})
_ = resp
_ = err
```

`damm_v2`:

```go
resp, err := client.GetMetricsByPool(ctx, market.GetMetricsByPoolRequest{
	Pool: market.PoolIdentifier{
		Dex:         market.DexMeteora,
		PoolVersion: market.PoolVersionMeteoraDAMMV2,
		MintA:       solana.MustPublicKeyFromBase58("EkJuyYyD3to61CHVPJn6wHb7xANxvqApnVJ4o2SdBAGS"),
		MintB:       solana.SolMint,
		PoolAddress: solana.MustPublicKeyFromBase58("8Lvqv2jgNvcx1NDtMHd5Ahx8ZUjETfLwygq9MtDfPHxe"),
	},
})
_ = resp
_ = err
```

### Orca

`whirlpool`:

```go
resp, err := client.GetMetricsByPool(ctx, market.GetMetricsByPoolRequest{
	Pool: market.PoolIdentifier{
		Dex:         market.DexOrca,
		PoolVersion: market.PoolVersionOrcaWhirlpool,
		MintA:       solana.MustPublicKeyFromBase58("5552z6Qp2xr596ox1UVN4ppDwwyjCfY8cXwzHMXgMcaS"),
		MintB:       solana.SolMint,
		PoolAddress: solana.MustPublicKeyFromBase58("EAzwsTfbdPmjW5eEWtxNpXaHLzVJmpTUGzkTPnCPrTHd"),
	},
})
_ = resp
_ = err
```

## Example Runner

The `examples` folder provides one interactive CLI to run all supported routes.

Interactive mode:

```bash
go run ./examples
```

Non-interactive batch mode:

```bash
go run ./examples -interactive=false
```

Optional flags:

- `-rpc` custom RPC endpoint
- `-timeout` per-request timeout (for example `60s`)
- `-debug` print or hide `LastRequestDebug()` output

## Extending With New Markets

The SDK routes calculators through a `Dex + PoolVersion` registry.

- built-in routes are registered by default in `market.NewService(...)`
- custom routes can be added or overridden with `market.WithPoolCalculatorFactory(...)`

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
	return &market.GetMetricsByPoolResponse{Pool: pool}, nil
}
```
