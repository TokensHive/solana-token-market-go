# Liquidity Version References

Each file documents one supported pool version and how the SDK computes metrics.

API note:

- `GetMetricsByPool` now accepts only `Dex + PoolVersion + PoolAddress` and resolves output mints from pool state in canonical on-chain order.
- Pump.fun `bonding_curve` is handled by `GetMetricsByPumpfunBondingCurve` because bonding-curve accounts do not embed mint fields.

- [`pumpfun_bonding_curve.md`](./pumpfun_bonding_curve.md)
- [`pumpfun_pumpswap_amm.md`](./pumpfun_pumpswap_amm.md)
- [`raydium_liquidity_v4.md`](./raydium_liquidity_v4.md)
- [`raydium_cpmm.md`](./raydium_cpmm.md)
- [`raydium_clmm.md`](./raydium_clmm.md)
- [`raydium_launchpad.md`](./raydium_launchpad.md)
- [`meteora_dlmm.md`](./meteora_dlmm.md)
- [`meteora_dbc.md`](./meteora_dbc.md)
- [`meteora_damm_v1.md`](./meteora_damm_v1.md)
- [`meteora_damm_v2.md`](./meteora_damm_v2.md)
- [`orca_whirlpool.md`](./orca_whirlpool.md)
