# Raydium `clmm`

## Protocol and IDL

| Item | Value |
| --- | --- |
| DEX | Raydium |
| Pool Version | `clmm` |
| Program ID | `CAMMCzo5YL8w4VFF8KVHrK22GGUsp5VTaW7grrKgrWqK` |
| IDL JSON | [`amm_v3.json`](https://github.com/raydium-io/raydium-idl/blob/master/raydium_clmm/amm_v3.json) |
| SDK calculator file | `sdk/protocols/raydium/clmm/metrics.go` |

## On-chain structure used by the SDK

- CLMM pool state with:
  - `sqrt_price_x64`
  - fee growth / protocol fee owed fields
  - token mint and vault addresses
- token vault balances
- mint decimals

## Parsing flow (step-by-step)

### Step 1: Decode CLMM pool state

```go
poolInfo, err := c.rpc.GetAccount(ctx, req.PoolAddress)
state, err := decodePoolState(poolInfo.Data)
```

### Step 2: Fetch vaults/mints and subtract protocol fees

```go
accounts, _ := c.rpc.GetMultipleAccounts(ctx, []solana.PublicKey{
    state.token0Vault, state.token1Vault, state.token0Mint, state.token1Mint,
})
token0Raw, _ := decodeTokenAmount(accounts[0].Data)
token1Raw, _ := decodeTokenAmount(accounts[1].Data)
token0Raw = subtractFees(token0Raw, state.protocolFeeOwed0)
token1Raw = subtractFees(token1Raw, state.protocolFeeOwed1)
```

### Step 3: Convert sqrt price and compute pair metrics

```go
priceToken1InToken0 := priceToken1InToken0FromSqrt(state.sqrtPriceX64Raw, token0Decimals, token1Decimals)
priceAInB := priceOfMintAInMintB(req, state, priceToken1InToken0)
liquidityInB := liquidityInMintB(req, snapshot)
```

### Step 4: SOL normalization + cap/FDV

```go
priceAInSOL, _ := c.priceOfMintAInSOL(ctx, req, priceAInB)
liquidityInSOL, _ := c.liquidityInSOL(ctx, req, liquidityInB)
totalSupply, circulating, supplyMethod, _ := c.supply.GetSupply(ctx, req.MintA)
fdvSupply, fdvMethod := supply.ResolveFDVSupply(ctx, c.rpc, req.MintA, totalSupply)
```

## Formula summary

- CLMM spot price from `sqrt_price_x64` conversion
- quote liquidity from both reserve sides
- `MarketCapInSOL = price_a_in_sol * circulating_supply`
- `FDVInSOL = price_a_in_sol * fdv_supply`

## Metadata emitted

Tick/price fields, fee fields, reserve snapshots, decimals, source/program, FDV metadata.
