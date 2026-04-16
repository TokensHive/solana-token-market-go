# Orca `whirlpool`

## Protocol and IDL

| Item | Value |
| --- | --- |
| DEX | Orca |
| Pool Version | `whirlpool` |
| Program ID | `whirLbMiicVdio4qvUfM5KAg6Ct8VwpYzGff3uctyCc` |
| IDL JSON / docs | [Orca Whirlpool IDL docs](https://dev.orca.so/More%20Resources/IDL/) |
| SDK calculator file | `sdk/protocols/orca/whirlpool/metrics.go` |

## On-chain structure used by the SDK

- whirlpool account (`decodePoolState`)
- sqrt price (`sqrtPriceRaw`)
- protocol fee owed (`protocolFeeOwedA`, `protocolFeeOwedB`)
- vault token balances + mint decimals

## Parsing flow (step-by-step)

### Step 1: Decode whirlpool state

```go
poolInfo, _ := c.rpc.GetAccount(ctx, req.PoolAddress)
state, _ := decodePoolState(poolInfo.Data)
```

### Step 2: Decode vault balances and net fees

```go
accounts, _ := c.rpc.GetMultipleAccounts(ctx, []solana.PublicKey{
    state.tokenVaultA, state.tokenVaultB, state.tokenMintA, state.tokenMintB,
})
tokenVaultARaw, _ := decodeTokenAmount(accounts[0].Data)
tokenVaultBRaw, _ := decodeTokenAmount(accounts[1].Data)
tokenANetRaw := subtractProtocolFee(tokenVaultARaw, state.protocolFeeOwedA)
tokenBNetRaw := subtractProtocolFee(tokenVaultBRaw, state.protocolFeeOwedB)
```

### Step 3: Price conversion and pair metrics

```go
priceTokenAInTokenB := priceTokenAInTokenBFromSqrt(state.sqrtPriceRaw, tokenADecimals, tokenBDecimals)
priceAInB := priceOfMintAInMintB(req, state, priceTokenAInTokenB)
liquidityInB := liquidityInMintB(req, state, snapshot, priceTokenAInTokenB)
```

### Step 4: SOL normalization + cap metrics

```go
priceAInSOL, _ := c.priceOfMintAInSOL(ctx, req, priceAInB)
liquidityInSOL, _ := c.liquidityInSOL(ctx, req, liquidityInB)
totalSupply, circulating, supplyMethod, _ := c.supply.GetSupply(ctx, req.MintA)
fdvSupply, fdvMethod := supply.ResolveFDVSupply(ctx, c.rpc, req.MintA, totalSupply)
```

## Formula summary

- `sqrt_price` to spot price conversion
- protocol-fee-net reserve valuation
- market cap and FDV from SOL-normalized price

## Metadata emitted

Whirlpool config/tick fields, protocol fee values, reserves, decimals, source/program, FDV details.
