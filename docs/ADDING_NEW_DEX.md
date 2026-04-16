# Adding a New DEX / Pool Version

This SDK extends markets through a calculator registry keyed by:

- `Dex`
- `PoolVersion`

## Implementation Steps

1. Create protocol package:
   - `sdk/protocols/<dex>/<pool_version>/metrics.go`
   - `sdk/protocols/<dex>/<pool_version>/metrics_test.go`
2. Implement calculator constructor and `Compute(...)`.
3. Decode accounts with owner/discriminator validation.
4. Compute:
   - `PriceOfAInB`
   - `PriceOfAInSOL`
   - `LiquidityInB`
   - `LiquidityInSOL`
   - `MarketCapInSOL`
   - `FDVInSOL`
   - `TotalSupply`, `CirculatingSupply`, `SupplyMethod`
5. Register route in `sdk/market/metrics_service.go`.
6. Add route tests in `sdk/market/metrics_service_test.go`.
7. Add an example preset in `examples/main.go` and tests in `examples/main_test.go`.
8. Document program ID + IDL/layout + formulas:
   - `README.md`
   - `docs/liquidity-versions/<dex>_<pool_version>.md`

## Quality Gate

- Run `go test ./...`
- Ensure full coverage remains at `100%`
- Ensure lint diagnostics remain clean

## Design Rules

- Keep metrics deterministic and on-chain-first.
- Do not add hidden API fallback logic in calculators.
- Keep metadata rich enough for debugging and audits.
