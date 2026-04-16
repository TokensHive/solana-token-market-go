# Raydium `liquidity_v4`

## Protocol and IDL

| Item | Value |
| --- | --- |
| DEX | Raydium |
| Pool Version | `liquidity_v4` |
| Program ID | `675kPX9MHTjS2zt1qfr1NYHuzeLXfQM9H24wFSUt1Mp8` |
| IDL JSON | [`raydium_amm/idl.json`](https://github.com/raydium-io/raydium-idl/blob/master/raydium_amm/idl.json) |
| SDK calculator file | `sdk/protocols/raydium/liquidity_v4/metrics.go` |

## On-chain structure used by the SDK

- Liquidity V4 pool state (`decodePoolState`)
- base vault + quote vault token account amounts
- optional OpenOrders totals from layout-backed fields
- mint accounts for decimals

## Parsing flow (step-by-step)

### Step 1: Read and validate pool state

```go
poolInfo, err := c.rpc.GetAccount(ctx, req.PoolAddress)
state, err := decodePoolState(poolInfo.Data)
if err != nil {
    return nil, err
}
```

### Step 2: Batch-fetch vault/mint accounts

```go
accounts, err := c.rpc.GetMultipleAccounts(ctx, []solana.PublicKey{
    state.baseVault, state.quoteVault, state.baseMint, state.quoteMint,
})
baseRaw, _ := decodeTokenAmount(accounts[0].Data)
quoteRaw, _ := decodeTokenAmount(accounts[1].Data)
baseDecimals, _ := decodeMintDecimals(accounts[2].Data)
quoteDecimals, _ := decodeMintDecimals(accounts[3].Data)
```

### Step 3: Build reserve snapshot and compute pair metrics

```go
snapshot := &reserveSnapshot{
    baseMint: state.baseMint,
    quoteMint: state.quoteMint,
    baseReserve: decimalFromU64(baseRaw, baseDecimals),
    quoteReserve: decimalFromU64(quoteRaw, quoteDecimals),
}

priceAInB := priceOfMintAInMintB(req, snapshot)
liquidityInB := liquidityInMintB(req, snapshot)
```

### Step 4: SOL normalization and cap metrics

```go
priceAInSOL, _ := c.priceOfMintAInSOL(ctx, req, priceAInB)
liquidityInSOL, _ := c.liquidityInSOL(ctx, req, liquidityInB)
totalSupply, circulating, supplyMethod, _ := c.supply.GetSupply(ctx, req.MintA)
fdvSupply, fdvMethod := supply.ResolveFDVSupply(ctx, c.rpc, req.MintA, totalSupply)
```

## Formula summary

- `PriceOfAInB`: reserve ratio with orientation-aware inversion
- `LiquidityInB`: quote reserve + base reserve converted to quote
- `MarketCapInSOL = PriceOfAInSOL * CirculatingSupply`
- `FDVInSOL = PriceOfAInSOL * fdv_supply`

## Metadata emitted

Pool mints/vaults, reserves, decimals, open-orders derived values, source/program, FDV method.
