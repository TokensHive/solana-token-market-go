# Meteora `damm_v1`

## Protocol and IDL

| Item | Value |
| --- | --- |
| DEX | Meteora |
| Pool Version | `damm_v1` |
| Program ID | `Eo7WjKq67rjJQSZxS6z3YkapzY3eMj6Xy8X5EQVn5UaB` |
| IDL JSON / layout | [`damm-v1 idl.ts`](https://github.com/MeteoraAg/damm-v1-sdk/blob/main/ts-client/src/amm/idl.ts) |
| SDK calculator file | `sdk/protocols/meteora/damm_v1/metrics.go` |

## On-chain structure used by the SDK

- DAMM V1 pool state
- vault A/B state (`vaultProgramID`)
- pool vault LP token accounts and LP mint supplies
- clock sysvar (`SysvarC1ock...`) for profit-degradation math

## Parsing flow (step-by-step)

### Step 1: Decode pool state

```go
poolInfo, _ := c.rpc.GetAccount(ctx, req.PoolAddress)
state, _ := decodePoolState(poolInfo.Data)
```

### Step 2: Fetch vault/LP/mint/clock batch and decode

```go
accounts, _ := c.rpc.GetMultipleAccounts(ctx, []solana.PublicKey{
    state.aVaultLPToken, state.bVaultLPToken, state.aVault, state.bVault,
    state.tokenAMint, state.tokenBMint, clockSysvarID,
})

poolVaultALPRaw, aVaultLPMint, _ := decodeTokenAccountAmountAndMint(accounts[0].Data)
aVaultState, _ := decodeVaultState(accounts[2].Data)
clockUnixTimestamp, _ := decodeClockUnixTimestamp(accounts[6].Data)
```

### Step 3: Compute effective reserves (locked-profit aware)

```go
aWithdrawableRaw, _ := vaultWithdrawableAmount(clockUnixTimestamp, aVaultState)
tokenAReserveRaw := amountByShare(poolVaultALPRaw, aWithdrawableRaw, aVaultLPSupplyRaw)
```

### Step 4: Pair metrics + SOL normalization

```go
priceAInB := priceOfMintAInMintB(req, snapshot)
liquidityInB := liquidityInMintB(req, snapshot, priceAInB)
priceAInSOL, _ := c.priceOfMintAInSOL(ctx, req, priceAInB)
liquidityInSOL, _ := c.liquidityInSOL(ctx, req, liquidityInB)
```

## Formula summary

- reserve share math from LP balance and withdrawable vault amount
- locked-profit degradation: `lockedProfit(...)`
- cap metrics from SOL-normalized price and supply values

## Metadata emitted

Curve type, vault locked-profit fields, LP supplies, reserve snapshots, source/program, FDV metadata.
