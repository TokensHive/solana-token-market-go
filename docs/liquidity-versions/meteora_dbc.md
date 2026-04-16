# Meteora `dbc` (Dynamic Bonding Curve)

## Protocol and IDL

| Item | Value |
| --- | --- |
| DEX | Meteora |
| Pool Version | `dbc` |
| Program ID | `dbcij3LWUppWqq96dh6gJWwBifmcGfLSB5D4DuSMaqN` |
| IDL JSON | [`dynamic-bonding-curve idl.json`](https://github.com/MeteoraAg/dynamic-bonding-curve-sdk/blob/main/packages/dynamic-bonding-curve/src/idl/dynamic-bonding-curve/idl.json) |
| SDK calculator file | `sdk/protocols/meteora/dbc/metrics.go` |

## On-chain structure used by the SDK

- DBC pool state (`decodePoolState`)
- vault token accounts for both sides
- mint accounts for decimals
- sqrt-price-style curve fields

## Parsing flow (step-by-step)

### Step 1: Decode DBC pool account

```go
poolInfo, _ := c.rpc.GetAccount(ctx, req.PoolAddress)
state, _ := decodePoolState(poolInfo.Data)
```

### Step 2: Read vault balances and decimals

```go
accounts, _ := c.rpc.GetMultipleAccounts(ctx, []solana.PublicKey{
    state.token0Vault, state.token1Vault, state.token0Mint, state.token1Mint,
})
token0Raw, _ := decodeTokenAccountMint(accounts[0].Data)
token1Raw, _ := decodeTokenAccountMint(accounts[1].Data)
token0Decimals, _ := decodeMintDecimals(accounts[2].Data)
token1Decimals, _ := decodeMintDecimals(accounts[3].Data)
```

### Step 3: Price from curve representation

```go
priceBaseInQuote := priceBaseInQuoteFromSqrt(state.sqrtPriceRaw, token0Decimals, token1Decimals)
priceAInB := priceOfMintAInMintB(req, snapshot, priceBaseInQuote)
liquidityInB := liquidityInMintB(req, snapshot, priceBaseInQuote)
```

### Step 4: SOL normalization and market metrics

```go
priceAInSOL, _ := c.priceOfMintAInSOL(ctx, req, priceAInB)
liquidityInSOL, _ := c.liquidityInSOL(ctx, req, liquidityInB)
totalSupply, circulating, supplyMethod, _ := c.supply.GetSupply(ctx, req.MintA)
fdvSupply, fdvMethod := supply.ResolveFDVSupply(ctx, c.rpc, req.MintA, totalSupply)
```

## Formula summary

- `PriceOfAInB`: curve sqrt-derived price with decimal normalization
- `LiquidityInB`: reserve valuation in quote
- `MarketCapInSOL`: `price_a_in_sol * circulating_supply`
- `FDVInSOL`: `price_a_in_sol * fdv_supply`

## Metadata emitted

Curve fields, mints/vaults, reserves, decimals, source/program, FDV method/supply.
