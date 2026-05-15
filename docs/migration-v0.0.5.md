# solana-token-market-go v0.0.5 Migration Notes

## Why this release

`v0.0.5` upgrades bulk market paths to true RPC-bulk execution so gateway workloads can stay SDK-first and reduce DexScreener fallback frequency.

## Key changes

- `GetMetricsByPools` now groups by route and preloads accounts with `GetMultipleAccounts` before compute.
- `GetMetricsByPumpfunBondingCurves` now derives all curve PDAs, bulk-loads them, then computes per item.
- Bulk output order is unchanged and still index-stable.
- Partial-success behavior is unchanged (per-item `Metrics` or `Error`).
- RPC retry/backoff is now configurable and enabled by default for transient failures.

## New per-item error codes

- `invalid_argument`
- `unsupported_route`
- `account_not_found`
- `decode_error`
- `rate_limited`
- `rpc_unavailable`
- `timeout`
- `rpc_error`
- `internal`

## Gateway integration guidance

- Use bulk methods as the default path for pool/token batches.
- Route retry/fallback decisions by `Error.Code` (especially `rate_limited`, `rpc_unavailable`, `timeout`).
- Keep partial-success handling in the gateway response model.
- Keep existing order-based correlation (`Results[i]` maps to request item `i`).

## Suggested rollout

1. Bump SDK dependency to `v0.0.5`.
2. Deploy to staging and verify gateway classification metrics by error code.
3. Compare RPC call volumes for 100-item batches before/after upgrade.
4. Roll out to production after latency and rate-limit metrics stabilize.
