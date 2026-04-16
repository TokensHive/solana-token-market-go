# Pump.fun `pumpswap_amm`

## Protocol and IDL

| Item | Value |
| --- | --- |
| DEX | Pump.fun |
| Pool Version | `pumpswap_amm` |
| Program ID | `pAMMBay6oceH9fJKBRHGP5D4bD4sWpmSwMn52FMfXEA` |
| IDL JSON | [`pump_amm.json`](https://github.com/pump-fun/pump-public-docs/blob/main/idl/pump_amm.json) |
| SDK calculator file | `sdk/protocols/pumpfun/pumpswap_amm/metrics.go` |

## On-chain structure used by the SDK

The calculator decodes:

- PumpSwap pool state (`PoolAddress`)
- token vault A + token vault B token-account balances
- token mint A + token mint B decimals

## Parsing flow (step-by-step)

### Step 1: Fetch and decode pool state

```go
poolInfo, err := c.rpc.GetAccount(ctx, req.PoolAddress)
state, err := decodePoolState(poolInfo.Data)
if err != nil {
    return nil, err
}
```

### Step 2: Fetch vault balances and mint decimals

```go
accounts, err := c.rpc.GetMultipleAccounts(ctx, []solana.PublicKey{
    state.tokenAVault, state.tokenBVault, state.tokenAMint, state.tokenBMint,
})

tokenARaw, _ := decodeTokenAmount(accounts[0].Data)
tokenBRaw, _ := decodeTokenAmount(accounts[1].Data)
tokenADecimals, _ := decodeMintDecimals(accounts[2].Data)
tokenBDecimals, _ := decodeMintDecimals(accounts[3].Data)
```

### Step 3: Compute reserve snapshot + pair metrics

```go
snapshot := &reserveSnapshot{
    tokenAMint: state.tokenAMint,
    tokenBMint: state.tokenBMint,
    tokenAReserve: decimalFromU64(tokenARaw, tokenADecimals),
    tokenBReserve: decimalFromU64(tokenBRaw, tokenBDecimals),
}

priceAInB := priceOfMintAInMintB(req, snapshot)
liquidityInB := liquidityInMintB(req, snapshot)
```

### Step 4: Normalize to SOL and cap metrics

```go
priceAInSOL, _ := c.priceOfMintAInSOL(ctx, req, priceAInB)
liquidityInSOL, _ := c.liquidityInSOL(ctx, req, liquidityInB)
totalSupply, circulating, supplyMethod, _ := c.supply.GetSupply(ctx, req.MintA)
fdvSupply, fdvMethod := supply.ResolveFDVSupply(ctx, c.rpc, req.MintA, totalSupply)
```

## Formula summary

- `PriceOfAInB`: direct or inverse reserve ratio depending on pair orientation
- `LiquidityInB`: quote-side reserve + converted base-side reserve
- `MarketCapInSOL`: `price_a_in_sol * circulating_supply`
- `FDVInSOL`: `price_a_in_sol * fdv_supply`

## Metadata emitted

Pool mints, vaults, token decimals, token reserves, source labels, FDV metadata.

## Validation/error guards

- invalid pool owner/discriminator
- missing vault/mint accounts
- zero-reserve protection
- mismatched pair vs pool tokens
