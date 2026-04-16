# Meteora `dlmm`

## Protocol and IDL

| Item | Value |
| --- | --- |
| DEX | Meteora |
| Pool Version | `dlmm` |
| Program ID | `LBUZKhRxPF3XUpBCjp4YzTKgLccjZhTSDM9YuVaPwxo` |
| IDL JSON | [`dlmm.json`](https://github.com/MeteoraAg/dlmm-sdk/blob/main/idls/dlmm.json) |
| SDK calculator file | `sdk/protocols/meteora/dlmm/metrics.go` |

## On-chain structure used by the SDK

- DLMM pool state (active bin id, bin step, mint/vault fields)
- token vault account balances
- mint decimals

## Parsing flow (step-by-step)

### Step 1: Decode pool state

```go
poolInfo, _ := c.rpc.GetAccount(ctx, req.PoolAddress)
state, _ := decodePoolState(poolInfo.Data)
```

### Step 2: Fetch vault and mint accounts

```go
accounts, _ := c.rpc.GetMultipleAccounts(ctx, []solana.PublicKey{
    state.tokenXVault, state.tokenYVault, state.tokenXMint, state.tokenYMint,
})
tokenXRaw, _ := decodeTokenAmount(accounts[0].Data)
tokenYRaw, _ := decodeTokenAmount(accounts[1].Data)
tokenXDecimals, _ := decodeMintDecimals(accounts[2].Data)
tokenYDecimals, _ := decodeMintDecimals(accounts[3].Data)
```

### Step 3: Compute pool price and pair metrics

```go
priceTokenXInY := priceTokenXInTokenY(state.activeID, state.binStepBPS, tokenXDecimals, tokenYDecimals)
priceAInB := priceOfMintAInMintB(req, state, priceTokenXInY)
liquidityInB := liquidityInMintB(req, snapshot)
```

### Step 4: Normalize SOL + market metrics

```go
priceAInSOL, _ := c.priceOfMintAInSOL(ctx, req, priceAInB)
liquidityInSOL, _ := c.liquidityInSOL(ctx, req, liquidityInB)
totalSupply, circulating, supplyMethod, _ := c.supply.GetSupply(ctx, req.MintA)
fdvSupply, fdvMethod := supply.ResolveFDVSupply(ctx, c.rpc, req.MintA, totalSupply)
```

## Formula summary

- bin-model price transformed into token units
- quote liquidity from reserve valuation
- `MarketCapInSOL` and `FDVInSOL` from SOL-normalized price and supply

## Metadata emitted

Bin parameters, reserves, decimals, mint/vault addresses, source/program, FDV metadata.
