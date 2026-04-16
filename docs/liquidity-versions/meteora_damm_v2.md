# Meteora `damm_v2`

## Protocol and IDL

| Item | Value |
| --- | --- |
| DEX | Meteora |
| Pool Version | `damm_v2` |
| Program ID | `cpamdpZCGKUy5JxQXB4dcpGPiikHawvSWAd6mEn1sGG` |
| IDL JSON | [`cp_amm.json`](https://github.com/MeteoraAg/damm-v2-sdk/blob/main/src/idl/cp_amm.json) |
| SDK calculator file | `sdk/protocols/meteora/damm_v2/metrics.go` |

## On-chain structure used by the SDK

- DAMM V2 pool state (`decodePoolState`)
- token vault balances
- protocol fee fields for each side
- mint decimals
- sqrt-price field

## Parsing flow (step-by-step)

### Step 1: Decode pool state

```go
poolInfo, _ := c.rpc.GetAccount(ctx, req.PoolAddress)
state, _ := decodePoolState(poolInfo.Data)
```

### Step 2: Decode token balances and decimals

```go
accounts, _ := c.rpc.GetMultipleAccounts(ctx, []solana.PublicKey{
    state.tokenAVault, state.tokenBVault, state.tokenAMint, state.tokenBMint,
})
tokenARaw, _ := decodeTokenAmount(accounts[0].Data)
tokenBRaw, _ := decodeTokenAmount(accounts[1].Data)
tokenARaw = subtractProtocolFee(tokenARaw, state.protocolFeeA)
tokenBRaw = subtractProtocolFee(tokenBRaw, state.protocolFeeB)
```

### Step 3: Compute price from sqrt and pair metrics

```go
priceTokenAInB := priceTokenAInTokenBFromSqrt(state.sqrtPriceX64Raw, tokenADecimals, tokenBDecimals)
priceAInB := priceOfMintAInMintB(req, state, priceTokenAInB)
liquidityInB := liquidityInMintB(req, state, snapshot, priceTokenAInB)
```

### Step 4: SOL normalization + cap/FDV

```go
priceAInSOL, _ := c.priceOfMintAInSOL(ctx, req, priceAInB)
liquidityInSOL, _ := c.liquidityInSOL(ctx, req, liquidityInB)
totalSupply, circulating, supplyMethod, _ := c.supply.GetSupply(ctx, req.MintA)
fdvSupply, fdvMethod := supply.ResolveFDVSupply(ctx, c.rpc, req.MintA, totalSupply)
```

## Formula summary

- sqrt-price conversion + decimal adjustment
- fee-net reserve valuation
- market cap and FDV from SOL-normalized price and supply

## Metadata emitted

sqrt-price, fee counters, net reserves, decimals, source/program, FDV metadata.
