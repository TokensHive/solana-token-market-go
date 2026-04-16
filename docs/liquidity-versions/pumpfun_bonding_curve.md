# Pump.fun `bonding_curve`

## Protocol and IDL

| Item | Value |
| --- | --- |
| DEX | Pump.fun |
| Pool Version | `bonding_curve` |
| Program ID | `6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P` |
| IDL JSON | [`pump.json`](https://github.com/pump-fun/pump-public-docs/blob/main/idl/pump.json) |
| SDK calculator file | `sdk/protocols/pumpfun/bonding_curve/metrics.go` |

## On-chain structure used by the SDK

The calculator reads one bonding-curve state account (`PoolAddress`) and decodes:

- `virtual_token_reserves`
- `virtual_sol_reserves`
- `real_token_reserves`
- `real_sol_reserves`
- `token_total_supply`
- flags (`complete`, optional mode flags)

## Parsing flow (step-by-step)

### Step 1: Fetch and decode curve state

```go
account, err := c.rpc.GetAccount(ctx, req.PoolAddress)
if err != nil || account == nil || !account.Exists {
    return nil, err
}

state, err := decodeCurveState(account.Data)
if err != nil {
    return nil, err
}
```

### Step 2: Compute base price and liquidity from curve reserves

```go
priceAInB, liquidityInB := computeCurvePriceAndLiquidity(state)
totalSupplyFromCurve, circulatingFromCurve := computeCurveSupplies(state)
```

### Step 3: Build pair-oriented metrics

```go
priceAInB, liquidityInB = computePairMetrics(req, state)
```

The helper handles request orientation (`MintA/MintB`) and SOL/WSOL equivalence.

### Step 4: Normalize to SOL + supply/cap metrics

```go
priceAInSOL, _ := c.priceInSOL(ctx, req, priceAInB)
liquidityInSOL, _ := c.liquidityInSOL(ctx, req, liquidityInB)
totalSupply, circulating, supplyMethod, _ := c.supply.GetSupply(ctx, req.MintA)
fdvSupply, fdvMethod := supply.ResolveFDVSupply(ctx, c.rpc, req.MintA, totalSupply)
```

## Formula summary

- `PriceOfAInB`: reserve-ratio-derived curve price (orientation-aware)
- `LiquidityInB`: two-sided liquidity expressed in quote token
- `PriceOfAInSOL`: direct if quote is SOL, otherwise quote bridge conversion
- `LiquidityInSOL`: direct if quote is SOL, otherwise quote bridge conversion
- `MarketCapInSOL = PriceOfAInSOL * CirculatingSupply`
- `FDVInSOL = PriceOfAInSOL * fdv_supply`

## Metadata emitted

Typical metadata keys include curve reserve values, raw reserve integers, source label, and FDV metadata (`fdv_method`, `fdv_supply`).

## Validation/error guards

- missing RPC/supply provider
- missing/non-existent pool account
- invalid curve state shape/length
- unsupported pair orientation or zero-reserve conditions
