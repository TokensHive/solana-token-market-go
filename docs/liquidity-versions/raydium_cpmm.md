# Raydium `cpmm`

## Protocol and IDL

| Item | Value |
| --- | --- |
| DEX | Raydium |
| Pool Version | `cpmm` |
| Program ID | `CPMMoo8L3F4NbTegBCKVNunggL7H1ZpdTHKxQB5qKP1C` |
| IDL JSON | [`raydium_cp_swap.json`](https://github.com/raydium-io/raydium-idl/blob/master/raydium_cpmm/raydium_cp_swap.json) |
| SDK calculator file | `sdk/protocols/raydium/cpmm/metrics.go` |

## On-chain structure used by the SDK

- CPMM pool state (`decodePoolState`)
- token vault A/B token accounts
- mint A/B decimals
- protocol-fee fields netted from vault balances (`subtractFees`)

## Parsing flow (step-by-step)

### Step 1: Decode pool account

```go
poolInfo, err := c.rpc.GetAccount(ctx, req.PoolAddress)
state, err := decodePoolState(poolInfo.Data)
```

### Step 2: Fetch and decode vault + mint accounts

```go
accounts, err := c.rpc.GetMultipleAccounts(ctx, []solana.PublicKey{
    state.token0Vault, state.token1Vault, state.token0Mint, state.token1Mint,
})

token0Raw, _ := decodeTokenAmount(accounts[0].Data)
token1Raw, _ := decodeTokenAmount(accounts[1].Data)
token0Raw = subtractFees(token0Raw, state.protocolFee0)
token1Raw = subtractFees(token1Raw, state.protocolFee1)
```

### Step 3: Price and liquidity in quote

```go
snapshot := &reserveSnapshot{ /* token mints + decimal reserves */ }
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

- `PriceOfAInB`: reserve ratio (orientation-aware)
- `LiquidityInB`: two-sided reserve valuation in quote
- `MarketCapInSOL`: `PriceOfAInSOL * CirculatingSupply`
- `FDVInSOL`: `PriceOfAInSOL * fdv_supply`

## Metadata emitted

Net reserves, raw reserves, fee counters, mint/vault addresses, decimals, source/program, FDV details.
