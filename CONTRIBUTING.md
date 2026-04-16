# Contributing Guide

Thanks for contributing to `solana-token-market-go`.

## Contribution Scope

This SDK is metrics-only. Contributions should focus on:

- `GetMetricsByPool` accuracy and determinism
- New `Dex + PoolVersion` calculators
- Performance improvements for RPC-heavy paths
- Test quality and documentation quality

## Local Setup

```bash
go mod download
go test ./...
```

## Pull Request Checklist

- Add or update tests for every changed branch/path.
- Keep package coverage at `100%` (CI gate).
- Update `README.md` and relevant docs in `docs/`.
- Keep changes scoped and avoid unrelated refactors.
- Include protocol references (program IDs + IDL/layout links) for new integrations.

## Coding Expectations

- Preserve current architecture: route by `Dex + PoolVersion`.
- Keep decoding explicit by byte offsets/discriminators where applicable.
- Return clear errors for invalid owner/discriminator/account shape.
- Populate useful `Metadata` fields for observability.

## Adding a New DEX or Pool Version

Follow [`docs/ADDING_NEW_DEX.md`](./docs/ADDING_NEW_DEX.md) and add a corresponding file in [`docs/liquidity-versions`](./docs/liquidity-versions).
