# Raydium `launchpad`

## Protocol and IDL

| Item | Value |
| --- | --- |
| DEX | Raydium |
| Pool Version | `launchpad` |
| Program ID | `LanMV9sAd7wArD4vJFi2qDdfnVhFxYSUg6eADduJ3uj` |
| IDL JSON / layout | [`layout.ts`](https://github.com/raydium-io/raydium-sdk-V2/blob/master/src/raydium/launchpad/layout.ts), [`states.rs`](https://github.com/raydium-io/raydium-cpi/blob/master/programs/launch-cpi/src/states.rs) |
| SDK calculator file | `sdk/protocols/raydium/launchpad/metrics.go` |

## On-chain structure used by the SDK

- Launchpad `PoolState` account
- Launchpad `GlobalConfig` account
- virtual + real reserves
- curve type discriminator:
  - constant product
  - fixed price
  - linear price

## Parsing flow (step-by-step)

### Step 1: Decode pool and config

```go
poolInfo, _ := c.rpc.GetAccount(ctx, req.PoolAddress)
state, _ := decodePoolState(poolInfo.Data)

configInfo, _ := c.rpc.GetAccount(ctx, state.configAddress)
config, _ := decodeConfigState(configInfo.Data)
```

### Step 2: Build reserve snapshot from launchpad fields

```go
snapshot := &reserveSnapshot{
    baseMint: state.baseMint,
    quoteMint: state.quoteMint,
    baseReserve: reserveForPoolState(state, true),
    quoteReserve: reserveForPoolState(state, false),
}
```

### Step 3: Price by curve type

```go
priceBaseInQuote, err := priceBaseInQuoteByCurve(state, state.curveType)
priceAInB := priceOfMintAInMintB(req, snapshot, priceBaseInQuote)
liquidityInB := liquidityInMintB(req, snapshot, priceBaseInQuote)
```

### Step 4: SOL normalization + cap metrics

```go
priceAInSOL, _ := c.priceOfMintAInSOL(ctx, req, priceAInB)
liquidityInSOL, _ := c.liquidityInSOL(ctx, req, liquidityInB)
totalSupply, circulating, supplyMethod, _ := c.supply.GetSupply(ctx, req.MintA)
fdvSupply, fdvMethod := supply.ResolveFDVSupply(ctx, c.rpc, req.MintA, totalSupply)
```

## Formula summary

- curve-specific price logic (`priceBaseInQuoteByCurve`)
- quote liquidity from reserve composition
- cap and FDV from SOL-normalized price and supply

## Metadata emitted

Curve type/name, config fields, virtual/real reserves, pool timestamps, source/program, FDV metadata.
